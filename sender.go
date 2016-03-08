package swp

import (
	"fmt"
	"sync"
	"time"
)

// TxqSlot is the sender's sliding window element.
type TxqSlot struct {
	OrigSendTime  time.Time
	RetryDeadline time.Time
	Pack          *Packet
}

// AckStatus conveys info from the receiver to the sender when an Ack is received.
type AckStatus struct {
	AckOnly             bool
	KeepAlive           bool
	AckNum              int64
	AckRetry            int64
	AckCameWithPacket   int64
	DataSendTm          time.Time
	AckReplyTm          time.Time
	AvailReaderBytesCap int64 // for sender throttling/flow-control
	AvailReaderMsgCap   int64 // for sender throttling/flow-control
}

// SenderState tracks the sender's sliding window state.
// To avoid circular deadlocks, the SenderState never talks
// directly to the RecvState. The RecvState will
// tell the Sender stuff on GotAck.
type SenderState struct {
	Clk              Clock
	Net              Network
	Inbox            string
	Dest             string
	LastAckRec       int64
	LastFrameSent    int64
	Txq              []*TxqSlot
	SenderWindowSize int64
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
	SendAck      chan *Packet
	DiscardCount int64

	LastSendTime      time.Time
	KeepAliveInterval time.Duration
	keepAlive         <-chan time.Time

	SentButNotAcked map[int64]*TxqSlot

	// flow control params
	// last seen from our downstream
	// receiver, we throttle ourselves
	// based on these.
	LastSeenAvailReaderBytesCap int64
	LastSeenAvailReaderMsgCap   int64

	// do synchronized access via GetFlow()
	// and UpdateFlow(s.Net)
	FlowCt         *FlowCtrl
	TotalBytesSent int64
	rtt            *RTT
}

// NewSenderState constructs a new SenderState struct.
func NewSenderState(net Network, sendSz int64, timeout time.Duration,
	inbox string, destInbox string, clk Clock) *SenderState {
	s := &SenderState{
		Clk:              clk,
		Net:              net,
		Inbox:            inbox,
		Dest:             destInbox,
		SenderWindowSize: sendSz,
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
		SentButNotAcked:  make(map[int64]*TxqSlot),

		// send keepalives (important especially for resuming flow from a
		// stopped state) at least this often:
		KeepAliveInterval: 100 * time.Millisecond,
		FlowCt: &FlowCtrl{flow: Flow{
			ReservedByteCap: 64 * 1024,
			ReservedMsgCap:  32,
		}},
		// don't start fast, as we could overwhelm
		// the receiver. Instead start very slowly,
		// allowing 2 messages so our 003 reorder test runs.
		// It will get updated after the first ack
		// or keep alive.
		LastSeenAvailReaderMsgCap:   2,
		LastSeenAvailReaderBytesCap: 1024 * 1024,
		rtt: NewRTT(),
	}
	for i := range s.Txq {
		s.Txq[i] = &TxqSlot{}
	}
	return s
}

// ComputeInflight returns the number of bytes and messages
// that are in-flight: they have been sent but not yet acked.
func (s *SenderState) ComputeInflight() (bytesInflight int64, msgInflight int64) {
	for _, slot := range s.SentButNotAcked {
		msgInflight++
		bytesInflight += int64(len(slot.Pack.Data))
	}
	return
}

