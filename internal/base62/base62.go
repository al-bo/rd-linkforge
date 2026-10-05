package base62

import "errors"

var (
	ErrEmpty            = errors.New("base62: empty code")
	ErrInvalidCharacter = errors.New("base62: invalid character")
	ErrOverflow         = errors.New("base62: code is too long")
)

// array of
var alphabet string = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// Encode renders a number with no padding.
func Encode(v uint64) string {
	return "noop"
}

// EncodeWidth does the same but left-pads with zeroes to at least width characters. The value itself is never truncated.
func EncodeWidth(v uint64, width int) string {
	return "noop"
}

// Decode parses a code back into a number.
func Decode(s string) (uint64, error) {
	var foo uint64 = 0

	return foo, ErrEmpty
}
