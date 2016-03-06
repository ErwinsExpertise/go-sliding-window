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

//msgp:ignore TxqSlot RxqSlot Semaphore SenderState RecvState SWP Session NatsNet SimNet

// Seqno is the sequence number used in the sliding window.
type Seqno int64

// Packet is what is transmitted between Sender and Recver.
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
	//
	// NB we'll want to reserve some headroom in our
	// nats buffers for receipt of acks and other
	// control messages (ReservedByteCap, ReservedMsgCap)
	//
	AvailReaderBytesCap int64 // for sender throttling/flow-control
	AvailReaderMsgCap   int64 // for sender throttling/flow-control

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
func NewSWP(net Network, windowSize int64,
	timeout time.Duration, inbox string, destInbox string) *SWP {
	recvSz := windowSize
	sendSz := windowSize
	snd := NewSenderState(net, sendSz, timeout, inbox, destInbox)
	rcv := NewRecvState(net, recvSz, timeout, inbox, snd)
	swp := &SWP{
		Sender: snd,
		Recver: rcv,
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

	Net Network
}

func NewSession(net Network,
	localInbox string,
	destInbox string,
	windowSz int64,
	timeout time.Duration) (*Session, error) {

	sess := &Session{
		Swp:         NewSWP(net, windowSz, timeout, localInbox, destInbox),
		MyInbox:     localInbox,
		Destination: destInbox,
		Net:         net,
	}
	sess.Swp.Start()

	return sess, nil
}

var ErrShutdown = fmt.Errorf("shutting down")

// Push sends a message packet, blocking until that is done.
func (sess *Session) Push(pack *Packet) {
	//q("%v Push called", sess.MyInbox)
	select {
	case sess.Swp.Sender.BlockingSend <- pack:
	case <-sess.Swp.Sender.ReqStop:
		// give up, Sender is shutting down.
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

type NatsNet struct {
	Cli *NatsClient
}

func NewNatsNet(cli *NatsClient) *NatsNet {
	net := &NatsNet{Cli: cli}
	return net
}

// Network describes our network abstraction, and is implemented
// by SimNet and NatsNet.
type Network interface {

	// Send transmits the packet. It is send and pray; no
	// guarantee of delivery is made by the Network.
	Send(pack *Packet) error

	// Listen starts receiving packets addressed to inbox on the returned channel.
	Listen(inbox string) (chan *Packet, error)

	// BufferCaps returns the byte and message limits
	// currently in effect, so that flow control
	// can be used to avoid sender overrunning them.
	BufferCaps() (bytecap int64, msgcap int64)
}

// BufferCaps returns the byte and message limits
// currently in effect, so that flow control
// can be used to avoid sender overrunning them.
func (n *NatsNet) BufferCaps() (bytecap int64, msgcap int64) {
	return GetSubscripCap(n.Cli.Scrip)
}

// Listen starts receiving packets addressed to inbox on the returned channel.
func (n *NatsNet) Listen(inbox string) (chan *Packet, error) {
	mr := make(chan *Packet)

	// do actual subscription
	err := n.Cli.MakeSub(inbox, func(msg *nats.Msg) {
		var pack Packet
		_, err := pack.UnmarshalMsg(msg.Data)
		panicOn(err)
		mr <- &pack
	})
	p("subscription by %v on subject %v succeeded", n.Cli.Cfg.NatsNodeName, inbox)
	return mr, err
}

// Send blocks until Send has started (but not until acked).
func (n *NatsNet) Send(pack *Packet) error {
	//q("in NatsNet.Send(pack=%#v)", *pack)
	bts, err := pack.MarshalMsg(nil)
	if err != nil {
		return err
	}
	return n.Cli.Nc.Publish(pack.Dest, bts)
}

// SimNet simulates a network with a given latency and loss characteristics.
type SimNet struct {
	Net      map[string]chan *Packet
	LossProb float64
	Latency  time.Duration

	TotalSent map[string]int64
	TotalRcvd map[string]int64
	mapMut    sync.Mutex

	// simulate loss of the first packets
	DiscardOnce Seqno

	// simulate re-ordering of packets by setting this to 1
	SimulateReorderNext int
	heldBack            *Packet

	// simulate duplicating the next packet
	DuplicateNext bool
}

// BufferCaps returns the byte and message limits
// currently in effect, so that flow control
// can be used to avoid sender overrunning them.
func (n *SimNet) BufferCaps() (bytecap int64, msgcap int64) {
	// limits so high they shouldn't be restrictive
	return 1024 * 1024 * 1024, 1024
}

// NewSimNet makes a network simulator. The
// latency is one-way trip time; lossProb is the probability of
// the packet getting lost on the network.
func NewSimNet(lossProb float64, latency time.Duration) *SimNet {
	return &SimNet{
		Net:         make(map[string]chan *Packet),
		LossProb:    lossProb,
		Latency:     latency,
		DiscardOnce: -1,
		TotalSent:   make(map[string]int64),
		TotalRcvd:   make(map[string]int64),
	}
}

func (sim *SimNet) Listen(inbox string) (chan *Packet, error) {
	ch := make(chan *Packet)
	sim.Net[inbox] = ch
	return ch, nil
}

func (sim *SimNet) Send(pack *Packet) error {
	//q("in SimNet.Send(pack=%#v)", *pack)

	sim.mapMut.Lock()
	sim.TotalSent[pack.From]++
	sim.mapMut.Unlock()

	ch, ok := sim.Net[pack.Dest]
	if !ok {
		return fmt.Errorf("sim sees packet for unknown node '%s'", pack.Dest)
	}

	switch sim.SimulateReorderNext {
	case 0:
		// do nothing
	case 1:
		sim.heldBack = pack
		q("sim reordering: holding back pack SeqNum %v to %v", pack.SeqNum, pack.Dest)
		sim.SimulateReorderNext++
		return nil
	default:
		q("sim: setting SimulateReorderNext %v -> 0", sim.SimulateReorderNext)
		sim.SimulateReorderNext = 0
	}

	if pack.SeqNum == sim.DiscardOnce {
		q("sim: packet lost because %v SeqNum == DiscardOnce (%v)", pack.SeqNum, sim.DiscardOnce)
		sim.DiscardOnce = -1
		return nil
	}

	pr := cryptoProb()
	isLost := pr <= sim.LossProb
	if sim.LossProb > 0 && isLost {
		q("sim: bam! packet-lost! %v to %v", pack.SeqNum, pack.Dest)
	} else {
		q("sim: %v to %v: not lost. packet will arrive after %v", pack.SeqNum, pack.Dest, sim.Latency)
		// start a goroutine per packet sent, to simulate arrival time with a timer.
		go sim.sendWithLatency(ch, pack, sim.Latency)
		if sim.heldBack != nil {
			q("sim: reordering now -- sending along heldBack packet %v to %v",
				sim.heldBack.SeqNum, sim.heldBack.Dest)
			go sim.sendWithLatency(ch, sim.heldBack, sim.Latency+20*time.Millisecond)
			sim.heldBack = nil
		}

		if sim.DuplicateNext {
			sim.DuplicateNext = false
			go sim.sendWithLatency(ch, pack, sim.Latency)
		}

	}
	return nil
}

func (sim *SimNet) sendWithLatency(ch chan *Packet, pack *Packet, lat time.Duration) {
	<-time.After(lat)
	q("sim: packet %v, after latency %v, ready to deliver to node %v, trying...",
		pack.SeqNum, lat, pack.Dest)
	ch <- pack
	//p("sim: packet (SeqNum: %v) delivered to node %v", pack.SeqNum, pack.Dest)

	sim.mapMut.Lock()
	sim.TotalRcvd[pack.Dest]++
	sim.mapMut.Unlock()
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

type Sum struct {
	ObsKeepRateFromA float64
	ObsKeepRateFromB float64
	tsa              int64
	tra              int64
	tsb              int64
	trb              int64
}

func (net *SimNet) Summary() *Sum {
	net.mapMut.Lock()
	defer net.mapMut.Unlock()

	s := &Sum{
		ObsKeepRateFromA: float64(net.TotalRcvd["B"]) / float64(net.TotalSent["A"]),
		ObsKeepRateFromB: float64(net.TotalRcvd["A"]) / float64(net.TotalSent["B"]),
		tsa:              net.TotalSent["A"],
		tra:              net.TotalRcvd["A"],
		tsb:              net.TotalSent["B"],
		trb:              net.TotalRcvd["B"],
	}
	return s
}

func (s *Sum) Print() {
	p("summary: packets A sent %v   -> B packets rcvd %v  [kept %.03f%%, lost %.03f%%]",
		s.tsa, s.trb, s.ObsKeepRateFromA, 1.0-s.ObsKeepRateFromA)
	p("summary: packets B sent %v   -> A packets rcvd %v  [kept %.03f%%, lost %.03f%%]",
		s.tsb, s.tra, s.ObsKeepRateFromB, 1.0-s.ObsKeepRateFromB)
}
