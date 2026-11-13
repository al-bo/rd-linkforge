package link

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/al-bo/rd-linkforge/pkg/cache"
)

func TestWithCacheResolveHit(t *testing.T) {
	want := Link{ID: 1, Code: "abc", Target: "https://example.com", OwnerID: "owner"}
	st := newCacheTestStore(t)
	st.lookupErr = errors.New("store must not be consulted")
	c := newServiceCacheSpy(t, want)
	svc := NewService(st, WithCache(c))

	requireCachedResolve(t, svc, want)
	if st.reads != 0 || c.gets != 1 || c.puts != 0 || c.deletes != 0 {
		t.Fatalf("store reads=%d; cache gets/puts/deletes=%d/%d/%d", st.reads, c.gets, c.puts, c.deletes)
	}
}

func TestWithCacheResolveMissFillsCache(t *testing.T) {
	want := Link{ID: 42, Code: "abc", Target: "https://example.com", OwnerID: "owner"}
	st := newCacheTestStore(t, want)
	c := newServiceCacheSpy(t)
	svc := NewService(st, WithCache(c))

	requireCachedResolve(t, svc, want)
	if got, ok := c.items[want.Code]; !ok || got != want {
		t.Fatalf("cached link = %+v, present=%t; want %+v", got, ok, want)
	}
	st.lookupErr = errors.New("second resolve must use cache")
	requireCachedResolve(t, svc, want)
	if st.reads != 1 || c.gets != 2 || c.puts != 1 || c.deletes != 0 {
		t.Fatalf("store reads=%d; cache gets/puts/deletes=%d/%d/%d", st.reads, c.gets, c.puts, c.deletes)
	}
}

func TestResolveWithoutCache(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
	}{
		{name: "no option"},
		{name: "explicit nil", opts: []Option{WithCache(nil)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			want := Link{ID: 1, Code: "abc", Target: "https://example.com"}
			st := newCacheTestStore(t, want)
			svc := NewService(st, tc.opts...)
			requireCachedResolve(t, svc, want)
			requireCachedResolve(t, svc, want)
			if st.reads != 2 {
				t.Fatalf("store reads=%d, want 2 without cache", st.reads)
			}
			if got, err := svc.Resolve(context.Background(), "unknown"); !errors.Is(err, ErrNotFound) || got != (Link{}) {
				t.Fatalf("unknown code: Resolve() = %+v, %v; want zero Link and ErrNotFound", got, err)
			}
		})
	}
}

func TestWithCacheResolveErrorsAreNotCached(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "unknown code", err: ErrNotFound},
		{name: "store failure", err: errors.New("store unavailable")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			want := Link{ID: 1, Code: "abc", Target: "https://example.com"}
			st := newCacheTestStore(t, want)
			st.lookupErr = tc.err
			c := newServiceCacheSpy(t)
			svc := NewService(st, WithCache(c))

			for i := 0; i < 2; i++ {
				got, err := svc.Resolve(context.Background(), want.Code)
				if !errors.Is(err, tc.err) || got != (Link{}) {
					t.Fatalf("Resolve() = %+v, %v; want zero Link and %v", got, err, tc.err)
				}
			}
			if st.reads != 2 || c.gets != 2 || c.puts != 0 || c.deletes != 0 || len(c.items) != 0 {
				t.Fatalf("store reads=%d; cache gets/puts/deletes=%d/%d/%d, entries=%d", st.reads, c.gets, c.puts, c.deletes, len(c.items))
			}

			st.lookupErr = nil
			requireCachedResolve(t, svc, want)
			if st.reads != 3 || c.puts != 1 || c.items[want.Code] != want {
				t.Fatal("successful retry did not populate cache")
			}
		})
	}
}

func TestWithCacheResolveExpiry(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	want := Link{ID: 1, Code: "abc", Target: "https://example.com", ExpiresAt: now.Add(time.Hour)}
	st := newCacheTestStore(t, want)
	c := newServiceCacheSpy(t)
	svc := NewService(st, WithCache(c), WithClock(func() time.Time { return now }))

	requireCachedResolve(t, svc, want)
	now = want.ExpiresAt.Add(-time.Nanosecond)
	requireCachedResolve(t, svc, want)
	for _, instant := range []time.Time{want.ExpiresAt, want.ExpiresAt.Add(time.Hour)} {
		now = instant
		got, err := svc.Resolve(context.Background(), want.Code)
		if !errors.Is(err, ErrExpired) || got != (Link{}) {
			t.Fatalf("at %v: Resolve() = %+v, %v; want zero Link and ErrExpired", instant, got, err)
		}
	}
	if st.reads != 1 || c.gets != 4 || c.puts != 1 {
		t.Fatalf("store reads=%d; cache gets/puts=%d/%d", st.reads, c.gets, c.puts)
	}
}

