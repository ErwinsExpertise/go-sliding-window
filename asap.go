package swp

import (
	"sync"
)

// AsapHelper is a simple queue
// goroutine that delivers packets
// to ASAP clients as soon as they
// become avaialable (duplicating
// the in order delivery).
//
type AsapHelper struct {
	ReqStop chan bool
	Done    chan bool

	rcv     chan *Packet
	enqueue chan *Packet
	mut     sync.Mutex
	q       []*Packet
}

// NewAsapHelper creates a new AsapHelper.
// Callers provide rcvUnordered which which they
// should then do blocking receives on to
// aquire new *Packets out of order but As
// Soon As Possible.
func NewAsapHelper(rcvUnordered chan *Packet) *AsapHelper {
	return &AsapHelper{
		ReqStop: make(chan bool),
		Done:    make(chan bool),
		rcv:     rcvUnordered,
		enqueue: make(chan *Packet),
	}
}

// Stop shuts down the AsapHelper goroutine.
func (r *AsapHelper) Stop() {
	r.mut.Lock()
	select {
	case <-r.ReqStop:
	default:
		close(r.ReqStop)
	}
	r.mut.Unlock()
	<-r.Done
}

// Start starts the AsapHelper tiny queuing service.
func (r *AsapHelper) Start() {
	go func() {
		var rch chan *Packet
		var next *Packet
		for {
			if next == nil {
				if len(r.q) > 0 {
					next = r.q[0]
					r.q = r.q[1:]
					rch = r.rcv
				} else {
					rch = nil
				}
			} else {
				rch = r.rcv
			}

			select {
			case rch <- next:
				next = nil
			case pack := <-r.enqueue:
				r.q = append(r.q, pack)
			case <-r.ReqStop:
				close(r.Done)
				return
			}
		}
	}()
}
