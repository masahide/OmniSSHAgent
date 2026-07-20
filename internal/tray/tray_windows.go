//go:build windows

package tray

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/masahide/OmniSSHAgent/internal/app"
	"github.com/masahide/OmniSSHAgent/internal/autostart"
	"github.com/masahide/OmniSSHAgent/internal/config"
	"github.com/masahide/OmniSSHAgent/internal/open"
	"golang.org/x/sys/windows"
)

//go:embed assets/tray.ico
var assets embed.FS

const (
	windowClass        = "OmniSSHAgentTrayWindow"
	windowTitle        = "OmniSSHAgent"
	wmDestroy          = 0x0002
	wmCommand          = 0x0111
	wmClose            = 0x0010
	wmLButtonUp        = 0x0202
	wmRButtonUp        = 0x0205
	wmUser             = 0x0400
	wmTray             = wmUser + 1
	wmApply            = wmUser + 2
	nimAdd             = 0
	nimModify          = 1
	nimDelete          = 2
	nifMessage         = 1
	nifIcon            = 2
	nifTip             = 4
	wsCaption          = 0x00C00000
	wsSysMenu          = 0x00080000
	wsThickFrame       = 0x00040000
	wsMinimizeBox      = 0x00020000
	wsMaximizeBox      = 0x00010000
	wsOverlappedWindow = wsCaption | wsSysMenu | wsThickFrame | wsMinimizeBox | wsMaximizeBox
	cwUseDefault       = 0x80000000
	swHide             = 0
	mfString           = 0
	mfDisabled         = 2
	mfChecked          = 8
	mfUnchecked        = 0
	mfSeparator        = 0x800
	tpmLeftAlign       = 0
	tpmBottomAlign     = 0x20
	menuStatus         = 1001
	menuOpenConfig     = 1002
	menuOpenConfigDir  = 1003
	menuOpenLogDir     = 1004
	menuSettings       = 1005
	menuPageant        = 1006
	menuCygwin         = 1007
	menuAutoStart      = 1008
	menuQuit           = 1009
	smCXSmallIcon      = 49
	smCYSmallIcon      = 50
	lrDefaultColor     = 0
	mbOK               = 0
	mbIconError        = 0x10
)

var (
	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	user32                 = windows.NewLazySystemDLL("user32.dll")
	shell32                = windows.NewLazySystemDLL("shell32.dll")
	getModuleHandle        = kernel32.NewProc("GetModuleHandleW")
	registerClass          = user32.NewProc("RegisterClassExW")
	createWindow           = user32.NewProc("CreateWindowExW")
	defWindowProc          = user32.NewProc("DefWindowProcW")
	destroyWindow          = user32.NewProc("DestroyWindow")
	showWindow             = user32.NewProc("ShowWindow")
	updateWindow           = user32.NewProc("UpdateWindow")
	getMessage             = user32.NewProc("GetMessageW")
	translateMessage       = user32.NewProc("TranslateMessage")
	dispatchMessage        = user32.NewProc("DispatchMessageW")
	postQuitMessage        = user32.NewProc("PostQuitMessage")
	postMessage            = user32.NewProc("PostMessageW")
	registerWindowMessage  = user32.NewProc("RegisterWindowMessageW")
	createPopupMenu        = user32.NewProc("CreatePopupMenu")
	appendMenu             = user32.NewProc("AppendMenuW")
	modifyMenu             = user32.NewProc("ModifyMenuW")
	checkMenuItem          = user32.NewProc("CheckMenuItem")
	destroyMenu            = user32.NewProc("DestroyMenu")
	getCursorPos           = user32.NewProc("GetCursorPos")
	setForegroundWindow    = user32.NewProc("SetForegroundWindow")
	trackPopupMenu         = user32.NewProc("TrackPopupMenu")
	messageBox             = user32.NewProc("MessageBoxW")
	getSystemMetrics       = user32.NewProc("GetSystemMetrics")
	createIconFromResource = user32.NewProc("CreateIconFromResourceEx")
	destroyIcon            = user32.NewProc("DestroyIcon")
	shellNotifyIcon        = shell32.NewProc("Shell_NotifyIconW")
	callback               = windows.NewCallback(windowProc)
	activeMu               sync.Mutex
	active                 *Tray
	openPath               = open.Path
	openConfiguration      = open.Configuration
	createDefaultConfig    = config.CreateDefault
	autoStartEnabled       = autostart.Enabled
	setAutoStartEnabled    = autostart.SetEnabled
	loadBooleanSettings    = config.LoadBooleanSettings
	toggleBooleanSetting   = config.ToggleBooleanSetting
)

