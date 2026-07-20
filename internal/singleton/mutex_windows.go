//go:build windows

package singleton

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

var ErrAlreadyRunning = errors.New("OmniSSHAgent is already running")

type Mutex struct{ handle windows.Handle }

func Acquire(name string) (*Mutex, error) {
	ptr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateMutex(nil, false, ptr)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return nil, ErrAlreadyRunning
	}
	if err != nil {
		return nil, fmt.Errorf("create singleton mutex: %w", err)
	}
	if windows.GetLastError() == windows.ERROR_ALREADY_EXISTS {
		windows.CloseHandle(h)
		return nil, ErrAlreadyRunning
	}
	return &Mutex{handle: h}, nil
}
func (m *Mutex) Close() error {
	if m == nil || m.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(m.handle)
	m.handle = 0
	return err
}
