package main

import (
	"fmt"
	"os"

	"experimental-clicker/license"
)

func main() {
	priv, pub := license.KeyPaths()
	if err := license.EnsureKeyPair(priv, pub); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
