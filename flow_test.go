package swp

import (
	"fmt"
	"time"

	cv "github.com/glycerine/goconvey/convey"
	"testing"
)

func Test008ProvidesFlowControlToThrottleOverSending(t *testing.T) {

	// Given a consumer able to read at 1k messages/sec,
	// and a producer able to produce at 5k messages/sec,
	// we should see bandwidth across the network at the
	// rate at which the consumer allows via flow-control.
	// i.e.
	// consumer reads at a fixed 20% of the rate at which the
	// producer can produce, then we should see the producer
	// only sending at that 20% rate.
	//
	// implications:
	//
	// We should see the internal buffers
	// (in the receiving nats client library) staying
	// within range. We should never get an error from
	// nats saying that the buffers have overflowed and
	// messages have been dropped.

	// ===============================
	// begin generic nats setup
	// ===============================

	host := "127.0.0.1"
	port := getAvailPort()
	gnats := startGnatsd(host, port)
	defer func() {
		p("calling gnats.Shutdown()")
		gnats.Shutdown() // when done
	}()

	// ===============================
	// setup nats clients for a publisher and a subscriber
	// ===============================

	subC := NewNatsClientConfig(host, port, "B", "B", true, false)
	//subC.AsyncErrPanics = true
	sub := NewNatsClient(subC)
	err := sub.Start()
	panicOn(err)
	defer sub.Close()

	pubC := NewNatsClientConfig(host, port, "A", "A", true, false)
	pub := NewNatsClient(pubC)
	err = pub.Start()
	panicOn(err)
	defer pub.Close()

	// ===============================
	// make a session for each
	// ===============================

	anet := NewNatsNet(pub)
	bnet := NewNatsNet(sub)

	q("sub = %#v", sub)
	q("pub = %#v", pub)

	//lossProb := float64(0)
	lat := 1 * time.Millisecond

	rtt := 1000 * lat

	A, err := NewSession(anet, "A", "B", 3, -1, rtt, RealClk)
	panicOn(err)
	B, err := NewSession(bnet, "B", "A", 3, -1, rtt, RealClk)
	B.Swp.Sender.LastFrameSent = 999
	panicOn(err)

	A.SelfConsumeForTesting()
	//B.SelfConsumeForTesting()

	// ===============================
	// setup subscriber to consume at 1 message/sec
	// ===============================

	rep := ReportOnSubscription(sub.Scrip)
	p("rep = %#v", rep)

	// this limit alone is the first test for flow
	// control, since with a 10 message limit we'll quickly
	// overflow the client-side nats internal
	// buffer, and panic since 	subC.AsyncErrPanics = true
	// when trying to send 100 messages in a row.
	msgLimit := 10
	bytesLimit := 20000
	B.Swp.Sender.FlowCt = &FlowCtrl{flow: Flow{
		ReservedByteCap: 5000,
		ReservedMsgCap:  9,
	}}
	SetSubscriptionLimits(sub.Scrip, msgLimit, bytesLimit)

	// msglimit 10 and reserved 10 should block
	// all de novo sending

	// ===============================
	// setup publisher to produce at 5 messages/sec
	// ===============================

	n := 100
	seq := make([]*Packet, n)
	for i := range seq {
		pack := &Packet{
			From: "A",
			Dest: "B",
			Data: []byte(fmt.Sprintf("%v", i)),
		}
		seq[i] = pack
	}

	readsAllDone := make(chan struct{})
	go func() {
		seen := 0
		for seen < n {
			ios := <-B.ReadMessagesCh
			seen += len(ios.Seq)
			p("go read total of %v", seen)
		}
		p("done with all reads")
		close(readsAllDone)
	}()

	for i := range seq {
		A.Push(seq[i])
		p("push i=%v done", i)
	}
	<-readsAllDone

	A.Stop()
	B.Stop()

	// NOT DONE, WORK IN PROGRESS
	cv.Convey("Given a faster sender A and a slower receiver B, flow-control in the SWP should throttle back the sender so it doesn't overwhelm the downstream receiver's buffers. The current test simply keeps a window of 3 messages on both sender and receiver, and runs 100 messages across the nats bus, checking that they all arrived at the end.", t, func() {
		//cv.So(A.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		//cv.So(B.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		cv.So(len(A.Swp.Sender.SendHistory), cv.ShouldEqual, 100)
		cv.So(len(B.Swp.Recver.RecvHistory), cv.ShouldEqual, 100)
		cv.So(HistoryEqual(A.Swp.Sender.SendHistory, B.Swp.Recver.RecvHistory), cv.ShouldBeTrue)
	})
}

func Test009SimNetVerifiesFlowControlNotViolated(t *testing.T) {

	// Same as Test008 but use SimNet in order to verify
	// that our flow control properties are not violated:
	// that the sender only ever sends the number of
	// frames <= the advertised available amounts of
	// the receiver.

	lossProb := float64(0)
	lat := time.Millisecond
	net := NewSimNet(lossProb, lat)
	rtt := 1000 * lat

	A, err := NewSession(net, "A", "B", 3, -1, rtt, RealClk)
	panicOn(err)
	B, err := NewSession(net, "B", "A", 3, -1, rtt, RealClk)
	panicOn(err)
	B.Swp.Sender.LastFrameSent = 999

	A.SelfConsumeForTesting()
	B.SelfConsumeForTesting()

	B.Swp.Sender.FlowCt = &FlowCtrl{flow: Flow{
		ReservedByteCap:     0,
		ReservedMsgCap:      0,
		AvailReaderBytesCap: 5000,
		AvailReaderMsgCap:   1,
	}}
	A.Swp.Sender.FlowCt = &FlowCtrl{flow: Flow{
		ReservedByteCap:     0,
		ReservedMsgCap:      0,
		AvailReaderBytesCap: 5000,
		AvailReaderMsgCap:   1,
	}}
	// SimNet should automatically panic if there
	// is a flow control violation.

	n := 100
	seq := make([]*Packet, n)
	for i := range seq {
		pack := &Packet{
			From: "A",
			Dest: "B",
			Data: []byte(fmt.Sprintf("%v", i)),
		}
		seq[i] = pack
	}

	for i := range seq {
		A.Push(seq[i])
	}

	time.Sleep(1000 * time.Millisecond)

	A.Stop()
	B.Stop()

	smy := net.Summary()
	smy.Print()

	// NOT DONE, WORK IN PROGRESS
	cv.Convey("Given a faster sender A and a slower receiver B, flow-control in the SWP should throttle back the sender so it doesn't overwhelm the downstream receiver's buffers. This version of the test uses the SimNet network simulator, and currently is incomplete: it just runs 100 messages across the wire using a small window of size 3 on either end.", t, func() {
		//cv.So(A.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		//cv.So(B.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		cv.So(len(A.Swp.Sender.SendHistory), cv.ShouldEqual, 100)
		cv.So(len(B.Swp.Recver.RecvHistory), cv.ShouldEqual, 100)
		cv.So(HistoryEqual(A.Swp.Sender.SendHistory, B.Swp.Recver.RecvHistory), cv.ShouldBeTrue)
	})
}
