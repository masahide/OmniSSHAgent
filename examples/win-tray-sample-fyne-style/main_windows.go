//go:build windows

package main

import (
	"embed"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The icon bytes are stored in the executable by the Go linker.
//
//go:embed assets/tray.ico
var assets embed.FS

const (
	windowClassName = "WinTraySampleWindowClass"
	windowTitle     = "Win Tray Sample"
	tooltipText     = "Win Tray Sample"

	trayIconID = 1

	menuNotify = 1001
	menuAlert  = 1002
	menuAbout  = 1003
	menuQuit   = 1004
)

const (
	wmDestroy   = 0x0002
	wmCommand   = 0x0111
	wmLButtonUp = 0x0202
	wmRButtonUp = 0x0205
	wmUser      = 0x0400
	wmTrayIcon  = wmUser + 1
)

const (
	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002
)

const (
	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004
	nifInfo    = 0x00000010
)

const niifInfo = 0x00000001

const (
	wsCaption     = 0x00C00000
	wsMaximizeBox = 0x00010000
	wsMinimizeBox = 0x00020000
	wsOverlapped  = 0x00000000
	wsSysMenu     = 0x00080000
	wsThickFrame  = 0x00040000

	wsOverlappedWindow = wsOverlapped | wsCaption | wsSysMenu | wsThickFrame | wsMinimizeBox | wsMaximizeBox
	cwUseDefault       = 0x80000000
	swHide             = 0
)

const (
	mfString    = 0x00000000
	mfSeparator = 0x00000800
)

const (
	tpmLeftAlign   = 0x0000
	tpmBottomAlign = 0x0020
)

const (
	mbOK           = 0x00000000
	mbIconInfo     = 0x00000040
	mbIconError    = 0x00000010
	lrDefaultColor = 0x00000000
)

const (
	smCXSmallIcon = 49
	smCYSmallIcon = 50
)

var (
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	user32   = windows.NewLazySystemDLL("user32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")

	procGetModuleHandleW         = kernel32.NewProc("GetModuleHandleW")
	procRegisterClassExW         = user32.NewProc("RegisterClassExW")
	procCreateWindowExW          = user32.NewProc("CreateWindowExW")
	procDefWindowProcW           = user32.NewProc("DefWindowProcW")
	procDestroyWindow            = user32.NewProc("DestroyWindow")
	procShowWindow               = user32.NewProc("ShowWindow")
	procUpdateWindow             = user32.NewProc("UpdateWindow")
	procGetMessageW              = user32.NewProc("GetMessageW")
	procTranslateMessage         = user32.NewProc("TranslateMessage")
	procDispatchMessageW         = user32.NewProc("DispatchMessageW")
	procPostQuitMessage          = user32.NewProc("PostQuitMessage")
	procRegisterWindowMessageW   = user32.NewProc("RegisterWindowMessageW")
	procCreatePopupMenu          = user32.NewProc("CreatePopupMenu")
	procAppendMenuW              = user32.NewProc("AppendMenuW")
	procDestroyMenu              = user32.NewProc("DestroyMenu")
	procGetCursorPos             = user32.NewProc("GetCursorPos")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procTrackPopupMenu           = user32.NewProc("TrackPopupMenu")
	procMessageBoxW              = user32.NewProc("MessageBoxW")
	procGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	procCreateIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")
	procDestroyIcon              = user32.NewProc("DestroyIcon")
	procShellNotifyIconW         = shell32.NewProc("Shell_NotifyIconW")

	wndProcCallback = windows.NewCallback(windowProc)
	currentApp      *trayApp
)

type point struct {
	X int32
	Y int32
}

type message struct {
	HWnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       point
	LPrivate uint32
}

type wndClassEx struct {
	CbSize     uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type notifyIconData struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	Version          uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     uintptr
}

type trayApp struct {
	hInstance         uintptr
	hWnd              uintptr
	hIcon             uintptr
	hMenu             uintptr
	className         *uint16
	taskbarCreatedMsg uint32
	nid               notifyIconData
	trayIconIsAdded   bool
}

func main() {
	// A Win32 message queue belongs to an OS thread. Keep all window creation
	// and message dispatching on one thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := run(); err != nil {
		showMessageBox(0, "Startup failed", err.Error(), mbOK|mbIconError)
	}
}

