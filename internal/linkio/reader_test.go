package linkio

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/al-bo/rd-linkforge/internal/link"
)

func TestReadLinks(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		count  int
		errMsg string
	}{
		{name: "empty"},
		{name: "two records", input: "{\"ID\":1}\n{\"ID\":2}\n", count: 2},
		{name: "no final newline", input: "{\"ID\":1}\n{\"ID\":2}", count: 2},
		{name: "CRLF", input: "{\"ID\":1}\r\n", count: 1},
		{name: "malformed record", input: "{\"ID\":1}\n{broken}\n", errMsg: "record 2:"},
		{name: "two objects on one line", input: "{} {}\n", errMsg: "record 1:"},
		{name: "null is not an object", input: "null\n", errMsg: "record 1:"},
		{name: "array is not an object", input: "[]\n", errMsg: "record 1:"},
		{name: "blank record", input: "{}\n\n", errMsg: "record 2:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReadLinks(strings.NewReader(tc.input))
			if tc.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errMsg) {
					t.Fatalf("error = %v, want %q", err, tc.errMsg)
				}
				if got != nil {
					t.Fatalf("returned partial links on failure: %+v", got)
				}
				return
			}
			if err != nil || len(got) != tc.count {
				t.Fatalf("got %d links, error %v; want %d", len(got), err, tc.count)
			}
			for i, l := range got {
				if l.ID != uint64(i+1) {
					t.Fatalf("record %d ID = %d", i+1, l.ID)
				}
			}
		})
	}
}

func TestReadLinksWrapsErrors(t *testing.T) {
	want := errors.New("read failed")
	_, err := ReadLinks(iotest.ErrReader(want))
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "record 1:") {
		t.Fatalf("reader error = %v", err)
	}
	_, err = ReadLinks(strings.NewReader("{broken}\n"))
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("expected wrapped JSON error, got %v", err)
	}
}

func TestReadLinksDoesNotExposeInvalidFieldValues(t *testing.T) {
	_, err := ReadLinks(strings.NewReader("{\"CreatedAt\":\"PRIVATE_TIMESTAMP_VALUE\"}\n"))
	var parseErr *time.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("timestamp error was not preserved: %v", err)
	}
	if !strings.Contains(err.Error(), "record 1:") || strings.Contains(err.Error(), "PRIVATE_TIMESTAMP_VALUE") {
		t.Fatalf("unsafe record error: %v", err)
	}
}

func TestLinksRoundTripAndLoadFile(t *testing.T) {
	want := []link.Link{
		{ID: 1, Code: "000001", Target: "https://example.com/" + strings.Repeat("a", 70000), OwnerID: "cli"},
		{ID: 2, Code: "000002", Target: "https://go.dev", OwnerID: "cli"},
	}
	var encoded bytes.Buffer
	if err := WriteLinks(&encoded, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLinks(&encoded)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON Lines round trip failed: %v", err)
	}
	path := filepath.Join(t.TempDir(), "links.jsonl")
	if err := SaveFile(path, want); err != nil {
		t.Fatal(err)
	}
	got, err = LoadFile(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("file round trip failed: %v", err)
	}
}

func TestLoadFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "links.jsonl")
	_, err := LoadFile(path)
	if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), path) {
		t.Fatalf("missing file error = %v", err)
	}
	if err := os.WriteFile(path, []byte("{}\n{broken}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = LoadFile(path)
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) || !strings.Contains(err.Error(), "record 2:") || !strings.Contains(err.Error(), path) {
		t.Fatalf("invalid record error = %v", err)
	}
}
