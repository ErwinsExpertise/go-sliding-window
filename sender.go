package swp

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/glycerine/idem"
)

// TxqSlot is the sender's sliding window element.
type TxqSlot struct {
	OrigSendTime  time.Time
	RetryDeadline time.Time
	Pack          *Packet
}

// SenderState tracks the sender's sliding window state.
// To avoid circular deadlocks, the SenderState never talks
// directly to the RecvState. The RecvState will
// tell the Sender stuff on GotPack.
//
// Acks, retries, keep-alives, and original-data: these
// are the four types of sends we do.
//
type SenderState struct {
	Clk              Clock
	Net              Network
	Inbox            string
	Dest             string
	LastAckRec       int64
	LastFrameSent    int64
	Txq              []*TxqSlot
	SenderWindowSize int64
	mut              sync.Mutex
	Timeout          time.Duration

	// the main goroutine safe way to request
	// sending a packet:
	BlockingSend chan *Packet
	GotPack      chan *Packet

	Halt         *idem.Halter
	SendHistory  []*Packet
	SendSz       int64
	SendAck      chan *Packet
	DiscardCount int64

	LastSendTime            time.Time
	LastHeardFromDownstream time.Time
	KeepAliveInterval       time.Duration
	keepAlive               <-chan time.Time

	// after this many failed keepalives, we
	// close down the session. Set to less than 1
	// to disable the auto-close.
	NumFailedKeepAlivesBeforeClosing int

	SentButNotAcked map[int64]*TxqSlot

	// flow control params
	// last seen from our downstream
	// receiver, we throttle ourselves
	// based on these.
	LastSeenAvailReaderBytesCap int64
	LastSeenAvailReaderMsgCap   int64

	// do synchronized access via GetFlow()
	// and UpdateFlow(s.Net)
	FlowCt         *FlowCtrl
	TotalBytesSent int64
	rtt            *RTT

	// nil after Stop() unless we terminated the session
	// due to too many outstanding acks
	ExitErr error

	FailTracker *EventRingBuf
	TermCfg     *TermConfig

	// tell the receiver that sender is terminating
	SenderShutdown chan bool
}

// NewSenderState constructs a new SenderState struct.
func NewSenderState(net Network, sendSz int64, timeout time.Duration,
	inbox string, destInbox string, clk Clock, termCfg *TermConfig) *SenderState {
	s := &SenderState{
		Clk:              clk,
		Net:              net,
		Inbox:            inbox,
		Dest:             destInbox,
		SenderWindowSize: sendSz,
		Txq:              make([]*TxqSlot, sendSz),
		Timeout:          timeout,
		LastFrameSent:    -1,
		LastAckRec:       -1,
		Halt:             idem.NewHalter(),
		SendHistory:      make([]*Packet, 0),
		BlockingSend:     make(chan *Packet),
		SendSz:           sendSz,
		GotPack:          make(chan *Packet),
		SendAck:          make(chan *Packet),
		SentButNotAcked:  make(map[int64]*TxqSlot),
		SenderShutdown:   make(chan bool),

		// send keepalives (important especially for resuming flow from a
		// stopped state) at least this often:
		KeepAliveInterval: 100 * time.Millisecond,
		FlowCt: &FlowCtrl{Flow: Flow{
			// Control messages such as acks and keepalives
			// should not be blocked by flow-control (for
			// correctness/resumption from no-flow), so we need
			// to reserve extra headroom in the nats
			// subscription limits of this much to
			// allow resumption of flow.
			//
			// These reserved headroom settings can be
			// manually made larger before calling Start()
			//  -- and might need to be if you are running
			// very large windowMsgSz and/or windowByteSz; or
			// if you have large messages.
			ReservedByteCap: 64 * 1024,
			ReservedMsgCap:  32,
		}},
		// don't start fast, as we could overwhelm
		// the receiver. Instead start with sendSz,
		// assuming the receiver has a slot size just
		// like sender'sx. Notice that we'll need to allow
		// at least 2 messages in our 003 reorder test runs.
		// LastSeenAvailReaderMsgCap will get updated
		// after the first ack or keep alive to reflect
		// the actual receiver capacity. This is just
		// an inital value to use before we've heard from
		// the actual receiver over network.
		LastSeenAvailReaderMsgCap:   sendSz,
		LastSeenAvailReaderBytesCap: 1024 * 1024,
		rtt:                     NewRTT(),
		TermCfg:                 termCfg,
		LastHeardFromDownstream: clk.Now(),
	}
	for i := range s.Txq {
		s.Txq[i] = &TxqSlot{}
	}
	if s.TermCfg != nil {
		if s.TermCfg.TermWindowDur > 0 && s.TermCfg.TermUnackedLimit > 0 {
			s.FailTracker = NewEventRingBuf(s.TermCfg.TermUnackedLimit, s.TermCfg.TermWindowDur)
		}
	}
	return s
}

