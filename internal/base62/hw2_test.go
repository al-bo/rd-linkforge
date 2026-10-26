package base62

import (
	"errors"
	"math"
	"testing"
)

func TestEncodeRecursive(t *testing.T) {
	for _, v := range []uint64{0, 1, 9, 10, 61, 62, 3843, 3844, 123456, 1 << 40, math.MaxUint64} {
		if got, want := EncodeRecursive(v), Encode(v); got != want {
			t.Errorf("EncodeRecursive(%d) = %q, want %q", v, got, want)
		}
	}
}

func TestDecodeRange(t *testing.T) {
	tests := []struct {
		in      string
		want    uint64
		wantErr error
	}{
		{"0000000000z", 61, nil},
		{"LygHa16AHYF", math.MaxUint64, nil},
		{"LygHa16AHYG", 0, ErrOverflow},
		{"zzzzzzzzzzz", 0, ErrOverflow},
		{"00000000000z", 0, ErrOverflow},
	}
	for _, tc := range tests {
		got, err := Decode(tc.in)
		if got != tc.want || !errors.Is(err, tc.wantErr) {
			t.Errorf("Decode(%q) = %d, %v; want %d, %v", tc.in, got, err, tc.want, tc.wantErr)
		}
	}
}