func TestWithCacheExpiredStoreHitIsCached(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	want := Link{ID: 1, Code: "abc", ExpiresAt: now}
	st := newCacheTestStore(t, want)
	c := newServiceCacheSpy(t)
	svc := NewService(st, WithCache(c), WithClock(func() time.Time { return now }))

	for i := 0; i < 2; i++ {
		if _, err := svc.Resolve(context.Background(), want.Code); !errors.Is(err, ErrExpired) {
			t.Fatalf("Resolve() error = %v, want ErrExpired", err)
		}
	}
	// Expiry is checked after a successful store lookup has filled the cache.
	if st.reads != 1 || c.puts != 1 || c.items[want.Code] != want {
		t.Fatalf("store reads=%d, cache puts=%d, cached link=%+v", st.reads, c.puts, c.items[want.Code])
	}
}

func TestWithCacheContextCheckedFirst(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		name := "canceled"
		if deadline {
			name = "deadline exceeded"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if deadline {
				ctx, cancel = context.WithDeadline(context.Background(), time.Unix(1, 0))
				defer cancel()
			}
			want := Link{ID: 1, Code: "abc", OwnerID: "owner"}
			st := newCacheTestStore(t, want)
			c := newServiceCacheSpy(t, want)
			svc := NewService(st, WithCache(c))

			if got, err := svc.Resolve(ctx, want.Code); !errors.Is(err, ctx.Err()) || got != (Link{}) {
				t.Fatalf("Resolve() = %+v, %v; want zero Link and %v", got, err, ctx.Err())
			}
			if err := svc.Delete(ctx, want.OwnerID, want.Code); !errors.Is(err, ctx.Err()) {
				t.Fatalf("Delete() error = %v, want %v", err, ctx.Err())
			}
			if st.reads != 0 || st.deletes != 0 || c.gets != 0 || c.puts != 0 || c.deletes != 0 {
				t.Fatal("stopped context reached the store or cache")
			}
			if c.items[want.Code] != want {
				t.Fatal("stopped context changed cached link")
			}
		})
	}
}

func TestWithCacheDelete(t *testing.T) {
	boom := errors.New("store failure")
	tests := []struct {
		name      string
		owner     string
		lookupErr error
		deleteErr error
		wantErr   error
		events    []string
	}{
		{name: "empty owner", wantErr: ErrEmptyOwner},
		{name: "unknown code", owner: "owner", lookupErr: ErrNotFound, wantErr: ErrNotFound, events: []string{"store.ByCode"}},
		{name: "lookup failure", owner: "owner", lookupErr: boom, wantErr: boom, events: []string{"store.ByCode"}},
		{name: "foreign owner", owner: "intruder", wantErr: ErrForbidden, events: []string{"store.ByCode"}},
		{name: "delete failure", owner: "owner", deleteErr: boom, wantErr: boom, events: []string{"store.ByCode", "store.Delete"}},
		{name: "success", owner: "owner", events: []string{"store.ByCode", "store.Delete", "cache.Delete"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := Link{ID: 42, Code: "abc", OwnerID: "owner"}
			st := newCacheTestStore(t, item)
			st.lookupErr, st.deleteErr = tc.lookupErr, tc.deleteErr
			// Ownership must come from the store even when cached data disagrees.
			cached := item
			cached.OwnerID = "intruder"
			c := newServiceCacheSpy(t, cached)
			var events []string
			st.events, c.events = &events, &events
			svc := NewService(st, WithCache(c))

			if err := svc.Delete(context.Background(), tc.owner, item.Code); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Delete() error = %v, want %v", err, tc.wantErr)
			}
			if !slices.Equal(events, tc.events) {
				t.Fatalf("calls = %v, want %v", events, tc.events)
			}
			if c.gets != 0 || c.puts != 0 {
				t.Fatalf("Delete called cache Get/Put %d/%d times", c.gets, c.puts)
			}
			if tc.wantErr != nil {
				if got, ok := c.items[item.Code]; !ok || got != cached || c.deletes != 0 {
					t.Fatal("failed deletion changed cache")
				}
			} else if _, ok := c.items[item.Code]; ok || c.deletes != 1 {
				t.Fatal("successful deletion did not remove cached link")
			}
		})
	}
}

