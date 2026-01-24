//go:build windows
// +build windows

package pageant

import (
	"context"
	"testing"
	"time"
)

func TestStartCancelWatcherPostsQuit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	called := make(chan struct{}, 1)
	original := postQuitMessage
	postQuitMessage = func(code int32) {
		called <- struct{}{}
	}
	defer func() {
		postQuitMessage = original
	}()

	stop := startCancelWatcher(ctx)
	defer stop()

	cancel()

	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("expected postQuitMessage to be called")
	}
}