type menuEntry struct {
	flags uint32
	id    uintptr
	text  string
}

func requiredMenuEntries() []menuEntry {
	return []menuEntry{
		{mfString | mfDisabled, menuStatus, "Status: Degraded"},
		{mfSeparator, 0, ""},
		{mfString, menuOpenConfig, "Open configuration"},
		{mfString, menuOpenConfigDir, "Open configuration directory"},
		{mfString, menuOpenLogDir, "Open log directory"},
		{mfSeparator, 0, ""},
		{mfString | mfDisabled, menuSettings, "Settings (restart required)"},
		{mfString | mfUnchecked, menuPageant, "Enable Pageant interface"},
		{mfString | mfUnchecked, menuCygwin, "Enable Cygwin/MSYS2 interface"},
		{mfString | mfUnchecked, menuAutoStart, "Start with Windows"},
		{mfSeparator, 0, ""},
		{mfString, menuQuit, "Quit"},
	}
}

type point struct{ X, Y int32 }
type message struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Point          point
	Private        uint32
}
type wndClassEx struct {
	Size, Style                        uint32
	WndProc                            uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	IconSmall                          uintptr
}
type notifyIconData struct {
	Size                uint32
	Window              uintptr
	ID, Flags, Callback uint32
	Icon                uintptr
	Tip                 [128]uint16
	State, StateMask    uint32
	Info                [256]uint16
	Version             uint32
	InfoTitle           [64]uint16
	InfoFlags           uint32
	GUID                windows.GUID
	BalloonIcon         uintptr
}

type Tray struct {
	configPath         string
	logDirectory       string
	onQuit             func()
	mu                 sync.Mutex
	state              app.State
	window, icon, menu uintptr
	nid                notifyIconData
	taskbarCreated     uint32
	added              bool
	shuttingDown       bool
}

func New(configPath, logDirectory string, onQuit func()) *Tray {
	return &Tray{configPath: configPath, logDirectory: logDirectory, onQuit: onQuit, state: app.StateDegraded}
}

func (t *Tray) SetState(state app.State) {
	t.mu.Lock()
	if t.shuttingDown {
		t.mu.Unlock()
		return
	}
	t.state = state
	window := t.window
	t.mu.Unlock()
	if window != 0 {
		postMessage.Call(window, wmApply, 0, 0)
	}
}

func (t *Tray) Run(ctx context.Context) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	activeMu.Lock()
	if active != nil {
		activeMu.Unlock()
		return errors.New("tray already active")
	}
	active = t
	activeMu.Unlock()
	defer func() { activeMu.Lock(); active = nil; activeMu.Unlock() }()
	if err := t.initialize(); err != nil {
		ShowFatal("Task tray initialization failed", err)
		return err
	}
	defer t.close()
	if err := t.addIcon(); err != nil {
		ShowFatal("Task tray initialization failed", err)
		return err
	}
	go func() { <-ctx.Done(); t.quit() }()
	var msg message
	for {
		result, _, callErr := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(result) == -1 {
			return winErr(callErr)
		}
		if result == 0 {
			return nil
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func (t *Tray) initialize() error {
	instance, _, callErr := getModuleHandle.Call(0)
	if instance == 0 {
		return winErr(callErr)
	}
	class, _ := windows.UTF16PtrFromString(windowClass)
	wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: callback, Instance: instance, ClassName: class}
	if result, _, callErr := registerClass.Call(uintptr(unsafe.Pointer(&wc))); result == 0 {
		return winErr(callErr)
	}
	title, _ := windows.UTF16PtrFromString(windowTitle)
	window, _, callErr := createWindow.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), wsOverlappedWindow, cwUseDefault, cwUseDefault, cwUseDefault, cwUseDefault, 0, 0, instance, 0)
	if window == 0 {
		return winErr(callErr)
	}
	t.window = window
	showWindow.Call(window, swHide)
	updateWindow.Call(window)
	taskbarName, _ := windows.UTF16PtrFromString("TaskbarCreated")
	result, _, callErr := registerWindowMessage.Call(uintptr(unsafe.Pointer(taskbarName)))
	if result == 0 {
		return winErr(callErr)
	}
	t.taskbarCreated = uint32(result)
	iconBytes, err := assets.ReadFile("assets/tray.ico")
	if err != nil {
		return err
	}
	width := int32Result(getSystemMetrics, smCXSmallIcon)
	height := int32Result(getSystemMetrics, smCYSmallIcon)
	t.icon, err = createIcon(iconBytes, width, height)
	if err != nil {
		return err
	}
	menu, _, callErr := createPopupMenu.Call()
	if menu == 0 {
		return winErr(callErr)
	}
	t.menu = menu
	for _, item := range requiredMenuEntries() {
		if err := appendMenuItem(menu, item.flags, item.id, item.text); err != nil {
			return err
		}
	}
	_ = t.applyMenuChecks()
	t.nid = notifyIconData{Size: uint32(unsafe.Sizeof(notifyIconData{})), Window: window, ID: 1, Flags: nifMessage | nifIcon | nifTip, Callback: wmTray, Icon: t.icon}
	t.applyState()
	return nil
}

