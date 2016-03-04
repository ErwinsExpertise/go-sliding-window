package swp

import (
	"sync"
	"time"
)

// RecvState tracks the receiver's sliding window state.
type RecvState struct {
	Net               Network
	Inbox             string
	NextFrameExpected Seqno
	Rxq               []*RxqSlot
	RecvWindowSize    Seqno
	mut               sync.Mutex
	Timeout           time.Duration
	RecvHistory       []*Packet

	MsgRecv chan *Packet

	ReqStop chan bool
	Done    chan bool

	RecvSz       int64
	DiscardCount int64

	snd *SenderState
}

// NewRecvState makes a new RecvState manager.
func NewRecvState(net Network, recvSz int64, timeout time.Duration, inbox string, snd *SenderState) *RecvState {
	return &RecvState{
		Net:            net,
		Inbox:          inbox,
		RecvWindowSize: Seqno(recvSz),
		Rxq:            make([]*RxqSlot, recvSz),
		Timeout:        timeout,
		RecvHistory:    make([]*Packet, 0),
		ReqStop:        make(chan bool),
		Done:           make(chan bool),
		RecvSz:         recvSz,
		snd:            snd,
	}
}

// RecvStart receives. It receives both data and acks from earlier sends.
// It starts a go routine in the background.
func (r *RecvState) Start() error {
	mr, err := r.Net.Listen(r.Inbox)
	if err != nil {
		return err
	}
	r.MsgRecv = mr

	go func() {
	recvloop:
		for {
			p("%v top of recvloop, receiver NFE: %v",
				r.Inbox, r.NextFrameExpected)
			select {
			case <-r.ReqStop:
				p("%v recvloop sees ReqStop, shutting down.", r.Inbox)
				close(r.Done)
				return
			case pack := <-r.MsgRecv:
				p("%v recvloop sees packet '%#v'", r.Inbox, pack)
				if pack.AckOnly {
					r.snd.GotAck <- AckStatus{
						AckNum: pack.AckNum,
						NFE:    r.NextFrameExpected,
						RWS:    r.RecvWindowSize,
					}
				} else {
					// actual data received, receiver side stuff:
					slot := r.Rxq[pack.SeqNum%r.RecvWindowSize]
					if !InWindow(pack.SeqNum, r.NextFrameExpected, r.NextFrameExpected+r.RecvWindowSize-1) {
						// drop the packet
						p("pack.SeqNum %v outside receiver's window, dropping it", pack.SeqNum)
						r.DiscardCount++
						continue recvloop
					}
					slot.Received = true
					slot.Pack = pack

					if pack.SeqNum == r.NextFrameExpected {
						p("%v packet.SeqNum %v matches r.NextFrameExpected",
							r.Inbox, pack.SeqNum)
						for slot.Received {

							p("%v actual in-order receive happening", r.Inbox)
							r.RecvHistory = append(r.RecvHistory, slot.Pack)
							p("%v r.RecvHistory now has length %v", r.Inbox, len(r.RecvHistory))

							slot.Received = false
							slot.Pack = nil
							r.NextFrameExpected++
							slot = r.Rxq[r.NextFrameExpected%r.RecvWindowSize]
						}
					}
					// send ack
					ack := &Packet{
						From:    r.Inbox,
						Dest:    pack.From,
						AckNum:  r.NextFrameExpected - 1,
						AckOnly: true,
					}
					r.snd.SendAck <- ack
				}
			}
		}
	}()
	return nil
}

// Stop the RecvState componennt
func (s *RecvState) Stop() {
	s.mut.Lock()
	select {
	case <-s.ReqStop:
	default:
		close(s.ReqStop)
	}
	s.mut.Unlock()
	<-s.Done
}
