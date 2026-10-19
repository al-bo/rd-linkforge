package store

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/al-bo/rd-linkforge/internal/link"
)

func TestEmptyStore(t *testing.T) {
	s := New()
	if s.Count() != 0 || len(s.All()) != 0 {
		t.Fatal("new store must be empty")
	}
	if got, ok := s.Get(1); ok || got != (link.Link{}) {
		t.Errorf("Get missing ID = %+v, %v; want zero Link, false", got, ok)
	}
}

func TestCreateAndGet(t *testing.T) {
	ctx := context.Background()
	s := New()
	want := link.Link{ID: 1, Code: "000001", Target: "https://example.com"}
	got, err := s.Create(ctx, want)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("Create = %+v, want %+v", got, want)
	}
	if _, err := s.Create(ctx, link.Link{ID: 1, Target: "https://other.example.com"}); !errors.Is(err, ErrDuplicateID) {
		t.Errorf("Create duplicate error = %v, want %v", err, ErrDuplicateID)
	}
	if got, ok := s.Get(1); !ok || got != want {
		t.Errorf("Get(1) = %+v, %v; want %+v, true", got, ok, want)
	}
	if got := s.Count(); got != 1 {
		t.Errorf("Count = %d, want 1", got)
	}
}

func TestAllSortedByID(t *testing.T) {
	ctx := context.Background()
	s := New()
	want := []link.Link{
		{ID: 1, Code: "000001"},
		{ID: 2, Code: "000002"},
		{ID: 3, Code: "000003"},
	}
	for _, index := range []int{2, 0, 1} {
		if _, err := s.Create(ctx, want[index]); err != nil {
			t.Fatal(err)
		}
	}
	if got := s.All(); !reflect.DeepEqual(got, want) {
		t.Errorf("All = %+v, want %+v", got, want)
	}
	if got := s.Count(); got != len(want) {
		t.Errorf("Count = %d, want %d", got, len(want))
	}
}
