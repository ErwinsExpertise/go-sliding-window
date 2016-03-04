package swp

import (
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

// check for:
//
// packet loss - eventually received and/or re-requested.
// duplicate receives - discard the 2nd one.
// out of order receives - reorder correctly.
//
func Test001(t *testing.T) {

	cv.Convey("", t, func() {
		cv.So(true, cv.ShouldBeTrue)
	})
}