func run() error {
	iconBytes, err := assets.ReadFile("assets/tray.ico")
	if err != nil {
		return fmt.Errorf("read embedded icon: %w", err)
	}

	app := &trayApp{}
	currentApp = app
	defer func() { currentApp = nil }()
	defer app.close()

	if err := app.initialize(iconBytes); err != nil {
		return err
	}

	if err := app.addTrayIcon(); err != nil {
		return err
	}

	return app.messageLoop()
}

func (a *trayApp) initialize(iconBytes []byte) error {
	var err error

	a.hInstance, err = getModuleHandle()
	if err != nil {
		return fmt.Errorf("GetModuleHandleW: %w", err)
	}

	a.className, err = windows.UTF16PtrFromString(windowClassName)
	if err != nil {
		return fmt.Errorf("encode window class name: %w", err)
	}

	wc := wndClassEx{
		CbSize:    uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:   wndProcCallback,
		Instance:  a.hInstance,
		ClassName: a.className,
	}
	if err := registerClass(&wc); err != nil {
		return fmt.Errorf("RegisterClassExW: %w", err)
	}

	windowName, err := windows.UTF16PtrFromString(windowTitle)
	if err != nil {
		return fmt.Errorf("encode window title: %w", err)
	}

	a.hWnd, err = createHiddenWindow(a.className, windowName, a.hInstance)
	if err != nil {
		return fmt.Errorf("CreateWindowExW: %w", err)
	}

	taskbarMessageName, err := windows.UTF16PtrFromString("TaskbarCreated")
	if err != nil {
		return fmt.Errorf("encode TaskbarCreated: %w", err)
	}
	a.taskbarCreatedMsg, err = registerWindowMessage(taskbarMessageName)
	if err != nil {
		return fmt.Errorf("RegisterWindowMessageW: %w", err)
	}

	iconWidth := getSystemMetric(smCXSmallIcon)
	iconHeight := getSystemMetric(smCYSmallIcon)
	if iconWidth <= 0 || iconHeight <= 0 {
		iconWidth, iconHeight = 16, 16
	}

	a.hIcon, err = createIconFromICO(iconBytes, iconWidth, iconHeight)
	if err != nil {
		return fmt.Errorf("create icon from embedded ICO: %w", err)
	}

	a.nid = notifyIconData{
		CbSize:           uint32(unsafe.Sizeof(notifyIconData{})),
		HWnd:             a.hWnd,
		UID:              trayIconID,
		UFlags:           nifMessage | nifIcon | nifTip,
		UCallbackMessage: wmTrayIcon,
		HIcon:            a.hIcon,
	}
	copyUTF16(a.nid.SzTip[:], tooltipText)

	a.hMenu, err = a.createTrayMenu()
	if err != nil {
		return fmt.Errorf("create tray menu: %w", err)
	}

	return nil
}

func (a *trayApp) addTrayIcon() error {
	if a.trayIconIsAdded {
		_ = a.removeTrayIcon()
	}

	if err := shellNotifyIcon(nimAdd, &a.nid); err != nil {
		return fmt.Errorf("Shell_NotifyIconW NIM_ADD: %w", err)
	}
	a.trayIconIsAdded = true

	// fyne-io/systray keeps the legacy callback contract instead of calling
	// NIM_SETVERSION. In that contract lParam is the complete mouse message.
	return nil
}

func (a *trayApp) removeTrayIcon() error {
	if !a.trayIconIsAdded {
		return nil
	}
	if err := shellNotifyIcon(nimDelete, &a.nid); err != nil {
		return err
	}
	a.trayIconIsAdded = false
	return nil
}

