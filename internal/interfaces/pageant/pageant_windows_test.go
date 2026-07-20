//go:build windows

package pageant

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"
	"unsafe"

	"github.com/masahide/OmniSSHAgent/internal/app"
	"github.com/masahide/OmniSSHAgent/internal/interfaces"
	"github.com/masahide/OmniSSHAgent/internal/testutil"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/sys/windows"
)

type failingListAgent struct{ agent.ExtendedAgent }

func (f failingListAgent) List() ([]*agent.Key, error) {
	return nil, errors.New("backend unavailable")
}

func TestSharedMemoryRequestAndBackendFailure(t *testing.T) {
	keyring := agent.NewKeyring().(agent.ExtendedAgent)
	for _, test := range []struct {
		name         string
		backend      agent.ExtendedAgent
		responseType byte
	}{
		{"list", keyring, 12},
		{"backend failure", failingListAgent{keyring}, 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			name := fmt.Sprintf("PageantRequest%08x%08x", os.Getpid(), time.Now().UnixNano())
			namePtr, err := windows.UTF16PtrFromString(name)
			if err != nil {
				t.Fatal(err)
			}
			mapping, err := windows.CreateFileMapping(windows.InvalidHandle, nil, windows.PAGE_READWRITE, 0, 8196, namePtr)
			if err != nil {
				t.Fatal(err)
			}
			defer windows.CloseHandle(mapping)
			address, err := windows.MapViewOfFile(mapping, windows.FILE_MAP_WRITE, 0, 0, 8196)
			if err != nil {
				t.Fatal(err)
			}
			defer windows.UnmapViewOfFile(address)
			memory := memoryAt(address, 8196)
			copy(memory, []byte{0, 0, 0, 1, 11})
			nameBytes := append([]byte(name), 0)
			component := New(test.backend, slog.New(slog.NewTextHandler(io.Discard, nil)))
			err = component.handle(&copyDataStruct{
				Data:    copyDataID,
				Size:    uint32(len(nameBytes)),
				Pointer: uintptr(unsafe.Pointer(&nameBytes[0])),
			})
			if err != nil {
				t.Fatal(err)
			}
			length := binary.BigEndian.Uint32(memory[:4])
			if length == 0 || memory[4] != test.responseType {
				t.Fatalf("length=%d type=%d", length, memory[4])
			}
		})
	}
}

func TestRejectsInvalidCopyData(t *testing.T) {
	component := New(agent.NewKeyring().(agent.ExtendedAgent), nil)
	for _, data := range []*copyDataStruct{
		nil,
		{Data: 123},
		{Data: copyDataID, Size: 1},
		{Data: copyDataID, Size: 300},
	} {
		if err := component.handle(data); err == nil {
			t.Fatalf("accepted %#v", data)
		}
	}
}

func TestStartReportsReadyConflictAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	component := New(agent.NewKeyring().(agent.ExtendedAgent), nil)
	done := make(chan error, 1)
	go func() { done <- component.Start(ctx) }()
	select {
	case err := <-component.Ready():
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Pageant did not become ready")
	}
	className, _ := windows.UTF16PtrFromString(ClassName)
	findWindow := windows.NewLazySystemDLL("user32.dll").NewProc("FindWindowW")
	sendMessage := windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageW")
	window, _, _ := findWindow.Call(uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(className)))
	if window == 0 {
		t.Fatal("Pageant window not found")
	}
	mappingName := fmt.Sprintf("PageantRequest%08x-live", os.Getpid())
	mappingNamePtr, _ := windows.UTF16PtrFromString(mappingName)
	mapping, err := windows.CreateFileMapping(windows.InvalidHandle, nil, windows.PAGE_READWRITE, 0, 8196, mappingNamePtr)
	if err != nil {
		t.Fatal(err)
	}
	address, err := windows.MapViewOfFile(mapping, windows.FILE_MAP_WRITE, 0, 0, 8196)
	if err != nil {
		windows.CloseHandle(mapping)
		t.Fatal(err)
	}
	memory := memoryAt(address, 8196)
	copy(memory, []byte{0, 0, 0, 1, 11})
	mappingNameBytes := append([]byte(mappingName), 0)
	copyData := copyDataStruct{Data: copyDataID, Size: uint32(len(mappingNameBytes)), Pointer: uintptr(unsafe.Pointer(&mappingNameBytes[0]))}
	result, _, _ := sendMessage.Call(window, wmCopyData, 0, uintptr(unsafe.Pointer(&copyData)))
	if result != 1 || memory[4] != 12 {
		t.Fatalf("WM_COPYDATA result=%d response=%d", result, memory[4])
	}
	_ = windows.UnmapViewOfFile(address)
	_ = windows.CloseHandle(mapping)

	conflict := New(agent.NewKeyring().(agent.ExtendedAgent), nil)
	application := app.New(
		[]interfaces.Component{conflict, &testutil.Component{ComponentName: "cygwin"}},
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	application.Run(context.Background())
	if application.State() != app.StateDegraded {
		t.Fatalf("state=%s", application.State())
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	if err := application.Shutdown(shutdownCtx); err != nil {
		shutdownCancel()
		t.Fatal(err)
	}
	shutdownCancel()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Pageant did not stop")
	}
}

func TestRejectsOversizedMappedRequest(t *testing.T) {
	name := fmt.Sprintf("PageantRequest%08x-oversized", os.Getpid())
	namePtr, _ := windows.UTF16PtrFromString(name)
	mapping, err := windows.CreateFileMapping(windows.InvalidHandle, nil, windows.PAGE_READWRITE, 0, 8196, namePtr)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(mapping)
	address, err := windows.MapViewOfFile(mapping, windows.FILE_MAP_WRITE, 0, 0, 8196)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.UnmapViewOfFile(address)
	memory := memoryAt(address, 8196)
	binary.BigEndian.PutUint32(memory[:4], MaxMessageLen+1)
	nameBytes := append([]byte(name), 0)
	component := New(agent.NewKeyring().(agent.ExtendedAgent), nil)
	if err := component.handle(&copyDataStruct{Data: copyDataID, Size: uint32(len(nameBytes)), Pointer: uintptr(unsafe.Pointer(&nameBytes[0]))}); err == nil {
		t.Fatal("oversized request was accepted")
	}
}
