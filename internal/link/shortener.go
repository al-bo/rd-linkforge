package link

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/al-bo/rd-linkforge/internal/base62"
)

// Shortener is the domain service every transport sits in front of. No
// transport may reach a Store directly.
type Shortener interface {
	Shorten(ctx context.Context, req ShortenRequest) (Link, error)
	Resolve(ctx context.Context, code string) (Link, error)
	List(ctx context.Context, ownerID string, page Page) ([]Link, error)
	Delete(ctx context.Context, ownerID, code string) error
}

// NoopShortener satisfies Shortener and does nothing. Implemented in M2.
type NoopShortener struct{}

var _ Shortener = NoopShortener{}

func (NoopShortener) Shorten(context.Context, ShortenRequest) (Link, error) { return Link{}, nil }

func (NoopShortener) Resolve(context.Context, string) (Link, error) { return Link{}, nil }

func (NoopShortener) List(context.Context, string, Page) ([]Link, error) { return nil, nil }

func (NoopShortener) Delete(context.Context, string, string) error { return nil }

type Service struct {
	store  Store
	now    func() time.Time
	nextID uint64
}

type Option func(*Service)

func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		s.now = now
	}
}

func NewService(store Store, opts ...Option) *Service {
	s := &Service{
		store:  store,
		now:    time.Now,
		nextID: 1,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func validateAlias(v string) error {
	switch strings.ToLower(v) {
	case "api", "ws", "healthz", "readyz", "metrics", "admin":
		return ErrReservedAlias
	}

	if len(v) < 3 || len(v) > 32 {
		return ErrInvalidAlias
	}

	for i := 0; i < len(v); i++ {
		if !isAliasChar(v[i]) {
			return ErrInvalidAlias
		}
	}

	return nil
}

func isAliasChar(c byte) bool {
	return 'a' <= c && c <= 'z' ||
		'A' <= c && c <= 'Z' ||
		'0' <= c && c <= '9' ||
		c == '_' || c == '-'
}

func (s *Service) Shorten(ctx context.Context, r ShortenRequest) (Link, error) {
	const maxAttempts = 3

	if err := ctx.Err(); err != nil {
		return Link{}, err
	}

	if r.OwnerID == "" {
		return Link{}, ErrEmptyOwner
	}

	l, err := New(s.nextID, r.Target)

	if err != nil {
		return Link{}, err
	}

	now := s.now().UTC()
	if !r.ExpiresAt.IsZero() && !r.ExpiresAt.After(now) {
		return Link{}, ErrInvalidExpiry
	}

	if r.Alias != "" {
		if err := validateAlias(r.Alias); err != nil {
			return Link{}, &AliasError{
				Alias: r.Alias,
				Err:   err,
			}
		}
	}

	l.OwnerID = r.OwnerID
	l.ExpiresAt = r.ExpiresAt
	l.CreatedAt = now

	for range maxAttempts {
		l.ID = s.nextID
		l.Code = r.Alias
		if l.Code == "" {
			l.Code = base62.EncodeWidth(l.ID, 6)
		}

		addedLink, createErr := s.store.Create(ctx, l)
		if createErr == nil {
			s.nextID++
			return addedLink, nil
		}

		if !errors.Is(createErr, ErrAliasTaken) {
			return Link{}, fmt.Errorf("link: shorten: create: %w", createErr)
		}

		if r.Alias != "" {
			return Link{}, &AliasError{
				Alias: r.Alias,
				Err:   createErr,
			}
		}

		s.nextID++
	}

	return Link{}, &AliasError{
		Alias: l.Code,
		Err:   ErrAliasTaken,
	}
}

func (s *Service) Resolve(ctx context.Context, code string) (Link, error) {
	l, err := s.store.ByCode(ctx, code)

	if err := ctx.Err(); err != nil {
		return Link{}, err
	}

	if err != nil {
		return Link{}, fmt.Errorf("link: resolve: %w", err)
	}

	if !l.ExpiresAt.IsZero() && !s.now().Before(l.ExpiresAt) {
		return Link{}, ErrExpired
	}

	return l, nil
}

func (s *Service) List(ctx context.Context, ownerID string, p Page) ([]Link, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if ownerID == "" {
		return nil, ErrEmptyOwner
	}

	links, err := s.store.ListByOwner(ctx, ownerID, p.Normalize())

	if err != nil {
		return nil, fmt.Errorf("link: list: %w", err)
	}

	return links, nil
}

func (s *Service) Delete(ctx context.Context, ownerID string, code string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if ownerID == "" {
		return ErrEmptyOwner
	}

	l, err := s.store.ByCode(ctx, code)

	if err != nil {
		return fmt.Errorf("link: delete: find: %w", err)
	}

	if l.OwnerID != ownerID {
		return ErrForbidden
	}

	if err := s.store.Delete(ctx, l.ID); err != nil {
		return fmt.Errorf("link: delete: %w", err)
	}

	return nil
}
