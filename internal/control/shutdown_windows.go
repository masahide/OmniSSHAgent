//go:build windows

package control

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sys/windows"
)

const ShutdownEventName = `Local\OmniSSHAgent-Shutdown`

type ShutdownEvent struct {
	handle windows.Handle
}

func NewShutdownEvent(name string) (*ShutdownEvent, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, fmt.Errorf("encode shutdown event name: %w", err)
	}
	handle, err := windows.CreateEvent(nil, 1, 0, namePtr)
	if err != nil {
		return nil, fmt.Errorf("create shutdown event: %w", err)
	}
	return &ShutdownEvent{handle: handle}, nil
}

func RequestShutdown(name string) (bool, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return false, fmt.Errorf("encode shutdown event name: %w", err)
	}
	handle, err := windows.OpenEvent(windows.EVENT_MODIFY_STATE, false, namePtr)
	if err != nil {
		if err == windows.ERROR_FILE_NOT_FOUND {
			return false, nil
		}
		return false, fmt.Errorf("open shutdown event: %w", err)
	}
	defer windows.CloseHandle(handle)
	if err := windows.SetEvent(handle); err != nil {
		return false, fmt.Errorf("signal shutdown event: %w", err)
	}
	return true, nil
}

func (e *ShutdownEvent) Wait(ctx context.Context) error {
	for {
		result, err := windows.WaitForSingleObject(e.handle, 100)
		if err != nil {
			return fmt.Errorf("wait for shutdown event: %w", err)
		}
		if result == windows.WAIT_OBJECT_0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (e *ShutdownEvent) Close() error {
	if e == nil || e.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(e.handle)
	e.handle = 0
	return err
}
