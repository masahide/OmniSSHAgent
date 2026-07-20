package pageant

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/cwchiu/go-winapi"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/sys/windows"
)

const (
	className = "Pageant"
	id        = 0x804e50ba
	checkID   = 0x02e08fc7
)

var (
	procPostThreadMessage = syscall.NewLazyDLL("user32.dll").NewProc("PostThreadMessageW")
	postQuitMessage       = func(threadID uint32) bool {
		ret, _, _ := procPostThreadMessage.Call(uintptr(threadID), uintptr(winapi.WM_QUIT), 0, 0)
		return ret != 0
	}
)

type Pageant struct {
	agent.ExtendedAgent
	Debug     bool
	AppName   string
	CheckFunc func()
}

type copyDataStruct struct {
	dwData uintptr
	cbData uint32
	lpData uintptr
}

func (a *Pageant) myRegisterClass(hInstance winapi.HINSTANCE) winapi.ATOM {
	var wc winapi.WNDCLASSEX

	wc.CbSize = uint32(unsafe.Sizeof(winapi.WNDCLASSEX{}))
	wc.Style = 0
	wc.LpfnWndProc = syscall.NewCallback(a.wndProc)
	wc.CbClsExtra = 0
	wc.CbWndExtra = 0
	wc.HInstance = hInstance
	wc.HIcon = winapi.LoadIcon(hInstance, winapi.MAKEINTRESOURCE(132))
	wc.HCursor = winapi.LoadCursor(0, winapi.MAKEINTRESOURCE(winapi.IDC_CROSS))
	wc.HbrBackground = 0
	wc.LpszMenuName = nil
	wc.LpszClassName, _ = syscall.UTF16PtrFromString(className)

	return winapi.RegisterClassEx(&wc)
}

func (a *Pageant) wndProc(hWnd winapi.HWND, message uint32, wParam uintptr, lParam uintptr) uintptr {
	if message == winapi.WM_COPYDATA {
		err := a.handleCopyMessage((*copyDataStruct)(unsafe.Pointer(lParam)))
		if err != nil {
			log.Print(err)
			return 0
		}
		return 1
	}
	return winapi.DefWindowProc(hWnd, uint32(message), wParam, lParam)
}

type mMap []byte

func (m *mMap) header() *reflect.SliceHeader { return (*reflect.SliceHeader)(unsafe.Pointer(m)) }

func ptr2Array(addr uintptr, sz int) (m mMap) {
	dh := m.header()
	dh.Data = addr
	dh.Len = sz
	dh.Cap = dh.Len
	return
}

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var openFileMappingW = kernel32.NewProc("OpenFileMappingW")
var fileMapAllAccess = uint32(0xF001F)
var zeroUint32 = uint32(0)

func (a *Pageant) handleCopyMessage(cdata *copyDataStruct) error {
	checkMode := false
	switch cdata.dwData {
	case id:
		break
	case checkID:
		checkMode = true
	default:
		return errors.New("ID is different")
	}
	if a.Debug {
		log.Println("Pageant: received message")
	}

	m := ptr2Array(cdata.lpData, int(cdata.cbData-1))
	mapname := string(m[:cdata.cbData-1])
	ret, _, _ := openFileMappingW.Call(
		uintptr(fileMapAllAccess),
		uintptr(zeroUint32),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(mapname))))
	h := syscall.Handle(ret)
	if h == 0 {
		return errors.New("err:OpenFileMappingW")
	}
	defer syscall.CloseHandle(h)
	addr, errno := syscall.MapViewOfFile(h, uint32(syscall.FILE_MAP_WRITE), 0, 0, 0)
	if addr == 0 {
		return fmt.Errorf("Failed: %s", os.NewSyscallError("MapViewOfFile", errno))
	}
	m = ptr2Array(addr, 4) // message size
	buf := bytes.NewBuffer(m)
	ln := int32(0)
	binary.Read(buf, binary.BigEndian, &ln)
	m = ptr2Array(addr, int(ln)+4) // read ssh-agent message

	out := bytes.Buffer{}
	if checkMode {
		if a.CheckFunc != nil {
			a.CheckFunc()
		}
		b := []byte(a.AppName)
		//out.WriteString(a.AppName)
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(b)))
		out.Write(length[:])
		out.Write(b)
		m = ptr2Array(addr, out.Len())
		copy(m, out.Bytes()[:out.Len()])
		return nil
	}
	err := agent.ServeAgent(a,
		struct {
			io.Reader
			io.Writer
		}{bytes.NewBuffer(m), &out},
	)
	if err != nil && err != io.EOF {
		return fmt.Errorf("ServeAgent err:%w", err)
	}
	m = ptr2Array(addr, out.Len())
	copy(m, out.Bytes()[:out.Len()])
	return nil
}

func initInstance(hInstance winapi.HINSTANCE, nCmdShow int) error {
	classNameUTF16, err := syscall.UTF16PtrFromString(className)
	if err != nil {
		return err
	}

	hWnd := winapi.CreateWindowEx(
		winapi.WS_EX_TRANSPARENT|winapi.WS_EX_TOOLWINDOW|winapi.WS_EX_TOPMOST|winapi.WS_EX_NOACTIVATE,
		classNameUTF16,
		classNameUTF16,
		winapi.WS_POPUP,
		0, 0, 0, 0,
		0, 0, hInstance, nil)
	if hWnd == 0 {
		return errors.New("cannot create window")
	}

	winapi.ShowWindow(hWnd, winapi.SW_SHOW)
	return nil
}

func (a *Pageant) RunAgent(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hInstance := winapi.GetModuleHandle(nil)
	a.myRegisterClass(hInstance)

	if err := initInstance(hInstance, winapi.SW_SHOW); err != nil {
		log.Printf("pageant: %v\n", err)
		return
	}

	threadID := windows.GetCurrentThreadId()
	stopWatcher := startCancelWatcher(ctx, threadID)
	defer stopWatcher()

	msg := (*winapi.MSG)(unsafe.Pointer(winapi.GlobalAlloc(0, unsafe.Sizeof(winapi.MSG{}))))
	defer winapi.GlobalFree(winapi.HGLOBAL(unsafe.Pointer(msg)))
	for winapi.GetMessage(msg, 0, 0, 0) != 0 {
		winapi.TranslateMessage(msg)
		winapi.DispatchMessage(msg)
		if msg.Message == winapi.WM_QUIT {
			break
		}
	}

	runtime.KeepAlive(&msg)
}

func startCancelWatcher(ctx context.Context, threadID uint32) func() {
	if ctx == nil {
		ctx = context.Background()
	}
	exitCh := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			postQuitMessage(threadID)
		case <-exitCh:
		}
	}()
	return func() {
		close(exitCh)
	}
}
