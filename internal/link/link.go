package link

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/skskuzan/rd-linkforge/internal/base62"
)

var (
	ErrEmptyTarget       = errors.New("link: error empty target")
	ErrUnsupportedScheme = errors.New("link: error unsupported scheme")
	ErrInvalidTarget     = errors.New("link: invalid target URL")
)

type Link struct {
	ID        uint64
	Code      string
	Target    string
	CreatedAt time.Time
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
	r := []rune(target)

	link := Link{
		ID:        id,
		Code:      base62.EncodeWidth(id, 6),
		Target:    target,
		CreatedAt: time.Now().UTC(),
	}

	if len(r) == 0 {
		return link, ErrEmptyTarget
	}

	if err := validateTarget(target); err != nil {
		return link, err
	}

	return link, nil

}
