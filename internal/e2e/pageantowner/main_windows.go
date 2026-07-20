//go:build windows && e2e

package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"

	"github.com/masahide/OmniSSHAgent/internal/interfaces/pageant"
	"github.com/masahide/OmniSSHAgent/internal/testutil"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	component := pageant.New(testutil.NewBackend(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := component.Start(ctx); err != nil {
		os.Exit(1)
	}
}
