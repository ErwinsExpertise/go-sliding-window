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

	LastMsgConsumed  Seqno
	LargestSeqnoRcvd Seqno

	LastAvailReaderBytesCap int64
	LastAvailReaderMsgCap   int64

	// todo: implement back pressure using this:
	// (currently we assume everything received has been consumed).
	RcvdButNotConsumed map[Seqno]*Packet

	ReadyForDelivery []*Packet
	ReadMessagesCh   chan InOrderSeq
	NumHeldMessages  chan int64
}

// InOrderSeq represents ordered data as delivered
// to the consumer who requests it by asking on
// the ConsumePacketsCh.
//
type InOrderSeq struct {
	Seq []*Packet
}

// NewRecvState makes a new RecvState manager.
func NewRecvState(net Network, recvSz int64, timeout time.Duration,
	inbox string, snd *SenderState) *RecvState {

	r := &RecvState{
		Net:                net,
		Inbox:              inbox,
		RecvWindowSize:     Seqno(recvSz),
		Rxq:                make([]*RxqSlot, recvSz),
		Timeout:            timeout,
		RecvHistory:        make([]*Packet, 0),
		ReqStop:            make(chan bool),
		Done:               make(chan bool),
		RecvSz:             recvSz,
		snd:                snd,
		RcvdButNotConsumed: make(map[Seqno]*Packet),
		ReadyForDelivery:   make([]*Packet, 0),
		ReadMessagesCh:     make(chan InOrderSeq),
		LastMsgConsumed:    -1,
		LargestSeqnoRcvd:   -1,
		NumHeldMessages:    make(chan int64),
	}

	for i := range r.Rxq {
		r.Rxq[i] = &RxqSlot{}
	}
	return r
}

