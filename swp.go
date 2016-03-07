package swp

import (
	"fmt"
	"sync/atomic"
	"time"
)

// sliding window protocol
//
// Reference: pp118-120, Computer Networks: A Systems Approach
//  by Peterson and Davie, Morgan Kaufmann Publishers, 1996.
//
// In addition to sliding window, we implement flow-control
// similar to how tcp does for throttling senders.
// See pp296-301 of Peterson and Davie.
//
// Most of the implementation is in sender.go and recv.go.

//go:generate msgp

//msgp:ignore TxqSlot RxqSlot Semaphore SenderState RecvState SWP Session NatsNet SimNet

// Seqno is the sequence number used in the sliding window.
type Seqno int64

// Packet is what is transmitted between Sender A and
// Recver B, where A and B are the two endpoints in a
// given Session. (Endpoints are specified by the strings localInbox and
// destInbox in the NewSession constructor.)
//
// Packets also flow symmetrically from Sender B to Recver A.
//
// Special packets are AckOnly and KeepAlive
// flagged; otherwise normal packets are data
// segments that have neither of these flags
// set. Only normal data packets are tracked
// for timeout and retry purposes.
type Packet struct {
	From string
	Dest string

	SeqNum    Seqno
	AckNum    Seqno
	AckOnly   bool
	KeepAlive bool

	// AvailReaderByteCap and AvailReaderMsgCap are
	// like the byte count AdvertisedWindow in TCP, but
	// since nats has both byte and message count
	// limits, we want convey these instead.
	AvailReaderBytesCap int64
	AvailReaderMsgCap   int64

	// CumulBytesTransmitted should give the total accumulated
	// count of bytes ever transmitted on this session
	// from `From` to `Dest`.
	// On data payloads, CumulBytesTransmitted allows
	// the receiver to figure out how
	// big any gaps are, so as to give accurate flow control
	// byte count info. The CumulBytesTransmitted count
	// should include this packet's len(Data), assuming
	// this is a data packet.
	CumulBytesTransmitted int64

	Data []byte
}

// TxqSlot is the sender's sliding window element.
type TxqSlot struct {
	RetryDeadline time.Time
	Pack          *Packet
}

// RxqSlot is the receiver's sliding window element.
type RxqSlot struct {
	Received bool
	Pack     *Packet
}

// SWP holds the Sliding Window Protocol state
type SWP struct {
	Sender *SenderState
	Recver *RecvState
}

// NewSWP makes a new sliding window protocol manager, holding
// both sender and receiver components.
func NewSWP(net Network, windowMsgCount int64, windowByteCount int64,
	timeout time.Duration, inbox string, destInbox string) *SWP {

	snd := NewSenderState(net, windowMsgCount, timeout, inbox, destInbox)
	rcv := NewRecvState(net, windowMsgCount, windowByteCount, timeout, inbox, snd)
	swp := &SWP{
		Sender: snd,
		Recver: rcv,
	}

	return swp
}

// Session tracks a given point-to-point sesssion and its
// sliding window state for one of the end-points.
type Session struct {
	Swp         *SWP
	Destination string
	MyInbox     string

	Net            Network
	ReadMessagesCh chan InOrderSeq

	packetsConsumed uint64
	packetsSent     uint64
}

// NewSession makes a new Session, and calls
// Swp.Start to begin the sliding-window-protocol.
//
// If windowByteSz is negative or less than windowMsgSz,
// we estimate a byte size based on 10kb messages and the given windowMsgSz.
//
func NewSession(net Network,
	localInbox string,
	destInbox string,
	windowMsgSz int64,
	windowByteSz int64,
	timeout time.Duration) (*Session, error) {

	if windowMsgSz < 1 {
		return nil, fmt.Errorf("windowMsgSz must be 1 or more")
	}

	if windowByteSz < windowMsgSz {
		// guestimate
		windowByteSz = windowMsgSz * 10 * 1024
	}

	sess := &Session{
		Swp:         NewSWP(net, windowMsgSz, windowByteSz, timeout, localInbox, destInbox),
		MyInbox:     localInbox,
		Destination: destInbox,
		Net:         net,
	}
	sess.Swp.Start()
	sess.ReadMessagesCh = sess.Swp.Recver.ReadMessagesCh

	return sess, nil
}

var ErrShutdown = fmt.Errorf("shutting down")

// Push sends a message packet, blocking until that is done.
// You can use sess.CountPacketsSentForTransfer() to get
// the total count of packets Push()-ed so far.
func (sess *Session) Push(pack *Packet) {
	select {
	case sess.Swp.Sender.BlockingSend <- pack:
		q("%v Push succeeded on payload '%s' into BlockingSend", sess.MyInbox, string(pack.Data))
		sess.IncrPacketsSentForTransfer(1)
	case <-sess.Swp.Sender.ReqStop:
		// give up, Sender is shutting down.
	}
}

// SelfConsumeForTesting sets up a reader to read all produced
// messages automatically. You can use CountPacketsReadConsumed() to
// see the total number consumed thus far.
func (sess *Session) SelfConsumeForTesting() {
	go func() {
		for {
			select {
			case <-sess.Swp.Recver.ReqStop:
				return
			case read := <-sess.ReadMessagesCh:
				sess.IncrPacketsReadConsumed(int64(len(read.Seq)))
			}
		}
	}()
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

// Stop shutsdown the session
func (s *Session) Stop() {
	s.Swp.Stop()
}

// Stop the sliding window protocol
func (s *SWP) Stop() {
	s.Recver.Stop()
	s.Sender.Stop()
}

// Start the sliding window protocol
func (s *SWP) Start() {
	//q("SWP Start() called")
	s.Recver.Start()
	s.Sender.Start()
}

// CountPacketsReadConsumed reports on how many packets
// the application has read from the session.
func (sess *Session) CountPacketsReadConsumed() int64 {
	return int64(atomic.LoadUint64(&sess.packetsConsumed))
}

// IncrPacketsReadConsumed increment packetsConsumed and return the new total.
func (sess *Session) IncrPacketsReadConsumed(n int64) int64 {
	return int64(atomic.AddUint64(&sess.packetsConsumed, uint64(n)))
}

// CountPacketsSentForTransfer reports on how many packets.
// the application has written to the session.
func (sess *Session) CountPacketsSentForTransfer() int64 {
	return int64(atomic.LoadUint64(&sess.packetsSent))
}

// IncrPacketsSentForTransfer increment packetsConsumed and return the new total.
func (sess *Session) IncrPacketsSentForTransfer(n int64) int64 {
	return int64(atomic.AddUint64(&sess.packetsSent, uint64(n)))
}
