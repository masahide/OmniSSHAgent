//go:build windows

package autostart

import (
	"errors"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	valueName  = "OmniSSHAgent"
)

// Enabled reports whether the current executable is registered to start when
// the current user signs in to Windows.
func Enabled() (bool, error) {
	executable, err := os.Executable()
	if err != nil {
		return false, err
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer key.Close()

	command, _, err := key.GetStringValue(valueName)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return commandTargetsExecutable(command, executable), nil
}

// SetEnabled registers or unregisters the current executable for the current
// user's Windows sign-in.
func SetEnabled(enabled bool) error {
	if !enabled {
		key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		defer key.Close()
		err = key.DeleteValue(valueName)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue(valueName, quoteExecutable(executable))
}

func quoteExecutable(executable string) string {
	return `"` + executable + `"`
}

func commandTargetsExecutable(command, executable string) bool {
	command = strings.TrimSpace(command)
	return strings.EqualFold(command, executable) ||
		strings.EqualFold(command, quoteExecutable(executable))
}
