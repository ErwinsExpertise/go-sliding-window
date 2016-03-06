package swp

import (
	"sync"
	"time"
)

// AckStatus conveys info from the receiver to the sender when an Ack is received.
type AckStatus struct {
	AckNum              Seqno
	AckCameWithPacket   Seqno
	AvailReaderBytesCap int64 // for sender throttling/flow-control
	AvailReaderMsgCap   int64 // for sender throttling/flow-control
}

// SenderState tracks the sender's sliding window state.
// To avoid circulate deadlocks, the Sender never talks
// directly to the RecvState. The RecvState will
// tell the Sender stuff on GotAck.
type SenderState struct {
	Net              Network
	Inbox            string
	Dest             string
	LastAckRec       Seqno
	LastFrameSent    Seqno
	Txq              []*TxqSlot
	SenderWindowSize Seqno
	mut              sync.Mutex
	Timeout          time.Duration

	// the main goroutine safe way to request
	// sending a packet:
	BlockingSend chan *Packet
	GotAck       chan AckStatus

	ReqStop      chan bool
	Done         chan bool
	SendHistory  []*Packet
	SendSz       int64
	slotsAvail   int64
	SendAck      chan *Packet
	DiscardCount int64

	LastSendTime      time.Time
	KeepAliveInterval time.Duration
	keepAlive         <-chan time.Time

	SentButNotAcked map[Seqno]*TxqSlot

	// flow control params
	MaxSendBuffer   int64
	LastByteSent    int64
	LastByteAcked   int64
	LastByteWritten int64

	LastSeenAdvertisedWindow int64

	// EffectiveWindow = AdvertisedWindow - (LastByteSent - LastByteAcked)
	EffectiveWindow int64

	// do synchronized access via GetFlow()
	// and UpdateFlow(s.Net)
	FlowCt FlowCtrl
}

func NewSenderState(net Network, sendSz int64, timeout time.Duration,
	inbox string, destInbox string) *SenderState {
	s := &SenderState{
		Net:              net,
		Inbox:            inbox,
		Dest:             destInbox,
		SenderWindowSize: Seqno(sendSz),
		Txq:              make([]*TxqSlot, sendSz),
		Timeout:          timeout,
		LastFrameSent:    -1,
		LastAckRec:       -1,
		ReqStop:          make(chan bool),
		Done:             make(chan bool),
		SendHistory:      make([]*Packet, 0),
		BlockingSend:     make(chan *Packet),
		SendSz:           sendSz,
		GotAck:           make(chan AckStatus),
		SendAck:          make(chan *Packet),
		SentButNotAcked:  make(map[Seqno]*TxqSlot),

		// send keepalives (important especially for resuming flow from a
		// stopped state) at least this often:
		KeepAliveInterval: 100 * time.Millisecond,
		FlowCt: FlowCtrl{flow: Flow{
			ReservedByteCap: 64 * 1024,
			ReservedMsgCap:  32,
		}},
	}

	return s
}

