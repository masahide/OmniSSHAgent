//go:build windows

package pageant

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/masahide/OmniSSHAgent/internal/backend"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/sys/windows"
)

const (
	ClassName        = "Pageant"
	copyDataID       = 0x804e50ba
	MaxMessageLen    = 256 * 1024
	wmCopyData       = 0x004A
	wmDestroy        = 0x0002
	wmQuit           = 0x0012
	wsPopup          = 0x80000000
	wsExTransparent  = 0x00000020
	wsExToolWindow   = 0x00000080
	wsExTopmost      = 0x00000008
	wsExNoActivate   = 0x08000000
	fileMapAllAccess = 0xF001F
)

var (
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	user32                = windows.NewLazySystemDLL("user32.dll")
	procGetModuleHandle   = kernel32.NewProc("GetModuleHandleW")
	procOpenFileMapping   = kernel32.NewProc("OpenFileMappingW")
	procMapViewOfFile     = kernel32.NewProc("MapViewOfFile")
	procUnmapViewOfFile   = kernel32.NewProc("UnmapViewOfFile")
	procRegisterClass     = user32.NewProc("RegisterClassExW")
	procCreateWindow      = user32.NewProc("CreateWindowExW")
	procFindWindow        = user32.NewProc("FindWindowW")
	procDestroyWindow     = user32.NewProc("DestroyWindow")
	procDefWindowProc     = user32.NewProc("DefWindowProcW")
	procGetMessage        = user32.NewProc("GetMessageW")
	procTranslateMessage  = user32.NewProc("TranslateMessage")
	procDispatchMessage   = user32.NewProc("DispatchMessageW")
	procPostThreadMessage = user32.NewProc("PostThreadMessageW")
	pageantMu             sync.Mutex
	active                *Component
	wndProcCallback       = windows.NewCallback(windowProc)
)

type copyDataStruct struct {
	Data    uintptr
	Size    uint32
	Pointer uintptr
}
type wndClassEx struct {
	Size        uint32
	Style       uint32
	WndProc     uintptr
	ClassExtra  int32
	WindowExtra int32
	Instance    uintptr
	Icon        uintptr
	Cursor      uintptr
	Background  uintptr
	MenuName    *uint16
	ClassName   *uint16
	IconSmall   uintptr
}
type point struct{ X, Y int32 }
type message struct {
	Window  uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   point
	Private uint32
}

type mappedMemory []byte

func (m *mappedMemory) header() *reflect.SliceHeader {
	return (*reflect.SliceHeader)(unsafe.Pointer(m))
}

func memoryAt(address uintptr, size int) (memory mappedMemory) {
	header := memory.header()
	header.Data = address
	header.Len = size
	header.Cap = size
	return memory
}

type Component struct {
	backend   backend.Backend
	logger    *slog.Logger
	ready     chan error
	readyOnce sync.Once
}

func New(b backend.Backend, logger *slog.Logger) *Component {
	return &Component{backend: b, logger: logger, ready: make(chan error, 1)}
}
func (c *Component) Name() string          { return "pageant" }
func (c *Component) Ready() <-chan error   { return c.ready }
func (c *Component) reportReady(err error) { c.readyOnce.Do(func() { c.ready <- err }) }