// ComputeInflight returns the number of bytes and messages
// that are in-flight: they have been sent but not yet acked.
func (s *SenderState) ComputeInflight() (bytesInflight int64, msgInflight int64) {
	for _, slot := range s.SentButNotAcked {
		msgInflight++
		bytesInflight += int64(len(slot.Pack.Data))
	}
	return
}

// Start initiates the SenderState goroutine, which manages
// sends, timeouts, and resends
func (s *SenderState) Start(sess *Session) {

	go func() {

		var acceptSend chan *Packet

		// check for expired timers at wakeFreq
		wakeFreq := s.Timeout / 2

		// send keepalives (for resuming flow from a
		// stopped state) at least this often:
		s.keepAlive = time.After(s.KeepAliveInterval)

		regularIntervalWakeup := time.After(wakeFreq)

		// shutdown stuff, all in one place for consistency
		defer func() {
			close(s.SenderShutdown) // stops the receiver
			s.Halt.ReqStop.Close()
			s.Halt.Done.Close()
			close(sess.Done) // lets clients detect shutdown
		}()

	sendloop:
		for {
			V("%v top of sendloop, sender LAR: %v, LFS: %v \n",
				s.Inbox, s.LastAckRec, s.LastFrameSent)

			// does the downstream reader have capacity to accept a send?
			// Block any new sends if so. We do a conditional receive. Start by
			// assuming no:
			acceptSend = nil

			// then check if we can set acceptSend.
			//
			// We accept a packet for sending if flow control info
			// from the receiver allows it.
			//
			bytesInflight, msgInflight := s.ComputeInflight()
			//q("%v bytesInflight = %v", s.Inbox, bytesInflight)
			//q("%v msgInflight = %v", s.Inbox, msgInflight)

			if s.LastSeenAvailReaderMsgCap-msgInflight > 0 &&
				s.LastSeenAvailReaderBytesCap-bytesInflight > 0 {
				V("%v flow-control: okay to send. s.LastSeenAvailReaderMsgCap: %v > msgInflight: %v",
					s.Inbox, s.LastSeenAvailReaderMsgCap, msgInflight)
				acceptSend = s.BlockingSend
			} else {
				V("%v flow-control kicked in: not sending. s.LastSeenAvailReaderMsgCap = %v,"+
					" msgInflight=%v, s.LastSeenAvailReaderBytesCap=%v bytesInflight=%v",
					s.Inbox, s.LastSeenAvailReaderMsgCap, msgInflight,
					s.LastSeenAvailReaderBytesCap, bytesInflight)
			}

			q("%v top of sender select loop", s.Inbox)
			select {
			case <-s.keepAlive:
				q("%v keepAlive at %v", s.Inbox, s.Clk.Now())
				s.doKeepAlive()

			case <-regularIntervalWakeup:
				now := s.Clk.Now()
				//p("%v regularIntervalWakeup at %v", s.Inbox, now)

				if s.NumFailedKeepAlivesBeforeClosing > 0 {
					thresh := s.KeepAliveInterval * time.Duration(s.NumFailedKeepAlivesBeforeClosing)
					//p("at regularInterval (every %v) doing check: SenderState.NumFailedKeepAlivesBeforeClosing=%v, checking for close after thresh %v (== %v * %v)", wakeFreq, s.NumFailedKeepAlivesBeforeClosing, thresh, s.KeepAliveInterval, s.NumFailedKeepAlivesBeforeClosing)
					elap := now.Sub(s.LastHeardFromDownstream)
					//p("elap = %v; s.LastHeardFromDownstream=%v", elap, s.LastHeardFromDownstream)
					if elap > thresh {
						// time to shutdown
						//p("too long (%v) since we've heard from the other end, declaring session dead and closing it.", thresh)
						return
					}
				}

				// have any of our packets timed-out and need to be
				// sent again?
				retry := []*TxqSlot{}
				//q("about to check s.FailTracker = %p", s.FailTracker)
				for _, slot := range s.SentButNotAcked {
					if slot.RetryDeadline.Before(now) {
						retry = append(retry, slot)
						if s.FailTracker != nil &&
							s.FailTracker.AddEventCheckOverflow(now) {
							//p("s.FailTracker detected too many errors, " +
							//	"terminating session")
							msg := fmt.Sprintf("Sender sees %v ack fails in "+
								"window of %v, terminating session",
								s.FailTracker.N, s.FailTracker.Window)
							te := &TerminatedError{Msg: msg}
							s.ExitErr = te
							sess.ExitErr = te
							return
						}
					}
				}
				if len(retry) > 0 {
					//q("%v sender retry list is len %v", s.Inbox, len(retry))
				}
			doRetryLoop:
				for _, slot := range retry {
					if slot.Pack == nil {
						//q("retry loop, slot = %#v", slot)
					}
					_, ok := s.SentButNotAcked[slot.Pack.SeqNum]
					if !ok {
						//q("already acked and gone from SentButNotAcked, so skip SeqNum %v and PopTop",
						//	slot.Pack.SeqNum)
						continue doRetryLoop
					}
					if slot.Pack.SeqNum <= s.LastAckRec {
						//q("already acked; is <= s.LastAckRecv (%v), so skip SeqNum %v and PopTop",
						//	s.LastAckRec, slot.Pack.SeqNum)
						continue doRetryLoop
					}

					// reset deadline and resend
					now := s.Clk.Now()
					flow := s.FlowCt.UpdateFlow(s.Inbox, s.Net, -1, -1, nil)
					slot.RetryDeadline = s.GetDeadline(now, flow)
					slot.Pack.SeqRetry++
					slot.Pack.DataSendTm = now

					slot.Pack.AvailReaderBytesCap = flow.AvailReaderBytesCap
					slot.Pack.AvailReaderMsgCap = flow.AvailReaderMsgCap
					slot.Pack.FromRttEstNsec = int64(s.rtt.GetEstimate())
					slot.Pack.FromRttSdNsec = int64(s.rtt.GetSd())
					slot.Pack.FromRttN = s.rtt.N

					//q("%v doing retry Net.Send() for pack = '%#v' of paydirt '%s'",
					//	s.Inbox, slot.Pack, string(slot.Pack.Data))
					err := s.Net.Send(slot.Pack, "retry")
					panicOn(err)
				}
				regularIntervalWakeup = time.After(wakeFreq)

			case <-s.Halt.ReqStop.Chan:
				return
			case pack := <-acceptSend:
				q("%v got <-acceptSend pack: '%#v'", s.Inbox, pack)
				s.doOrigDataSend(pack)

			case a := <-s.GotPack:
				s.LastHeardFromDownstream = a.ArrivedAtDestTm

				// ack/keepalive/data packet received in 'a' -
				// do sender side stuff
				//
				q("%v sender GotPack a: %#v", s.Inbox, a)
				//
				// flow control: respect a.AvailReaderBytesCap
				// and a.AvailReaderMsgCap info that we have
				// received from this ack
				//
				q("%v sender GotPack, updating s.LastSeenAvailReaderMsgCap %v -> %v",
					s.Inbox, s.LastSeenAvailReaderMsgCap, a.AvailReaderMsgCap)
				s.LastSeenAvailReaderBytesCap = a.AvailReaderBytesCap
				s.LastSeenAvailReaderMsgCap = a.AvailReaderMsgCap

				s.UpdateRTT(a)

				// need to update our map of SentButNotAcked
				// and remove everything before AckNum, which is cumulative.
				for _, slot := range s.SentButNotAcked {
					if slot.Pack.SeqNum <= a.AckNum {
						delete(s.SentButNotAcked, a.AckNum)
					}
				}

				if !a.AckOnly {
					// it wasn't an Ack, just updated flow info
					// from a received data message.
					//q("%s sender Gotack: just updated flow control, continuing sendloop", s.Inbox)
					continue sendloop
				}
				//q("%s sender Gotack: more than just flowcontrol...", s.Inbox)
				delete(s.SentButNotAcked, a.AckNum)
				if !InWindow(a.AckNum, s.LastAckRec+1, s.LastFrameSent) {
					//q("%v a.AckNum = %v outside sender's window [%v, %v], dropping it.",
					//	s.Inbox, a.AckNum, s.LastAckRec+1, s.LastFrameSent)
					s.DiscardCount++
					continue sendloop
				}
				//q("%v packet.AckNum = %v inside sender's window, keeping it.", s.Inbox, a.AckNum)
				for {
					s.LastAckRec++

					// release the send slot
					// do this before changing slot, since we point into slot.
					delete(s.SentButNotAcked, s.LastAckRec)

					//slot := s.Txq[s.LastAckRec%s.SenderWindowSize]
					//q("%v ... slot = %#v", s.Inbox, slot)

					if s.LastAckRec == a.AckNum {
						//q("%v s.LastAskRec[%v] matches a.AckNum[%v], breaking",
						//	s.Inbox, s.LastAckRec, a.AckNum)
						break
					}
					//q("%v s.LastAskRec[%v] != a.AckNum[%v], looping",
					//	s.Inbox, s.LastAckRec, a.AckNum)
				}
			case ackPack := <-s.SendAck:
				// request to send an ack:
				// don't go though the BlockingSend protocol; since
				// could effectively livelock us.
				q("%v doing ack Net.Send() SendAck request on ackPack: '%#v'",
					s.Inbox, ackPack)
				ackPack.FromRttEstNsec = int64(s.rtt.GetEstimate())
				ackPack.FromRttSdNsec = int64(s.rtt.GetSd())
				ackPack.FromRttN = s.rtt.N
				err := s.Net.Send(ackPack, "SendAck/ackPack")
				panicOn(err)
			}
		}
	}()
}

