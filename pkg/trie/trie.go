package trie

import "slices"

type Trie struct {
	words []string
}

func New() *Trie {
	return &Trie{
		words: make([]string, 0),
	}
}

// Insert adds word. It returns false, and changes nothing, when word is empty
// or already there.
func (t *Trie) Insert(word string) bool {
	if word == "" {
		return false
	}

	if ok := slices.Contains(t.words, word); ok != false {
		return false
	}

	t.words = append(t.words, word)

	return true
}

func (t *Trie) Contains(word string) bool

// Delete removes word and reports whether it was there.
func (t *Trie) Delete(word string) bool

// WithPrefix returns the words that start with prefix (the prefix itself too, if it
// is a word), in ascending byte order. A limit of zero or less means no limit.
func (t *Trie) WithPrefix(prefix string, limit int) []string
