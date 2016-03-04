package swp

import (
	"time"
	//"fmt"
	//"os"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

// check for:
//
// packet loss - eventually received and/or re-requested.
// duplicate receives - discard the 2nd one.
// out of order receives - reorder correctly.
//
func Test001Network(t *testing.T) {

	lossProb := float64(0)
	lat := time.Millisecond
	net := NewSimNet(lossProb, lat)
	rtt := 2 * lat

	A, err := NewSession(net, "A", "B", 3, rtt)
	panicOn(err)
	B, err := NewSession(net, "B", "A", 3, rtt)
	panicOn(err)

	p1 := &Packet{
		From: "A",
		Dest: "B",
		Data: []byte("one"),
	}

	err = A.Push(p1)
	panicOn(err)

	time.Sleep(time.Second)

	A.Stop()
	B.Stop()

	cv.Convey("Given two nodes A and B, sending a packet on a non-lossy network from A to B, the packet should arrive at B", t, func() {
		cv.So(A.DiscardCount, cv.ShouldEqual, 0)
		cv.So(B.DiscardCount, cv.ShouldEqual, 0)
		cv.So(len(B.SendHistory), cv.ShouldEqual, 1)
		cv.So(len(B.RecvHistory), cv.ShouldEqual, 1)
		cv.So(HistoryEqual(A.SendHistory, B.RecvHistory), cv.ShouldBeTrue)
	})
}

func Test002LostPacketTimesOutAndIsRetransmitted(t *testing.T) {

	lossProb := float64(0)
	lat := time.Millisecond
	net := NewSimNet(lossProb, lat)
	net.DiscardUntil = Seqno(1)
	rtt := 2 * lat

	A, err := NewSession(net, "A", "B", 3, rtt)
	panicOn(err)
	B, err := NewSession(net, "B", "A", 3, rtt)
	panicOn(err)

	p1 := &Packet{
		From: "A",
		Dest: "B",
		Data: []byte("one"),
	}

	err = A.Push(p1)
	panicOn(err)

	time.Sleep(time.Second)

	A.Stop()
	B.Stop()

	cv.Convey("Given two nodes A and B, if a packet from A to B is lost, the timeout mechanism in the sender should notice that it didn't get the ack, and should resend.", t, func() {
		cv.So(A.DiscardCount, cv.ShouldEqual, 0)
		cv.So(B.DiscardCount, cv.ShouldEqual, 0)
		cv.So(len(B.SendHistory), cv.ShouldEqual, 1)
		cv.So(len(B.RecvHistory), cv.ShouldEqual, 1)
		cv.So(HistoryEqual(A.SendHistory, B.RecvHistory), cv.ShouldBeTrue)
	})
}
