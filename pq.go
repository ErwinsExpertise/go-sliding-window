package swp

import (
	"container/heap"
	"fmt"
)

// priority queue for packet timeouts

// A PriorityQueue implements heap.Interface and holds TxqSlot(s)
// with the soonest RetryDeadline
// getting highest priority, and zero-time deadlines (disabled timeouts) sorting last.
type PriorityQueue struct {
	Slc []*TxqSlot
	Max int
	Idx map[Seqno]int
}

func NewPriorityQueue(n int64) *PriorityQueue {
	return &PriorityQueue{
		Idx: make(map[Seqno]int),
		Slc: make([]*TxqSlot, 0, n),
	}
}

func (pq PriorityQueue) Len() int { return len(pq.Slc) }

func (pq PriorityQueue) Less(i, j int) bool {
	// nil Packets sort to end as well
	if pq.Slc[i] == nil {
		return false
	}
	if pq.Slc[j] == nil {
		return true
	}

	// sort 0's to the end, no deadline.
	if pq.Slc[i].RetryDeadline.IsZero() {
		return false
	}
	if pq.Slc[j].RetryDeadline.IsZero() {
		return true
	}
	return pq.Slc[i].RetryDeadline.Before(pq.Slc[j].RetryDeadline)
}

func (pq PriorityQueue) Swap(i, j int) {
	// track location in Idx as well, for fast processing;
	// so we can find our Pack by SeqNum in the queue.
	n := len(pq.Slc) - 1
	p("n = %v", n)
	iseq := pq.Slc[i].Pack.SeqNum
	jseq := pq.Slc[j].Pack.SeqNum
	p("iseq = %v", iseq)
	p("jseq = %v", jseq)
	p("pq.Slc[i=%v] = %#v", i, pq.Slc[i].Pack)
	p("pq.Slc[j=%v] = %#v", j, pq.Slc[j].Pack)
	pq.Idx[iseq] = j
	pq.Idx[jseq] = i
	pq.Slc[i], pq.Slc[j] = pq.Slc[j], pq.Slc[i]
}

// Users: DO NOT CALL THIS. use heap.Push(pg, x) instead.
func (pq *PriorityQueue) Push(x interface{}) {
	item := x.(*TxqSlot)
	pq.Slc = append(pq.Slc, item)
}

// Users: DO NOT CALL THIS. use heap.Pop(pg) instead.
// This is only to meet the interface, it
// will not do what you expect.
func (pq *PriorityQueue) Pop() interface{} {
	old := pq.Slc
	n := len(old)
	item := old[n-1]
	delete(pq.Idx, item.Pack.SeqNum)
	pq.Slc = old[0 : n-1]
	return item
}

func (pq *PriorityQueue) Add(x *TxqSlot) {
	if x == nil {
		return
	}
	if x.Pack == nil {
		return
	}
	where, already := pq.Idx[x.Pack.SeqNum]
	if already {
		p("where = %v", where)
		dumppq(pq)
		p("already have that SeqNum %v at pos %v, updating deadline from %v -> %v",
			x.Pack.SeqNum, where, pq.Slc[where].RetryDeadline, x.RetryDeadline)
		pq.Slc[where].RetryDeadline = x.RetryDeadline
		heap.Fix(pq, where)
		return
	}
	p("seqnum %v not already found", x.Pack.SeqNum)
	n := len(pq.Slc)
	pq.Slc = append(pq.Slc, x)
	pq.Idx[x.Pack.SeqNum] = n
	heap.Fix(pq, n)

	if pq.Max > 0 && len(pq.Slc) > pq.Max {
		dumppq(pq)
		panic(fmt.Sprintf("over max!, trying to add %#v", x.Pack))
	}
}

func (pq *PriorityQueue) PopTop() *TxqSlot {
	p("at top of PopTop()")
	dumppq(pq)
	//	if pq.Slc[0] != nil && pq.Slc[0].Pack != nil {
	sn := pq.Slc[0].Pack.SeqNum
	delete(pq.Idx, sn)
	p("PopTop is deleting sn %v", sn)
	//	}
	top := heap.Pop(pq).(*TxqSlot)
	p("at bottom of PopTop()")
	dumppq(pq)
	return top
}

func (pq *PriorityQueue) RemoveByIndex(i int) {
	if pq.Slc[i] != nil && pq.Slc[i].Pack != nil {
		sn := pq.Slc[i].Pack.SeqNum
		delete(pq.Idx, sn)
		p("RemoveByIndex %v is deleting sn %v", i, sn)
	}
	heap.Remove(pq, i)
}

func (pq *PriorityQueue) RemoveBySeqNum(sn Seqno) {
	where, already := pq.Idx[sn]
	if already {
		heap.Remove(pq, where)
		p("RemoveBySeqNum is deleting sn %v", sn)
	}
	delete(pq.Idx, sn)
}

func dumppq(ppq *PriorityQueue) {
	pq := *ppq
	n := 0
	p("")
	for i := range pq.Slc {
		if pq.Slc[i] != nil && pq.Slc[i].Pack != nil {
			n++
			fmt.Printf("pq.Slc[%v] at  %v  seqnum: %v\n",
				i, pq.Slc[i].RetryDeadline, pq.Slc[i].Pack.SeqNum)
		} else {
			fmt.Printf("pq.Slc[%v] is %#v\n",
				i, pq.Slc[i])
		}
	}
	if n == 0 {
		fmt.Printf("pq is empty")
	}
	fmt.Printf("begin Idx ===============\n")
	for k, v := range pq.Idx {
		fmt.Printf("SeqNum %v -> position %v\n", k, v)
	}
	fmt.Printf("end Idx ===============\n")
}
