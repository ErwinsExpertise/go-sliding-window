package swp

// priority queue for packet timeouts

// A PriorityQueue implements heap.Interface and holds TxqSlot(s)
// with the soonest RetryDeadline
// getting highest priority, and zero-time deadlines (disabled timeouts) sorting last.
type PriorityQueue []*TxqSlot

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// nil Packets sort to end as well
	if pq[i] == nil {
		return false
	}
	if pq[j] == nil {
		return true
	}

	// sort 0's to the end, no deadline.
	if pq[i].RetryDeadline.IsZero() {
		return false
	}
	if pq[j].RetryDeadline.IsZero() {
		return true
	}
	return pq[i].RetryDeadline.Before(pq[j].RetryDeadline)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PriorityQueue) Push(x interface{}) {
	item := x.(*TxqSlot)
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}