// Stop the SenderState componennt
func (s *SenderState) Stop() {
	s.Halt.RequestStop()
	<-s.Halt.Done.Chan
}

// for first time sends of data, not retries or acks.
func (s *SenderState) doOrigDataSend(pack *Packet) {
	//q("%v sender in acceptSend", s.Inbox)

	s.LastFrameSent++
	//q("%v LastFrameSent is now %v", s.Inbox, s.LastFrameSent)

	s.TotalBytesSent += int64(len(pack.Data))
	pack.CumulBytesTransmitted = s.TotalBytesSent

	lfs := s.LastFrameSent
	pos := lfs % s.SenderWindowSize
	slot := s.Txq[pos]

	pack.SeqNum = lfs
	if pack.From != s.Inbox {
		pack.From = s.Inbox
	}
	pack.From = s.Inbox
	slot.Pack = pack
	// data sends get stored in SentButNotAcked
	s.SentButNotAcked[lfs] = slot

	now := s.Clk.Now()
	s.SendHistory = append(s.SendHistory, pack)
	slot.OrigSendTime = now

	flow := s.FlowCt.UpdateFlow(s.Inbox+":sender", s.Net, -1, -1, nil)
	slot.RetryDeadline = s.GetDeadline(now, flow)
	s.LastSendTime = now

	//q("%v doSend(), flow = '%#v'", s.Inbox, flow)
	pack.AvailReaderBytesCap = flow.AvailReaderBytesCap
	pack.AvailReaderMsgCap = flow.AvailReaderMsgCap
	pack.DataSendTm = now

	// tell Dest about our RTT estimate.
	pack.FromRttEstNsec = int64(s.rtt.GetEstimate())
	pack.FromRttSdNsec = int64(s.rtt.GetSd())
	pack.FromRttN = s.rtt.N

	err := s.Net.Send(slot.Pack, fmt.Sprintf("doOrigDataSend() for %v", s.Inbox))
	panicOn(err)
}

