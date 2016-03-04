package swp

import (
	"fmt"
	"sync"
	"time"
)

type AckStatus struct {
	AckNum Seqno
	LAR    Seqno
	NFE    Seqno
	RWS    Seqno
}

// SenderState tracks the sender's sliding window state.
type SenderState struct {
	Net              Network
	Inbox            string
	LastAckRec       Seqno
	LastFrameSent    Seqno
	Txq              []*TxqSlot
	ssem             Semaphore
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
}

func NewSenderState(net Network, sendSz int64, timeout time.Duration, inbox string) *SenderState {
	return &SenderState{
		Net:              net,
		Inbox:            inbox,
		SenderWindowSize: Seqno(sendSz),
		Txq:              make([]*TxqSlot, sendSz),
		ssem:             NewSemaphore(sendSz),
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
}

// Start initiates the SenderState goroutine, which manages
// sends, timeouts, and resends
func (s *SenderState) Start() {
	go func() {
		s.slotsAvail = s.SendSz

		var acceptSend chan *Packet

	sendloop:
		for {
			p("%v top of snedloop, sender LAR: %v, LFS: %v \n",
				s.Inbox, s.LastAckRec, s.LastFrameSent)

			// do we have capacity to accept a send?
			if s.slotsAvail > 0 {
				// yes
				acceptSend = s.BlockingSend
			} else {
				// no
				acceptSend = nil
			}

			select {
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

				// todo: start go routine that listens for this timeout
				// most of this logic probably moves to that goroutine too.
				slot.Timer = time.NewTimer(s.Timeout)
				slot.TimerCancelled = false
				// todo: where are the timeouts handled?

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
					slot.TimerCancelled = true
					slot.Timer.Stop()
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
