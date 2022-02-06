package pageant

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"syscall"
	"unsafe"

	"github.com/cwchiu/go-winapi"
	"golang.org/x/crypto/ssh/agent"
)

const (
	className = "Pageant"
	id        = 0x804e50ba
)

type Pageant struct {
	agent.Agent
	Debug bool
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
	if cdata.dwData != id {
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
	err := agent.ServeAgent(a,
		struct {
			io.Reader
			io.Writer
		}{bytes.NewBuffer(m), &out},
	)
	if err == nil {
		return fmt.Errorf("ServeAgent err:%v", err)
	}
	if err != io.EOF {
		return fmt.Errorf("ServeAgent err:%w", err)
	}
	m = ptr2Array(addr, out.Len())
	copy(m, out.Bytes()[:out.Len()])
	return nil
}

func initInstance(hInstance winapi.HINSTANCE, nCmdShow int) bool {
	hWnd := winapi.CreateWindowEx(
		winapi.WS_EX_TRANSPARENT|winapi.WS_EX_TOOLWINDOW|winapi.WS_EX_TOPMOST|winapi.WS_EX_NOACTIVATE,
		syscall.StringToUTF16Ptr(className),
		syscall.StringToUTF16Ptr(className),
		winapi.WS_POPUP,
		0, 0, 0, 0,
		0, 0, hInstance, nil)
	if hWnd == 0 {
		return false
	}

	winapi.ShowWindow(hWnd, winapi.SW_SHOW)
	return true
}

func (a *Pageant) RunAgent() {
	hInstance := winapi.GetModuleHandle(nil)
	a.myRegisterClass(hInstance)

	if initInstance(hInstance, winapi.SW_SHOW) == false {
		return
	}
	var msg winapi.MSG
	for winapi.GetMessage(&msg, 0, 0, 0) != 0 {
		winapi.TranslateMessage(&msg)
		winapi.DispatchMessage(&msg)
		if msg.Message == winapi.WM_QUIT {
			break
		}
	}
}
