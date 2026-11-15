package topk

import "container/heap"

// The worst retained item stays at the root so Add can evict it.
type itemHeap []Item

var _ heap.Interface = (*itemHeap)(nil)

func (h itemHeap) Len() int {
	return len(h)
}

func (h itemHeap) Less(i, j int) bool {
	return compareItems(h[i], h[j]) > 0
}

func (h itemHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *itemHeap) Push(value any) {
	*h = append(*h, value.(Item))
}

func (h *itemHeap) Pop() any {
	last := len(*h) - 1
	item := (*h)[last]

	(*h)[last] = Item{}
	*h = (*h)[:last]

	return item
}
