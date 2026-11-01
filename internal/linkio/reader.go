package linkio

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/al-bo/rd-linkforge/internal/link"
)

// Target is one line of an import file.
type Target struct {
	Line int // 1-based line number in the file
	URL  string
}

// ReadTargets returns every line that is not blank and does not start with '#',
// with surrounding spaces trimmed. A reader error is returned wrapped.
func ReadTargets(r io.Reader) ([]Target, error) {
	var targets []Target
	buf := make([]byte, 2048)
	_, err := r.Read(buf)

	if err != nil {
		return []Target{}, err
	}

	src := string(buf)

	lines := strings.Split(src, "\n")

	for i, str := range lines {
		r := []rune(str)

		if strings.TrimSpace(str) != "" && r[0] != '#' && i != len(lines)-1 {
			targets = append(targets, Target{Line: i + 1, URL: str})
		}
	}

	return targets, nil
}

// ReadLinks reads JSON Lines: one JSON object per link. A bad record returns an
// error that names it, for example "record 3: ...". An empty input gives no links.
func ReadLinks(r io.Reader) ([]link.Link, error) {
	reader := bufio.NewReader(r)
	var links []link.Link
	for record := 1; ; record++ {
		line, err := reader.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("record %d: read: %w", record, err)
		}
		if len(line) == 0 && errors.Is(err, io.EOF) {
			return links, nil
		}

		var l *link.Link
		if decodeErr := json.Unmarshal(line, &l); decodeErr != nil {
			return nil, fmt.Errorf("record %d: decode: %w", record, &decodeError{err: decodeErr})
		}
		if l == nil {
			return nil, fmt.Errorf("record %d: expected a JSON object", record)
		}
		links = append(links, *l)
		if errors.Is(err, io.EOF) {
			return links, nil
		}
	}
}

// Decoder messages can include field values; keep those out of errors and logs.
type decodeError struct{ err error }

func (e *decodeError) Error() string { return "invalid link JSON" }

func (e *decodeError) Unwrap() error { return e.err }

// WriteLinks writes JSON Lines: one link per line.
func WriteLinks(w io.Writer, links []link.Link) error {
	for _, link := range links {
		b, err := json.Marshal(link)

		if err != nil {
			return fmt.Errorf("encode link: %w", err)
		}
		newline := []byte("\n")
		b = append(b, newline...)
		n, err := w.Write(b)

		if err != nil {
			return fmt.Errorf("write links after %d bytes: %w", n, err)
		}

	}

	return nil
}

// LoadFile reads a JSON Lines file. A missing file returns an error that wraps fs.ErrNotExist.
func LoadFile(path string) ([]link.Link, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("load file %q: %w", path, err)
	}
	defer f.Close()

	links, err := ReadLinks(f)
	if err != nil {
		return nil, fmt.Errorf("load file %q: %w", path, err)
	}
	return links, nil
}

// read file
func ReadTargetsFile(path string) (targets []Target, err error) {
	f, err := os.Open(path)

	if err != nil {
		return []Target{}, err
	}
	defer f.Close()

	r := bufio.NewReader(f)

	t, err := ReadTargets(r)

	if err != nil {
		return []Target{}, err
	}

	return t, nil
}

// SaveFile creates (or replaces) the file at path and writes the links to it.
// The error from Close is part of the result.
func SaveFile(path string, links []link.Link) (err error) {

	file, err := os.Create(path)

	if err != nil {
		return fmt.Errorf("create file %q: %w", path, err)
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close file %q: %w", path, closeErr))
		}
	}()

	w := bufio.NewWriter(file)

	if err := WriteLinks(w, links); err != nil {
		return fmt.Errorf("write file %q: %w", path, err)
	}

	e := w.Flush()

	if e != nil {
		return fmt.Errorf("flush file %q: %w", path, e)
	}

	return nil
}
