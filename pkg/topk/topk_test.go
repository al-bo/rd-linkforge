package topk_test

import (
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/al-bo/rd-linkforge/pkg/topk"
)

func TestNewInvalidN(t *testing.T) {
	for _, n := range []int{-10, -1, 0} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			if _, err := topk.New(n); !errors.Is(err, topk.ErrInvalidN) {
				t.Fatalf("New(%d) error = %v, want ErrInvalidN", n, err)
			}
		})
	}
}

func TestTopKEmpty(t *testing.T) {
	for _, n := range []int{1, 3} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			tk := newTestTopK(t, n)
			requireResult(t, tk)
			requireResult(t, tk)
		})
	}
}

func TestTopKHomeworkExample(t *testing.T) {
	tk := newTestTopK(t, 3)
	for i, count := range []int64{5, 1, 9, 3, 9, 7, 2} {
		tk.Add("k"+strconv.Itoa(i), count)
	}
	requireResult(t, tk,
		topk.Item{Key: "k2", Count: 9},
		topk.Item{Key: "k4", Count: 9},
		topk.Item{Key: "k5", Count: 7},
	)
}

func TestTopKAdd(t *testing.T) {
	tests := []struct {
		name  string
		n     int
		items []topk.Item
		want  []topk.Item
	}{
		{
			name:  "fewer than n",
			n:     5,
			items: []topk.Item{{Key: "a", Count: 1}, {Key: "b", Count: 9}},
			want:  []topk.Item{{Key: "b", Count: 9}, {Key: "a", Count: 1}},
		},
		{
			name:  "exactly n",
			n:     3,
			items: []topk.Item{{Key: "low", Count: 1}, {Key: "high", Count: 9}, {Key: "middle", Count: 5}},
			want:  []topk.Item{{Key: "high", Count: 9}, {Key: "middle", Count: 5}, {Key: "low", Count: 1}},
		},
		{
			name:  "ascending counts replace worst",
			n:     2,
			items: []topk.Item{{Key: "a", Count: 1}, {Key: "b", Count: 2}, {Key: "c", Count: 3}, {Key: "d", Count: 4}},
			want:  []topk.Item{{Key: "d", Count: 4}, {Key: "c", Count: 3}},
		},
		{
			name:  "descending counts reject worse",
			n:     2,
			items: []topk.Item{{Key: "d", Count: 4}, {Key: "c", Count: 3}, {Key: "b", Count: 2}, {Key: "a", Count: 1}},
			want:  []topk.Item{{Key: "d", Count: 4}, {Key: "c", Count: 3}},
		},
		{
			name:  "smaller key wins tie at capacity",
			n:     2,
			items: []topk.Item{{Key: "b", Count: 5}, {Key: "c", Count: 5}, {Key: "a", Count: 5}},
			want:  []topk.Item{{Key: "a", Count: 5}, {Key: "b", Count: 5}},
		},
		{
			name:  "larger key loses tie at capacity",
			n:     2,
			items: []topk.Item{{Key: "b", Count: 5}, {Key: "a", Count: 5}, {Key: "c", Count: 5}},
			want:  []topk.Item{{Key: "a", Count: 5}, {Key: "b", Count: 5}},
		},
		{
			name:  "count takes priority over key",
			n:     1,
			items: []topk.Item{{Key: "a", Count: 5}, {Key: "z", Count: 6}},
			want:  []topk.Item{{Key: "z", Count: 6}},
		},
		{
			name:  "zero and negative counts",
			n:     3,
			items: []topk.Item{{Key: "low", Count: -10}, {Key: "middle", Count: -2}, {Key: "zero", Count: 0}, {Key: "near", Count: -1}},
			want:  []topk.Item{{Key: "zero", Count: 0}, {Key: "near", Count: -1}, {Key: "middle", Count: -2}},
		},
		{
			name:  "duplicate keys are separate items",
			n:     3,
			items: []topk.Item{{Key: "same", Count: 1}, {Key: "same", Count: 3}, {Key: "same", Count: 2}, {Key: "same", Count: 2}},
			want:  []topk.Item{{Key: "same", Count: 3}, {Key: "same", Count: 2}, {Key: "same", Count: 2}},
		},
		{
			name:  "equal worst item leaves result unchanged",
			n:     2,
			items: []topk.Item{{Key: "best", Count: 9}, {Key: "worst", Count: 1}, {Key: "worst", Count: 1}},
			want:  []topk.Item{{Key: "best", Count: 9}, {Key: "worst", Count: 1}},
		},
		{
			name:  "byte order and empty key",
			n:     6,
			items: []topk.Item{{Key: "é", Count: 5}, {Key: "a", Count: 5}, {Key: "Z", Count: 5}, {Key: "", Count: 5}, {Key: "z", Count: 5}, {Key: "\xff", Count: 5}},
			want:  []topk.Item{{Key: "", Count: 5}, {Key: "Z", Count: 5}, {Key: "a", Count: 5}, {Key: "z", Count: 5}, {Key: "é", Count: 5}, {Key: "\xff", Count: 5}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tk := newTestTopK(t, tc.n)
			for _, item := range tc.items {
				tk.Add(item.Key, item.Count)
			}
			requireResult(t, tk, tc.want...)
		})
	}
}

