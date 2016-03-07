package swp

import (
	"time"

	cv "github.com/glycerine/goconvey/convey"
	"testing"
)

func Test011RTTfromPackets(t *testing.T) {

	lossProb := float64(0)
	lat := time.Millisecond
	net := NewSimNet(lossProb, lat)
	rtt := time.Hour

	var simClk = &SimClock{}
	t0 := time.Now()
	t1 := t0.Add(time.Second)
	t2 := t1.Add(time.Second)
	simClk.Set(t0)

	A, err := NewSession(net, "A", "B", 3, -1, rtt, simClk)
	panicOn(err)
	B, err := NewSession(net, "B", "A", 3, -1, rtt, simClk)
	panicOn(err)

	A.IncrementClockOnReceiveForTesting()
	B.IncrementClockOnReceiveForTesting()

	p1 := &Packet{
		From: "A",
		Dest: "B",
		Data: []byte("one"),
	}

	var ack1 *Packet
	var rcTm time.Time
	A.setPacketRecvCallback(func(pack *Packet) {
		p("A packet receive callback happened!")
		if ack1 == nil {
			ack1 = pack
			rcTm = simClk.Now()
		}
	})

	A.Push(p1)

	r1 := <-B.ReadMessagesCh
	p("r1 = %#v", r1)
	time.Sleep(100 * time.Millisecond)
	A.Stop()
	B.Stop()

	now := simClk.Now()
	p("simClk = %#v", simClk)
	p("now = %v", now)
	p("ack1 = %#v", ack1)
	p("rcTm - t0 = %v", rcTm.Sub(t0))
	rtt1 := rcTm.Sub(ack1.DataSendTm)
	cv.Convey("Given A sending packet p1 to B at t0, arriving at t1, and the ack returning at t2, the RTT computed should be t2-t0 or 2 seconds", t, func() {
		cv.So(rcTm, cv.ShouldResemble, t2)
		cv.So(rtt1, cv.ShouldResemble, 2*time.Second)
	})
}
