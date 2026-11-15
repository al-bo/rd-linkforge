// Package trie provides a byte-based prefix index for autocomplete.
package trie

type node struct {
	children [256]*node
	isWord   bool
}

// Trie stores non-empty words in a byte-based prefix tree.
// Its zero value is ready to use. It is not safe for concurrent use.
type Trie struct {
	root node
}

// New returns an empty trie.
func New() *Trie {
	return &Trie{}
}

// Insert adds word. It returns false, and changes nothing, when word is empty
// or already there.
func (t *Trie) Insert(word string) bool {
	if word == "" {
		return false
	}

	current := &t.root

	for i := 0; i < len(word); i++ {
		b := word[i]

		if current.children[b] == nil {
			current.children[b] = &node{}
		}

		current = current.children[b]
	}

	if current.isWord {
		return false
	}
	current.isWord = true

	return true
}

// Contains reports whether word is stored as a complete word.
func (t *Trie) Contains(word string) bool {
	current := &t.root

	for i := 0; i < len(word); i++ {
		b := word[i]

		if current.children[b] == nil {
			return false
		}

		current = current.children[b]
	}

	return current.isWord
}

// Delete removes word, prunes unused branches, and reports whether it was there.
func (t *Trie) Delete(word string) bool {
	current := &t.root
	parents := make([]*node, 0, len(word))

	for i := 0; i < len(word); i++ {
		b := word[i]

		if current.children[b] == nil {
			return false
		}

		parents = append(parents, current)
		current = current.children[b]
	}

	if !current.isWord {
		return false
	}

	current.isWord = false

	for i := len(word) - 1; i >= 0; i-- {
		if current.isWord || current.hasChildren() {
			break
		}
		parent := parents[i]
		parent.children[word[i]] = nil
		current = parent
	}

	return true
}

func (n *node) hasChildren() bool {
	for _, child := range &n.children {
		if child != nil {
			return true
		}
	}
	return false
}

// WithPrefix returns the words that start with prefix (the prefix itself too, if it
// is a word), in ascending byte order. A limit of zero or less means no limit.
func (t *Trie) WithPrefix(prefix string, limit int) []string {
	var result []string

	current := &t.root

	for i := 0; i < len(prefix); i++ {
		b := prefix[i]

		if current.children[b] == nil {
			return result
		}

		current = current.children[b]
	}

	if current.isWord {
		result = append(result, prefix)
		if limit == 1 {
			return result
		}
	}

	buf := []byte(prefix)
	var process func(n *node)

	process = func(n *node) {
		for i, child := range &n.children {
			if limit > 0 && len(result) >= limit {
				return
			}

			if child == nil {
				continue
			}

			buf = append(buf, byte(i))

			if child.isWord {
				result = append(result, string(buf))
			}

			process(child)
			buf = buf[:len(buf)-1]
		}
	}

	process(current)

	return result
}