func TestTopKCapacityOne(t *testing.T) {
	tk := newTestTopK(t, 1)
	steps := []struct {
		item topk.Item
		want topk.Item
	}{
		{item: topk.Item{Key: "b", Count: 5}, want: topk.Item{Key: "b", Count: 5}},
		{item: topk.Item{Key: "a", Count: 4}, want: topk.Item{Key: "b", Count: 5}},
		{item: topk.Item{Key: "c", Count: 5}, want: topk.Item{Key: "b", Count: 5}},
		{item: topk.Item{Key: "a", Count: 5}, want: topk.Item{Key: "a", Count: 5}},
		{item: topk.Item{Key: "a", Count: 5}, want: topk.Item{Key: "a", Count: 5}},
		{item: topk.Item{Key: "z", Count: 6}, want: topk.Item{Key: "z", Count: 6}},
	}
	for i, step := range steps {
		tk.Add(step.item.Key, step.item.Count)
		if got := tk.Result(); len(got) != 1 || got[0] != step.want {
			t.Fatalf("step %d, Add(%q, %d): Result() = %v, want [%v]", i, step.item.Key, step.item.Count, got, step.want)
		}
	}
}

func TestTopKCountExtremes(t *testing.T) {
	const maxCount int64 = 1<<63 - 1
	const minCount int64 = -1 << 63
	items := []topk.Item{
		{Key: "min", Count: minCount},
		{Key: "zero", Count: 0},
		{Key: "max", Count: maxCount},
		{Key: "negative", Count: -1},
		{Key: "near-max", Count: maxCount - 1},
	}
	want := []topk.Item{
		{Key: "max", Count: maxCount},
		{Key: "near-max", Count: maxCount - 1},
		{Key: "zero", Count: 0},
		{Key: "negative", Count: -1},
		{Key: "min", Count: minCount},
	}
	for _, n := range []int{3, 5} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			tk := newTestTopK(t, n)
			for _, item := range items {
				tk.Add(item.Key, item.Count)
			}
			requireResult(t, tk, want[:n]...)
		})
	}
}

func TestTopKResultPreservesHeap(t *testing.T) {
	tk := newTestTopK(t, 3)
	tk.Add("high", 9)
	tk.Add("low", 1)
	tk.Add("middle", 5)
	want := []topk.Item{{Key: "high", Count: 9}, {Key: "middle", Count: 5}, {Key: "low", Count: 1}}
	requireResult(t, tk, want...)
	requireResult(t, tk, want...)

	// Sorting the backing heap in Result would move the best item to its root.
	tk.Add("next", 4)
	requireResult(t, tk,
		topk.Item{Key: "high", Count: 9},
		topk.Item{Key: "middle", Count: 5},
		topk.Item{Key: "next", Count: 4},
	)
	tk.Add("best", 10)
	requireResult(t, tk,
		topk.Item{Key: "best", Count: 10},
		topk.Item{Key: "high", Count: 9},
		topk.Item{Key: "middle", Count: 5},
	)
}

func TestTopKResultReturnsIndependentCopy(t *testing.T) {
	tk := newTestTopK(t, 3)
	tk.Add("a", 10)
	tk.Add("b", 8)
	tk.Add("c", 2)
	want := []topk.Item{{Key: "a", Count: 10}, {Key: "b", Count: 8}, {Key: "c", Count: 2}}

	first := tk.Result()
	if !slices.Equal(first, want) {
		t.Fatalf("first Result() = %v, want %v", first, want)
	}
	second := tk.Result()
	if !slices.Equal(second, want) {
		t.Fatalf("second Result() = %v, want %v", second, want)
	}
	first[0] = topk.Item{Key: "changed", Count: -100}
	if !slices.Equal(second, want) {
		t.Fatalf("changing one result changed another: got %v, want %v", second, want)
	}
	requireResult(t, tk, want...)

	tk.Add("d", 9)
	requireResult(t, tk,
		topk.Item{Key: "a", Count: 10},
		topk.Item{Key: "d", Count: 9},
		topk.Item{Key: "b", Count: 8},
	)
	if !slices.Equal(second, want) {
		t.Fatalf("Add changed a previous result: got %v, want %v", second, want)
	}
}

func newTestTopK(t *testing.T, n int) *topk.TopK {
	t.Helper()
	tk, err := topk.New(n)
	if err != nil {
		t.Fatalf("New(%d): %v", n, err)
	}
	if tk == nil {
		t.Fatal("New returned nil without an error")
	}
	return tk
}

func requireResult(t *testing.T, tk *topk.TopK, want ...topk.Item) {
	t.Helper()
	if got := tk.Result(); !slices.Equal(got, want) {
		t.Fatalf("Result() = %v, want %v", got, want)
	}
}
