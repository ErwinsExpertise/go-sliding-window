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
	lat := time.Second
	net := NewSimNet(lossProb, lat)
	rtt := 2 * lat

	A, err := NewSession(nil, net, "A", "B", 3, rtt)
	panicOn(err)
	B, err := NewSession(nil, net, "B", "A", 3, rtt)
	panicOn(err)
	net.AddNode("A", A)
	net.AddNode("B", B)

	data1 := []byte("one")
	//data2 := []byte("two")

	err = A.Push(data1)
	panicOn(err)

	cv.Convey("Given two nodes A and B, sending a packet on a non-lossy network from A to B, the packet should arrive at B", t, func() {
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