func TestWithCacheShortenAndListLeaveCacheAlone(t *testing.T) {
	cached := Link{ID: 42, Code: "cached", OwnerID: "owner"}
	st := newCacheTestStore(t)
	c := newServiceCacheSpy(t, cached)
	svc := NewService(st, WithCache(c))
	if _, err := svc.Shorten(context.Background(), ShortenRequest{Target: "https://example.com", OwnerID: "owner"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.List(context.Background(), "owner", Page{}); err != nil {
		t.Fatal(err)
	}
	if c.gets != 0 || c.puts != 0 || c.deletes != 0 || len(c.items) != 1 || c.items[cached.Code] != cached {
		t.Fatal("Shorten or List touched the cache")
	}
}

func TestWithCacheLRUIntegration(t *testing.T) {
	first := Link{ID: 1, Code: "first", Target: "https://example.com", OwnerID: "owner"}
	second := Link{ID: 2, Code: "second", Target: "https://go.dev", OwnerID: "owner"}
	st := newCacheTestStore(t, first, second)
	c, err := cache.NewLRU[string, Link](1)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, WithCache(c))
	requireCachedResolve(t, svc, first)
	requireCachedResolve(t, svc, first)
	if st.reads != 1 {
		t.Fatalf("repeated resolve made %d store reads, want 1", st.reads)
	}
	requireCachedResolve(t, svc, second)
	requireCachedResolve(t, svc, first)
	if st.reads != 3 || c.Len() != 1 {
		t.Fatalf("after eviction: store reads=%d, cache length=%d; want 3 and 1", st.reads, c.Len())
	}
	if err := svc.Delete(context.Background(), first.OwnerID, first.Code); err != nil {
		t.Fatal(err)
	}
	if c.Len() != 0 {
		t.Fatal("deleted link remains in LRU")
	}
	if _, err := svc.Resolve(context.Background(), first.Code); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Resolve after Delete: %v, want ErrNotFound", err)
	}
}

type cacheTestStore struct {
	NoopStore
	items     map[string]Link
	lookupErr error
	deleteErr error
	reads     int
	deletes   int
	events    *[]string
}

func (s *cacheTestStore) ByCode(_ context.Context, code string) (Link, error) {
	s.reads++
	if s.events != nil {
		*s.events = append(*s.events, "store.ByCode")
	}
	if s.lookupErr != nil {
		return Link{}, s.lookupErr
	}
	item, ok := s.items[code]
	if !ok {
		return Link{}, ErrNotFound
	}
	return item, nil
}

func (s *cacheTestStore) Delete(_ context.Context, id uint64) error {
	s.deletes++
	if s.events != nil {
		*s.events = append(*s.events, "store.Delete")
	}
	if s.deleteErr != nil {
		return s.deleteErr
	}
	for code, item := range s.items {
		if item.ID == id {
			delete(s.items, code)
			return nil
		}
	}
	return ErrNotFound
}

type serviceCacheSpy struct {
	items   map[string]Link
	gets    int
	puts    int
	deletes int
	events  *[]string
}

func (c *serviceCacheSpy) Get(code string) (Link, bool) {
	c.gets++
	item, ok := c.items[code]
	return item, ok
}

func (c *serviceCacheSpy) Put(code string, item Link) {
	c.puts++
	c.items[code] = item
}

func (c *serviceCacheSpy) Delete(code string) {
	c.deletes++
	if c.events != nil {
		*c.events = append(*c.events, "cache.Delete")
	}
	delete(c.items, code)
}

func newCacheTestStore(t *testing.T, items ...Link) *cacheTestStore {
	t.Helper()
	st := &cacheTestStore{items: make(map[string]Link)}
	for _, item := range items {
		st.items[item.Code] = item
	}
	return st
}

func newServiceCacheSpy(t *testing.T, items ...Link) *serviceCacheSpy {
	t.Helper()
	c := &serviceCacheSpy{items: make(map[string]Link)}
	for _, item := range items {
		c.items[item.Code] = item
	}
	return c
}

func requireCachedResolve(t *testing.T, svc *Service, want Link) {
	t.Helper()
	got, err := svc.Resolve(context.Background(), want.Code)
	if err != nil || got != want {
		t.Fatalf("Resolve(%q) = %+v, %v; want %+v, nil", want.Code, got, err, want)
	}
}
