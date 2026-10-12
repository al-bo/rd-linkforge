package store

import (
	"errors"
	"reflect"
	"testing"

	"github.com/skskuzan/rd-linkforge/internal/link"
)

func TestEmptyStore(t *testing.T) {
	s := New()
	if Count(s) != 0 || len(All(s)) != 0 {
		t.Fatal("new store must be empty")
	}
	if got, ok := Get(s, 1); ok || got != (link.Link{}) {
		t.Errorf("Get missing ID = %+v, %v; want zero Link, false", got, ok)
	}
}

func TestAddAndGet(t *testing.T) {
	s := New()
	want := link.Link{ID: 1, Code: "000001", Target: "https://example.com"}
	if err := Add(s, want); err != nil {
		t.Fatal(err)
	}
	if err := Add(s, link.Link{ID: 1, Target: "https://other.example.com"}); !errors.Is(err, ErrDuplicateID) {
		t.Errorf("Add duplicate error = %v, want %v", err, ErrDuplicateID)
	}
	if got, ok := Get(s, 1); !ok || got != want {
		t.Errorf("Get(1) = %+v, %v; want %+v, true", got, ok, want)
	}
	if got := Count(s); got != 1 {
		t.Errorf("Count = %d, want 1", got)
	}
}

func TestAllSortedByID(t *testing.T) {
	s := New()
	for _, id := range []uint64{3, 1, 2} {
		if err := Add(s, link.Link{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	want := []link.Link{{ID: 1}, {ID: 2}, {ID: 3}}
	if got := All(s); !reflect.DeepEqual(got, want) {
		t.Errorf("All = %+v, want %+v", got, want)
	}
	if got := Count(s); got != len(want) {
		t.Errorf("Count = %d, want %d", got, len(want))
	}
}