func (s *SenderState) doKeepAlive() {
	if time.Since(s.LastSendTime) < s.KeepAliveInterval {
		return
	}
	flow := s.FlowCt.UpdateFlow(s.Inbox+":sender", s.Net, -1, -1, nil)
	//q("%v doKeepAlive(), flow = '%#v'", s.Inbox, flow)
	// send a packet with no data, to elicit an ack
	// with a new advertised window. This is
	// *not* an ack, because we need it to be
	// acked itself so we get any updated
	// flow control info from the other end.
	now := s.Clk.Now()
	s.LastSendTime = now
	kap := &Packet{
		From:                s.Inbox,
		Dest:                s.Dest,
		SeqNum:              -777, // => keepalive
		SeqRetry:            -777,
		DataSendTm:          now,
		AckRetry:            -777,
		KeepAlive:           true,
		AvailReaderBytesCap: flow.AvailReaderBytesCap,
		AvailReaderMsgCap:   flow.AvailReaderMsgCap,

		FromRttEstNsec: int64(s.rtt.GetEstimate()),
		FromRttSdNsec:  int64(s.rtt.GetSd()),
		FromRttN:       s.rtt.N,
	}
	//q("%v doing keepalive Net.Send()", s.Inbox)
	err := s.Net.Send(kap, fmt.Sprintf("keepalive from %v", s.Inbox))
	panicOn(err)

	s.keepAlive = time.After(s.KeepAliveInterval)
}