func (a *trayApp) close() {
	_ = a.removeTrayIcon()

	if a.hMenu != 0 {
		_, _, _ = procDestroyMenu.Call(a.hMenu)
		a.hMenu = 0
	}
	if a.hWnd != 0 {
		_, _, _ = procDestroyWindow.Call(a.hWnd)
		a.hWnd = 0
	}
	if a.hIcon != 0 {
		_, _, _ = procDestroyIcon.Call(a.hIcon)
		a.hIcon = 0
	}
}

func (a *trayApp) messageLoop() error {
	var msg message
	for {
		r1, _, callErr := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0,
			0,
			0,
		)

		switch int32(r1) {
		case -1:
			return windowsCallError(callErr)
		case 0:
			return nil
		default:
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}
}

func (a *trayApp) createTrayMenu() (uintptr, error) {
	menu, err := createPopupMenu()
	if err != nil {
		return 0, err
	}

	ok := false
	defer func() {
		if !ok {
			_, _, _ = procDestroyMenu.Call(menu)
		}
	}()

	if err := appendMenu(menu, mfString, menuNotify, "Show notification"); err != nil {
		return 0, err
	}
	if err := appendMenu(menu, mfString, menuAlert, "Show alert dialog"); err != nil {
		return 0, err
	}
	if err := appendMenu(menu, mfSeparator, 0, ""); err != nil {
		return 0, err
	}
	if err := appendMenu(menu, mfString, menuAbout, "About"); err != nil {
		return 0, err
	}
	if err := appendMenu(menu, mfString, menuQuit, "Quit"); err != nil {
		return 0, err
	}

	ok = true
	return menu, nil
}

func (a *trayApp) showContextMenu() {
	if a.hMenu == 0 {
		return
	}

	var cursor point
	if err := getCursorPos(&cursor); err != nil {
		showMessageBox(a.hWnd, windowTitle, err.Error(), mbOK|mbIconError)
		return
	}

	// This intentionally follows fyne-io/systray's Windows implementation:
	// make the hidden owner window foreground, then let TrackPopupMenu dispatch
	// WM_COMMAND to that window. Do not use TPM_RETURNCMD, TPM_NONOTIFY, a
	// topmost owner, or a custom popup window.
	procSetForegroundWindow.Call(a.hWnd)
	procTrackPopupMenu.Call(
		a.hMenu,
		tpmBottomAlign|tpmLeftAlign,
		uintptr(cursor.X),
		uintptr(cursor.Y),
		0,
		a.hWnd,
		0,
	)
}

func (a *trayApp) handleMenuCommand(command uintptr) {
	switch command {
	case menuNotify:
		if err := a.showBalloonNotification(
			windowTitle,
			"The tray application is running normally.",
		); err != nil {
			showMessageBox(a.hWnd, windowTitle, err.Error(), mbOK|mbIconError)
		}

	case menuAlert:
		showMessageBox(
			a.hWnd,
			"Alert",
			"This dialog was opened from the notification-area menu.",
			mbOK|mbIconInfo,
		)

	case menuAbout:
		showMessageBox(
			a.hWnd,
			windowTitle,
			"The application is running in the Windows notification area.",
			mbOK|mbIconInfo,
		)

	case menuQuit:
		procDestroyWindow.Call(a.hWnd)
	}
}

func (a *trayApp) showBalloonNotification(title, text string) error {
	// NIF_INFO uses the Timeout/Version union as a timeout value. Work with a
	// copy so the tray icon registration data remains unchanged.
	nid := a.nid
	nid.UFlags = nifInfo
	nid.Version = 10_000
	nid.DwInfoFlags = niifInfo
	copyUTF16(nid.SzInfoTitle[:], title)
	copyUTF16(nid.SzInfo[:], text)

	if err := shellNotifyIcon(nimModify, &nid); err != nil {
		return fmt.Errorf("Shell_NotifyIconW NIM_MODIFY notification: %w", err)
	}
	return nil
}

