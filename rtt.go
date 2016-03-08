package swp

import (
	"time"
)

// RTT provides round-trip time estimation.
// Currently it is implemented as a simple single
// exponential moving average with alpha = 0.1
// and no seasonal/cyclic terms.
type RTT struct {
	Est   float64
	Alpha float64
	N     int64
}

// NewRTT makes a new RTT.
func NewRTT() *RTT {
	return &RTT{
		Alpha: 0.1,
	}
}

// GetEstimate returns the current estimate.
func (r *RTT) GetEstimate() time.Duration {
	return time.Duration(int64(r.Est))
}

// AddSample adds a new RTT sample to the
// estimate.
func (r *RTT) AddSample(newSample time.Duration) {
	r.N++
	cur := float64(newSample)
	if r.N == 1 {
		r.Est = cur
		return
	}
	r.Est = r.Alpha*cur + (1.0-r.Alpha)*r.Est
}
