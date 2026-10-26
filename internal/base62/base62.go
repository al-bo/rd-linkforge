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

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const size = uint64(len(alphabet))

func Encode(v uint64) string {
	var res [11]byte
	i := len(res)

	for {
		i--
		res[i] = alphabet[v%size]

		v /= size
		if v == 0 {
			break
		}
	}

	return string(res[i:])
}

func EncodeRecursive(v uint64) string {
	if v < size {
		return string(alphabet[v])
	}
	return EncodeRecursive(v/size) + string(alphabet[v%size])
}

func EncodeWidth(v uint64, width int) string {
	encoded := Encode(v)
	length := len([]rune(encoded))
	var res []rune

	for i := 0; i < width-length; i++ {
		res = append(res, '0')
	}

	return string(res) + encoded
}

func Decode(s string) (uint64, error) {
	var result uint64
	src := []rune(s)
	max := uint64(math.MaxUint64)

	if len(src) == 0 {
		return 0, ErrEmpty
	}
	if len(src) > 11 {
		return 0, ErrOverflow
	}

	for _, char := range src {
		index := strings.IndexRune(alphabet, char)

		if index == -1 {
			return 0, ErrInvalidCharacter
		}

		d := uint64(index)

		if result > (max-d)/size {
			return 0, ErrOverflow
		}

		result = result*size + d
	}

	return result, nil
}
