package swp

import (
	"fmt"
	"sync"
	"time"
)

// AckStatus conveys info from the receiver to the sender when an Ack is received.
type AckStatus struct {
	AckNum Seqno
	LAR    Seqno // LastAckReceived
	NFE    Seqno // NextFrameExpected
	RWS    Seqno // ReceiverWindowSize
}

// SenderState tracks the sender's sliding window state.
type SenderState struct {
	Net              Network
	Inbox            string
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

	timerPq         *PriorityQueue
	SentButNotAcked map[Seqno]bool
}

func NewSenderState(net Network, sendSz int64, timeout time.Duration, inbox string) *SenderState {
	s := &SenderState{
		Net:              net,
		Inbox:            inbox,
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
		timerPq:          NewPriorityQueue(sendSz),
		SentButNotAcked:  make(map[Seqno]bool),
	}

	s.initPriorityQ()
	return s
}

// Start initiates the SenderState goroutine, which manages
// sends, timeouts, and resends
func (s *SenderState) Start() {
	q("%v SenderStart Start() called.", s.Inbox)

	go func() {
		s.slotsAvail = s.SendSz

		var acceptSend chan *Packet

		// check for expired timers at wakeFreq
		wakeFreq := s.Timeout / 2

		regularIntervalWakeup := time.After(wakeFreq)
		//retry := []*TxqSlot{}

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
			case <-regularIntervalWakeup:
				q("%v regularIntervalWakeup at %v", time.Now(), s.Inbox)
				//s.dumpTimerPq()

				// have any of our packets timed-out and need to be
				// sent again?
				if len(s.timerPq.Slc) == 0 || s.timerPq.Slc[0] == nil || s.timerPq.Slc[0].Pack == nil {
					q("%v no outstanding packets on wakeup.", s.Inbox)
					// just reset wakeup and keep going
				} else {
					origLenPq := s.timerPq.Len()
					p("%v we have outstanding packets, with origLenPq = %v", s.Inbox, origLenPq)
					s.dumpTimerPq()
				doRetryLoop:
					for len(s.timerPq.Slc) > 0 && s.timerPq.Slc[0].RetryDeadline.Before(time.Now()) {
						/*
							                             // should never get here now.
														if s.timerPq.Slc[0].Pack == nil || s.timerPq.Slc[0].RetryDeadline.IsZero() {
															// just ignore and continue
															s.timerPq.PopTop()
															//p("%v after heap.Pop(), ignoring this packet, now pqlen = %v",
															//	s.Inbox, s.timerPq.Len())
															//s.dumpTimerPq()
															continue doRetryLoop
														}
						*/
						if !s.SentButNotAcked[s.timerPq.Slc[0].Pack.SeqNum] {
							p("already acked and gone from SentButNotAcked, so skip SeqNum %v and PopTop",
								s.timerPq.Slc[0].Pack.SeqNum)
							s.timerPq.PopTop()
							continue doRetryLoop
						}

						// need to retry this guy
						slot := s.timerPq.PopTop()
						p("%v we have timed-out packet that needs to be retried: %v", s.Inbox, slot.Pack.SeqNum)

						// reset deadline and resend
						slot.RetryDeadline = time.Now().Add(s.Timeout)

						//p("%v pre heap.Push() pqlen = %v", s.Inbox, s.timerPq.Len())
						//s.dumpTimerPq()

						s.timerPq.Add(slot)
						err := s.Net.Send(slot.Pack)
						panicOn(err)

						//p("%v after resend, now pqlen = %v", s.Inbox, s.timerPq.Len())
						//s.dumpTimerPq()
					}

				}
				regularIntervalWakeup = time.After(wakeFreq)
				q("%v reset the regularIntervalWakeup.", s.Inbox)

			case <-s.ReqStop:
				close(s.Done)
				return
			case pack := <-acceptSend:
				s.slotsAvail--
				q("%v sender in acceptSend, now %v slotsAvail", s.Inbox, s.slotsAvail)

				s.LastFrameSent++
				q("%v LastFrameSent is now %v", s.Inbox, s.LastFrameSent)

				lfs := s.LastFrameSent
				s.SentButNotAcked[lfs] = true
				pos := lfs % s.SenderWindowSize
				slot := s.Txq[pos]

				pack.SeqNum = lfs
				if pack.From != s.Inbox {
					panic(fmt.Errorf("error detected: From mis-set to '%s', should be '%s'",
						pack.From, s.Inbox))
				}
				pack.From = s.Inbox
				slot.Pack = pack

				s.SendHistory = append(s.SendHistory, pack)
				slot.RetryDeadline = time.Now().Add(s.Timeout)
				q("%v sender set retry deadline to %v", s.Inbox, slot.RetryDeadline)
				s.timerPq.Add(slot)

				err := s.Net.Send(slot.Pack)
				panicOn(err)

				//q("%v debug: after send of slot.Pack='%#v', here is our timerPq:", s.Inbox, slot.Pack)
				//s.dumpTimerPq()

			case a := <-s.GotAck:
				p("%v sender GotAck a: %#v", s.Inbox, a)
				// only an ack received - do sender side stuff
				delete(s.SentButNotAcked, a.AckNum)
				if !InWindow(a.AckNum, s.LastAckRec+1, s.LastFrameSent) {
					p("%v a.AckNum = %v outside sender's window [%v, %v], dropping it.",
						s.Inbox, a.AckNum, s.LastAckRec+1, s.LastFrameSent)
					s.DiscardCount++
					continue sendloop
				}
				p("%v packet.AckNum = %v inside sender's window, keeping it.", s.Inbox, a.AckNum)
				for {
					s.LastAckRec++

					// do this before changing slot, since we point into slot.
					s.timerPq.RemoveBySeqNum(a.AckNum)

					slot := s.Txq[s.LastAckRec%s.SenderWindowSize]
					p("%v ... slot = %#v", s.Inbox, slot)
					if slot != nil {
						slot.RetryDeadline = time.Time{}
						slot.Pack = nil
					}

					// release the send slot
					s.slotsAvail++
					if s.LastAckRec == a.AckNum {
						p("%v s.LastAskRec[%v] matches a.AckNum[%v], breaking",
							s.Inbox, s.LastAckRec, a.AckNum)
						break
					}
					p("%v s.LastAskRec[%v] != a.AckNum[%v], looping",
						s.Inbox, s.LastAckRec, a.AckNum)
				}
			case ackPack := <-s.SendAck:
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

func (s *SenderState) initPriorityQ() {
	s.timerPq.Max = int(s.SendSz)
}

func (s *SenderState) dumpTimerPq() {
	n := 0
	for i := range s.timerPq.Slc {
		if s.timerPq.Slc[i] != nil && s.timerPq.Slc[i].Pack != nil {
			n++
			fmt.Printf("%v s.timerPq.Slc[%v] at  %v  seqnum: %v\n",
				s.Inbox, i, s.timerPq.Slc[i].RetryDeadline, s.timerPq.Slc[i].Pack.SeqNum)
		} else {
			fmt.Printf("%v s.timerPq.Slc[%v] is %#v\n",
				s.Inbox, i, s.timerPq.Slc[i])
		}
	}
	if n == 0 {
		fmt.Printf("%v s.timerPq is empty", s.Inbox)
	}

	fmt.Printf("begin Idx ===============\n")
	for k, v := range s.timerPq.Idx {
		fmt.Printf("SeqNum %v -> position %v\n", k, v)
	}
	fmt.Printf("end Idx ===============\n")

}
