// Package topk retains the highest-ranked items from a stream.
package topk

import (
	"cmp"
	"container/heap"
	"errors"
	"slices"
)

// ErrInvalidN indicates that the requested capacity is not positive.
var ErrInvalidN = errors.New("topk: n must be positive")

// Item is a ranking entry: higher Count ranks first, then smaller Key.
// Concurrent mutation of an Item requires external synchronization.
type Item struct {
	Key   string
	Count int64
}

// TopK retains a bounded set of the highest-ranked stream items.
// It is not safe for concurrent use. Use New to initialize a TopK.
type TopK struct {
	n     int
	items itemHeap
}

// New creates a TopK with capacity n, or returns ErrInvalidN when n is not positive.
func New(n int) (*TopK, error) {
	if n <= 0 {
		return nil, ErrInvalidN
	}

	return &TopK{
		n:     n,
		items: make(itemHeap, 0, n),
	}, nil
}

// Add offers an item. Items with equal keys are not merged.
func (t *TopK) Add(key string, count int64) {
	item := Item{Key: key, Count: count}

	if len(t.items) < t.n {
		heap.Push(&t.items, item)
		return
	}

	if compareItems(item, t.items[0]) < 0 {
		heap.Pop(&t.items)
		heap.Push(&t.items, item)
	}
}

// Result returns the kept items, best first. It does not change t.
func (t *TopK) Result() []Item {
	result := make([]Item, len(t.items))

	copy(result, t.items)
	slices.SortFunc(result, compareItems)

	return result
}

func compareItems(a, b Item) int {
	if a.Count != b.Count {
		return cmp.Compare(b.Count, a.Count)
	}
	return cmp.Compare(a.Key, b.Key)
}
