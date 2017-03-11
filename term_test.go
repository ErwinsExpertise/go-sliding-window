package swp

import (
	"fmt"
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

		tc := TermConfig{
			TermWindowDur:    time.Duration(n) * time.Second,
			TermUnackedLimit: n - 1,
		}

		A, err := NewSession(SessionConfig{Net: net, LocalInbox: "A", DestInbox: "B",
			WindowMsgSz: 20, WindowByteSz: -1, Timeout: rtt, Clk: simClk,
			TermCfg: tc,
			NumFailedKeepAlivesBeforeClosing: -1,
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

		select {
		case <-A.Done:
		case <-time.After(time.Second):
			panic("should have gotten session Done closed and TerminatedError by now")
		}
		msg := fmt.Sprintf("Sender sees %v ack fails in window of %v, terminating session",
			tc.TermUnackedLimit, tc.TermWindowDur)
		cv.So(A.ExitErr, cv.ShouldResemble, &TerminatedError{Msg: msg})
		A.Stop()
	})
}