func (t *Tray) addIcon() error {
	if result, _, callErr := shellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&t.nid))); result == 0 {
		return winErr(callErr)
	}
	t.added = true
	return nil
}

func (t *Tray) applyState() {
	t.mu.Lock()
	state := t.state
	t.mu.Unlock()
	tooltip := app.Tooltip(state)
	copyUTF16(t.nid.Tip[:], tooltip)
	if t.added {
		shellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&t.nid)))
	}
	label := "Status: " + stateLabel(state)
	ptr, _ := windows.UTF16PtrFromString(label)
	modifyMenu.Call(t.menu, menuStatus, mfString|mfDisabled, menuStatus, uintptr(unsafe.Pointer(ptr)))
}

func stateLabel(state app.State) string {
	switch state {
	case app.StateRunning:
		return "Running"
	case app.StateConfigurationError:
		return "Configuration error"
	default:
		return "Degraded"
	}
}

func (t *Tray) showMenu() {
	_ = t.applyMenuChecks()
	var p point
	if result, _, _ := getCursorPos.Call(uintptr(unsafe.Pointer(&p))); result == 0 {
		return
	}
	setForegroundWindow.Call(t.window)
	trackPopupMenu.Call(t.menu, tpmBottomAlign|tpmLeftAlign, uintptr(p.X), uintptr(p.Y), 0, t.window, 0)
}

func (t *Tray) command(id uintptr) {
	t.mu.Lock()
	if t.shuttingDown {
		t.mu.Unlock()
		return
	}
	t.mu.Unlock()
	switch id {
	case menuOpenConfig:
		_, _ = createDefaultConfig(t.configPath)
		_ = openConfiguration(t.configPath)
	case menuOpenConfigDir:
		_ = openPath(filepathDir(t.configPath))
	case menuOpenLogDir:
		_ = openPath(t.logDirectory)
	case menuPageant:
		t.toggleConfigSetting(config.PageantEnabled)
	case menuCygwin:
		t.toggleConfigSetting(config.CygwinEnabled)
	case menuAutoStart:
		enabled, err := autoStartEnabled()
		if err == nil {
			err = setAutoStartEnabled(!enabled)
		}
		if err != nil {
			ShowFatal("Auto-start setting failed", err)
			return
		}
		_ = t.applyMenuChecks()
	case menuQuit:
		if t.onQuit != nil {
			go t.onQuit()
		}
	}
}

func (t *Tray) toggleConfigSetting(setting config.BooleanSetting) {
	if _, err := createDefaultConfig(t.configPath); err != nil {
		ShowFatal("Configuration update failed", err)
		return
	}
	if _, err := toggleBooleanSetting(t.configPath, setting); err != nil {
		ShowFatal("Configuration update failed", err)
		return
	}
	_ = t.applyMenuChecks()
}

func (t *Tray) applyMenuChecks() error {
	settings, err := loadBooleanSettings(t.configPath)
	if err == nil {
		t.setMenuCheck(menuPageant, settings.PageantEnabled)
		t.setMenuCheck(menuCygwin, settings.CygwinEnabled)
	}
	enabled, err := autoStartEnabled()
	if err != nil {
		return err
	}
	t.setMenuCheck(menuAutoStart, enabled)
	return nil
}

func (t *Tray) setMenuCheck(id uintptr, enabled bool) {
	flags := uintptr(mfUnchecked)
	if enabled {
		flags = mfChecked
	}
	checkMenuItem.Call(t.menu, id, flags)
}

func (t *Tray) quit() {
	t.mu.Lock()
	if t.shuttingDown {
		t.mu.Unlock()
		return
	}
	t.shuttingDown = true
	window := t.window
	t.mu.Unlock()
	if window != 0 {
		postMessage.Call(window, wmClose, 0, 0)
	}
}