func (c *Component) Start(ctx context.Context) (resultErr error) {
	defer func() {
		if resultErr != nil {
			c.reportReady(resultErr)
		}
	}()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pageantMu.Lock()
	if active != nil {
		pageantMu.Unlock()
		return errors.New("Pageant interface is already active in this process")
	}
	active = c
	pageantMu.Unlock()
	defer func() { pageantMu.Lock(); active = nil; pageantMu.Unlock() }()

	instance, _, callErr := procGetModuleHandle.Call(0)
	if instance == 0 {
		return fmt.Errorf("GetModuleHandleW: %w", windowsError(callErr))
	}
	class, _ := windows.UTF16PtrFromString(ClassName)
	if existing, _, _ := procFindWindow.Call(
		uintptr(unsafe.Pointer(class)),
		uintptr(unsafe.Pointer(class)),
	); existing != 0 {
		return errors.New("Pageant window conflict: another Pageant-compatible application is running")
	}
	wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: wndProcCallback, Instance: instance, ClassName: class}
	atom, _, callErr := procRegisterClass.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		return fmt.Errorf("Pageant window class conflict: %w", windowsError(callErr))
	}
	window, _, callErr := procCreateWindow.Call(
		wsExTransparent|wsExToolWindow|wsExTopmost|wsExNoActivate,
		uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(class)), wsPopup,
		0, 0, 0, 0, 0, 0, instance, 0,
	)
	if window == 0 {
		return fmt.Errorf("create Pageant window: %w", windowsError(callErr))
	}
	defer procDestroyWindow.Call(window)
	c.reportReady(nil)

	threadID := windows.GetCurrentThreadId()
	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			procPostThreadMessage.Call(uintptr(threadID), wmQuit, 0, 0)
		case <-stop:
		}
	}()
	defer close(stop)

	var msg message
	for {
		result, _, callErr := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(result) == -1 {
			return fmt.Errorf("GetMessageW: %w", windowsError(callErr))
		}
		if result == 0 {
			return nil
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func windowProc(window uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	pageantMu.Lock()
	c := active
	pageantMu.Unlock()
	if c != nil && msg == wmCopyData {
		defer func() {
			if recovered := recover(); recovered != nil && c.logger != nil {
				c.logger.Error("Pageant request panic", "error", fmt.Sprint(recovered))
			}
		}()
		raw := memoryAt(lParam, int(unsafe.Sizeof(copyDataStruct{})))
		if err := c.handle((*copyDataStruct)(unsafe.Pointer(&raw[0]))); err != nil {
			if c.logger != nil {
				c.logger.Warn("Pageant request failed", "error", err)
			}
			return 0
		}
		return 1
	}
	if msg == wmDestroy {
		return 0
	}
	result, _, _ := procDefWindowProc.Call(window, uintptr(msg), wParam, lParam)
	return result
}

func (c *Component) handle(data *copyDataStruct) error {
	if data == nil || data.Data != copyDataID {
		return errors.New("invalid WM_COPYDATA identifier")
	}
	if data.Size < 2 || data.Size > 260 || data.Pointer == 0 {
		return errors.New("invalid shared memory name")
	}
	nameBytes := memoryAt(data.Pointer, int(data.Size))
	if nameBytes[len(nameBytes)-1] != 0 {
		return errors.New("shared memory name is not terminated")
	}
	name := string(nameBytes[:len(nameBytes)-1])
	if !strings.HasPrefix(name, "PageantRequest") {
		return errors.New("unexpected shared memory name")
	}
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	handle, _, callErr := procOpenFileMapping.Call(fileMapAllAccess, 0, uintptr(unsafe.Pointer(namePtr)))
	if handle == 0 {
		return fmt.Errorf("OpenFileMappingW: %w", windowsError(callErr))
	}
	defer windows.CloseHandle(windows.Handle(handle))
	address, _, callErr := procMapViewOfFile.Call(handle, windows.FILE_MAP_WRITE, 0, 0, 0)
	if address == 0 {
		return fmt.Errorf("MapViewOfFile: %w", windowsError(callErr))
	}
	defer procUnmapViewOfFile.Call(address)
	var memoryInfo windows.MemoryBasicInformation
	if err := windows.VirtualQuery(address, &memoryInfo, unsafe.Sizeof(memoryInfo)); err != nil {
		return fmt.Errorf("VirtualQuery: %w", err)
	}
	if memoryInfo.RegionSize < 4 {
		return errors.New("shared memory mapping is too small")
	}
	mappedSize := int(memoryInfo.RegionSize)
	mapped := memoryAt(address, mappedSize)
	length := int(binary.BigEndian.Uint32(mapped[:4]))
	if length <= 0 || length > MaxMessageLen || length+4 > mappedSize {
		return fmt.Errorf("invalid SSH agent message length %d", length)
	}
	request := append([]byte(nil), mapped[:length+4]...)
	var response bytes.Buffer
	err = agent.ServeAgent(c.backend, struct {
		io.Reader
		io.Writer
	}{bytes.NewReader(request), &response})
	if err != nil && !errors.Is(err, io.EOF) {
		response.Reset()
		// SSH_AGENT_FAILURE is message type 5.
		response.Write([]byte{0, 0, 0, 1, 5})
	}
	if response.Len() < 5 || response.Len() > len(mapped) {
		return errors.New("invalid SSH agent response length")
	}
	copy(mapped, response.Bytes())
	return nil
}

func windowsError(err error) error {
	if err == nil || errors.Is(err, windows.ERROR_SUCCESS) {
		return errors.New("Windows API call failed")
	}
	return err
}
