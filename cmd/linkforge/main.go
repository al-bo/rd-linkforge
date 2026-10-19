package main

import (
	"context"
	"fmt"
	"os"

	"github.com/al-bo/rd-linkforge/internal/link"
	"github.com/al-bo/rd-linkforge/internal/store"
)

func main() {
	s := store.New()
	ctx := context.Background()
	service := link.NewService(s)
	owner := "demo"
	userArgs := os.Args[1:]

	for _, v := range userArgs {
		req := link.ShortenRequest{Target: v, Alias: "", OwnerID: owner}
		link, err := service.Shorten(ctx, req)

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error shorting: %q %v\n", link, err)
		}
	}

	for _, l := range s.All() {
		fmt.Printf("%s    %s \n", l.Code, l.Target)
	}

	fmt.Printf("Total: %v \n", s.Count())
}
