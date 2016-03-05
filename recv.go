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
			q("%v top of recvloop, receiver NFE: %v",
				r.Inbox, r.NextFrameExpected)
			select {
			case <-r.ReqStop:
				q("%v recvloop sees ReqStop, shutting down.", r.Inbox)
				close(r.Done)
				return
			case pack := <-r.MsgRecv:
				q("%v recvloop sees packet '%#v'", r.Inbox, pack)
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
						// variation from the textbook algorithm: In the
						// presence of packet loss, if we drop the packet,
						// the sender may re-try forever,
						// So we'll ack our present known good values anyway.
						p("%v pack.SeqNum %v outside receiver's window [%v, %v], dropping it",
							r.Inbox, pack.SeqNum, r.NextFrameExpected,
							r.NextFrameExpected+r.RecvWindowSize-1)
						r.DiscardCount++
						r.ack(r.NextFrameExpected-1, pack.From)
						continue recvloop
					}
					slot.Received = true
					slot.Pack = pack
					q("%v packet %#v queued for ordered delivery, checking to see if we can deliver now",
						r.Inbox, slot.Pack)

					if pack.SeqNum == r.NextFrameExpected {
						q("%v packet.SeqNum %v matches r.NextFrameExpected",
							r.Inbox, pack.SeqNum)
						for slot.Received {

							p("%v actual in-order receive happening for SeqNum %v",
								r.Inbox, slot.Pack.SeqNum)
							r.RecvHistory = append(r.RecvHistory, slot.Pack)
							p("%v r.RecvHistory now has length %v", r.Inbox, len(r.RecvHistory))

							slot.Received = false
							slot.Pack = nil
							r.NextFrameExpected++
							slot = r.Rxq[r.NextFrameExpected%r.RecvWindowSize]
						}
						r.ack(r.NextFrameExpected-1, pack.From)
					} else {
						q("%v packet SeqNum %v was not NextFrameExpected %v; stored packet but not delivered.",
							r.Inbox, pack.SeqNum, r.NextFrameExpected)
					}
				}
			}
		}
	}()
	return nil
}

// ack is a helper function, used in the recvloop above
func (r *RecvState) ack(seqno Seqno, dest string) {
	q("%v about to send ack with AckNum: %v to %v",
		r.Inbox, seqno, dest)
	// send ack
	ack := &Packet{
		From:    r.Inbox,
		Dest:    dest,
		AckNum:  seqno,
		AckOnly: true,
	}
	r.snd.SendAck <- ack
}

// Stop the RecvState componennt
func (r *RecvState) Stop() {
	r.mut.Lock()
	select {
	case <-r.ReqStop:
	default:
		close(r.ReqStop)
	}
	r.mut.Unlock()
	<-r.Done
}
