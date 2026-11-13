package cache

import (
	"container/list"
	"errors"
)

// ErrInvalidCapacity indicates that the requested capacity is not positive.
var ErrInvalidCapacity = errors.New("cache: capacity must be positive")

// LRU is a bounded least-recently-used cache. It is not safe for concurrent use.
// Use NewLRU to initialize an LRU.
type LRU[K comparable, V any] struct {
	list   list.List
	keyMap map[K]*list.Element
	size   int
}

type entry[K comparable, V any] struct {
	key   K
	value V
}

var _ Cache[string, int] = (*LRU[string, int])(nil)

// NewLRU returns ErrInvalidCapacity when capacity is zero or negative.
func NewLRU[K comparable, V any](capacity int) (*LRU[K, V], error) {
	if capacity <= 0 {
		return &LRU[K, V]{}, ErrInvalidCapacity
	}

	return &LRU[K, V]{
		keyMap: make(map[K]*list.Element),
		size:   capacity,
	}, nil
}

// Get marks a hit as most recently used. A miss returns the zero value and false.
func (c *LRU[K, V]) Get(key K) (V, bool) {
	var empty V
	v, ok := c.keyMap[key]

	if ok {
		c.list.MoveToFront(v)
		return v.Value.(entry[K, V]).value, true
	}

	return empty, false
}

// Put adds or updates key and marks it as most recently used.
// At capacity, it evicts the least recently used entry before adding a new key.
func (c *LRU[K, V]) Put(key K, value V) {
	if element, ok := c.keyMap[key]; ok {
		element.Value = entry[K, V]{key: key, value: value}
		c.list.MoveToFront(element)
		return
	}

	if c.list.Len() == c.size {
		oldest := c.list.Back()
		oldKey := oldest.Value.(entry[K, V]).key
		c.Delete(oldKey)
	}

	element := c.list.PushFront(entry[K, V]{key: key, value: value})
	c.keyMap[key] = element
}

// Delete removes key if present; a missing key has no effect.
func (c *LRU[K, V]) Delete(key K) {
	v, ok := c.keyMap[key]
	if ok {
		c.list.Remove(v)
		delete(c.keyMap, key)
	}
}

// Len returns the number of stored entries.
func (c *LRU[K, V]) Len() int {
	return len(c.keyMap)
}

// Keys returns keys from most to least recently used without changing their order.
func (c *LRU[K, V]) Keys() []K {
	keys := make([]K, 0, c.list.Len())
	for element := c.list.Front(); element != nil; element = element.Next() {
		keys = append(keys, element.Value.(entry[K, V]).key)
	}
	return keys
}
