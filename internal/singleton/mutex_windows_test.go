//go:build windows

package singleton

import (
	"errors"
	"fmt"
	"os"
	"testing"
)

func TestAcquireConflictAndRelease(t *testing.T) {
	name := fmt.Sprintf(`Local\OmniSSHAgent-Test-%d`, os.Getpid())
	first, err := Acquire(name)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(name)
	if !errors.Is(err, ErrAlreadyRunning) || second != nil {
		t.Fatalf("second=%v err=%v", second, err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := Acquire(name)
	if err != nil {
		t.Fatal(err)
	}
	_ = third.Close()
}