// RecvStart receives. It receives both data and acks from earlier sends.
// It starts a go routine in the background.
func (r *RecvState) Start() error {
	mr, err := r.Net.Listen(r.Inbox)
	if err != nil {
		return err
	}
	r.MsgRecv = mr

	var deliverToConsumer chan InOrderSeq
	var delivery InOrderSeq

	go func() {

	recvloop:
		for {
			//q("%v top of recvloop, receiver NFE: %v",
			// r.Inbox, r.NextFrameExpected)

			deliverToConsumer = nil
			if len(r.ReadyForDelivery) > 0 {
				delivery.Seq = r.ReadyForDelivery
				deliverToConsumer = r.ReadMessagesCh
			}

			select {
			case r.NumHeldMessages <- int64(len(r.RcvdButNotConsumed)):

			case deliverToConsumer <- delivery:
				p("%v made deliverToConsumer delivery of %v packets starting with %v",
					r.Inbox, len(delivery.Seq), delivery.Seq[0].SeqNum)
				for _, pack := range delivery.Seq {
					p("%v after delivery, deleting from r.RcvdButNotConsumed pack.SeqNum=%v",
						r.Inbox, pack.SeqNum)
					delete(r.RcvdButNotConsumed, pack.SeqNum)
					r.LastMsgConsumed = pack.SeqNum
				}
				r.ReadyForDelivery = r.ReadyForDelivery[:0]
			case <-r.ReqStop:
				//q("%v recvloop sees ReqStop, shutting down.", r.Inbox)
				close(r.Done)
				return
			case pack := <-r.MsgRecv:
				// assert that our flow control was respected
				/*
					if r.LastAvailReaderMsgCap == 0 && pack.SeqNum >= 0 {
						rep := ReportOnSubscription(r.Net.(*NatsNet).Cli.Scrip)
						p("assert failing, cap was zero, rep = %#v", rep)
						panic(fmt.Errorf("%v no receive capacity, yet received a packet: %#v / Data:'%s'", r.Inbox, pack, string(pack.Data)))
					}
				*/
				if pack.SeqNum > r.LargestSeqnoRcvd {
					r.LargestSeqnoRcvd = pack.SeqNum
				}
				p("%v recvloop sees packet '%#v'", r.Inbox, pack)
				// stuff has changed, so update
				r.UpdateFlowControl()
				// and tell snd about the new flow-control info
				as := AckStatus{
					OnlyUpdateFlowCtrl:  !pack.AckOnly,
					AckNum:              pack.AckNum,
					AckCameWithPacket:   pack.SeqNum,
					AvailReaderBytesCap: pack.AvailReaderBytesCap,
					AvailReaderMsgCap:   pack.AvailReaderMsgCap,
				}
				q("%v tellng r.snd.GotAck <- as: '%#v'", r.Inbox, as)
				select {
				case r.snd.GotAck <- as:
				case <-r.ReqStop:
					close(r.Done)
					return
				}
				if !pack.AckOnly {
					if pack.KeepAlive {
						r.ack(r.NextFrameExpected-1, pack.From)
						continue recvloop
					}
					// actual data received, receiver side stuff

					// if not old dup, add to hash of to-be-consumed
					if pack.SeqNum >= r.NextFrameExpected {
						r.RcvdButNotConsumed[pack.SeqNum] = pack
						p("%v adding to r.RcvdButNotConsumed pack.SeqNum=%v",
							r.Inbox, pack.SeqNum)
					}

					slot := r.Rxq[pack.SeqNum%r.RecvWindowSize]
					if !InWindow(pack.SeqNum, r.NextFrameExpected, r.NextFrameExpected+r.RecvWindowSize-1) {
						// Variation from textbook TCP: In the
						// presence of packet loss, if we drop certain packets,
						// the sender may re-try forever if we have non-overlapping windows.
						// So we'll ack out of bounds known good values anyway.
						// We could also do every 5th discard, but we want to get
						// the flow control ramp-up-from-zero correct and not acking
						// may inhibit that.
						//q("%v pack.SeqNum %v outside receiver's window [%v, %v], dropping it",
						//	r.Inbox, pack.SeqNum, r.NextFrameExpected,
						//	r.NextFrameExpected+r.RecvWindowSize-1)
						r.DiscardCount++
						r.ack(r.NextFrameExpected-1, pack.From)
						continue recvloop
					}
					slot.Received = true
					slot.Pack = pack
					//q("%v packet %#v queued for ordered delivery, checking to see if we can deliver now",
					//	r.Inbox, slot.Pack)

					if pack.SeqNum == r.NextFrameExpected {
						// horray, we can deliver one or more frames in order

						//q("%v packet.SeqNum %v matches r.NextFrameExpected",
						//	r.Inbox, pack.SeqNum)
						for slot.Received {

							//q("%v actual in-order receive happening for SeqNum %v",
							//	r.Inbox, slot.Pack.SeqNum)

							r.ReadyForDelivery = append(r.ReadyForDelivery, slot.Pack)
							r.RecvHistory = append(r.RecvHistory, slot.Pack)
							//q("%v r.RecvHistory now has length %v", r.Inbox, len(r.RecvHistory))

							slot.Received = false
							slot.Pack = nil
							r.NextFrameExpected++
							slot = r.Rxq[r.NextFrameExpected%r.RecvWindowSize]
						}
						r.ack(r.NextFrameExpected-1, pack.From)
					} else {
						//q("%v packet SeqNum %v was not NextFrameExpected %v; stored packet but not delivered.",
						//	r.Inbox, pack.SeqNum, r.NextFrameExpected)
					}
				}
			}
		}
	}()
	return nil
}

func (r *RecvState) UpdateFlowControl() {

	// just like TCP flow control, where
	// advertisedWindow = maxRecvBuffer - (lastByteRcvd - nextByteRead)
	r.LastAvailReaderMsgCap = int64(r.RecvWindowSize - (r.LargestSeqnoRcvd - r.LastMsgConsumed))
	r.snd.FlowCt.UpdateFlow(r.Inbox+":recver", r.Net, r.LastAvailReaderMsgCap)

	/*
		begVal := r.LastAvailReaderMsgCap
		flow := r.snd.FlowCt.UpdateFlow(r.Inbox+":recver", r.Net)
		r.LastAvailReaderBytesCap = flow.AvailReaderBytesCap
		r.LastAvailReaderMsgCap = flow.AvailReaderMsgCap
		p("%v UpdateFlowControl in RecvState, bottom: r.LastAvailReaderMsgCap= %v -> %v",
			r.Inbox, begVal, r.LastAvailReaderMsgCap)
	*/
}

// ack is a helper function, used in the recvloop above.
// Currently seqno is always r.NextFrameExpected-1
func (r *RecvState) ack(seqno Seqno, dest string) {
	r.UpdateFlowControl()
	//q("%v about to send ack with AckNum: %v to %v",
	//	r.Inbox, seqno, dest)
	// send ack
	ack := &Packet{
		From:                r.Inbox,
		Dest:                dest,
		SeqNum:              -99, // => ack flag
		AckNum:              seqno,
		AckOnly:             true,
		AvailReaderBytesCap: r.LastAvailReaderBytesCap,
		AvailReaderMsgCap:   r.LastAvailReaderMsgCap,
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
