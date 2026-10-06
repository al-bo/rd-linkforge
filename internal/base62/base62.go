package base62

import (
	"errors"
	"math"
	"strings"
)

var (
	ErrEmpty            = errors.New("base62: empty code")
	ErrInvalidCharacter = errors.New("base62: invalid character")
	ErrOverflow         = errors.New("base62: code is too long")
)

// array of
var alphabet []rune = []rune("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")
var alphabetString = string(alphabet)
var size = uint64(len(alphabet))

// Encode renders a number with no padding.
func Encode(v uint64) string {
	var res []rune

	for v > 0 {
		r := v % size
		res = append(res, alphabet[r])

		v = v / size
	}

	for i, j := 0, len(res)-1; i < j; i, j = i+1, j-1 {
		res[i], res[j] = res[j], res[i]
	}

	return string(res)
}

// EncodeWidth does the same but left-pads with zeroes to at least width characters. The value itself is never truncated.
func EncodeWidth(v uint64, width int) string {
	encoded := Encode(v)
	length := len([]rune(encoded))
	var res []rune

	for i := 0; i < width-length; i++ {
		res = append(res, '0')
	}

	return string(res) + encoded
}

// Decode parses a code back into a number.
func Decode(s string) (uint64, error) {
	var result uint64
	src := []rune(s)
	max := uint64(math.MaxUint64)

	for _, char := range src {
		index := strings.IndexRune(alphabetString, char)

		if index == -1 {
			return 0, ErrInvalidCharacter
		}

		d := uint64(index)

		if result > max/size {
			return 0, ErrOverflow
		}

		result = result*size + d
	}

	return result, nil
}