// Start initiates the SenderState goroutine, which manages
// sends, timeouts, and resends
func (s *SenderState) Start() {

	go func() {

		var acceptSend chan *Packet

		// check for expired timers at wakeFreq
		wakeFreq := s.Timeout / 2

		// send keepalives (for resuming flow from a
		// stopped state) at least this often:
		s.keepAlive = time.After(s.KeepAliveInterval)

		regularIntervalWakeup := time.After(wakeFreq)

	sendloop:
		for {
			//q("%v top of sendloop, sender LAR: %v, LFS: %v \n",
			//	s.Inbox, s.LastAckRec, s.LastFrameSent)

			// does the downstream reader have capacity to accept a send?
			// Block any new sends if so. We do a conditional receive. Start by
			// assuming no:
			acceptSend = nil

			// then check if we can set acceptSend.
			//
			// We accept a packet for sending if flow control info
			// from the receiver allows it.
			//
			bytesInflight, msgInflight := s.ComputeInflight()
			//q("%v bytesInflight = %v", s.Inbox, bytesInflight)
			//q("%v msgInflight = %v", s.Inbox, msgInflight)

			if s.LastSeenAvailReaderMsgCap-msgInflight > 0 &&
				s.LastSeenAvailReaderBytesCap-bytesInflight > 0 {
				q("%v flow-control: okay to send. s.LastSeenAvailReaderMsgCap: %v > msgInflight: %v",
					s.Inbox, s.LastSeenAvailReaderMsgCap, msgInflight)
				acceptSend = s.BlockingSend
			} else {
				q("%v flow-control kicked in: not sending. s.LastSeenAvailReaderMsgCap = %v,"+
					" msgInflight=%v, s.LastSeenAvailReaderBytesCap=%v bytesInflight=%v",
					s.Inbox, s.LastSeenAvailReaderMsgCap, msgInflight,
					s.LastSeenAvailReaderBytesCap, bytesInflight)
			}

			q("%v top of sender select loop", s.Inbox)
			select {
			case <-s.keepAlive:
				q("%v keepAlive at %v", s.Inbox, s.Clk.Now())
				s.doKeepAlive()

			case <-regularIntervalWakeup:
				now := s.Clk.Now()
				q("%v regularIntervalWakeup at %v", s.Inbox, now)

				// have any of our packets timed-out and need to be
				// sent again?
				retry := []*TxqSlot{}
				for _, slot := range s.SentButNotAcked {
					if slot.RetryDeadline.Before(now) {
						retry = append(retry, slot)
					}
				}
			doRetryLoop:
				for _, slot := range retry {
					if slot.Pack == nil {
						//q("retry loop, slot = %#v", slot)
					}
					_, ok := s.SentButNotAcked[slot.Pack.SeqNum]
					if !ok {
						//q("already acked and gone from SentButNotAcked, so skip SeqNum %v and PopTop",
						//	slot.Pack.SeqNum)
						continue doRetryLoop
					}
					if slot.Pack.SeqNum <= s.LastAckRec {
						//q("already acked; is <= s.LastAckRecv (%v), so skip SeqNum %v and PopTop",
						//	s.LastAckRec, slot.Pack.SeqNum)
						continue doRetryLoop
					}

					// reset deadline and resend
					now := s.Clk.Now()
					slot.RetryDeadline = s.GetDeadline(now)
					slot.Pack.SeqRetry++
					slot.Pack.DataSendTm = now

					flow := s.FlowCt.UpdateFlow(s.Inbox, s.Net, -1, -1, nil)
					slot.Pack.AvailReaderBytesCap = flow.AvailReaderBytesCap
					slot.Pack.AvailReaderMsgCap = flow.AvailReaderMsgCap
					q("%v doing retry Net.Send() for pack = '%#v' of paydirt '%s'",
						s.Inbox, slot.Pack, string(slot.Pack.Data))
					err := s.Net.Send(slot.Pack, "retry")
					panicOn(err)
				}
				regularIntervalWakeup = time.After(wakeFreq)

			case <-s.ReqStop:
				close(s.Done)
				return
			case pack := <-acceptSend:
				q("%v got <-acceptSend pack: '%#v'", s.Inbox, pack)
				s.doOrigDataSend(pack)

			case a := <-s.GotAck:
				// ack received - do sender side stuff
				//
				q("%v sender GotAck a: %#v", s.Inbox, a)
				//
				// flow control: respect a.AvailReaderBytesCap
				// and a.AvailReaderMsgCap info that we have
				// received from this ack
				//
				q("%v sender GotAck, updating s.LastSeenAvailReaderMsgCap %v -> %v",
					s.Inbox, s.LastSeenAvailReaderMsgCap, a.AvailReaderMsgCap)
				s.LastSeenAvailReaderBytesCap = a.AvailReaderBytesCap
				s.LastSeenAvailReaderMsgCap = a.AvailReaderMsgCap

				s.UpdateRTT(&a)

				// need to update our map of SentButNotAcked
				// and remove everything before AckNum, which is cumulative.
				for _, slot := range s.SentButNotAcked {
					if slot.Pack.SeqNum <= a.AckNum {
						delete(s.SentButNotAcked, a.AckNum)
					}
				}

				if !a.AckOnly {
					// it wasn't an Ack, just updated flow info
					// from a received data message.
					//q("%s sender Gotack: just updated flow control, continuing sendloop", s.Inbox)
					continue sendloop
				}
				//q("%s sender Gotack: more than just flowcontrol...", s.Inbox)
				delete(s.SentButNotAcked, a.AckNum)
				if !InWindow(a.AckNum, s.LastAckRec+1, s.LastFrameSent) {
					//q("%v a.AckNum = %v outside sender's window [%v, %v], dropping it.",
					//	s.Inbox, a.AckNum, s.LastAckRec+1, s.LastFrameSent)
					s.DiscardCount++
					continue sendloop
				}
				//q("%v packet.AckNum = %v inside sender's window, keeping it.", s.Inbox, a.AckNum)
				for {
					s.LastAckRec++

					// release the send slot
					// do this before changing slot, since we point into slot.
					delete(s.SentButNotAcked, s.LastAckRec)

					//slot := s.Txq[s.LastAckRec%s.SenderWindowSize]
					//q("%v ... slot = %#v", s.Inbox, slot)

					if s.LastAckRec == a.AckNum {
						//q("%v s.LastAskRec[%v] matches a.AckNum[%v], breaking",
						//	s.Inbox, s.LastAckRec, a.AckNum)
						break
					}
					//q("%v s.LastAskRec[%v] != a.AckNum[%v], looping",
					//	s.Inbox, s.LastAckRec, a.AckNum)
				}
			case ackPack := <-s.SendAck:
				// request to send an ack:
				// don't go though the BlockingSend protocol; since
				// could effectively livelock us.
				q("%v doing Net.Send() SendAck request on ackPack: '%#v'",
					s.Inbox, ackPack)
				err := s.Net.Send(ackPack, "SendAck/ackPack")
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

// for first time sends of data, not retries or acks.
func (s *SenderState) doOrigDataSend(pack *Packet) {
	//q("%v sender in acceptSend", s.Inbox)

	s.LastFrameSent++
	//q("%v LastFrameSent is now %v", s.Inbox, s.LastFrameSent)

	s.TotalBytesSent += int64(len(pack.Data))
	pack.CumulBytesTransmitted = s.TotalBytesSent

	lfs := s.LastFrameSent
	pos := lfs % s.SenderWindowSize
	slot := s.Txq[pos]

	pack.SeqNum = lfs
	if pack.From != s.Inbox {
		pack.From = s.Inbox
	}
	pack.From = s.Inbox
	slot.Pack = pack
	// data sends get stored in SentButNotAcked
	s.SentButNotAcked[lfs] = slot

	now := s.Clk.Now()
	s.SendHistory = append(s.SendHistory, pack)
	slot.OrigSendTime = now
	slot.RetryDeadline = s.GetDeadline(now)
	s.LastSendTime = now

	flow := s.FlowCt.UpdateFlow(s.Inbox+":sender", s.Net, -1, -1, nil)
	//q("%v doSend(), flow = '%#v'", s.Inbox, flow)
	pack.AvailReaderBytesCap = flow.AvailReaderBytesCap
	pack.AvailReaderMsgCap = flow.AvailReaderMsgCap
	pack.DataSendTm = now
	err := s.Net.Send(slot.Pack, fmt.Sprintf("doSend() for %v", s.Inbox))
	panicOn(err)
}

func (s *SenderState) doKeepAlive() {
	if time.Since(s.LastSendTime) < s.KeepAliveInterval {
		return
	}
	flow := s.FlowCt.UpdateFlow(s.Inbox+":sender", s.Net, -1, -1, nil)
	//q("%v doKeepAlive(), flow = '%#v'", s.Inbox, flow)
	// send a packet with no data, to elicit an ack
	// with a new advertised window. This is
	// *not* an ack, because we need it to be
	// acked itself so we get any updated
	// flow control info from the other end.
	now := s.Clk.Now()
	s.LastSendTime = now
	kap := &Packet{
		From:                s.Inbox,
		Dest:                s.Dest,
		SeqNum:              -777, // => keepalive
		SeqRetry:            -777,
		DataSendTm:          now,
		AckRetry:            -777,
		KeepAlive:           true,
		AvailReaderBytesCap: flow.AvailReaderBytesCap,
		AvailReaderMsgCap:   flow.AvailReaderMsgCap,
	}
	//q("%v doing keepalive Net.Send()", s.Inbox)
	err := s.Net.Send(kap, fmt.Sprintf("keepalive from %v", s.Inbox))
	panicOn(err)

	s.keepAlive = time.After(s.KeepAliveInterval)
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func (s *SenderState) UpdateRTT(a *AckStatus) {
	if a.KeepAlive {
		// avoid clock skew between machines by
		// not sampling one-way elapsed times.
		return
	}
	// acks have roundtrip times we can measure
	// use our own clock, thus avoiding clock
	// skew.

	obs := s.Clk.Now().Sub(a.DataSendTm)

	// exclude obvious outliers where a round trip
	// took 60 seconds or more
	if obs > time.Minute {
		p("%v UpdateRTT exluding outlier outside 60 seconds: a.DataSendTm = %v", s.Inbox, a.DataSendTm)
		return
	}

	//q("%v a.DataSendTm = %v", s.Inbox, a.DataSendTm)
	s.rtt.AddSample(obs)
	//q("%v UpdateRTT: observed rtt was %v. new smoothed estimate after %v samples is %v", s.Inbox, obs, s.rtt.N, s.rtt.GetEstimate())
}

func (s *SenderState) GetDeadline(now time.Time) time.Time {
	if s.rtt.N < 1 {
		return now.Add(s.Timeout)
	}
	// exponential moving average of observed RTT
	ema := s.rtt.GetEstimate()

	// allow 50%/some slop before consuming bandwidth for retry
	fin := time.Duration(int64(float64(ema) * 1.5))
	return now.Add(fin)
}
