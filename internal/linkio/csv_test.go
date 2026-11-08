package linkio

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/al-bo/rd-linkforge/internal/link"
)

func TestWriteCSV(t *testing.T) {
	created := time.Date(2026, 11, 1, 12, 30, 0, 0, time.FixedZone("UTC+2", 2*60*60))
	links := []link.Link{
		{ID: 1, Code: "000001", Target: "https://example.com/a,b", OwnerID: "owner\"name\nnext", CreatedAt: created},
		{ID: 2, Code: "000002", Target: "https://go.dev", OwnerID: "cli", CreatedAt: created, ExpiresAt: created.Add(time.Hour)},
	}
	want := "id,code,target,owner_id,created_at,expires_at\n" +
		"1,000001,\"https://example.com/a,b\",\"owner\"\"name\nnext\",2026-11-01T10:30:00Z,\n" +
		"2,000002,https://go.dev,cli,2026-11-01T10:30:00Z,2026-11-01T11:30:00Z\n"
	var out bytes.Buffer
	if err := WriteCSV(&out, links); err != nil {
		t.Fatal(err)
	}
	if out.String() != want {
		t.Fatalf("CSV = %q, want %q", out.String(), want)
	}
}

func TestWriteCSVEmpty(t *testing.T) {
	var out bytes.Buffer
	if err := WriteCSV(&out, nil); err != nil {
		t.Fatal(err)
	}
	if want := "id,code,target,owner_id,created_at,expires_at\n"; out.String() != want {
		t.Fatalf("CSV = %q, want %q", out.String(), want)
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestWriteCSVWrapsFlushError(t *testing.T) {
	want := errors.New("write failed")
	if err := WriteCSV(failingWriter{err: want}, nil); !errors.Is(err, want) {
		t.Fatalf("error = %v, want wrapped %v", err, want)
	}
}