func (t *Tray) close() {
	if t.added {
		shellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&t.nid)))
		t.added = false
	}
	if t.menu != 0 {
		destroyMenu.Call(t.menu)
		t.menu = 0
	}
	if t.window != 0 {
		destroyWindow.Call(t.window)
		t.window = 0
	}
	if t.icon != 0 {
		destroyIcon.Call(t.icon)
		t.icon = 0
	}
}

func windowProc(window uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	activeMu.Lock()
	t := active
	activeMu.Unlock()
	if t != nil {
		if msg == t.taskbarCreated && msg != 0 {
			t.added = false
			_ = t.addIcon()
			return 0
		}
		switch msg {
		case wmTray:
			if uint32(lParam) == wmLButtonUp || uint32(lParam) == wmRButtonUp {
				t.showMenu()
			}
			return 0
		case wmCommand:
			t.command(uintptr(uint16(wParam & 0xffff)))
			return 0
		case wmApply:
			t.applyState()
			return 0
		case wmDestroy:
			if t.added {
				shellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&t.nid)))
				t.added = false
			}
			t.window = 0
			postQuitMessage.Call(0)
			return 0
		}
	}
	result, _, _ := defWindowProc.Call(window, uintptr(msg), wParam, lParam)
	return result
}

func appendMenuItem(menu uintptr, flags uint32, id uintptr, text string) error {
	var ptr *uint16
	if flags != mfSeparator {
		ptr, _ = windows.UTF16PtrFromString(text)
	}
	if result, _, callErr := appendMenu.Call(menu, uintptr(flags), id, uintptr(unsafe.Pointer(ptr))); result == 0 {
		return winErr(callErr)
	}
	return nil
}

func copyUTF16(dst []uint16, value string) {
	encoded, err := windows.UTF16FromString(value)
	if err != nil {
		return
	}
	copy(dst, encoded)
	dst[len(dst)-1] = 0
}

func int32Result(proc *windows.LazyProc, value uintptr) int32 {
	result, _, _ := proc.Call(value)
	if int32(result) <= 0 {
		return 16
	}
	return int32(result)
}

func filepathDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '\\' || path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}

func winErr(err error) error {
	if err == nil {
		return errors.New("Windows API call failed")
	}
	if errno, ok := err.(syscall.Errno); ok && errno == 0 {
		return errors.New("Windows API call failed without an extended error code")
	}
	return err
}

func ShowFatal(title string, err error) {
	titlePtr, _ := windows.UTF16PtrFromString(title)
	textPtr, _ := windows.UTF16PtrFromString(err.Error())
	messageBox.Call(0, uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(titlePtr)), mbOK|mbIconError)
}

func createIcon(ico []byte, width, height int32) (uintptr, error) {
	if len(ico) < 6 || binaryLE16(ico[2:4]) != 1 {
		return 0, errors.New("invalid embedded ICO")
	}
	count := int(binaryLE16(ico[4:6]))
	if count == 0 || 6+count*16 > len(ico) {
		return 0, errors.New("truncated embedded ICO")
	}
	bestOffset, bestSize, bestScore := uint32(0), uint32(0), int64(1<<62)
	for i := 0; i < count; i++ {
		base := 6 + i*16
		w, h := int32(ico[base]), int32(ico[base+1])
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}
		score := abs(int64(w-width)) + abs(int64(h-height))
		if score < bestScore {
			bestScore = score
			bestSize = binaryLE32(ico[base+8 : base+12])
			bestOffset = binaryLE32(ico[base+12 : base+16])
		}
	}
	end := uint64(bestOffset) + uint64(bestSize)
	if bestSize == 0 || end > uint64(len(ico)) {
		return 0, errors.New("ICO image is outside file")
	}
	image := ico[bestOffset:end]
	result, _, callErr := createIconFromResource.Call(uintptr(unsafe.Pointer(&image[0])), uintptr(len(image)), 1, 0x00030000, uintptr(width), uintptr(height), lrDefaultColor)
	runtime.KeepAlive(ico)
	if result == 0 {
		return 0, fmt.Errorf("CreateIconFromResourceEx: %w", winErr(callErr))
	}
	return result, nil
}
func binaryLE16(b []byte) uint16 { return uint16(b[0]) | uint16(b[1])<<8 }
func binaryLE32(b []byte) uint32 { return uint32(binaryLE16(b)) | uint32(binaryLE16(b[2:]))<<16 }
func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
