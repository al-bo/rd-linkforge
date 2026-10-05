// Command linkforged is the Linkforge service binary.
package main

import (
	"fmt"
	"os"
)

// version is overwritten at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	userArgs := os.Args[1:]
	fmt.Printf("linkforged %s\n", version)
	fmt.Println(userArgs)

}
