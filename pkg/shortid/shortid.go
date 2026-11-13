package shortid

import (
	"fmt"
	"io"
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var _ Generator = (*CodeGenerator)(nil)

// CodeGenerator generates six-character base62 codes.
// It is not safe for concurrent use.
type CodeGenerator struct {
	random  io.Reader
	counter uint16
}

func encodeCode(value uint64) string {
	var code [6]byte

	for i := len(code) - 1; i >= 0; i-- {
		code[i] = alphabet[value%62]
		value /= 62
	}

	return string(code[:])
}

// NewCodeGenerator uses random as its source of random bytes.
// Applications can pass crypto/rand.Reader; tests can use a deterministic reader.
func NewCodeGenerator(random io.Reader) *CodeGenerator {
	return &CodeGenerator{random: random}
}

// Next consumes three random bytes and returns a six-character base62 code.
// A read error is wrapped and leaves the counter unchanged.
func (c *CodeGenerator) Next() (string, error) {
	var buf [3]byte

	if _, err := io.ReadFull(c.random, buf[:]); err != nil {
		return "", fmt.Errorf("shortid: read random bytes: %w", err)
	}
	r := uint64(buf[0])<<16 |
		uint64(buf[1])<<8 |
		uint64(buf[2])

	r = r & ((1 << 19) - 1)

	value := r<<16 | uint64(c.counter)
	code := encodeCode(value)

	c.counter++

	return code, nil
}
