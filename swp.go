/*
Package swp implements the same Sliding Window Protocol that
TCP uses for flow-control and reliable, ordered delivery.

The Nats event bus (https://nats.io/) is a
software model of a hardware multicast
switch. Nats provides multicast, but no guarantees of delivery
and no flow-control. This works fine as long as your
downstream read/subscribe capacity is larger than your
publishing rate.

If your nats publisher evers produces
faster than your subscriber can keep up, you may overrun
your buffers and drop messages. If your sender is local
and replaying a disk file of traffic over nats, you are
guanateed to exhaust even the largest of the internal
nats client buffers. In addition you may wish guaranteed
order of delivery (even with dropped messages), which
swp provides.

Hence swp was built to provide flow-control and reliable, ordered
delivery on top of the nats event bus. It reproduces the
TCP sliding window and flow-control mechanism in a
Session between two nats clients. It provides flow
control between exactly two nats endpoints; in many
cases this is sufficient to allow all subscribers to
keep up.  If you have a wide variation in consumer
performance, establish the rate-controlling
swp Session between your producer and your
slowest consumer.

There is also a Session.RegisterAsap() API that can be
used to obtain possibly-out-of-order and possibly-duplicated
but as-soon-as-possible delivery (similar to that which
nats give you natively), while retaining the
flow-control required to avoid client-buffer overrun.
This can be used in tandem with the main always-ordered-and-lossless
API if so desired.
*/
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

	SeqNum    int64
	AckNum    int64
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

// Push sends a message packet, blocking until that is done.
// You can use s.CountPacketsSentForTransfer() to get
// the total count of packets Push()-ed so far.
func (s *Session) Push(pack *Packet) {
	select {
	case s.Swp.Sender.BlockingSend <- pack:
		q("%v Push succeeded on payload '%s' into BlockingSend", s.MyInbox, string(pack.Data))
		s.IncrPacketsSentForTransfer(1)
	case <-s.Swp.Sender.ReqStop:
		// give up, Sender is shutting down.
	}
}

// SelfConsumeForTesting sets up a reader to read all produced
// messages automatically. You can use CountPacketsReadConsumed() to
// see the total number consumed thus far.
func (s *Session) SelfConsumeForTesting() {
	go func() {
		for {
			select {
			case <-s.Swp.Recver.ReqStop:
				return
			case read := <-s.ReadMessagesCh:
				s.IncrPacketsReadConsumed(int64(len(read.Seq)))
			}
		}
	}()
}

// InWindow returns true iff seqno is in [min, max].
func InWindow(seqno, min, max int64) bool {
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
func (s *Session) CountPacketsReadConsumed() int64 {
	return int64(atomic.LoadUint64(&s.packetsConsumed))
}

// IncrPacketsReadConsumed increment packetsConsumed and return the new total.
func (s *Session) IncrPacketsReadConsumed(n int64) int64 {
	return int64(atomic.AddUint64(&s.packetsConsumed, uint64(n)))
}

// CountPacketsSentForTransfer reports on how many packets.
// the application has written to the session.
func (s *Session) CountPacketsSentForTransfer() int64 {
	return int64(atomic.LoadUint64(&s.packetsSent))
}

// IncrPacketsSentForTransfer increment packetsConsumed and return the new total.
func (s *Session) IncrPacketsSentForTransfer(n int64) int64 {
	return int64(atomic.AddUint64(&s.packetsSent, uint64(n)))
}

// RegisterAsap registers a call back channel,
// rcvUnordered, which will get *Packet that are
// unordered and possibly
// have gaps in their sequence (where packets
// where dropped). However the channel will see
// the packets as soon as possible. The session
// will still be flow controlled however, so
// if the receiver throttles the sender, packets
// may be delayed. Clients should be prepared
// to deal with duplicated, dropped, and mis-ordered packets
// on the rcvUnordered channel.
func (s *Session) RegisterAsap(rcvUnordered chan *Packet) error {
	s.Swp.Recver.setAsapHelper <- NewAsapHelper(rcvUnordered)
	return nil
}
