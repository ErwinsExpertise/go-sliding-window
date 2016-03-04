package swp

import (
	"atomic/sync"
	"github.com/nats-io/nats"
	"time"
)

// sliding window protocol

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
	Msg     Packet
}

// RxqSlot is the receiver's sliding window element.
type RxqSlot struct {
	Valid *bool
	Msg   *Packet
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
	Swp         *SWP
	Destination string
	MyInBox     string
	Nc          *nats.Conn
}

// Push sends a message packet, blocking until that is done.
// It does not copy data, once sent data should not be touched.
// Hence the caller should arrange for copying data if need be.
func Push(sess *Session, data []byte) error {

	s := sess.Swp.Sender

	// wait for send window to open before sending anything
	s.Ssem <- true
	s.Mut.Lock()
	defer s.Mut.Unlock()

	s.LastFrameSent++
	lfs = s.LastFrameSent
	msg.SeqNum = lfs
	slot := &s.Swindow[msg.SeqNum%s.SenderWindowSize]
	slot.Msg.Data = data
	slot.Timeout = time.NewTimer(s.Timeout)
	slot.Session = sess

	return sess.Nc.Publish(sess.Destination, data)
}

// Pop receives a message packet, blocking until that is done.
// Pop recevies acks (for sends from this node), and data.
func Pop(sess *Session) ([]byte, error) {

}
