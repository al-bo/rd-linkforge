// Package link holds the domain model and the contracts its consumers require.
// It may not import a transport or a storage package.
package link

import (
	"fmt"
	"time"

	"github.com/skskuzan/rd-linkforge/internal/base62"
)

// ShortenRequest carries everything needed to create a link. An empty Alias
// asks for a generated code.
type ShortenRequest struct {
	TargetURL string
	Alias     string
	OwnerID   string
	ExpiresAt time.Time
}

// Page describes one slice of a cursor-paginated listing.
type Page struct {
	Cursor string
	Limit  int
}

// Normalize повертає копію p, де Limit у межах [1, MaxPageLimit]:
// нуль чи від'ємне стає DefaultPageLimit, понад максимум – MaxPageLimit.
// Вона не змінює p.
func (p Page) Normalize() Page {
	if p.Limit > 0 && p.Limit <= MaxPageLimit {
		return p
	} else if p.Limit < 1 {
		return Page{Cursor: p.Cursor, Limit: DefaultPageLimit}
	} else {
		return Page{Cursor: p.Cursor, Limit: MaxPageLimit}
	}
}

// AfterID декодує курсор. Порожній курсор повертає 0, nil.
// Курсор, відхилений base62.Decode, дає помилку, що обгортає ErrInvalidCursor.
func (p Page) AfterID() (uint64, error) {
	if p.Cursor == "" {
		return 0, nil
	}

	d, err := base62.Decode(p.Cursor)

	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrInvalidCursor, err)
	}

	return d, nil
}
