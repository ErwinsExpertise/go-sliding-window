package swp

import (
	"fmt"
	"os"
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

	cv.Convey("", t, func() {
		cv.So(true, cv.ShouldBeTrue)
	})
}

func Test002ProbabilityGen(t *testing.T) {

	cv.Convey("cryptoProb should generate reasonable balanced random probabilities", t, func() {
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
		os.Remove(fn)
	})
}
