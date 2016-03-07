package swp

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// SimNet simulates a network with the given latency and loss characteristics.
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

	// enforce that advertised windows are never
	// violated by having more messages in flight
	// than have been advertised.
	Advertised map[string]int64
	Inflight   map[string]int64

	ReqStop chan bool
	Done    chan bool
}

// NewSimNet makes a network simulator. The
// latency is one-way trip time; lossProb is the probability of
// the packet getting lost on the network.
func NewSimNet(lossProb float64, latency time.Duration) *SimNet {
	s := &SimNet{
		Net:         make(map[string]chan *Packet),
		LossProb:    lossProb,
		Latency:     latency,
		DiscardOnce: -1,
		TotalSent:   make(map[string]int64),
		TotalRcvd:   make(map[string]int64),
		Advertised:  make(map[string]int64),
		Inflight:    make(map[string]int64),
		//LinearizeReceives: make(chan Linear),
		ReqStop: make(chan bool),
		Done:    make(chan bool),
	}
	return s
}

func (sim *SimNet) Listen(inbox string) (chan *Packet, error) {
	ch := make(chan *Packet)
	sim.Net[inbox] = ch
	return ch, nil
}

func (sim *SimNet) Send(pack *Packet, why string) error {
	//q("in SimNet.Send(pack=%#v) why:'%v'", *pack, why)

	sim.mapMut.Lock()
	sim.TotalSent[pack.From]++
	defer sim.mapMut.Unlock()

	ch, ok := sim.Net[pack.Dest]
	if !ok {
		return fmt.Errorf("sim sees packet for unknown node '%s'", pack.Dest)
	}

	switch sim.SimulateReorderNext {
	case 0:
		// do nothing
	case 1:
		sim.heldBack = pack
		//q("sim reordering: holding back pack SeqNum %v to %v", pack.SeqNum, pack.Dest)
		sim.SimulateReorderNext++
		return nil
	default:
		//q("sim: setting SimulateReorderNext %v -> 0", sim.SimulateReorderNext)
		sim.SimulateReorderNext = 0
	}

	if pack.SeqNum == sim.DiscardOnce {
		//q("sim: packet lost because %v SeqNum == DiscardOnce (%v)", pack.SeqNum, sim.DiscardOnce)
		sim.DiscardOnce = -1
		return nil
	}

	pr := cryptoProb()
	isLost := pr <= sim.LossProb
	if sim.LossProb > 0 && isLost {
		//q("sim: bam! packet-lost! %v to %v", pack.SeqNum, pack.Dest)
	} else {
		//q("sim: %v to %v: not lost. packet will arrive after %v", pack.SeqNum, pack.Dest, sim.Latency)
		// start a goroutine per packet sent, to simulate arrival time with a timer.
		go sim.sendWithLatency(ch, pack, sim.Latency)
		if sim.heldBack != nil {
			//q("sim: reordering now -- sending along heldBack packet %v to %v",
			//	sim.heldBack.SeqNum, sim.heldBack.Dest)
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

// helper for Send
func (sim *SimNet) sendWithLatency(ch chan *Packet, pack *Packet, lat time.Duration) {
	<-time.After(lat)
	//q("sim: packet %v, after latency %v, ready to deliver to node %v, trying...",
	//	pack.SeqNum, lat, pack.Dest)

	//	sim.preCheckFlowControlNotViolated(pack)

	ch <- pack
	q("sim: packet (SeqNum: %v) delivered to node %v", pack.SeqNum, pack.Dest)

	//	sim.postCheckFlowControlNotViolated(pack)

	sim.mapMut.Lock()
	sim.TotalRcvd[pack.Dest]++
	sim.mapMut.Unlock()
}

// helper for sendWithLatency, after send, before receive
func (sim *SimNet) preCheckFlowControlNotViolated(pack *Packet) {
	// check for advertising of flow parameters, and that data sends don't
	// exceed the advertised
	sim.mapMut.Lock()

	var isData bool
	if !pack.AckOnly && !pack.KeepAlive {
		isData = true
	}

	// update
	if isData {
		// only if data:
		sim.Inflight[pack.Dest]++
	}

	if isData {
		// verify correct:
		advert := sim.Advertised[pack.Dest]
		inflight := sim.Inflight[pack.Dest]
		if inflight > advert {
			panic(fmt.Sprintf("inflight(%v) > advert(%v) so flow-control has been violated",
				inflight, advert))
		}
	}

	// update
	// any kind of packet:
	sim.Advertised[pack.From] = pack.AvailReaderMsgCap
	q("sim: latest advertised from '%s' is pack.AvailReaderMsgCap: %v", pack.From, pack.AvailReaderMsgCap)

	sim.mapMut.Unlock()
}

// helper for sendWithLatency, after receive
func (sim *SimNet) postCheckFlowControlNotViolated(pack *Packet) {
	sim.mapMut.Lock()

	var isData bool
	if !pack.AckOnly && !pack.KeepAlive {
		isData = true
	}
	// update
	if isData {
		// only if data:
		sim.Inflight[pack.Dest]--
	}
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
	fmt.Printf("\n summary: packets A sent %v   -> B packets rcvd %v  [kept %.01f%%, lost %.01f%%]\n",
		s.tsa, s.trb, 100.0*s.ObsKeepRateFromA, 100.0*(1.0-s.ObsKeepRateFromA))
	fmt.Printf("\n summary: packets B sent %v   -> A packets rcvd %v  [kept %.01f%%, lost %.01f%%]\n",
		s.tsb, s.tra, 100.0*s.ObsKeepRateFromB, 100.0*(1.0-s.ObsKeepRateFromB))
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
			q("packet histories disagree at i=%v, a[%v].SeqNum = %v, while b[%v].SeqNum = %v",
				i, a[i].SeqNum, b[i].SeqNum)
			return false
		}
	}
	return true
}
