package swp

import (
	"fmt"
)

// Portions copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

var dummyPack = &Packet{
	SeqNum: -111999,
}

var nilSlot = &TxqSlot{
	Pack: dummyPack,
}

func init() {
	// far, far in the future
	nilSlot.RetryDeadline, _ = time.Parse(RFC3339, "3333-01-02T15:04:05Z07:00")
}

type PQ struct {
	Slc []*TxqSlot
	Max int64
	Idx map[Seqno]int
}

func NewPQ(n int64) *PQ {
	s := &PQ{
		Idx: make(map[Seqno]int),
		Slc: make([]*TxqSlot, n),
		Max: n,
	}
	for i := range s.Slc {
		s.Slc[i] = nilSlot
	}
	return s
}

func (s *PQ) Len() int {
	return len(s.Slc)
}

func (pq PQ) less(i, j int) bool {
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

func (pq *PQ) swap(i, j int) {
	// track location in Idx as well, for fast processing;
	// so we can find our Pack by SeqNum in the queue.
	//n := len(pq.Slc) - 1
	//p("n = %v", n)
	iseq := pq.Slc[i].Pack.SeqNum
	jseq := pq.Slc[j].Pack.SeqNum
	//	p("iseq = %v", iseq)
	//	p("jseq = %v", jseq)
	//	p("pq.Slc[i=%v] = %#v", i, pq.Slc[i].Pack)
	//	p("pq.Slc[j=%v] = %#v", j, pq.Slc[j].Pack)
	pq.Idx[iseq] = j
	pq.Idx[jseq] = i
	pq.Slc[i], pq.Slc[j] = pq.Slc[j], pq.Slc[i]
}

func (s *PQ) addToEnd(x *TxqSlot) {
	s.Slc = append(s.Slc, x)
	s.Idx[x.Pack.SeqNum] = len(s.Slc) - 1
}

func (s *PQ) readFromEnd() *TxqSlot {
	old := s.Slc
	n := len(old)
	item := old[n-1]
	delete(s.Idx, item.Pack.SeqNum)
	s.Slc = old[0 : n-1]
	return item
}

func (s *PQ) Add(x *TxqSlot) {
	if x == nil {
		return
	}
	if x.Pack == nil {
		return
	}
	where, already := s.Idx[x.Pack.SeqNum]
	if already {
		p("where = %v", where)
		dumppq2(s)
		p("already have that SeqNum %v at pos %v, updating deadline from %v -> %v",
			x.Pack.SeqNum, where, s.Slc[where].RetryDeadline, x.RetryDeadline)
		s.Slc[where].RetryDeadline = x.RetryDeadline
		s.fix(where)
		return
	}
	p("seqnum %v not already found in PQ", x.Pack.SeqNum)
	s.Slc = append(s.Slc, x)
	s.Idx[x.Pack.SeqNum] = len(s.Slc) - 1
	s.fix(len(s.Slc) - 1)

	if s.Max > 0 && int64(len(s.Slc)) > s.Max {
		p("we are over Max = %v, here is the PQ, len %v", s.Max, len(s.Slc))
		dumppq2(s)
		panic(fmt.Sprintf("over max!, after adding %#v", x.Pack))
	}
}

func (s *PQ) PopTop() *TxqSlot {
	//p("at top of PopTop()")
	//dumppq(p)
	sn := s.Slc[0].Pack.SeqNum
	delete(s.Idx, sn)
	//p("PopTop is deleting sn %v", sn)
	top := s.pop2()
	//p("at bottom of PopTop()")
	//dumppq(p)
	return top
}

func (s *PQ) removeByIndex(i int) {
	if s.Slc[i] != nil && s.Slc[i].Pack != nil {
		sn := s.Slc[i].Pack.SeqNum
		delete(s.Idx, sn)
		p("removeByIndex %v is deleting sn %v", i, sn)
	}
	s.remove(i)
}

func (s *PQ) RemoveBySeqNum(sn Seqno) {
	where, already := s.Idx[sn]
	if already {
		s.remove(where)
		p("removeBySeqNum is deleting sn %v", sn)
	}
	delete(s.Idx, sn)
}

func dumppq2(ppq *PQ) {
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

// A heap must be initialized before any of the heap operations
// can be used. Init is idempotent with respect to the heap invariants
// and may be called whenever the heap invariants may have been invalidated.
// Its complexity is O(n) where n = len(s.Slc).
//
func (s *PQ) InitHeap() {
	n := len(s.Slc)
	for i := n/2 - 1; i >= 0; i-- {
		s.down(i, n)
	}
}

// Push pushes the element x onto the heas. The complexity is
// O(log(n)) where n = len(s.Slc).
//
func (s *PQ) Push(x *TxqSlot) {
	s.addToEnd(x)
	s.up(len(s.Slc) - 1)
}

// pop2 removes the minimum element (according to less) from the heap
// and returns it. The complexity is O(log(n)) where n = len(s.Slc).
// It is equivalent to remove(h, 0).
//
func (s *PQ) pop2() *TxqSlot {
	n := len(s.Slc) - 1
	s.swap(0, n)
	s.down(0, n)
	return s.readFromEnd()
}

// remove removes the element at index i from the heas.
// The complexity is O(log(n)) where n = len(s.Slc).
//
func (s *PQ) remove(i int) *TxqSlot {
	n := len(s.Slc) - 1
	if n != i {
		s.swap(i, n)
		s.down(i, n)
		s.up(i)
	}
	return s.readFromEnd()
}

// fix re-establishes the heap ordering after the element at index i has changed its value.
// Changing the value of the element at index i and then calling fix is equivalent to,
// but less expensive than, calling remove(h, i) followed by a Push of the new value.
// The complexity is O(log(n)) where n = len(s.Slc).
func (s *PQ) fix(i int) {
	s.down(i, len(s.Slc))
	s.up(i)
}

func (s *PQ) up(j int) {
	for {
		i := (j - 1) / 2 // parent
		if i == j || !s.less(j, i) {
			break
		}
		s.swap(i, j)
		j = i
	}
}

func (s *PQ) down(i, n int) {
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 { // j1 < 0 after int overflow
			break
		}
		j := j1 // left child
		if j2 := j1 + 1; j2 < n && !s.less(j1, j2) {
			j = j2 // = 2*i + 2  // right child
		}
		if !s.less(j, i) {
			break
		}
		s.swap(i, j)
		i = j
	}
}