func (s *SenderState) doSendClose() {
	flow := s.FlowCt.UpdateFlow(s.Inbox+":sender", s.Net, -1, -1, nil)
	now := s.Clk.Now()
	s.LastSendTime = now
	kap := &Packet{
		From:                s.Inbox,
		Dest:                s.Dest,
		SeqNum:              -888, // => close
		SeqRetry:            -888,
		DataSendTm:          now,
		AckRetry:            -888,
		KeepAlive:           false,
		Closing:             true,
		AvailReaderBytesCap: flow.AvailReaderBytesCap,
		AvailReaderMsgCap:   flow.AvailReaderMsgCap,

		FromRttEstNsec: int64(s.rtt.GetEstimate()),
		FromRttSdNsec:  int64(s.rtt.GetSd()),
		FromRttN:       s.rtt.N,
	}
	//q("%v doing keepalive Net.Send()", s.Inbox)
	err := s.Net.Send(kap, fmt.Sprintf("keepalive from %v", s.Inbox))
	panicOn(err)

	s.keepAlive = time.After(s.KeepAliveInterval)
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func (s *SenderState) UpdateRTT(pack *Packet) {
	// avoid clock skew between machines by
	// not sampling one-way elapsed times.
	if pack.KeepAlive {
		return
	}
	if !pack.AckOnly {
		return
	}
	q("%v UpdateRTT top, pack = %#v", s.Inbox, pack)

	// acks have roundtrip times we can measure
	// use our own clock, thus avoiding clock
	// skew.

	obs := s.Clk.Now().Sub(pack.DataSendTm)

	// exclude obvious outliers where a round trip
	// took 60 seconds or more
	if obs > time.Minute {
		fmt.Printf("\n %v now: %v; UpdateRTT exluding outlier outside 60 seconds:"+
			" pack.DataSendTm = %v, observed rtt = %v.  pack = '%#v'\n",
			s.Inbox, s.Clk.Now(), pack.DataSendTm, obs, pack)
		return
	}

	//q("%v pack.DataSendTm = %v", s.Inbox, pack.DataSendTm)
	s.rtt.AddSample(obs)

	//	sd := s.rtt.GetSd()
	//	q("%v UpdateRTT: observed rtt was %v. new smoothed estimate after %v samples is %v. sd = %v", s.Inbox, obs, s.rtt.N, s.rtt.GetEstimate(), sd)
}

// GetDeadline sets the receive deadline using a
// weighted average of our observed RTT info and the remote
// end's observed RTT info.
func (s *SenderState) GetDeadline(now time.Time, flow Flow) time.Time {
	var ema time.Duration
	var sd time.Duration
	var n int64 = s.rtt.N

	if s.rtt.N < 1 {
		if flow.RemoteRttN > 2 {
			ema = time.Duration(flow.RemoteRttEstNsec)
			sd = time.Duration(flow.RemoteRttSdNsec)
		} else {
			// nobody has good info, just guess.
			return now.Add(20 * time.Millisecond)
		}
	} else {
		// we have at least one local round-trip sample

		// exponential moving average of observed RTT
		ema = s.rtt.GetEstimate()

		if s.rtt.N > 1 {
			sd = s.rtt.GetSd()
		} else {
			// default until we have 2 or more data points
			sd = ema
			// sanity check and cap if need be
			if sd > 2*ema {
				sd = 2 * ema
			}
		}
	}

	// blend local and remote info, in a weighted average.
	if flow.RemoteRttN > 2 && s.rtt.N > 2 {

		// Satterthwaite appoximation
		sd1 := float64(sd)
		var1 := sd1 * sd1
		n1 := float64(n)
		sd2 := float64(flow.RemoteRttSdNsec)
		var2 := sd2 * sd2
		n2 := float64(flow.RemoteRttN)
		SathSd := math.Sqrt((var1 / n1) + (var2 / n2))
		newSd := time.Duration(int64(SathSd))

		// variance weighted RTT estimate
		rtt1 := float64(ema)
		rtt2 := float64(flow.RemoteRttEstNsec)
		invvar1 := 1 / var1
		invvar2 := 1 / var2
		newEma := time.Duration(int64((rtt1*invvar1 + rtt2*invvar2) / (invvar1 + invvar2)))

		// update:
		//p("RTTema1=%v  n1=%v  sd1=%v    RTTemaRemote=%v  nRemote=%v  sdRemote=%v", rtt1, n1, sd1, rtt2, n2, sd2)
		//p("weighted average ema : %v -> %v", ema, newEma)
		//p("weighted average sd  : %v -> %v", sd, newSd)
		sd = newSd
		ema = newEma
	}

	// allow two standard deviations of margin
	// before consuming bandwidth for retry.
	fin := ema + 2*sd
	//q("%v ema is %v", s.Inbox, ema)
	//q("setting deadline of duration %v", fin)
	return now.Add(fin)
}
