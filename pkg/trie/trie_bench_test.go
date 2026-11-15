package trie_test

import (
	"fmt"
	"testing"

	"github.com/al-bo/rd-linkforge/pkg/trie"
)

func BenchmarkTrieWithPrefix(b *testing.B) {
	const wordCount = 10000
	const prefix = "alias-0"
	tr := trie.New()
	for i := 0; i < wordCount; i++ {
		word := fmt.Sprintf("alias-%04d", i)
		if !tr.Insert(word) {
			b.Fatalf("Insert(%q) failed", word)
		}
	}

	benchmarks := []struct {
		name  string
		limit int
		want  int
	}{
		{name: "limit_10", limit: 10, want: 10},
		{name: "unlimited", limit: 0, want: 1000},
	}
	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			var result []string
			b.ReportAllocs()
			for b.Loop() {
				result = tr.WithPrefix(prefix, benchmark.limit)
			}
			if len(result) != benchmark.want {
				b.Fatalf("WithPrefix(%q, %d) returned %d words, want %d", prefix, benchmark.limit, len(result), benchmark.want)
			}
		})
	}
}
