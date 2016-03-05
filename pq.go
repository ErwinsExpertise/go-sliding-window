package swp

import (
	"container/heap"
)

// priority queue for packet timeouts

// A PqEle is tracks TxqSlots in a priority queue, with the soonest RetryDeadline
// getting highest priority, and zero-time deadlines (disabled timeouts) sorting last.
type PqEle struct {
	slot *TxqSlot

	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
}

// A PriorityQueue implements heap.Interface and holds PqEles.
type PriorityQueue []*PqEle

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// sort 0's to the end, no deadline.
	if pq[i].slot.RetryDeadline.IsZero() {
		return false
	}
	if pq[j].slot.RetryDeadline.IsZero() {
		return true
	}
	return pq[i].slot.RetryDeadline.Before(pq[j].slot.RetryDeadline)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*PqEle)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// update modifies the priority and slot of an PqEle in the queue.
func (pq *PriorityQueue) update(item *PqEle, slot *TxqSlot) {
	item.slot = slot
	heap.Fix(pq, item.index)
}
