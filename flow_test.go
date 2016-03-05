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
	// and so we should see the internal buffers staying
	// within range. we should never get an error from
	// nats saying that the buffers have overflowed and
	// messages have been dropped.

	// ===============================
	// begin generic nats setup
	// ===============================

	origdir, tempdir := MakeAndMoveToTempDir() // cd to tempdir
	p("origdir = '%s'", origdir)
	p("tempdir = '%s'", tempdir)
	defer TempDirCleanup(origdir, tempdir)

	host := "127.0.0.1"
	port := GetAvailPort()
	gnats := StartGnatsd(host, port)
	defer func() {
		p("calling gnats.Shutdown()")
		gnats.Shutdown() // when done
	}()
	gnatsdUrl := fmt.Sprintf("nats://%v:%v", host, port)

	subj := "my-topic"
	sub := StartSubscriber("B-subscriber", gnatsdUrl, subj)
	defer sub.Close()

	pub := StartPublisher("A-publisher", gnatsdUrl, subj)
	defer pub.Close()

	q("sub = %#v", sub)
	q("pub = %#v", pub)

	lossProb := float64(0)
	lat := 1 * time.Millisecond
	net := NewSimNet(lossProb, lat)
	rtt := 3 * lat

	A, err := NewSession(net, "A", "B", 3, rtt)
	panicOn(err)
	B, err := NewSession(net, "B", "A", 3, rtt)
	B.Swp.Sender.LastFrameSent = 999
	panicOn(err)

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

	// NOT DONE, WORK IN PROGRESS
	cv.Convey("Given a faster sender A and a slower receiver B, flow-control in the SWP should throttle back the sender so it doesn't overwhelm the downstream receiver's buffers", t, func() {
		//cv.So(A.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		//cv.So(B.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		cv.So(len(A.Swp.Sender.SendHistory), cv.ShouldEqual, 100)
		cv.So(len(B.Swp.Recver.RecvHistory), cv.ShouldEqual, 100)
		cv.So(HistoryEqual(A.Swp.Sender.SendHistory, B.Swp.Recver.RecvHistory), cv.ShouldBeTrue)
	})
}
