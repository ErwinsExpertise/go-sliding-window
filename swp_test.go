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
	//p2 := &Packet{Data: []byte("two")}

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

func Test002ProbabilityGen(t *testing.T) {

	cv.Convey("cryptoProb should generate reasonably balanced random probabilities - manually confirmed in R", t, func() {
		/*
			pr := make([]float64, 0)
			fn := "random.number.check"
			f, err := os.Create(fn)
			panicOn(err)
			for i := 1; i < 100000; i++ {
				cp := cryptoProb()
				pr = append(pr, cp)
				fmt.Fprintf(f, "%v\n", cp)
			}
			f.Close()
			// manually check in R
			cv.So(true, cv.ShouldBeTrue)
			//os.Remove(fn)
		*/
	})
}
