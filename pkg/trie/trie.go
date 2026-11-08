package trie

type node struct {
	children [256]*node
	isWord   bool
}

type Trie struct {
	root node
}

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

		if i == len(word)-1 {
			if current.isWord {
				return false
			}
			current.isWord = true
		}
	}

	return true
}

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

// Delete removes word and reports whether it was there.
func (t *Trie) Delete(word string) bool {
	current := &t.root

	for i := 0; i < len(word); i++ {
		b := word[i]

		if current.children[b] == nil {
			return false
		}

		current = current.children[b]
	}

	if current.isWord == false {
		return false
	}

	current.isWord = false

	return true
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
	}

	var process func(n *node, str string)

	process = func(n *node, str string) {
		for i, child := range n.children {
			if limit > 0 && len(result) >= limit {
				return
			}

			if child == nil {
				continue
			}

			s := str + string([]byte{byte(i)})

			if child.isWord {
				result = append(result, s)
			}

			process(child, s)
		}
	}

	process(current, prefix)

	return result
}
