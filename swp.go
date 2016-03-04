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

// Semaphore is the classic counting semaphore.
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
	MsgRecv           chan *Packet

	ReqStop  chan bool
	RecvDone chan bool

	// networks. use Sim in preference to Nc if Sim is present
	Nc  *nats.Conn
	Sim *SimNet

	RecvHistory []*Packet
	SendHistory []*Packet

	mut sync.Mutex
}

func NewSession(nc *nats.Conn, sim *SimNet,
	localInbox string,
	destInbox string,
	windowSz int64,
	timeout time.Duration) (*Session, error) {

	sess := &Session{
		Swp:         NewSWP(windowSz, timeout),
		MyInbox:     localInbox,
		Destination: destInbox,
		MsgRecv:     make(chan *Packet),
		SendHistory: make([]*Packet, 0),
		RecvHistory: make([]*Packet, 0),
		Sim:         sim,
		Nc:          nc,
		ReqStop:     make(chan bool),
		RecvDone:    make(chan bool),
	}
	sess.RecvStart()

	var err error
	if sim == nil {
		// do actual subscription
		sess.InboxSubscription, err = nc.Subscribe(sess.MyInbox, func(msg *nats.Msg) {
			var pack Packet
			_, err := pack.UnmarshalMsg(msg.Data)
			panicOn(err)
			sess.MsgRecv <- &pack
		})
		if err != nil {
			return nil, err
		}
	}
	return sess, nil
}

// Push sends a message packet, blocking until that is done.
// It will copy data, so data can be recycled once Push returns.
func (sess *Session) Push(data []byte) error {

	s := sess.Swp.Sender

	// wait for send window to open before sending anything
	s.ssem <- true
	s.mut.Lock()
	defer s.mut.Unlock()

	s.LastFrameSent++
	lfs := s.LastFrameSent
	slot := s.Txq[lfs%s.SenderWindowSize]
	pack := &Packet{
		From:   sess.MyInbox,
		Dest:   sess.Destination,
		SeqNum: lfs,
		Data:   data,
	}
	slot.Pack = pack

	sess.SendHistory = append(sess.SendHistory, pack)

	// todo: start go routine that listens for this timeout
	// most of this logic probably moves to that goroutine too.
	slot.Timeout = time.NewTimer(s.Timeout)
	slot.TimerCancelled = false
	slot.Session = sess
	// todo: where are the timeouts handled?

	return sess.send(slot.Pack)
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
			p("Session %v top of recvloop", sess.MyInbox)
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
						p("packet.AckNum = %v outside sender's window, dropping it.", pack.AckNum)
						continue recvloop
					}
					p("packet.AckNum = %v inside sender's window, keeping it.", pack.AckNum)
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
				} else {
					// actual data received, receiver side stuff:
					slot := r.Rxq[pack.SeqNum%r.RecvWindowSize]
					if !InWindow(pack.SeqNum, r.NextFrameExpected, r.NextFrameExpected+r.RecvWindowSize-1) {
						// drop the packet
						p("packet outside receiver's window, dropping it")
						continue recvloop
					}
					slot.Received = true
					slot.Pack = pack

					if slot.Pack.SeqNum == r.NextFrameExpected {
						for slot.Received {

							// actual receive happens here:
							sess.RecvHistory = append(sess.RecvHistory, slot.Pack)

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
					err := sess.send(ack)
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

func (sess *Session) send(pack *Packet) error {
	p("in Session.send(pack=%#v), with sess.Sim = %v", *pack, sess.Sim)
	var err error
	if sess.Sim != nil {
		err = sess.Sim.Send(pack)
	} else {
		var bts []byte
		bts, err = pack.MarshalMsg(nil)
		if err != nil {
			return err
		}
		err = sess.Nc.Publish(sess.Destination, bts)
	}
	return err
}

type SimNet struct {
	Net      map[string]*Session
	LossProb float64
	Latency  time.Duration
}

// NewSimNet makes a network simulator. The
// latency is one-way trip time; lossProb is the probability of
// the packet getting lost on the network.
func NewSimNet(lossProb float64, latency time.Duration) *SimNet {
	return &SimNet{
		Net:      make(map[string]*Session),
		LossProb: lossProb,
		Latency:  latency,
	}
}

func (sim *SimNet) AddNode(name string, s *Session) {
	sim.Net[name] = s
}

func (sim *SimNet) Send(pack *Packet) error {
	node, ok := sim.Net[pack.Dest]
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
		go func(node *Session, pack *Packet) {
			<-time.After(sim.Latency)
			p("sim: packet %v with latency %v ready to deliver to node %v, trying...",
				pack.SeqNum, sim.Latency, node.MyInbox)
			node.MsgRecv <- pack
			p("sim: packet (SeqNum: %v) delivered to node %v", pack.SeqNum, node.MyInbox)
		}(node, pack)
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
