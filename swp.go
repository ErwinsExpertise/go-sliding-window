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
	Timeout        *time.Timer `msg:"-"`
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

	// networks. use Sim in preference to Nc if Sim is present
	Nc  *nats.Conn
	Sim *SimNet
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
	}
	var err error
	sess.InboxSubscription, err = nc.Subscribe(sess.MyInbox, func(msg *nats.Msg) {
		var pack Packet
		_, err := pack.UnmarshalMsg(msg.Data)
		panicOn(err)
		sess.MsgRecv <- &pack
	})
	if err != nil {
		return nil, err
	}
	sess.Nc = nc
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
	slot.Pack = &Packet{
		From:   sess.MyInbox,
		Dest:   sess.Destination,
		SeqNum: lfs,
		Data:   data,
	}

	// todo: start go routine that listens for this timeout
	// most of this logic probably moves to that goroutine too.
	slot.Timeout = time.NewTimer(s.Timeout)
	slot.TimerCancelled = false
	slot.Session = sess
	// todo: where are the timeouts handled?

	bts, err := slot.Pack.MarshalMsg(nil)
	if err != nil {
		return err
	}
	return sess.Nc.Publish(sess.Destination, bts)
}

// Pop receives. It receives both data and acks from earlier sends.
// Pop recevies acks (for sends from this node), and data.
func (sess *Session) Pop() ([]byte, error) {

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
					p("packet.AckNum = %v outside sender's window, dropping it.", pack.AckNum)
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
				slot.Received = true
				if slot.Pack.SeqNum == r.NextFrameExpected {

					for slot.Received {
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
				err = sess.send(ack)
				logOn(err)
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

func (sess *Session) send(pack *Packet) error {
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
		p("sim: packet will arrive after %v", sim.Latency)
		// start a goroutine per packet sent, to simulate arrival time with a timer.
		go func(node *Session, pack *Packet) {
			<-time.After(sim.Latency)
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
