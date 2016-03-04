package swp

import (
	"github.com/nats-io/nats"
	"sync"
	"time"
)

// sliding window protocol
//
// Reference: pp118-120, Computer Networks: A Systems Approach
//  by Peterson and Davie, Morgan Kaufmann Publishers, 1996.

//go:generate msgp

//msgp:ignore TxqSlot RxqSlot Semaphore SenderState RecvState SWP Session

// Seqno is the sequence number used in the sliding window.
type Seqno int64

// Semaphore is the classic counting semaphore.
type Semaphore chan bool

// NewSemaphore makes a new semaphore with capacity sz.
func NewSemaphore(sz int64) Semaphore {
	return make(chan bool, sz)
}

// Packet is what is transmitted between Sender and Recver.
type Packet struct {
	SeqNum  Seqno
	AckNum  Seqno
	AckOnly bool
	Data    []byte
}

// TxqSlot is the sender's sliding window element.
type TxqSlot struct {
	Timeout        *time.Timer `msg:"-"`
	TimerCancelled bool
	Session        *Session
	Pack           *Packet
}

// RxqSlot is the receiver's sliding window element.
type RxqSlot struct {
	Valid bool
	Pack  *Packet
}

// SenderState tracks the sender's sliding window state.
type SenderState struct {
	LastAckRec       Seqno
	LastFrameSent    Seqno
	Txq              []*TxqSlot
	ssem             Semaphore
	SenderWindowSize Seqno
	mut              sync.Mutex
	Timeout          time.Duration
}

// RecvState tracks the receiver's sliding window state.
type RecvState struct {
	NextFrameExpected Seqno
	Rxq               []*RxqSlot
	rsem              Semaphore
	RecvWindowSize    Seqno
	mut               sync.Mutex
	Timeout           time.Duration
}

// SWP holds the Sliding Window Protocol state
type SWP struct {
	Sender SenderState
	Recver RecvState
}

// NewSWP makes a new sliding window protocol manager, holding
// both sender and receiver components.
func NewSWP(windowSize int64, timeout time.Duration) *SWP {
	recvSz := windowSize
	sendSz := windowSize
	swp := &SWP{
		Sender: SenderState{
			SenderWindowSize: Seqno(sendSz),
			Txq:              make([]*TxqSlot, sendSz),
			ssem:             NewSemaphore(sendSz),
			Timeout:          timeout,
		},
		Recver: RecvState{
			RecvWindowSize: Seqno(recvSz),
			Rxq:            make([]*RxqSlot, recvSz),
			rsem:           NewSemaphore(recvSz),
			Timeout:        timeout,
		},
	}
	for i := range swp.Sender.Txq {
		swp.Sender.Txq[i] = &TxqSlot{}
	}
	for i := range swp.Recver.Rxq {
		swp.Recver.Rxq[i] = &RxqSlot{}
	}

	return swp
}

// Session tracks a given point-to-point sesssion and its
// sliding window state for one of the end-points.
type Session struct {
	Swp               *SWP
	Destination       string
	MyInbox           string
	InboxSubscription *nats.Subscription
	Nc                *nats.Conn
	MsgRecv           chan *nats.Msg
}

func NewSession(nc *nats.Conn,
	localInbox string,
	destInbox string,
	windowSz int64,
	timeout time.Duration) (*Session, error) {

	sess := &Session{
		Swp:         NewSWP(windowSz, timeout),
		MyInbox:     localInbox,
		Destination: destInbox,
		MsgRecv:     make(chan *nats.Msg),
	}
	var err error
	sess.InboxSubscription, err = nc.Subscribe(sess.MyInbox, func(msg *nats.Msg) {
		sess.MsgRecv <- msg
	})
	if err != nil {
		return nil, err
	}
	sess.Nc = nc
	return sess, nil
}

// Push sends a message packet, blocking until that is done.
// It will copy data, so data can be recycled once Push returns.
func Push(sess *Session, data []byte) error {

	s := sess.Swp.Sender

	// wait for send window to open before sending anything
	s.ssem <- true
	s.mut.Lock()
	defer s.mut.Unlock()

	s.LastFrameSent++
	lfs := s.LastFrameSent
	slot := s.Txq[lfs%s.SenderWindowSize]
	slot.Pack = &Packet{
		SeqNum: lfs,
		Data:   data,
	}

	// todo: start go routine that listens for this timeout
	// most of this logic probably moves to that goroutine too.
	slot.Timeout = time.NewTimer(s.Timeout)
	slot.TimerCancelled = false
	slot.Session = sess

	bts, err := slot.Pack.MarshalMsg(nil)
	if err != nil {
		return err
	}
	return sess.Nc.Publish(sess.Destination, bts)
}

// Pop receives. It receives both data and acks from earlier sends.
// Pop recevies acks (for sends from this node), and data.
func Pop(sess *Session) ([]byte, error) {

	r := sess.Swp.Recver
	s := sess.Swp.Sender

recvloop:
	for {
		select {
		case msg := <-sess.MsgRecv:
			var pack Packet
			_, err := pack.UnmarshalMsg(msg.Data)
			panicOn(err)
			switch {
			case pack.AckOnly:
				// only an ack received - do sender side stuff
				if !InWindow(pack.AckNum, s.LastAckRec+1, s.LastFrameSent) {
					p("packet outside sender's window, dropping it")
					continue recvloop
				}
				for {
					s.LastAckRec++
					slot := s.Txq[s.LastAckRec%s.SenderWindowSize]
					slot.TimerCancelled = true
					slot.Timeout.Stop()
					slot.Pack = nil
					<-s.ssem
					if s.LastAckRec == pack.AckNum {
						break
					}
				}

			default:
				// actual data received, receiver side stuff:
				slot := r.Rxq[pack.SeqNum%r.RecvWindowSize]
				if !InWindow(pack.SeqNum, r.NextFrameExpected, r.NextFrameExpected+r.RecvWindowSize-1) {
					// drop the packet
					p("packet outside receiver's window, dropping it")
					continue recvloop
				}
				slot.Valid = true
				if slot.Pack.SeqNum == r.NextFrameExpected {

					for slot.Valid {
						slot.Valid = false
						slot.Pack = nil
						r.NextFrameExpected++
						slot = r.Rxq[r.NextFrameExpected%r.RecvWindowSize]
					}
				}
			}
		}
	}

}

// InWindow returns true iff seqno is in [min, max].
func InWindow(seqno, min, max Seqno) bool {
	if seqno < min {
		return false
	}
	if seqno > max {
		return false
	}
	return true
}
