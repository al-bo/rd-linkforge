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
	ErrUnsupportedScheme = errors.New("link: error unsopported scheme")
)

type Link struct {
	ID        uint64
	Code      string
	Target    string
	CreatedAt time.Time
}

func validateScheme(rawURL string) bool {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	scheme := strings.ToLower(parsedURL.Scheme)
	return scheme == "http" || scheme == "https"
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

	if !validateScheme(target) {
		return link, ErrUnsupportedScheme
	}

	return link, nil

}
