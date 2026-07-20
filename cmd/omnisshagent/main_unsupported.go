//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "OmniSSHAgent is supported only on Windows")
	os.Exit(7)
}
