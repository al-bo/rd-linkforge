// Package link holds the domain model and the contracts its consumers require.
// It may not import a transport or a storage package.
package link

import "time"

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
