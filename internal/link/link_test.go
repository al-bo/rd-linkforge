package link

import (
	"errors"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	for _, target := range []string{"http://example.com", "https://example.com/path", "HTTPS://example.com"} {
		t.Run(target, func(t *testing.T) {
			got, err := New(62, target)
			if err != nil {
				t.Fatal(err)
			}
			if got.ID != 62 || got.Code != "000010" || got.Target != target {
				t.Errorf("New(62, %q) = %+v", target, got)
			}
			if got.CreatedAt.IsZero() || got.CreatedAt.Location() != time.UTC {
				t.Errorf("CreatedAt = %v, want a nonzero UTC timestamp", got.CreatedAt)
			}
		})
	}
}

func TestNewInvalidTarget(t *testing.T) {
	tests := []struct {
		target string
		want   error
	}{
		{"", ErrEmptyTarget},
		{"ftp://example.com", ErrUnsupportedScheme},
		{"example.com", ErrUnsupportedScheme},
		{"https://example.com/%zz", ErrUnsupportedScheme},
	}
	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			if _, err := New(1, tt.target); !errors.Is(err, tt.want) {
				t.Errorf("New(1, %q) error = %v, want %v", tt.target, err, tt.want)
			}
		})
	}
}
