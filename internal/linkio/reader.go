package linkio

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/al-bo/rd-linkforge/internal/link"
)

// Target is one line of an import file.
type Target struct {
	Line int // 1-based line number in the file
	URL  string
}

// ReadTargets returns every line that is not blank and does not start with '#',
// with surrounding spaces trimmed. A reader error is returned wrapped.
func ReadTargets(r io.Reader) ([]Target, error)

// ReadLinks reads JSON Lines: one JSON object per link. A bad record returns an
// error that names it, for example "record 3: ...". An empty input gives no links.
func ReadLinks(r io.Reader) ([]link.Link, error)

// WriteLinks writes JSON Lines: one link per line.
func WriteLinks(w io.Writer, links []link.Link) error {
	b, err := json.Marshal(links)

	if err != nil {
		return fmt.Errorf("Error encoding links: %w", err)
	}

	n, err := w.Write(b)

	if err != nil {
		return fmt.Errorf("Error writing links: %w %v", err, n)
	}
}

// WriteCSV writes the header id,code,target,owner_id,created_at,expires_at and one row per link.
// Times are RFC 3339 in UTC; a link that never expires has an empty expires_at.
func WriteCSV(w io.Writer, links []link.Link) error

// LoadFile reads a JSON Lines file. A missing file returns an error that wraps fs.ErrNotExist.
func LoadFile(path string) ([]link.Link, error) {
	f, err := os.Open(path)

	if err == fs.ErrNotExist {
		return nil, fmt.Errorf("Error loading file: %w", err)

	}

	defer f.Close()

	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("Error reading file: %w", err)
	}
	fmt.Println(string(buf[:n]))
}

// SaveFile creates (or replaces) the file at path and writes the links to it.
// The error from Close is part of the result.
func SaveFile(path string, links []link.Link) error
