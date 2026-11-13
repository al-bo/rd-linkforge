package cache_test

import (
	"fmt"
	"testing"

	"github.com/al-bo/rd-linkforge/pkg/cache"
)

func BenchmarkLRUGetHit(b *testing.B) {
	const capacity = 1024
	c, err := cache.NewLRU[string, int](capacity)
	if err != nil {
		b.Fatal(err)
	}
	keys := make([]string, capacity)
	for i := range keys {
		keys[i] = fmt.Sprintf("%06d", i)
		c.Put(keys[i], i)
	}

	// Cycling keys measures promotion as well as lookup; every access is a hit.
	index := 0
	b.ReportAllocs()
	for b.Loop() {
		if _, hit := c.Get(keys[index]); !hit {
			b.Fatal("expected a cache hit")
		}
		index = (index + 1) % capacity
	}
}
