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

	called := make(chan uint32, 1)
	original := postQuitMessage
	postQuitMessage = func(threadID uint32) bool {
		called <- threadID
		return true
	}
	defer func() {
		postQuitMessage = original
	}()

	const threadID = 0x1234
	stop := startCancelWatcher(ctx, threadID)
	defer stop()

	cancel()

	select {
	case tid := <-called:
		if tid != threadID {
			t.Fatalf("unexpected thread id %d", tid)
		}
	case <-time.After(time.Second):
		t.Fatal("expected postQuitMessage to be called")
	}
}
