package wintray

import (
	"crypto/rand"
	"log"
	"unsafe"

	"github.com/cwchiu/go-winapi"
	"golang.org/x/sys/windows"
)

const (
	TrayIconMsg = winapi.WM_APP + 1

	NIN_BALLOONSHOW      = 0x0402
	NIN_BALLOONTIMEOUT   = 0x0404
	NIN_BALLOONUSERCLICK = 0x0405

	// NotifyIcon flags
	NIF_GUID     = 0x00000020
	NIF_REALTIME = 0x00000040
	NIF_SHOWTIP  = 0x00000080
)

func (ti *TrayIcon) wndProc(hWnd winapi.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case TrayIconMsg:
		switch nmsg := winapi.LOWORD(uint32(lParam)); nmsg {
		case NIN_BALLOONUSERCLICK:
			map[bool]func(){
				true:  func() { log.Println("user clicked the balloon notification") },
				false: ti.BalloonClickFunc,
			}[ti.BalloonClickFunc == nil]()
		case winapi.WM_LBUTTONDOWN:
			map[bool]func(){
				true:  func() { log.Println("user clicked the tray icon") },
				false: ti.TrayClickFunc,
			}[ti.TrayClickFunc == nil]()
		}
	case winapi.WM_DESTROY:
		winapi.PostQuitMessage(0)
	default:
		r := winapi.DefWindowProc(hWnd, msg, wParam, lParam)
		return r
	}
	return 0
}

func newGUID() winapi.GUID {
	var buf [16]byte
	rand.Read(buf[:])
	return *(*winapi.GUID)(unsafe.Pointer(&buf[0]))
}

type TrayIcon struct {
	hwnd             winapi.HWND
	guid             winapi.GUID
	BalloonClickFunc func()
	TrayClickFunc    func()
}

func (ti *TrayIcon) createMainWindow() winapi.HWND {
	hInstance := winapi.GetModuleHandle(nil)

	wndClass := windows.StringToUTF16Ptr("MyWindow")

	var wcex winapi.WNDCLASSEX

	wcex.CbSize = uint32(unsafe.Sizeof(wcex))
	wcex.LpfnWndProc = windows.NewCallback(ti.wndProc)
	wcex.HInstance = hInstance
	wcex.LpszClassName = wndClass
	winapi.RegisterClassEx(&wcex)

	hwnd := winapi.CreateWindowEx(
		0,
		wndClass,
		windows.StringToUTF16Ptr("Tray Icons Example"),
		winapi.WS_OVERLAPPEDWINDOW,
		winapi.CW_USEDEFAULT,
		winapi.CW_USEDEFAULT,
		winapi.CW_USEDEFAULT, //400,
		winapi.CW_USEDEFAULT, //300,
		0,
		0,
		hInstance,
		nil)

	return hwnd
}

func (ti *TrayIcon) initData() *winapi.NOTIFYICONDATA {
	var data winapi.NOTIFYICONDATA
	data.CbSize = uint32(unsafe.Sizeof(data))
	data.UFlags = NIF_GUID
	data.HWnd = ti.hwnd
	data.GuidItem = ti.guid
	return &data
}

func (ti *TrayIcon) Dispose() {
	winapi.Shell_NotifyIcon(winapi.NIM_DELETE, ti.initData())
}

func (ti *TrayIcon) SetIcon(icon winapi.HICON) {
	data := ti.initData()
	data.UFlags |= winapi.NIF_ICON
	data.HIcon = icon
	winapi.Shell_NotifyIcon(winapi.NIM_MODIFY, data)
}

func (ti *TrayIcon) SetTooltip(tooltip string) {
	data := ti.initData()
	data.UFlags |= winapi.NIF_TIP
	copy(data.SzTip[:], windows.StringToUTF16(tooltip))
	winapi.Shell_NotifyIcon(winapi.NIM_MODIFY, data)
}

func (ti *TrayIcon) ShowBalloonNotification(title, text string) {
	data := ti.initData()
	data.UFlags |= winapi.NIF_INFO
	if title != "" {
		copy(data.SzInfoTitle[:], windows.StringToUTF16(title))
	}
	copy(data.SzInfo[:], windows.StringToUTF16(text))
	winapi.Shell_NotifyIcon(winapi.NIM_MODIFY, data)
}

func NewTrayIcon() *TrayIcon {
	ti := &TrayIcon{guid: newGUID()}
	ti.hwnd = ti.createMainWindow()
	icon := winapi.LoadIcon(winapi.GetModuleHandle(nil), winapi.MAKEINTRESOURCE(3))
	data := ti.initData()
	data.UFlags |= winapi.NIF_MESSAGE
	data.UCallbackMessage = TrayIconMsg
	winapi.Shell_NotifyIcon(winapi.NIM_ADD, data)
	ti.SetIcon(icon)
	ti.SetTooltip("ssh-agent")
	return ti
}

func (ti *TrayIcon) RunTray() {
	/*
		go func() {
			for i := 1; i <= 3; i++ {
				time.Sleep(3 * time.Second)
				ti.ShowBalloonNotification(
					fmt.Sprintf("Message %d", i),
					"This is a balloon message",
				)
			}
		}()
	*/
	//winapi.ShowWindow(hwnd, winapi.SW_SHOW)
	winapi.ShowWindow(ti.hwnd, winapi.SW_HIDE)
	var msg winapi.MSG
	for {
		r := winapi.GetMessage(&msg, 0, 0, 0)
		if r == 0 {
			ti.Dispose()
			break
		}
		winapi.TranslateMessage(&msg)
		winapi.DispatchMessage(&msg)
	}

}
