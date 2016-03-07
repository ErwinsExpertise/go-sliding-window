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
	// flow control params
	ReservedByteCap int64
	ReservedMsgCap  int64

	// flow control params:
	// These to advertised to senders with
	// both acks and data segments, and kept
	// up to date as conditions change.
	AvailReaderBytesCap int64
	AvailReaderMsgCap   int64
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
	availReaderMsgCap int64, availReaderBytesCap int64) Flow {

	r.mut.Lock()
	defer r.mut.Unlock()
	if availReaderMsgCap >= 0 {
		r.flow.AvailReaderMsgCap = availReaderMsgCap
	}
	if availReaderBytesCap >= 0 {
		r.flow.AvailReaderBytesCap = availReaderBytesCap
	}
	/*
		// ask nats/sim for current consumption
		// of internal client buffers.
		blim, mlim := net.BufferCaps()
		//p("%v UpdateFlow sees blim = %v,  mlim = %v", who, blim, mlim)
		r.flow.AvailReaderBytesCap = blim - r.flow.ReservedByteCap
		r.flow.AvailReaderMsgCap = mlim - r.flow.ReservedMsgCap

		if r.flow.AvailReaderBytesCap < 0 {
			r.flow.AvailReaderBytesCap = 0
		}
		if r.flow.AvailReaderMsgCap < 0 {
			r.flow.AvailReaderMsgCap = 0
		}
		p("%v end of UpdateFlow(), FlowCtrl.flow = '%#v'  b/c blim=%v  mlim=%v", who, r.flow, blim, mlim)
	*/
	cp := r.flow
	return cp
}