// Start initiates the SenderState goroutine, which manages
// sends, timeouts, and resends
func (s *SenderState) Start() {

	go func() {
		s.slotsAvail = s.SendSz

		var acceptSend chan *Packet

		// check for expired timers at wakeFreq
		wakeFreq := s.Timeout / 2

		// send keepalives (for resuming flow from a
		// stopped state) at least this often:
		s.keepAlive = time.After(s.KeepAliveInterval)

		regularIntervalWakeup := time.After(wakeFreq)

	sendloop:
		for {
			q("%v top of sendloop, sender LAR: %v, LFS: %v \n",
				s.Inbox, s.LastAckRec, s.LastFrameSent)

			// do we have capacity to accept a send?
			// do a conditional receive
			if s.slotsAvail > 0 {
				// yes
				acceptSend = s.BlockingSend
			} else {
				// no
				acceptSend = nil
			}

			select {
			case <-s.keepAlive:
				s.doKeepAlive()

			case <-regularIntervalWakeup:
				q("%v regularIntervalWakeup at %v", time.Now(), s.Inbox)

				// have any of our packets timed-out and need to be
				// sent again?
				retry := []*TxqSlot{}
				for _, slot := range s.SentButNotAcked {
					if slot.RetryDeadline.Before(time.Now()) {
						retry = append(retry, slot)
					}
				}
			doRetryLoop:
				for _, slot := range retry {
					if slot.Pack == nil {
						q("retry loop, slot = %#v", slot)
					}
					_, ok := s.SentButNotAcked[slot.Pack.SeqNum]
					if !ok {
						q("already acked and gone from SentButNotAcked, so skip SeqNum %v and PopTop",
							slot.Pack.SeqNum)
						continue doRetryLoop
					}
					if slot.Pack.SeqNum <= s.LastAckRec {
						q("already acked; is <= s.LastAckRecv (%v), so skip SeqNum %v and PopTop",
							s.LastAckRec, slot.Pack.SeqNum)
						continue doRetryLoop
					}

					// reset deadline and resend
					slot.RetryDeadline = time.Now().Add(s.Timeout)

					flow := s.FlowCt.UpdateFlow(s.Net)
					slot.Pack.AvailReaderBytesCap = flow.AvailReaderBytesCap
					slot.Pack.AvailReaderMsgCap = flow.AvailReaderMsgCap
					err := s.Net.Send(slot.Pack)
					panicOn(err)
				}
				regularIntervalWakeup = time.After(wakeFreq)

			case <-s.ReqStop:
				close(s.Done)
				return
			case pack := <-acceptSend:
				s.doSend(pack)

			case a := <-s.GotAck:
				q("%v sender GotAck a: %#v", s.Inbox, a)
				// ack received - do sender side stuff
				//
				// flow control: respect a.AvailReaderBytesCap
				// and a.AvailReaderMsgCap info that we have
				// received from this ack
				//
				delete(s.SentButNotAcked, a.AckNum)
				if !InWindow(a.AckNum, s.LastAckRec+1, s.LastFrameSent) {
					q("%v a.AckNum = %v outside sender's window [%v, %v], dropping it.",
						s.Inbox, a.AckNum, s.LastAckRec+1, s.LastFrameSent)
					s.DiscardCount++
					continue sendloop
				}
				q("%v packet.AckNum = %v inside sender's window, keeping it.", s.Inbox, a.AckNum)
				for {
					s.LastAckRec++

					// do this before changing slot, since we point into slot.
					delete(s.SentButNotAcked, s.LastAckRec)

					slot := s.Txq[s.LastAckRec%s.SenderWindowSize]
					q("%v ... slot = %#v", s.Inbox, slot)

					// release the send slot
					s.slotsAvail++
					if s.LastAckRec == a.AckNum {
						q("%v s.LastAskRec[%v] matches a.AckNum[%v], breaking",
							s.Inbox, s.LastAckRec, a.AckNum)
						break
					}
					q("%v s.LastAskRec[%v] != a.AckNum[%v], looping",
						s.Inbox, s.LastAckRec, a.AckNum)
				}
			case ackPack := <-s.SendAck:
				// don't go though the BlockingSend protocol; since
				// could effectively livelock us.
				err := s.Net.Send(ackPack)
				panicOn(err)
			}
		}
	}()
}

// Stop the SenderState componennt
func (s *SenderState) Stop() {
	s.mut.Lock()
	select {
	case <-s.ReqStop:
	default:
		close(s.ReqStop)
	}
	s.mut.Unlock()
	<-s.Done
}

func (s *SenderState) doSend(pack *Packet) {
	s.slotsAvail--
	//q("%v sender in acceptSend, now %v slotsAvail", s.Inbox, s.slotsAvail)

	s.LastFrameSent++
	//q("%v LastFrameSent is now %v", s.Inbox, s.LastFrameSent)

	lfs := s.LastFrameSent
	pos := lfs % s.SenderWindowSize
	slot := s.Txq[pos]

	pack.SeqNum = lfs
	if pack.From != s.Inbox {
		pack.From = s.Inbox
	}
	pack.From = s.Inbox
	slot.Pack = pack
	s.SentButNotAcked[lfs] = slot

	now := time.Now()
	s.SendHistory = append(s.SendHistory, pack)
	slot.RetryDeadline = now.Add(s.Timeout)
	s.LastSendTime = now

	flow := s.FlowCt.UpdateFlow(s.Net)
	pack.AvailReaderBytesCap = flow.AvailReaderBytesCap
	pack.AvailReaderMsgCap = flow.AvailReaderMsgCap
	err := s.Net.Send(slot.Pack)
	panicOn(err)
}

func (s *SenderState) doKeepAlive() {
	if time.Since(s.LastSendTime) < s.KeepAliveInterval {
		return
	}
	flow := s.FlowCt.UpdateFlow(s.Net)
	// send a packet with no data, to elicit an ack
	// with a new advertised window
	s.LastSendTime = time.Now()
	kap := &Packet{
		From:                s.Inbox,
		Dest:                s.Dest,
		KeepAlive:           true,
		AvailReaderBytesCap: flow.AvailReaderBytesCap,
		AvailReaderMsgCap:   flow.AvailReaderMsgCap,
	}
	err := s.Net.Send(kap)
	panicOn(err)

	s.keepAlive = time.After(s.KeepAliveInterval)
}