func windowProc(hWnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	a := currentApp
	if a != nil {
		if msg == a.taskbarCreatedMsg && msg != 0 {
			// Explorer was restarted. The notification icon must be added again.
			a.trayIconIsAdded = false
			_ = a.addTrayIcon()
			return 0
		}

		switch msg {
		case wmTrayIcon:
			// Match fyne-io/systray's legacy Shell_NotifyIcon callback contract.
			switch uint32(lParam) {
			case wmLButtonUp, wmRButtonUp:
				a.showContextMenu()
			}
			return 0

		case wmCommand:
			a.handleMenuCommand(uintptr(uint16(wParam & 0xffff)))
			return 0

		case wmDestroy:
			_ = a.removeTrayIcon()
			a.hWnd = 0
			procPostQuitMessage.Call(0)
			return 0
		}
	}

	r1, _, _ := procDefWindowProcW.Call(hWnd, uintptr(msg), wParam, lParam)
	return r1
}

func getModuleHandle() (uintptr, error) {
	r1, _, callErr := procGetModuleHandleW.Call(0)
	if r1 == 0 {
		return 0, windowsCallError(callErr)
	}
	return r1, nil
}

func registerClass(wc *wndClassEx) error {
	r1, _, callErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(wc)))
	if r1 == 0 {
		return windowsCallError(callErr)
	}
	return nil
}

func createHiddenWindow(className, windowName *uint16, hInstance uintptr) (uintptr, error) {
	r1, _, callErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		wsOverlappedWindow,
		cwUseDefault,
		cwUseDefault,
		cwUseDefault,
		cwUseDefault,
		0,
		0,
		hInstance,
		0,
	)
	if r1 == 0 {
		return 0, windowsCallError(callErr)
	}

	// fyne-io/systray creates a normal overlapped owner window and then hides
	// it. This gives TrackPopupMenu a conventional owner and foreground target.
	procShowWindow.Call(r1, swHide)
	procUpdateWindow.Call(r1)

	return r1, nil
}

func registerWindowMessage(name *uint16) (uint32, error) {
	r1, _, callErr := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(name)))
	if r1 == 0 {
		return 0, windowsCallError(callErr)
	}
	return uint32(r1), nil
}

func shellNotifyIcon(action uint32, data *notifyIconData) error {
	r1, _, callErr := procShellNotifyIconW.Call(
		uintptr(action),
		uintptr(unsafe.Pointer(data)),
	)
	if r1 == 0 {
		return windowsCallError(callErr)
	}
	return nil
}

func createPopupMenu() (uintptr, error) {
	r1, _, callErr := procCreatePopupMenu.Call()
	if r1 == 0 {
		return 0, windowsCallError(callErr)
	}
	return r1, nil
}

func appendMenu(menu uintptr, flags uint32, itemID uintptr, text string) error {
	var textPtr *uint16
	var err error
	if flags != mfSeparator {
		textPtr, err = windows.UTF16PtrFromString(text)
		if err != nil {
			return err
		}
	}

	r1, _, callErr := procAppendMenuW.Call(
		menu,
		uintptr(flags),
		itemID,
		uintptr(unsafe.Pointer(textPtr)),
	)
	if r1 == 0 {
		return windowsCallError(callErr)
	}
	return nil
}

func getCursorPos(cursor *point) error {
	r1, _, callErr := procGetCursorPos.Call(uintptr(unsafe.Pointer(cursor)))
	if r1 == 0 {
		return windowsCallError(callErr)
	}
	return nil
}

func getSystemMetric(index int32) int32 {
	r1, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return int32(r1)
}

func showMessageBox(hWnd uintptr, title, text string, flags uint32) {
	titlePtr, titleErr := windows.UTF16PtrFromString(title)
	textPtr, textErr := windows.UTF16PtrFromString(text)
	if titleErr != nil || textErr != nil {
		return
	}

	procMessageBoxW.Call(
		hWnd,
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(flags),
	)
}

func copyUTF16(dst []uint16, value string) {
	encoded, err := windows.UTF16FromString(value)
	if err != nil || len(dst) == 0 {
		return
	}
	copy(dst, encoded)
	dst[len(dst)-1] = 0
}

func windowsCallError(err error) error {
	if err == nil {
		return errors.New("Windows API call failed")
	}
	if errno, ok := err.(syscall.Errno); ok && errno == 0 {
		return errors.New("Windows API call failed without an extended error code")
	}
	return err
}
