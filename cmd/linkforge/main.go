// Command linkforged is the Linkforge service binary.
package main

import (
	"fmt"
	"os"

	"github.com/skskuzan/rd-linkforge/internal/link"
	"github.com/skskuzan/rd-linkforge/internal/store"
)

// version is overwritten at build time with -ldflags "-X main.version=...".

func main() {
	userArgs := os.Args[1:]
	s := store.New()

	for i, v := range userArgs {
		l, err := link.New(uint64(i+1), v)
		if err == nil {
			fmt.Printf("%s    %s \n", l.Code, l.Target)
			store.Add(s, l)
		}
	}

	fmt.Printf("Total: %v \n", store.Count(s))
}
