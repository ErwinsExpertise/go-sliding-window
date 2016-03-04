package swp

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"github.com/nats-io/nats"
	"sync"
	"time"
)

// sliding window protocol
//
// Reference: pp118-120, Computer Networks: A Systems Approach
//  by Peterson and Davie, Morgan Kaufmann Publishers, 1996.

// NB this is only sliding window, and while planned,
// doesn't have the AdvertisedWindow yet for flow-control
// and throttling the sender. See pp296-301 of Peterson and Davie.

//go:generate msgp

//msgp:ignore TxqSlot RxqSlot Semaphore SenderState RecvState SWP Session

// Seqno is the sequence number used in the sliding window.
type Seqno int64

// Semaphore is the classic counting semaphore. Here
// it is simulated with a buffered channel.
type Semaphore chan bool

// NewSemaphore makes a new semaphore with capacity sz.
func NewSemaphore(sz int64) Semaphore {
	return make(chan bool, sz)
}

// Packet is what is transmitted between Sender and Recver.
type Packet struct {
	From string
	Dest string

	SeqNum  Seqno
	AckNum  Seqno
	AckOnly bool
	Data    []byte
}

// TxqSlot is the sender's sliding window element.
type TxqSlot struct {
	Timeout        *time.Timer
	TimerCancelled bool
	Session        *Session
	Pack           *Packet
}

// RxqSlot is the receiver's sliding window element.
type RxqSlot struct {
	Received bool
	Pack     *Packet
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
	RecvWindowSize    Seqno
	mut               sync.Mutex
	Timeout           time.Duration
}

// SWP holds the Sliding Window Protocol state
type SWP struct {
	Sender *SenderState
	Recver *RecvState
}

