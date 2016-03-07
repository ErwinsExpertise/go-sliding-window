package swp

import (
	"sync"
)

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

	// flow control params
	// current: use these to advertise; kept
	// up to date as conditions change.
	// These already have Reserved capcities above
	// subtracted from them, so they are
	// safe to ack to sender.
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
// flow information from the underlying
// (nats) network. It returns the latest
// info in the Flow structure.
func (r *FlowCtrl) UpdateFlow(who string, net Network) Flow {
	r.mut.Lock()
	defer r.mut.Unlock()

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
	//p("%v end of UpdateFlow(), FlowCtrl.flow = '%#v'", who, r.flow)
	cp := r.flow
	return cp
}
