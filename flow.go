package swp

import (
	"sync"
)

// FlowCtrl serializes access to the Flow state information
// so that sender and receiver don't trample reads/writes.
type FlowCtrl struct {
	mut  sync.Mutex
	flow Flow
}

// FlowCtrl data is shared by sender and receiver,
// so use the sender.FlowCt.UpdateFlow() method to safely serialize
// access.
type Flow struct {
	// Control messages such as acks and keepalives
	// should not be blocked by flow-control (for
	// correctness/resumption from no-flow), so we need
	// to reserve extra headroom in the nats
	// subscription limits of this much to
	// allow resumption of flow.
	//
	// These reserved headroom settings can be
	// manually made larger before calling Start()
	//  -- and might need to be if you are running
	// very large windowMsgSz and/or windowByteSz; or
	// if you have large messages. After Start()
	// the nats buffer sizes on the subscription are
	// fixed and ReservedByteCap and ReservedMsgCap
	// are not consulted again.
	ReservedByteCap int64
	ReservedMsgCap  int64

	// flow control params:
	// These to advertised to senders with
	// both acks and data segments, and kept
	// up to date as conditions change.
	AvailReaderBytesCap int64
	AvailReaderMsgCap   int64

	// Estimate of the round-trip-time from
	// the senders point of view. In nanoseconds.
	FromRttEstNsec int64

	// Estimate of the standard deviation of
	// the round-trip-time from the senders
	// point of view. In nanoseconds.
	FromRttSdNsec int64

	// number of RTT observations in the From RTT
	// estimates above, also avoids double counting.
	FromRttN int64
}

// GetFlow atomically returns a copy
// of the current Flow; it does not
// itself call UpdateFlow, but one
// should have done so recently to
// get the most up-to-date info
func (r *FlowCtrl) GetFlow() Flow {
	r.mut.Lock()
	defer r.mut.Unlock()
	cp := r.flow
	return cp
}

// UpdateFlow updates the
// flow information.
// It returns the latest
// info in the Flow structure.
//
// NB: availReaderMsgCap is ignored if < 0, so
// use -1 to indicate no update (just query existing values).
// Same with availReaderBytesCap.
func (r *FlowCtrl) UpdateFlow(who string, net Network,
	availReaderMsgCap int64, availReaderBytesCap int64,
	pack *Packet) Flow {

	r.mut.Lock()
	defer r.mut.Unlock()
	if availReaderMsgCap >= 0 {
		r.flow.AvailReaderMsgCap = availReaderMsgCap
	}
	if availReaderBytesCap >= 0 {
		r.flow.AvailReaderBytesCap = availReaderBytesCap
	}
	if pack != nil && pack.FromRttN > r.flow.FromRttN {
		r.flow.FromRttEstNsec = pack.FromRttEstNsec
		r.flow.FromRttSdNsec = pack.FromRttSdNsec
		r.flow.FromRttN = pack.FromRttN
	}
	cp := r.flow
	return cp
}
