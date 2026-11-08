package link

import (
	"context"
	"errors"
	"testing"
	"time"
)

type contextStoreSpy struct{ calls int }

func (s *contextStoreSpy) Create(_ context.Context, l Link) (Link, error) {
	s.calls++
	return l, nil
}

func (s *contextStoreSpy) ByCode(context.Context, string) (Link, error) {
	s.calls++
	return Link{}, ErrNotFound
}

func (s *contextStoreSpy) ListByOwner(context.Context, string, Page) ([]Link, error) {
	s.calls++
	return nil, nil
}

func (s *contextStoreSpy) Delete(context.Context, uint64) error {
	s.calls++
	return nil
}

func TestServiceStoppedContext(t *testing.T) {
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
				call func(*Service) error
			}{
				{"Shorten", func(s *Service) error {
					_, err := s.Shorten(ctx, ShortenRequest{Target: "https://example.com", OwnerID: "cli"})
					return err
				}},
				{"Shorten invalid request", func(s *Service) error { _, err := s.Shorten(ctx, ShortenRequest{}); return err }},
				{"Resolve", func(s *Service) error { _, err := s.Resolve(ctx, "missing"); return err }},
				{"List", func(s *Service) error { _, err := s.List(ctx, "", Page{Cursor: "!"}); return err }},
				{"Delete", func(s *Service) error { return s.Delete(ctx, "", "missing") }},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					st := &contextStoreSpy{}
					err := tc.call(NewService(st))
					if !errors.Is(err, ctx.Err()) {
						t.Errorf("error = %v, want %v", err, ctx.Err())
					}
					if st.calls != 0 {
						t.Errorf("stopped context reached store %d times", st.calls)
					}
				})
			}
		})
	}
}

func TestCanceledShortenPreservesFirstID(t *testing.T) {
	st := &contextStoreSpy{}
	s := NewService(st, WithFirstID(42))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := ShortenRequest{Target: "https://example.com", OwnerID: "cli"}
	if _, err := s.Shorten(ctx, req); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want canceled", err)
	}
	if st.calls != 0 {
		t.Fatal("canceled Shorten reached store")
	}
	l, err := s.Shorten(context.Background(), req)
	if err != nil || l.ID != 42 {
		t.Fatalf("next link = %+v, error=%v; want ID 42", l, err)
	}
}

func TestWithFirstID(t *testing.T) {
	cases := []struct {
		name string
		opts []Option
		want uint64
	}{
		{"default", nil, 1},
		{"custom", []Option{WithFirstID(42)}, 42},
		{"zero ignored", []Option{WithFirstID(0)}, 1},
		{"zero preserves earlier option", []Option{WithFirstID(42), WithFirstID(0)}, 42},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewService(&contextStoreSpy{}, tc.opts...)
			for offset := uint64(0); offset < 2; offset++ {
				l, err := s.Shorten(context.Background(), ShortenRequest{Target: "https://example.com", OwnerID: "cli"})
				if err != nil || l.ID != tc.want+offset {
					t.Errorf("ID=%d error=%v, want ID %d", l.ID, err, tc.want+offset)
				}
			}
		})
	}
}
