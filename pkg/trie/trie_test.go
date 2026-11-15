package trie_test

import (
	"slices"
	"testing"

	"github.com/al-bo/rd-linkforge/pkg/trie"
)

func TestTrieEmpty(t *testing.T) {
	tr := newTestTrie(t)
	if tr.Insert("") {
		t.Fatal("Insert(empty) = true, want false")
	}
	for _, word := range []string{"", "missing"} {
		if tr.Contains(word) {
			t.Errorf("Contains(%q) = true on an empty trie", word)
		}
		if tr.Delete(word) {
			t.Errorf("Delete(%q) = true on an empty trie", word)
		}
	}
	for _, prefix := range []string{"", "d"} {
		for _, limit := range []int{-1, 0, 1} {
			requireWords(t, tr, prefix, limit)
		}
	}
}

func TestTrieInsert(t *testing.T) {
	tests := []struct {
		name  string
		words []string
	}{
		{name: "shorter first", words: []string{"doc", "docs", "dock", "dog"}},
		{name: "longer first", words: []string{"docs", "dock", "dog", "doc"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := newTestTrie(t, tc.words...)
			for _, word := range tc.words {
				if !tr.Contains(word) {
					t.Errorf("Contains(%q) = false after Insert", word)
				}
				if tr.Insert(word) {
					t.Errorf("Insert(%q) = true for a duplicate", word)
				}
			}
			if tr.Insert("") {
				t.Error("Insert(empty) = true, want false")
			}
			requireWords(t, tr, "", 0, "doc", "dock", "docs", "dog")
		})
	}
}

func TestTrieContains(t *testing.T) {
	tr := newTestTrie(t, "doc", "docs", "dog", "api")
	tests := []struct {
		word string
		want bool
	}{
		{word: "doc", want: true},
		{word: "docs", want: true},
		{word: "dog", want: true},
		{word: "api", want: true},
		{word: "", want: false},
		{word: "d", want: false},
		{word: "do", want: false},
		{word: "ap", want: false},
		{word: "dock", want: false},
		{word: "docs-extra", want: false},
		{word: "DOC", want: false},
		{word: "missing", want: false},
	}
	for _, tc := range tests {
		if got := tr.Contains(tc.word); got != tc.want {
			t.Errorf("Contains(%q) = %t, want %t", tc.word, got, tc.want)
		}
	}
}

func TestTrieWithPrefix(t *testing.T) {
	tr := newTestTrie(t, "docs", "doc", "dog", "do-it", "day", "api", "dock")
	all := []string{"api", "day", "do-it", "doc", "dock", "docs", "dog"}
	doWords := []string{"do-it", "doc", "dock", "docs", "dog"}
	tests := []struct {
		name   string
		prefix string
		limit  int
		want   []string
	}{
		{name: "zero means unlimited", prefix: "do", limit: 0, want: doWords},
		{name: "negative means unlimited", prefix: "do", limit: -3, want: doWords},
		{name: "one result", prefix: "do", limit: 1, want: []string{"do-it"}},
		{name: "two results", prefix: "do", limit: 2, want: []string{"do-it", "doc"}},
		{name: "exact limit", prefix: "do", limit: 5, want: doWords},
		{name: "larger limit", prefix: "do", limit: 10, want: doWords},
		{name: "prefix is a word", prefix: "doc", want: []string{"doc", "dock", "docs"}},
		{name: "prefix word counts toward limit", prefix: "doc", limit: 1, want: []string{"doc"}},
		{name: "leaf word", prefix: "docs", want: []string{"docs"}},
		{name: "punctuation", prefix: "do-", want: []string{"do-it"}},
		{name: "limit across branches", prefix: "d", limit: 3, want: []string{"day", "do-it", "doc"}},
		{name: "empty prefix", prefix: "", want: all},
		{name: "empty prefix with limit", prefix: "", limit: 2, want: []string{"api", "day"}},
		{name: "empty prefix negative limit", prefix: "", limit: -1, want: all},
		{name: "unknown prefix", prefix: "cat"},
		{name: "unknown prefix with limit", prefix: "cat", limit: 2},
		{name: "prefix longer than word", prefix: "docs-extra"},
		{name: "case sensitive", prefix: "DO"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			requireWords(t, tr, tc.prefix, tc.limit, tc.want...)
		})
	}

	// Limited reads must leave every stored word available for subsequent reads.
	requireWords(t, tr, "", 0, all...)
	for _, word := range all {
		if !tr.Contains(word) {
			t.Errorf("Contains(%q) = false after WithPrefix", word)
		}
	}
}

