package swp

import (
	"container/heap"
	//"fmt"
	cv "github.com/glycerine/goconvey/convey"
	"testing"
	"time"
)

func Test005PriorityQueue(t *testing.T) {

	cv.Convey("given a priority queue of packet timeouts, the earliest timeout should sort first, and zero timeouts (cancelled) should sort last", t, func() {
		// Some items and their priorities.
		n := int64(10)
		txq := make([]*TxqSlot, n)
		for k := range txq {
			i := int64(k)
			txq[i] = &TxqSlot{
				RetryDeadline: time.Unix(10+(n-i)-1, 0),
				Pack: &Packet{
					SeqNum: Seqno(i),
				},
			}
		}

		pq := PriorityQueue(txq)
		heap.Init(&pq)

		// Insert a new item and then modify its priority.
		slot := &TxqSlot{
			RetryDeadline: time.Time{},
			Pack: &Packet{
				SeqNum: Seqno(99987),
			},
		}
		heap.Push(&pq, slot)
		/*
			fmt.Printf("\n\n")
			for i := range pq {
				fmt.Printf(" at: %v, seqnum: %v\n", i, pq[i].slot.Pack.SeqNum)
			}
		*/
		p("with zero time, the TxnEle should sort to the end of the priority queue")
		cv.So(pq[n].Pack.SeqNum, cv.ShouldEqual, 99987)

		p("and if we change that time to be non-zero and sooner than everyone else, we should sort first")
		pq[n].RetryDeadline = time.Unix(1, 0)
		heap.Fix(&pq, int(n))

		// Take the items out; they arrive in decreasing priority order.
		j := 0
		for pq.Len() > 0 {
			item := heap.Pop(&pq).(*TxqSlot)
			if j == 0 {
				cv.So(item.Pack.SeqNum, cv.ShouldEqual, 99987)
			}
			//fmt.Printf("%v: seqnum: %v\n", item.RetryDeadline, item.Pack.SeqNum)
			j++
		}
	})
}
