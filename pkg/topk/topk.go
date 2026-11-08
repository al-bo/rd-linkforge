package topk

import "errors"

var ErrInvalidN = errors.New("topk: n must be positive")

type Item struct {
	Key   string
	Count int64
}

type TopK struct { /* your fields */
}

// New returns ErrInvalidN when n is zero or negative.
func New(n int) (*TopK, error) {
	if n == 0 {
		return &TopK{}, ErrInvalidN
	}

	return &TopK{}, nil
}

// Add offers an item. Items with equal keys are not merged.
func (t *TopK) Add(key string, count int64) {

}

// Result returns the kept items, best first. It does not change t.
func (t *TopK) Result() []Item {
	var result []Item
	return result
}
