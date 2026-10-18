package link

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/skskuzan/rd-linkforge/internal/base62"
)

var (
	ErrEmptyTarget       = errors.New("link: error empty target")
	ErrUnsupportedScheme = errors.New("link: error unsupported scheme")
	ErrInvalidTarget     = errors.New("link: invalid target URL")
	ErrNotFound          = errors.New("link: not found")
	ErrAliasTaken        = errors.New("link: alias already taken")
	ErrInvalidAlias      = errors.New("link: invalid alias")
	ErrReservedAlias     = errors.New("link: reserved alias")
	ErrForbidden         = errors.New("link: forbidden")
	ErrExpired           = errors.New("link: expired")
	ErrInvalidExpiry     = errors.New("link: invalid expiry")
	ErrEmptyOwner        = errors.New("link: owner is required")
	ErrInvalidCursor     = errors.New("link: invalid cursor")
)

const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100
)

type Link struct {
	ID        uint64
	Code      string
	Target    string
	OwnerID   string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type AliasError struct {
	Alias string
	Err   error
}

func (e *AliasError) Error() string {
	return fmt.Sprintf("alias %q: %v", e.Alias, e.Err)
}

func (e *AliasError) Unwrap() error {
	return e.Err
}

func validateTarget(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ErrInvalidTarget
	}

	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrUnsupportedScheme
	}
	if parsedURL.Hostname() == "" {
		return ErrInvalidTarget
	}
	return nil
}

func New(id uint64, target string) (Link, error) {
	if target == "" {
		return Link{}, ErrEmptyTarget
	}

	if err := validateTarget(target); err != nil {
		return Link{}, err
	}

	return Link{
		ID:        id,
		Code:      base62.EncodeWidth(id, 6),
		Target:    target,
		CreatedAt: time.Now().UTC(),
	}, nil
}
