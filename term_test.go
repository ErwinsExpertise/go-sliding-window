package swp

import (
	"time"

	cv "github.com/glycerine/goconvey/convey"
	"testing"
)

func Test015TerminationOfSessions(t *testing.T) {

	cv.Convey(`Given these two parameters that are both set (non zero):

	     TermWindowDur time.Duration
	     TermUnackedLimit  int

	   when a downstream subscriber doesn't respond to TermUnackedLimit messages in within any TermWindowDur time window (backward looking from Now), then the Sender should stop sending to that subscriber, effectively dropping that endpoint, and terminate the session with prejudice/an error/log.`, t, func() {

		lossProb := float64(0)
		lat := 1 * time.Millisecond
		net := NewSimNet(lossProb, lat)
		net.AllowBlackHoleSends = true
		rtt := 2 * lat

		n := 10
		var simClk = &SimClock{}
		t0 := time.Now()
		t1 := t0.Add(time.Duration(n) * time.Second)
		//t2 := t1.Add(time.Second)
		simClk.Set(t0)

		A, err := NewSession(SessionConfig{Net: net, LocalInbox: "A", DestInbox: "B",
			WindowMsgSz: 3, WindowByteSz: -1, Timeout: rtt, Clk: simClk,
			TermWindowDur:    1 * time.Second,
			TermUnackedLimit: 1,
		})
		panicOn(err)

		A.SelfConsumeForTesting()

		p1 := &Packet{
			From: "A",
			Dest: "B",
			Data: []byte("one"),
		}

		for i := 0; i < n; i++ {
			A.Push(p1)
		}
		simClk.Set(t1)
		A.Push(p1)

		<-A.Done
		cv.So(A.ExitErr, cv.ShouldEqual, TerminatedError)
		A.Stop()
	})
}
