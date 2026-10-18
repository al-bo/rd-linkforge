package link

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/skskuzan/rd-linkforge/internal/base62"
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

func ValidateAlias(v string) error {
	var aliasRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

	var reservedAliases = map[string]bool{
		"api":     true,
		"ws":      true,
		"healthz": true,
		"readyz":  true,
		"metrics": true,
		"admin":   true,
	}
	src := []rune(v)
	l := len(src)

	if l < 3 || l > 32 {
		return ErrInvalidAlias
	}

	if !aliasRegex.MatchString(v) {
		return ErrInvalidAlias
	}

	la := strings.ToLower(v)
	if reservedAliases[la] {
		return ErrReservedAlias
	}

	return nil
}

func (s *Service) Shorten(ctx context.Context, r ShortenRequest) (Link, error) {
	if r.OwnerID == "" {
		return Link{}, ErrEmptyOwner
	}

	l, err := New(s.nextID, r.TargetURL)

	if err != nil {
		return Link{}, err
	}

	now := s.now().UTC()
	if !r.ExpiresAt.IsZero() && !r.ExpiresAt.After(now) {
		return Link{}, ErrInvalidExpiry
	}

	if r.Alias != "" {
		if err := ValidateAlias(r.Alias); err != nil {
			return Link{}, &AliasError{
				Alias: r.Alias,
				Err:   err,
			}
		}
	}

	l.OwnerID = r.OwnerID
	l.ExpiresAt = r.ExpiresAt
	l.CreatedAt = now

	for attempt := 0; attempt < 3; attempt++ {
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
			return Link{}, fmt.Errorf("shorten: create link: %w", createErr)
		}

		if r.Alias != "" {
			return Link{}, &AliasError{
				Alias: r.Alias,
				Err:   createErr,
			}
		}

		s.nextID++
	}

	return Link{}, fmt.Errorf("shorten: create link after 3 attempts: %w", ErrAliasTaken)
}

func (s *Service) Resolve(ctx context.Context, code string) (Link, error) {
	l, err := s.store.ByCode(ctx, code)

	if err != nil {
		return Link{}, fmt.Errorf("Service resolve: %w", err)
	}

	if !l.ExpiresAt.IsZero() && !s.now().Before(l.ExpiresAt) {
		return Link{}, ErrExpired
	}

	return l, nil
}

func (s *Service) List(ctx context.Context, ownerId string, p Page) ([]Link, error) {

	if ownerId == "" {
		return make([]Link, 0), ErrEmptyOwner
	}

	links, err := s.store.ListByOwner(ctx, ownerId, p.Normalize())

	if err != nil {
		return make([]Link, 0), fmt.Errorf("Service List: %w", err)
	}

	return links, nil
}

func (s *Service) Delete(ctx context.Context, ownerId string, code string) error {
	if ownerId == "" {
		return ErrEmptyOwner
	}

	l, err := s.store.ByCode(ctx, code)

	if err != nil {
		return fmt.Errorf("delete: find link: %w", err)
	}

	if l.OwnerID != ownerId {
		return ErrForbidden
	}

	deleteError := s.store.Delete(ctx, l.ID)

	if deleteError != nil {
		return fmt.Errorf("Service Delete: %w", deleteError)
	}

	return nil
}
