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
	gotAck := make(chan struct{})
	A.setPacketRecvCallback(func(pack *Packet) {
		p("A packet receive callback happened!")
		if ack1 == nil {
			ack1 = pack
			rcTm = simClk.Now()
			close(gotAck)
		}
	})

	A.Push(p1)

	r1 := <-B.ReadMessagesCh
	<-gotAck
	p("r1 = %#v", r1.Seq[0])

	p("ack1 = %#v", ack1)
	p("rcTm = %#v", rcTm)
	p("rcTm - t0 = %v", rcTm.Sub(t0))
	rtt1 := rcTm.Sub(ack1.DataSendTm)
	cv.Convey("Given 1 second clock steps at each receive, when A sends packet p1 to B at t0, arriving at t1, and the ack is returned at t2; then: the RTT computed should be t2-t0 or 2 seconds", t, func() {
		cv.So(rcTm, cv.ShouldResemble, t2)
		cv.So(rtt1, cv.ShouldResemble, 2*time.Second)

		p("Even if there is a retry, the Ack packet should be self contained, allowing RTT sampling from the DataSendTm that is updated on each retry")
		cv.So(ack1.ArrivedAtDestTm.Sub(ack1.DataSendTm), cv.ShouldResemble, rtt1)
	})

	A.Stop()
	B.Stop()

}
