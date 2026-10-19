package main

import (
	"fmt"
	"os"

	"github.com/skskuzan/rd-linkforge/internal/link"
	"github.com/skskuzan/rd-linkforge/internal/store"
)

func main() {
	userArgs := os.Args[1:]
	s := store.New()
	exitCode := 0

	for i, v := range userArgs {
		l, err := link.New(uint64(i+1), v)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create link for %q: %v\n", v, err)
			exitCode = 1
			continue
		}
		if err := store.Add(s, l); err != nil {
			fmt.Fprintf(os.Stderr, "store link for %q: %v\n", v, err)
			exitCode = 1
			continue
		}
	}

	for _, l := range store.All(s) {
		fmt.Printf("%s    %s \n", l.Code, l.Target)
	}

	fmt.Printf("Total: %v \n", store.Count(s))
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
