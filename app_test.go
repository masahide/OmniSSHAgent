package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/masahide/OmniSSHAgent/pkg/agentlistener"
)

func TestAppShutdownCancelsAgents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var app App
	app.cancelAgents = cancel
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		<-ctx.Done()
	}()

	done := make(chan struct{})
	go func() {
		app.shutdown(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("shutdown did not finish in time")
	}

	if err := ctx.Err(); err != context.Canceled {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestAppShutdownStopsListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen failed: %v", err)
	}
	defer listener.Close()

	var app App
	ctx, cancel := context.WithCancel(context.Background())
	app.agentCtx = ctx
	app.cancelAgents = cancel

	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		if err := agentlistener.Serve(app.agentCtx, listener, func(ctx context.Context, c net.Conn) {
			c.Close()
		}); err != nil && err != net.ErrClosed {
			t.Fatalf("Serve returned unexpected error: %v", err)
		}
	}()

	done := make(chan struct{})
	go func() {
		app.shutdown(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not finish in time")
	}
}
