package cache_test

import (
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/al-bo/rd-linkforge/pkg/cache"
)

func TestNewLRUInvalidCapacity(t *testing.T) {
	for _, capacity := range []int{-10, -1, 0} {
		t.Run(strconv.Itoa(capacity), func(t *testing.T) {
			if _, err := cache.NewLRU[string, int](capacity); !errors.Is(err, cache.ErrInvalidCapacity) {
				t.Fatalf("NewLRU(%d) error = %v, want ErrInvalidCapacity", capacity, err)
			}
		})
	}
}

func TestLRUEmpty(t *testing.T) {
	c := newTestLRU(t, 3)
	requireState(t, c)
	requireGet(t, c, "missing", 0, false)
	c.Delete("missing")
	requireState(t, c)
}

func TestLRUGetUpdatesRecency(t *testing.T) {
	c := newTestLRU(t, 3)
	c.Put("a", 1)
	requireState(t, c, "a")
	c.Put("b", 2)
	requireState(t, c, "b", "a")
	c.Put("c", 3)
	requireState(t, c, "c", "b", "a")

	requireGet(t, c, "a", 1, true)
	requireState(t, c, "a", "c", "b")
	requireGet(t, c, "c", 3, true)
	requireState(t, c, "c", "a", "b")
	requireGet(t, c, "c", 3, true)
	requireState(t, c, "c", "a", "b")

	requireGet(t, c, "missing", 0, false)
	requireState(t, c, "c", "a", "b")

	c.Put("d", 4)
	requireState(t, c, "d", "c", "a")
	requireGet(t, c, "b", 0, false)
	requireGet(t, c, "d", 4, true)

	c.Put("e", 5)
	requireState(t, c, "e", "d", "c")
	requireGet(t, c, "a", 0, false)
	requireGet(t, c, "c", 3, true)
}

func TestLRUPutUpdatesExistingKey(t *testing.T) {
	c := newTestLRU(t, 2)
	c.Put("a", 1)
	c.Put("b", 2)

	c.Put("a", 10)
	requireState(t, c, "a", "b")

	// An update at capacity must preserve the other entry and refresh recency.
	c.Put("c", 3)
	requireState(t, c, "c", "a")
	requireGet(t, c, "b", 0, false)
	requireGet(t, c, "a", 10, true)

	c.Put("a", 0)
	requireGet(t, c, "a", 0, true)
	requireState(t, c, "a", "c")
	requireGet(t, c, "c", 3, true)
}

func TestLRUDelete(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want []string
	}{
		{name: "most recent", key: "c", want: []string{"b", "a"}},
		{name: "middle", key: "b", want: []string{"c", "a"}},
		{name: "least recent", key: "a", want: []string{"c", "b"}},
		{name: "missing", key: "missing", want: []string{"c", "b", "a"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestLRU(t, 3)
			c.Put("a", 1)
			c.Put("b", 2)
			c.Put("c", 3)

			c.Delete(tc.key)
			requireState(t, c, tc.want...)
			requireGet(t, c, tc.key, 0, false)
			c.Delete(tc.key)
			requireState(t, c, tc.want...)

			for _, key := range tc.want {
				requireGet(t, c, key, int(key[0]-'a')+1, true)
			}
		})
	}
}

func TestLRUDeleteFreesCapacity(t *testing.T) {
	c := newTestLRU(t, 2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Delete("b")
	c.Put("c", 3)
	requireState(t, c, "c", "a")
	requireGet(t, c, "a", 1, true)

	c.Delete("a")
	c.Delete("c")
	requireState(t, c)

	c.Put("a", 10)
	c.Put("b", 20)
	requireState(t, c, "b", "a")
	requireGet(t, c, "a", 10, true)
	requireGet(t, c, "b", 20, true)
}

func TestLRUCapacityOne(t *testing.T) {
	c := newTestLRU(t, 1)
	c.Put("a", 1)
	requireState(t, c, "a")
	c.Put("a", 2)
	requireState(t, c, "a")
	requireGet(t, c, "a", 2, true)

	c.Put("b", 3)
	requireState(t, c, "b")
	requireGet(t, c, "a", 0, false)
	requireGet(t, c, "b", 3, true)

	c.Put("a", 4)
	requireState(t, c, "a")
	requireGet(t, c, "b", 0, false)
	requireGet(t, c, "a", 4, true)
	c.Delete("a")
	requireState(t, c)
	c.Put("c", 5)
	requireGet(t, c, "c", 5, true)
	requireState(t, c, "c")
}

func TestLRUZeroKeyAndValue(t *testing.T) {
	c := newTestLRU(t, 2)
	c.Put("", 0)
	requireGet(t, c, "", 0, true)
	c.Put("a", 1)
	requireState(t, c, "a", "")
	c.Delete("")
	requireGet(t, c, "", 0, false)
	requireState(t, c, "a")

	c.Put("", 0)
	c.Put("b", 2)
	c.Put("c", 3)
	requireGet(t, c, "", 0, false)
	requireState(t, c, "c", "b")
}

func TestLRUKeysPreservesRecency(t *testing.T) {
	c := newTestLRU(t, 3)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)

	requireState(t, c, "c", "b", "a")
	requireState(t, c, "c", "b", "a")
	c.Put("d", 4)
	requireState(t, c, "d", "c", "b")
	requireGet(t, c, "a", 0, false)
}

func TestLRUGenericTypes(t *testing.T) {
	type key struct{ ID int }
	c, err := cache.NewLRU[key, []string](2)
	if err != nil {
		t.Fatalf("NewLRU: %v", err)
	}
	if c == nil {
		t.Fatal("NewLRU returned nil without an error")
	}

	k := key{ID: 42}
	want := []string{"first", "second"}
	c.Put(k, want)
	if got, ok := c.Get(k); !ok || !slices.Equal(got, want) {
		t.Fatalf("Get(%v) = %v, %t; want %v, true", k, got, ok, want)
	}
	c.Put(key{}, nil)
	if got, ok := c.Get(key{}); !ok || got != nil {
		t.Fatalf("Get(zero key) = %v, %t; want nil, true", got, ok)
	}
	if got := c.Keys(); !slices.Equal(got, []key{{}, k}) {
		t.Fatalf("Keys() = %v, want [%v %v]", got, key{}, k)
	}

	c.Delete(k)
	if got, ok := c.Get(k); ok || got != nil {
		t.Fatalf("Get(deleted key) = %v, %t; want nil, false", got, ok)
	}
}

func newTestLRU(t *testing.T, capacity int) *cache.LRU[string, int] {
	t.Helper()
	c, err := cache.NewLRU[string, int](capacity)
	if err != nil {
		t.Fatalf("NewLRU(%d): %v", capacity, err)
	}
	if c == nil {
		t.Fatal("NewLRU returned nil without an error")
	}
	return c
}

func requireState(t *testing.T, c *cache.LRU[string, int], wantKeys ...string) {
	t.Helper()
	if got := c.Len(); got != len(wantKeys) {
		t.Fatalf("Len() = %d, want %d", got, len(wantKeys))
	}
	if got := c.Keys(); !slices.Equal(got, wantKeys) {
		t.Fatalf("Keys() = %v, want %v", got, wantKeys)
	}
}

func requireGet(t *testing.T, c *cache.LRU[string, int], key string, want int, wantOK bool) {
	t.Helper()
	if got, ok := c.Get(key); got != want || ok != wantOK {
		t.Fatalf("Get(%q) = %d, %t; want %d, %t", key, got, ok, want, wantOK)
	}
}
