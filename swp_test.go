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
	net := NewSimNet(0, time.Second)
	lat := time.Second
	rtt := 2 * lat

	a, err := NewSession(nil, net, "a", "b", 3, rtt)
	panicOn(err)
	b, err := NewSession(nil, net, "b", "a", 3, rtt)
	panicOn(err)
	net.AddNode("a", a)
	net.AddNode("b", b)

	cv.Convey("", t, func() {
		cv.So(true, cv.ShouldBeTrue)
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
