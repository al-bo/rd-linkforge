package trie

import "testing"

func TestDeletePrunesUnusedNodes(t *testing.T) {
	tr := New()
	for _, word := range []string{"api", "app", "dog"} {
		if !tr.Insert(word) {
			t.Fatalf("Insert(%q) = false", word)
		}
	}

	if !tr.Delete("api") {
		t.Fatal("Delete(api) = false")
	}
	if !tr.Contains("app") || !tr.Contains("dog") {
		t.Fatal("deleting api removed another word")
	}
	if tr.root.children['a'].children['p'].children['i'] != nil {
		t.Fatal("deleted leaf is still reachable")
	}

	if !tr.Delete("app") {
		t.Fatal("Delete(app) = false")
	}
	if tr.root.children['a'] != nil {
		t.Fatal("empty shared branch is still reachable")
	}
	if !tr.Contains("dog") {
		t.Fatal("deleting app removed dog")
	}

	if !tr.Delete("dog") {
		t.Fatal("Delete(dog) = false")
	}
	for b, child := range tr.root.children {
		if child != nil {
			t.Fatalf("empty trie retains root child %d", b)
		}
	}
}
