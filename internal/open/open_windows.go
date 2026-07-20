//go:build windows

package open

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

var shellExecute = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteW")

func Path(path string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	target, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	result, _, _ := shellExecute.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(target)), 0, 0, 1)
	if result <= 32 {
		return fmt.Errorf("ShellExecuteW failed with code %d", result)
	}
	return nil
}

func Configuration(path string) error {
	if err := Path(path); err == nil {
		return nil
	}
	notepad := filepath.Join(os.Getenv("SYSTEMROOT"), "System32", "notepad.exe")
	if err := exec.Command(notepad, path).Start(); err != nil {
		return fmt.Errorf("open configuration: %w", err)
	}
	return nil
}
