package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/al-bo/rd-linkforge/internal/link"
)

func TestStoreStoppedContext(t *testing.T) {
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
			cases := []struct {
				name string
				call func(*Store) error
			}{
				{"Create new", func(s *Store) error {
					_, err := s.Create(ctx, link.Link{ID: 8, Code: "000008", OwnerID: "cli"})
					return err
				}},
				{"Create duplicate", func(s *Store) error {
					_, err := s.Create(ctx, link.Link{ID: 7, Code: "000007", OwnerID: "other"})
					return err
				}},
				{"ByCode", func(s *Store) error { _, err := s.ByCode(ctx, "000007"); return err }},
				{"ByCode missing", func(s *Store) error { _, err := s.ByCode(ctx, "missing"); return err }},
				{"ListByOwner", func(s *Store) error { _, err := s.ListByOwner(ctx, "cli", link.Page{}); return err }},
				{"ListByOwner invalid cursor", func(s *Store) error {
					_, err := s.ListByOwner(ctx, "cli", link.Page{Cursor: "!"})
					return err
				}},
				{"Delete", func(s *Store) error { return s.Delete(ctx, 7) }},
				{"Delete missing", func(s *Store) error { return s.Delete(ctx, 99) }},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					st := New()
					if _, err := st.Create(context.Background(), link.Link{ID: 7, Code: "000007", OwnerID: "cli", Target: "https://example.com"}); err != nil {
						t.Fatal(err)
					}
					before := st.All()
					if err := tc.call(st); !errors.Is(err, ctx.Err()) {
						t.Errorf("error = %v, want %v", err, ctx.Err())
					}
					if !reflect.DeepEqual(before, st.All()) {
						t.Error("stopped operation changed the store")
					}
				})
			}
		})
	}
}
