package wintray

import (
	"sync"
	"testing"
)

func TestTrayIconCommandQueueExecutesCommands(t *testing.T) {
	ti := NewTrayIcon()
	var mu sync.Mutex
	var order []int

	for i := 0; i < 3; i++ {
		idx := i
		ti.enqueueCommand(func() {
			mu.Lock()
			order = append(order, idx)
			mu.Unlock()
		})
	}

	for i := 0; i < 3; i++ {
		select {
		case cmd := <-ti.commandCh:
			if cmd.fn == nil {
				t.Fatalf("expected command function, got nil")
			}
			cmd.fn()
		default:
			t.Fatalf("expected command #%d in queue", i)
		}
	}

	if len(order) != 3 {
		t.Fatalf("expected 3 executed commands, got %d", len(order))
	}
	for i := range order {
		if order[i] != i {
			t.Fatalf("expected command order %v, got %v", []int{0, 1, 2}, order)
		}
	}
}

func TestTrayIconCommandQueueStopsAfterShutdown(t *testing.T) {
	ti := NewTrayIcon()
	ti.stopCommandQueue()
	ti.enqueueCommand(func() {
		t.Fatalf("should not run commands after shutdown")
	})

	select {
	case <-ti.commandCh:
		t.Fatal("expected command queue to reject new commands after shutdown")
	default:
	}
}

func TestShowBalloonNotificationEnqueuesCommand(t *testing.T) {
	ti := NewTrayIcon()
	initial := len(ti.commandCh)
	ti.ShowBalloonNotification("Title", "Message")
	if len(ti.commandCh) != initial+1 {
		t.Fatalf("expected balloon command enqueued, got queue size %d", len(ti.commandCh))
	}
	cmd := <-ti.commandCh
	if cmd.fn == nil {
		t.Fatal("expected balloon command function")
	}
}

func TestMenuItemUpdateEnqueuesCommand(t *testing.T) {
	ti := NewTrayIcon()
	item := newMenuItem(ti, "Test", "Tooltip", nil)
	item.update()
	if len(ti.commandCh) != 1 {
		t.Fatalf("expected menu update command enqueued, got %d", len(ti.commandCh))
	}
	cmd := <-ti.commandCh
	if cmd.fn == nil {
		t.Fatal("expected menu update command function")
	}
}
