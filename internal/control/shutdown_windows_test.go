//go:build windows

package control

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestShutdownEventRequest(t *testing.T) {
	name := fmt.Sprintf(`Local\OmniSSHAgent-Shutdown-Test-%d`, os.Getpid())
	event, err := NewShutdownEvent(name)
	if err != nil {
		t.Fatal(err)
	}
	defer event.Close()

	requested, err := RequestShutdown(name)
	if err != nil {
		t.Fatal(err)
	}
	if !requested {
		t.Fatal("shutdown event was not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := event.Wait(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestShutdownEventMissingAndCancellation(t *testing.T) {
	name := fmt.Sprintf(`Local\OmniSSHAgent-Shutdown-Missing-%d`, os.Getpid())
	requested, err := RequestShutdown(name)
	if err != nil {
		t.Fatal(err)
	}
	if requested {
		t.Fatal("missing shutdown event was reported as signaled")
	}

	event, err := NewShutdownEvent(name)
	if err != nil {
		t.Fatal(err)
	}
	defer event.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := event.Wait(ctx); err != context.Canceled {
		t.Fatalf("Wait error=%v, want context.Canceled", err)
	}
}
