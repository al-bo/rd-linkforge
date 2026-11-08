package linkio

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

func TestReadTargets(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []Target
	}{
		{"empty", "", nil},
		{"blank and comments", "\n  \n# comment\n  # indented\n", nil},
		{"line numbers", "# comment\nhttps://example.com\n\nnot-a-url\nhttps://go.dev\n", []Target{{2, "https://example.com"}, {4, "not-a-url"}, {5, "https://go.dev"}}},
		{"no final newline", "https://example.com\nhttps://go.dev", []Target{{1, "https://example.com"}, {2, "https://go.dev"}}},
		{"surrounding whitespace", " \thttps://example.com \t\n", []Target{{1, "https://example.com"}}},
		{"CRLF", "# comment\r\nhttps://example.com\r\n\r\nhttps://go.dev\r\n", []Target{{2, "https://example.com"}, {4, "https://go.dev"}}},
		{"URL fragment", "https://example.com/#section\n", []Target{{1, "https://example.com/#section"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReadTargets(strings.NewReader(tc.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("targets = %+v; want %+v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("target %d = %+v; want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestReadTargetsEntireInput(t *testing.T) {
	got, err := ReadTargets(strings.NewReader(strings.Repeat("https://example.com\n", 200)))
	if err != nil || len(got) != 200 {
		t.Fatalf("got %d targets, error=%v; want 200", len(got), err)
	}
	for i, target := range got {
		if target.Line != i+1 || target.URL != "https://example.com" {
			t.Errorf("target %d = %+v", i, target)
		}
	}
}

func TestReadTargetsReaderBoundaries(t *testing.T) {
	input := "https://example.com\nhttps://go.dev"
	for _, tc := range []struct {
		name string
		r    io.Reader
	}{
		{"one byte reads", iotest.OneByteReader(strings.NewReader(input))},
		{"data and EOF together", iotest.DataErrReader(strings.NewReader(input))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReadTargets(tc.r)
			if err != nil || len(got) != 2 {
				t.Fatalf("got %d targets, error=%v; want 2", len(got), err)
			}
			if got[0] != (Target{1, "https://example.com"}) || got[1] != (Target{2, "https://go.dev"}) {
				t.Fatalf("unexpected targets: %+v", got)
			}
		})
	}
}

func TestReadTargetsWrapsReaderError(t *testing.T) {
	want := errors.New("source unavailable")
	_, err := ReadTargets(iotest.ErrReader(want))
	if !errors.Is(err, want) || errors.Unwrap(err) == nil {
		t.Fatalf("error = %v; want wrapped reader error", err)
	}
}

func TestReadTargetsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "urls.txt")
	if err := os.WriteFile(path, []byte("# comment\nhttps://example.com\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadTargetsFile(path)
	if err != nil || len(got) != 1 || got[0] != (Target{2, "https://example.com"}) {
		t.Fatalf("targets=%+v error=%v", got, err)
	}
	if _, err := ReadTargetsFile(filepath.Join(t.TempDir(), "absent.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file error = %v", err)
	}
}
