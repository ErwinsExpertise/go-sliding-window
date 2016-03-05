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

	A.Push(p1)

	time.Sleep(time.Second)

	A.Stop()
	B.Stop()

	cv.Convey("Given two nodes A and B, sending a packet on a non-lossy network from A to B, the packet should arrive at B", t, func() {
		cv.So(A.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		//cv.So(B.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		cv.So(len(A.Swp.Sender.SendHistory), cv.ShouldEqual, 1)
		cv.So(len(B.Swp.Recver.RecvHistory), cv.ShouldEqual, 1)
		cv.So(HistoryEqual(A.Swp.Sender.SendHistory, B.Swp.Recver.RecvHistory), cv.ShouldBeTrue)
	})
}

func Test002LostPacketTimesOutAndIsRetransmitted(t *testing.T) {

	lossProb := float64(0)
	lat := time.Millisecond
	net := NewSimNet(lossProb, lat)
	net.DiscardOnce = Seqno(0)
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

	A.Push(p1)

	time.Sleep(time.Second)

	A.Stop()
	B.Stop()

	cv.Convey("Given two nodes A and B, if a packet from A to B is lost, the timeout mechanism in the sender should notice that it didn't get the ack, and should resend.", t, func() {
		cv.So(A.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		//cv.So(B.Swp.Recver.DiscardCount, cv.ShouldEqual, 0)
		cv.So(len(A.Swp.Sender.SendHistory), cv.ShouldEqual, 1)
		cv.So(len(B.Swp.Recver.RecvHistory), cv.ShouldEqual, 1)
		cv.So(HistoryEqual(A.Swp.Sender.SendHistory, B.Swp.Recver.RecvHistory), cv.ShouldBeTrue)
	})
}
