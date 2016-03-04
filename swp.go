package swp

import (
	"atomic/sync"
	"github.com/nats-io/nats"
	"time"
)

// sliding window protocol
//
// Reference: pp118-120, Computer Networks: A Systems Approach
//  by Peterson and Davie, Morgan Kaufmann Publishers, 1996.

//go:generate msgp

// Seqno is the sequence number used in the sliding window.
type Seqno int64

// Semaphore is the classic counting semaphore.
type Semaphore chan bool

// NewSemaphore makes a new semaphore with capacity sz.
func NewSemaphore(sz int) Semaphore {
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
	Timeout *time.Timer
	Session *Session
	Pack    Packet
}

// RxqSlot is the receiver's sliding window element.
type RxqSlot struct {
	Valid bool
	Pack  Packet
}

// SenderState tracks the sender's sliding window state.
type SenderState struct {
	LastAckRecvd     Seqno
	LastFrameSent    Seqno
	Swindow          []TxqSlot
	Ssem             Semaphore
	SenderWindowSize int
	Mut              sync.Mutex
	Timeout          time.Duration
}

// RecvState tracks the receiver's sliding window state.
type RecvState struct {
	NextFrameExpected Seqno
	Rwindow           []RxqSlot
	Rsem              Semaphore
	RecvWindowSize    int
	Mut               sync.Mutex
	Timeout           time.Duration
}

// SWP holds the Sliding Window Protocol state
type SWP struct {
	Sender SendState
	Recver RecvState
}

// NewSWP makes a new sliding window protocol manager, holding
// both sender and receiver components.
func NewSWP(windowSize int, timeout time.Duration) *SWP {
	recvSz := windowSize
	sendSz := windowSize
	return &SWP{
		Sender: SendState{
			SenderWindowSize: sendSz,
			Swindow:          make([]TxqSlot, sendSz),
			Ssem:             NewSemaphore(sendSz),
			Timeout:          timeout,
		},
		Recver: RecvState{
			RecvWindowSize: recvSz,
			Rwindow:        make([]RxqSlot, recvSz),
			Rsem:           NewSemaphore(recvSz),
			Timeout:        timeout,
		},
	}
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
	windowSz int,
	timeout time.Duration) (*Session, error) {

	sess := &Session{
		Swp:         NewSWP(windowSz, timeout),
		MyInbox:     localInbox,
		Destination: destInbox,
		MsgRecv:     make(chan *nats.Msg),
	}

	sess.InboxSubscription, err = nc.Subscribe(sess.MyInbox, func(msg *nats.Msg) {
		sess.MsgRecv <- msg
	})
	if err != nil {
		return nil, err
	}
	sess.Nc = nc
	return sess
}

// Push sends a message packet, blocking until that is done.
// It will copy data, so data can be recycled once Push returns.
func Push(sess *Session, data []byte) error {

	s := sess.Swp.Sender

	// wait for send window to open before sending anything
	s.Ssem <- true
	s.Mut.Lock()
	defer s.Mut.Unlock()

	s.LastFrameSent++
	lfs = s.LastFrameSent
	slot := &s.Swindow[lfs%s.SenderWindowSize]
	slot.Pack.SeqNum = lfs
	slot.Pack.Data = data

	// todo: start go routine that listens for this timeout
	// most of this logic probably moves to that goroutine too.
	slot.Timeout = time.NewTimer(s.Timeout)
	slot.Session = sess

	bts, err := slot.Pack.MarshalMsg(nil)
	if err != nil {
		return err
	}
	return sess.Nc.Publish(sess.Destination, bts)
}

// Pop receives a message packet, blocking until that is done.
// Pop recevies acks (for sends from this node), and data.
func Pop(sess *Session) ([]byte, error) {

	r := sess.Swp.Recver

	// finish this out.
	//sess.Nc.

	left, err := v.UnmarshalMsg(bts)

}
