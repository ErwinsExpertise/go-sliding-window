package swp

import (
	"container/heap"
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

	timerPq PriorityQueue
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
	}

	s.initPriorityQ()
	return s
}

// Start initiates the SenderState goroutine, which manages
// sends, timeouts, and resends
func (s *SenderState) Start() {
	go func() {
		s.slotsAvail = s.SendSz

		var acceptSend chan *Packet

		// check for expired timers
		wakeFreq := 10 * time.Millisecond
		regularIntervalWakeup := time.After(wakeFreq)

	sendloop:
		for {
			p("%v top of snedloop, sender LAR: %v, LFS: %v \n",
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
				// have any of our packets timed-out and need to be
				// resent?

				regularIntervalWakeup = time.After(wakeFreq)
			case <-s.ReqStop:
				close(s.Done)
				return
			case pack := <-acceptSend:
				s.slotsAvail--

				s.LastFrameSent++
				p("%v LastFrameSent is now %v", s.Inbox, s.LastFrameSent)

				lfs := s.LastFrameSent
				slot := s.Txq[lfs%s.SenderWindowSize]
				pack.SeqNum = lfs
				if pack.From != s.Inbox {
					panic(fmt.Errorf("error detected: From mis-set to '%s', should be '%s'",
						pack.From, s.Inbox))
				}
				pack.From = s.Inbox
				slot.Pack = pack

				s.SendHistory = append(s.SendHistory, pack)
				slot.RetryDeadline = time.Now().Add(s.Timeout)

				err := s.Net.Send(slot.Pack)
				panicOn(err)

			case a := <-s.GotAck:
				// only an ack received - do sender side stuff
				if !InWindow(a.AckNum, a.LAR+1, s.LastFrameSent) {
					p("a.AckNum = %v outside sender's window [%v, %v], dropping it.",
						a.AckNum, a.LAR+1, s.LastFrameSent)
					s.DiscardCount++
					continue sendloop
				}
				p("packet.AckNum = %v inside sender's window, keeping it.", a.AckNum)
				for {
					s.LastAckRec++
					slot := s.Txq[a.LAR%s.SenderWindowSize]
					// lazily repair the timer heap to avoid O(n) operation each time.
					// i.e. just ignore IsZero time entries, they are cancelled.
					slot.RetryDeadline = time.Time{}
					slot.Pack = nil

					// release the send slot
					s.slotsAvail++
					if s.LastAckRec == a.AckNum {
						break
					}
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

	s.timerPq = make(PriorityQueue, s.SendSz)
	for i, p := range s.Txq {
		s.timerPq[i] = &PqEle{
			slot:  p,
			index: i,
		}
	}

	heap.Init(&s.timerPq)
}