func TestTrieDelete(t *testing.T) {
	all := []string{"api", "doc", "dock", "docs", "dog"}
	tests := []struct {
		name    string
		word    string
		deleted bool
		want    []string
	}{
		{name: "word with descendants", word: "doc", deleted: true, want: []string{"api", "dock", "docs", "dog"}},
		{name: "leaf with word ancestor", word: "docs", deleted: true, want: []string{"api", "doc", "dock", "dog"}},
		{name: "sibling branch", word: "dog", deleted: true, want: []string{"api", "doc", "dock", "docs"}},
		{name: "separate branch", word: "api", deleted: true, want: []string{"doc", "dock", "docs", "dog"}},
		{name: "prefix that is not a word", word: "do", want: all},
		{name: "unknown word", word: "cat", want: all},
		{name: "word extending a leaf", word: "docs-extra", want: all},
		{name: "empty word", word: "", want: all},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := newTestTrie(t, "docs", "doc", "dog", "api", "dock")
			if got := tr.Delete(tc.word); got != tc.deleted {
				t.Fatalf("Delete(%q) = %t, want %t", tc.word, got, tc.deleted)
			}
			if tr.Contains(tc.word) {
				t.Errorf("Contains(%q) = true after Delete", tc.word)
			}
			if tr.Delete(tc.word) {
				t.Errorf("second Delete(%q) = true, want false", tc.word)
			}
			requireWords(t, tr, "", 0, tc.want...)
			for _, word := range tc.want {
				if !tr.Contains(word) {
					t.Errorf("Delete(%q) removed remaining word %q", tc.word, word)
				}
			}
		})
	}
}

func TestTrieDeleteAndReinsertPrefix(t *testing.T) {
	tr := newTestTrie(t, "doc", "docs", "dock")
	if !tr.Delete("doc") {
		t.Fatal("Delete(doc) = false, want true")
	}
	requireWords(t, tr, "doc", 1, "dock")
	requireWords(t, tr, "doc", 0, "dock", "docs")
	if !tr.Insert("doc") {
		t.Fatal("Insert(doc) = false after Delete")
	}
	requireWords(t, tr, "doc", 1, "doc")
	requireWords(t, tr, "doc", 0, "doc", "dock", "docs")
}

func TestTrieDeleteAllAndReuse(t *testing.T) {
	words := []string{"doc", "docs", "dock", "dog", "api"}
	tr := newTestTrie(t, words...)
	for _, word := range words {
		if !tr.Delete(word) {
			t.Fatalf("Delete(%q) = false, want true", word)
		}
		if tr.Contains(word) {
			t.Errorf("Contains(%q) = true after Delete", word)
		}
	}
	requireWords(t, tr, "", 0)
	requireWords(t, tr, "do", 2)
	for _, word := range words {
		if !tr.Insert(word) {
			t.Fatalf("Insert(%q) = false after deleting all words", word)
		}
	}
	requireWords(t, tr, "", 0, "api", "doc", "dock", "docs", "dog")
}

func TestTrieByteOrder(t *testing.T) {
	tr := newTestTrie(t, "éclair", "a", "Z", "\xff", "\x00", "é", "\x80")
	requireWords(t, tr, "", 0, "\x00", "Z", "a", "\x80", "é", "éclair", "\xff")
	requireWords(t, tr, "", 3, "\x00", "Z", "a")

	// A byte prefix may end inside a UTF-8 sequence.
	requireWords(t, tr, "\xc3", 0, "é", "éclair")
	requireWords(t, tr, "\xc3", 1, "é")
	if tr.Contains("\xc3") {
		t.Error("Contains(partial UTF-8 prefix) = true, want false")
	}
	requireWords(t, tr, "\x00", 0, "\x00")
	requireWords(t, tr, "\xff", 0, "\xff")
	if !tr.Delete("\xff") {
		t.Fatal("Delete(0xff) = false, want true")
	}
	requireWords(t, tr, "\xff", 0)
}

func newTestTrie(t *testing.T, words ...string) *trie.Trie {
	t.Helper()
	tr := trie.New()
	if tr == nil {
		t.Fatal("New() returned nil")
	}
	for _, word := range words {
		if !tr.Insert(word) {
			t.Fatalf("Insert(%q) = false, want true", word)
		}
	}
	return tr
}

func requireWords(t *testing.T, tr *trie.Trie, prefix string, limit int, want ...string) {
	t.Helper()
	if got := tr.WithPrefix(prefix, limit); !slices.Equal(got, want) {
		t.Fatalf("WithPrefix(%q, %d) = %q, want %q", prefix, limit, got, want)
	}
}
