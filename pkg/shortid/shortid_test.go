package shortid_test

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/al-bo/rd-linkforge/pkg/shortid"
)

func TestNoopGenerator(t *testing.T) {
	var generator shortid.NoopGenerator
	code, err := generator.Next()
	if err != nil || code != "" {
		t.Fatalf("Next() = %q, %v; want empty code and nil error", code, err)
	}
}

func TestCodeGeneratorControlCodes(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []string
	}{
		{
			name: "zero random bits",
			data: make([]byte, 6),
			want: []string{"000000", "000001"},
		},
		{
			name: "one with counter",
			data: bytes.Repeat([]byte{0, 0, 1}, 3),
			want: []string{"000H32", "000H33", "000H34"},
		},
		{
			name: "maximum random bits",
			data: []byte{0xff, 0xff, 0xff},
			want: []string{"bVJgVU"},
		},
		{
			name: "discard upper five bits",
			data: []byte{0xf8, 0, 1},
			want: []string{"000H32"},
		},
		{
			name: "fresh bytes on every call",
			data: []byte{0, 0, 1, 0, 0, 0, 0, 0, 1},
			want: []string{"000H32", "000001", "000H34"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewReader(tc.data)
			generator := shortid.NewCodeGenerator(reader)
			for i, want := range tc.want {
				if got := nextCode(t, generator); got != want {
					t.Fatalf("call %d: Next() = %q, want %q", i, got, want)
				}
				if wantRemaining := len(tc.data) - 3*(i+1); reader.Len() != wantRemaining {
					t.Fatalf("call %d: %d unread bytes, want %d", i, reader.Len(), wantRemaining)
				}
			}
		})
	}
}

func TestCodeGeneratorReadsFullBuffer(t *testing.T) {
	reader := iotest.OneByteReader(bytes.NewReader([]byte{0, 0, 1}))
	generator := shortid.NewCodeGenerator(reader)
	if got := nextCode(t, generator); got != "000H32" {
		t.Fatalf("Next() with one-byte reads = %q, want 000H32", got)
	}
}

func TestCodeGeneratorReadErrors(t *testing.T) {
	for n := 0; n < 3; n++ {
		t.Run(strconv.Itoa(n)+" bytes", func(t *testing.T) {
			generator := shortid.NewCodeGenerator(bytes.NewReader(make([]byte, n)))
			wantErr := io.ErrUnexpectedEOF
			if n == 0 {
				wantErr = io.EOF
			}
			requireReadError(t, generator, wantErr)
		})
	}
}

func TestCodeGeneratorCounterUnchangedAfterError(t *testing.T) {
	for _, partial := range []int{0, 1, 2} {
		t.Run(strconv.Itoa(partial)+" bytes before error", func(t *testing.T) {
			sourceErr := errors.New("reader failed")
			reader := &recoveringReader{partial: partial, err: sourceErr}
			generator := shortid.NewCodeGenerator(reader)

			if got := nextCode(t, generator); got != "000000" {
				t.Fatalf("first Next() = %q, want 000000", got)
			}
			requireReadError(t, generator, sourceErr)
			for _, want := range []string{"000001", "000002"} {
				if got := nextCode(t, generator); got != want {
					t.Fatalf("Next() after recovery = %q, want %q", got, want)
				}
			}
		})
	}
}

func TestCodeGeneratorCounterWraps(t *testing.T) {
	const cycle = 1 << 16
	generator := shortid.NewCodeGenerator(bytes.NewReader(make([]byte, 3*(cycle+2))))
	seen := make(map[string]struct{}, cycle)
	for i := 0; i < cycle; i++ {
		code := nextCode(t, generator)
		if _, exists := seen[code]; exists {
			t.Fatalf("call %d repeated code %q before counter wrap", i, code)
		}
		seen[code] = struct{}{}
		if i == cycle-1 && code != "000H31" {
			t.Fatalf("counter 65535 produced %q, want 000H31", code)
		}
	}
	for _, want := range []string{"000000", "000001"} {
		if got := nextCode(t, generator); got != want {
			t.Fatalf("Next() after counter wrap = %q, want %q", got, want)
		}
	}
}

func TestCodeGeneratorCountersAreIndependent(t *testing.T) {
	first := shortid.NewCodeGenerator(bytes.NewReader(bytes.Repeat([]byte{0, 0, 1}, 2)))
	second := shortid.NewCodeGenerator(bytes.NewReader([]byte{0, 0, 1}))
	for _, want := range []string{"000H32", "000H33"} {
		if got := nextCode(t, first); got != want {
			t.Fatalf("first generator: Next() = %q, want %q", got, want)
		}
	}
	if got := nextCode(t, second); got != "000H32" {
		t.Fatalf("second generator: Next() = %q, want 000H32", got)
	}
}

type recoveringReader struct {
	calls   int
	partial int
	err     error
}

func (r *recoveringReader) Read(p []byte) (int, error) {
	r.calls++
	if r.calls == 2 {
		return copy(p, []byte{0xff, 0xff}[:r.partial]), r.err
	}
	clear(p)
	return len(p), nil
}

func nextCode(t *testing.T, generator *shortid.CodeGenerator) string {
	t.Helper()
	if generator == nil {
		t.Fatal("NewCodeGenerator returned nil")
	}
	code, err := generator.Next()
	if err != nil {
		t.Fatalf("Next(): %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("Next() = %q, length = %d, want 6", code, len(code))
	}
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	for i := 0; i < len(code); i++ {
		if strings.IndexByte(alphabet, code[i]) < 0 {
			t.Fatalf("Next() = %q contains a non-base62 byte at index %d", code, i)
		}
	}
	return code
}

func requireReadError(t *testing.T, generator *shortid.CodeGenerator, wantErr error) {
	t.Helper()
	code, err := generator.Next()
	if code != "" || !errors.Is(err, wantErr) {
		t.Fatalf("Next() = %q, %v; want empty code and %v", code, err, wantErr)
	}
	if errors.Unwrap(err) == nil {
		t.Fatalf("Next() error %v does not wrap the reader error", err)
	}
}
