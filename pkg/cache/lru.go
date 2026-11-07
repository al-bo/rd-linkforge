package cache

import (
	"container/list"
	"errors"
)

var ErrInvalidCapacity = errors.New("cache: capacity must be positive")

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

func NewLRU[K comparable, V any](capacity int) (*LRU[K, V], error) {
	if capacity <= 0 {
		return &LRU[K, V]{}, ErrInvalidCapacity
	}

	return &LRU[K, V]{
		keyMap: make(map[K]*list.Element),
		size:   capacity,
	}, nil
}

func (c *LRU[K, V]) Get(key K) (V, bool) {
	var empty V
	v, ok := c.keyMap[key]

	if ok {
		c.list.MoveToFront(v)
		return v.Value.(entry[K, V]).value, true
	}

	return empty, false
}

func (c *LRU[K, V]) Put(key K, value V) {
	_, ok := c.keyMap[key]

	if ok {
		c.Delete(key)
	}

	element := c.list.PushFront(entry[K, V]{key: key, value: value})
	c.keyMap[key] = element

	if c.list.Len() > c.size {
		oldest := c.list.Back()
		oldKey := oldest.Value.(entry[K, V]).key
		c.Delete(oldKey)
	}
}
func (c *LRU[K, V]) Delete(key K) {
	v, ok := c.keyMap[key]
	if ok {
		c.list.Remove(v)
		delete(c.keyMap, key)
	}

}
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