// NewSWP makes a new sliding window protocol manager, holding
// both sender and receiver components.
func NewSWP(windowSize int64, timeout time.Duration) *SWP {
	recvSz := windowSize
	sendSz := windowSize
	swp := &SWP{
		Sender: &SenderState{
			SenderWindowSize: Seqno(sendSz),
			Txq:              make([]*TxqSlot, sendSz),
			ssem:             NewSemaphore(sendSz),
			Timeout:          timeout,
			LastFrameSent:    -1,
			LastAckRec:       -1,
		},
		Recver: &RecvState{
			RecvWindowSize: Seqno(recvSz),
			Rxq:            make([]*RxqSlot, recvSz),
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
	Swp         *SWP
	Destination string
	MyInbox     string
	MsgRecv     chan *Packet

	ReqStop  chan bool
	RecvDone chan bool

	Net         Network
	RecvHistory []*Packet
	SendHistory []*Packet

	mut          sync.Mutex
	DiscardCount int64
}

func NewSession(net Network,
	localInbox string,
	destInbox string,
	windowSz int64,
	timeout time.Duration) (*Session, error) {

	sess := &Session{
		Swp:         NewSWP(windowSz, timeout),
		MyInbox:     localInbox,
		Destination: destInbox,
		SendHistory: make([]*Packet, 0),
		RecvHistory: make([]*Packet, 0),
		Net:         net,
		ReqStop:     make(chan bool),
		RecvDone:    make(chan bool),
	}
	mr, err := net.Listen(localInbox)
	if err != nil {
		return nil, err
	}
	sess.MsgRecv = mr
	sess.RecvStart()

	return sess, nil
}

var ErrShutdown = fmt.Errorf("shutting down")

// Push sends a message packet, blocking until that is done.
// It will copy data, so data can be recycled once Push returns.
func (sess *Session) Push(pack *Packet) error {
	p("%v Push called", sess.MyInbox)
	s := sess.Swp.Sender

	// wait for send window to open before sending anything.
	// This will block if we are out of send slots.
	select {
	case s.ssem <- true:
	case <-sess.ReqStop:
		return ErrShutdown
	}

	// blocking done, we have a send slot
	s.mut.Lock()
	defer s.mut.Unlock()

	s.LastFrameSent++
	p("%v LastFrameSent is now %v", sess.MyInbox, s.LastFrameSent)

	lfs := s.LastFrameSent
	slot := s.Txq[lfs%s.SenderWindowSize]
	pack.SeqNum = lfs
	slot.Pack = pack

	sess.SendHistory = append(sess.SendHistory, pack)

	// todo: start go routine that listens for this timeout
	// most of this logic probably moves to that goroutine too.
	slot.Timeout = time.NewTimer(s.Timeout)
	slot.TimerCancelled = false
	slot.Session = sess
	// todo: where are the timeouts handled?

	return sess.Net.Send(slot.Pack)
}

// Stop shutsdown the session, including the background receiver.
func (s *Session) Stop() {
	s.mut.Lock()
	select {
	case <-s.ReqStop:
	default:
		close(s.ReqStop)
	}
	s.mut.Unlock()
	<-s.RecvDone
}

// RecvStart receives. It receives both data and acks from earlier sends.
// It starts a go routine in the background.
func (sess *Session) RecvStart() {
	ready := make(chan bool)
	go func() {
		r := sess.Swp.Recver
		s := sess.Swp.Sender
		close(ready)
	recvloop:
		for {
			p("Session %v top of recvloop, with sender LastAckRec: %v  LastFrameSent: %v",
				sess.MyInbox, s.LastAckRec, s.LastFrameSent)
			select {
			case <-sess.ReqStop:
				p("Session %v recvloop sees ReqStop, shutting down.", sess.MyInbox)
				close(sess.RecvDone)
				return
			case pack := <-sess.MsgRecv:
				p("Session %v recvloop sees packet '%#v'", sess.MyInbox, pack)
				if pack.AckOnly {
					// only an ack received - do sender side stuff
					if !InWindow(pack.AckNum, s.LastAckRec+1, s.LastFrameSent) {
						p("packet.AckNum = %v outside sender's window [%v, %v], dropping it.",
							pack.AckNum, s.LastAckRec+1, s.LastFrameSent)
						sess.DiscardCount++
						continue recvloop
					}
					p("packet.AckNum = %v inside sender's window, keeping it.", pack.AckNum)
					for {
						s.LastAckRec++
						slot := s.Txq[s.LastAckRec%s.SenderWindowSize]
						slot.TimerCancelled = true
						slot.Timeout.Stop()
						slot.Pack = nil

						// release the send slot
						select {
						case <-s.ssem:
						case <-sess.ReqStop:
							p("Session %v recvloop sees ReqStop, shutting down.", sess.MyInbox)
							close(sess.RecvDone)
							return
						}
						if s.LastAckRec == pack.AckNum {
							break
						}
					}
				} else {
					// actual data received, receiver side stuff:
					slot := r.Rxq[pack.SeqNum%r.RecvWindowSize]
					if !InWindow(pack.SeqNum, r.NextFrameExpected, r.NextFrameExpected+r.RecvWindowSize-1) {
						// drop the packet
						p("pack.SeqNum %v outside receiver's window, dropping it", pack.SeqNum)
						sess.DiscardCount++
						continue recvloop
					}
					slot.Received = true
					slot.Pack = pack

					if pack.SeqNum == r.NextFrameExpected {
						p("%v packet.SeqNum %v matches r.NextFrameExpected",
							sess.MyInbox, pack.SeqNum)
						for slot.Received {

							p("%v actual in-order receive happening", sess.MyInbox)
							sess.RecvHistory = append(sess.RecvHistory, slot.Pack)
							p("%v sess.RecvHistory now has length %v", sess.MyInbox, len(sess.RecvHistory))

							slot.Received = false
							slot.Pack = nil
							r.NextFrameExpected++
							slot = r.Rxq[r.NextFrameExpected%r.RecvWindowSize]
						}
					}
					// send ack
					ack := &Packet{
						From:    sess.MyInbox,
						Dest:    sess.Destination,
						AckNum:  r.NextFrameExpected - 1,
						AckOnly: true,
					}
					err := sess.Push(ack)
					logOn(err)
				}
			}
		}
	}()
	<-ready
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

type NatsNet struct {
	Nc                *nats.Conn
	InboxSubscription *nats.Subscription
}

type Network interface {
	Send(pack *Packet) error
	Listen(inbox string) (chan *Packet, error)
}

func (n *NatsNet) Listen(inbox string) (chan *Packet, error) {
	mr := make(chan *Packet)

	// do actual subscription
	var err error
	n.InboxSubscription, err = n.Nc.Subscribe(inbox, func(msg *nats.Msg) {
		var pack Packet
		_, err := pack.UnmarshalMsg(msg.Data)
		panicOn(err)
		mr <- &pack
	})
	return mr, err
}

func (n *NatsNet) Send(pack *Packet) error {
	p("in NatsNet.Send(pack=%#v)", *pack)
	bts, err := pack.MarshalMsg(nil)
	if err != nil {
		return err
	}
	return n.Nc.Publish(pack.Dest, bts)
}

type SimNet struct {
	Net      map[string]chan *Packet
	LossProb float64
	Latency  time.Duration
}

// NewSimNet makes a network simulator. The
// latency is one-way trip time; lossProb is the probability of
// the packet getting lost on the network.
func NewSimNet(lossProb float64, latency time.Duration) *SimNet {
	return &SimNet{
		Net:      make(map[string]chan *Packet),
		LossProb: lossProb,
		Latency:  latency,
	}
}

func (sim *SimNet) Listen(inbox string) (chan *Packet, error) {
	ch := make(chan *Packet)
	sim.Net[inbox] = ch
	return ch, nil
}

func (sim *SimNet) Send(pack *Packet) error {
	p("in SimNet.Send(pack=%#v)", *pack)

	ch, ok := sim.Net[pack.Dest]
	if !ok {
		return fmt.Errorf("sim sees packet for unknown node '%s'", pack.Dest)
	}
	pr := cryptoProb()
	isLost := pr <= sim.LossProb
	if sim.LossProb > 0 && isLost {
		p("sim: packet lost")
	} else {
		p("sim: not lost. packet will arrive after %v", sim.Latency)
		// start a goroutine per packet sent, to simulate arrival time with a timer.
		go func(ch chan *Packet, pack *Packet) {
			<-time.After(sim.Latency)
			p("sim: packet %v with latency %v ready to deliver to node %v, trying...",
				pack.SeqNum, sim.Latency, pack.Dest)
			ch <- pack
			p("sim: packet (SeqNum: %v) delivered to node %v", pack.SeqNum, pack.Dest)
		}(ch, pack)
	}
	return nil
}

const resolution = 1 << 20

func cryptoProb() float64 {
	b := make([]byte, 8)
	_, err := cryptorand.Read(b)
	panicOn(err)
	r := int(binary.LittleEndian.Uint64(b))
	if r < 0 {
		r = -r
	}
	r = r % (resolution + 1)

	return float64(r) / float64(resolution)
}

// HistoryEqual lets one easily compare and send and a recv history
func HistoryEqual(a, b []*Packet) bool {
	na := len(a)
	nb := len(b)
	if na != nb {
		return false
	}
	for i := 0; i < na; i++ {
		if a[i].SeqNum != b[i].SeqNum {
			p("packet histories disagree at i=%v, a[%v].SeqNum = %v, while b[%v].SeqNum = %v",
				i, a[i].SeqNum, b[i].SeqNum)
			return false
		}
	}
	return true
}
