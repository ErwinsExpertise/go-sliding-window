package swp

import (
	"time"

	cv "github.com/glycerine/goconvey/convey"
	"testing"
)

func Test010ConsumerSideFlowControl(t *testing.T) {

	cv.Convey("Given node A sending to node B, the downstream reader-consumer application reading from B, if slow, the B node should reflect the reader's rate. So with a minimal 1 message size of buffer, the sender is blocked until that first message is consumed from B. Here there is no auto-reading, so we should have one held message until we read manually.", t, func() {

		lossProb := float64(0)
		lat := time.Millisecond
		net := NewSimNet(lossProb, lat)
		rtt := 2 * lat

		A, err := NewSession(SessionConfig{Net: net, LocalInbox: "A", DestInbox: "B",
			WindowMsgSz: 3, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
		panicOn(err)
		B, err := NewSession(SessionConfig{Net: net, LocalInbox: "B", DestInbox: "A",
			WindowMsgSz: 3, WindowByteSz: -1, Timeout: rtt, Clk: RealClk})
		panicOn(err)

		p1 := &Packet{
			From: "A",
			Dest: "B",
			Data: []byte("one"),
		}

		A.Push(p1)

		time.Sleep(100 * time.Millisecond)
		held := <-B.Swp.Recver.NumHeldMessages

		cv.So(held, cv.ShouldEqual, 1)

		packetsConsumed := 0
		read := <-B.ReadMessagesCh
		for i := range read.Seq {
			p("B consumer sees packet '%#v', paystuff '%s'", read.Seq[i], string(read.Seq[i].Data))
			packetsConsumed++
		}

		held = <-B.Swp.Recver.NumHeldMessages
		cv.So(held, cv.ShouldEqual, 0)
		cv.So(packetsConsumed, cv.ShouldEqual, 1)
		p("good: got first packet delivery")

		p2 := &Packet{
			From: "A",
			Dest: "B",
			Data: []byte("two"),
		}
		A.Push(p2)
		time.Sleep(500 * time.Millisecond)

		held = <-B.Swp.Recver.NumHeldMessages
		cv.So(held, cv.ShouldEqual, 1) // sometimes 0 if super slow, probably 100 msec not enough

		read = <-B.ReadMessagesCh
		for i := range read.Seq {
			p("B consumer sees packet '%#v', paystuff '%s'", read.Seq[i], string(read.Seq[i].Data))
			packetsConsumed++
		}
		p("good: got second packet delivery")

		held = <-B.Swp.Recver.NumHeldMessages
		A.Stop()
		B.Stop()

		cv.So(packetsConsumed, cv.ShouldEqual, 2)
		cv.So(held, cv.ShouldEqual, 0)
	})
}
