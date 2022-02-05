package pageant

import (
	"reflect"
	"syscall"
	"unsafe"

	"github.com/cwchiu/go-winapi"
)

const (
	className = "Pageant"
	id        = 0x804e50ba
)

type copyDataStruct struct {
	dwData uintptr
	cbData uint32
	lpData uintptr
}

func MyRegisterClass(hInstance winapi.HINSTANCE) winapi.ATOM {
	var wc winapi.WNDCLASSEX

	wc.CbSize = uint32(unsafe.Sizeof(winapi.WNDCLASSEX{}))
	wc.Style = 0
	wc.LpfnWndProc = syscall.NewCallback(WndProc)
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

func WndProc(hWnd winapi.HWND, message uint32, wParam uintptr, lParam uintptr) uintptr {
	if message == winapi.WM_COPYDATA {
		ldata := (*copyDataStruct)(unsafe.Pointer(lParam))
		handleCopyMessage(ldata)
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
var OpenFileMappingW = kernel32.NewProc("OpenFileMappingW")

func handleCopyMessage(cdata *copyDataStruct) {
	if cdata.dwData != id {
		return
	}
	m := ptr2Array(cdata.lpData, int(cdata.cbData-1))
	mapname := string(m[:cdata.cbData-1])

}

func InitInstance(hInstance winapi.HINSTANCE, nCmdShow int) bool {
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

func RunAgent() {
	hInstance := winapi.GetModuleHandle(nil)
	MyRegisterClass(hInstance)

	if InitInstance(hInstance, winapi.SW_SHOW) == false {
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
