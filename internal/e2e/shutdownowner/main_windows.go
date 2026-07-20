//go:build windows && e2e

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/masahide/OmniSSHAgent/internal/control"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--legacy" {
		for {
			time.Sleep(time.Second)
		}
	}
	name := control.ShutdownEventName
	if len(os.Args) == 2 {
		name = os.Args[1]
	}
	event, err := control.NewShutdownEvent(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer event.Close()
	if err := event.Wait(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
