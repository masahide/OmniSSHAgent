//go:build windows

package tray

import (
	"errors"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wsChild               = 0x40000000
	wsVisible             = 0x10000000
	wsTabStop             = 0x00010000
	wsGroup               = 0x00020000
	wsBorder              = 0x00800000
	wsVScroll             = 0x00200000
	wsHScroll             = 0x00100000
	wsPopup               = 0x80000000
	wsClipSiblings        = 0x04000000
	wsExDlgModalFrame     = 0x00000001
	wsExControlParent     = 0x00010000
	wsExClientEdge        = 0x00000200
	wsExWindowEdge        = 0x00000100
	bsPushButton          = 0
	bsDefPushButton       = 1
	bsAutoCheckBox        = 3
	bsGroupBox            = 7
	esAutoHScroll         = 0x80
	esPassword            = 0x20
	cbsDropDownList       = 3
	lbsNotify             = 1
	lbsNoIntegralHeight   = 0x100
	idOK                  = 1
	idCancel              = 2
	wmSetFont             = 0x0030
	wmSetText             = 0x000C
	wmGetText             = 0x000D
	wmGetTextLength       = 0x000E
	bmGetCheck            = 0x00F0
	bmSetCheck            = 0x00F1
	bstUnchecked          = 0
	bstChecked            = 1
	cbAddString           = 0x0143
	cbSetCurSel           = 0x014E
	cbGetCurSel           = 0x0147
	lbAddString           = 0x0180
	lbResetContent        = 0x0184
	lbGetCurSel           = 0x0188
	lbSetHorizontalExtent = 0x0194
	swShow                = 5
	swpNoSize             = 0x0001
	swpNoZOrder           = 0x0004
	spiGetWorkArea        = 0x0030
	defaultGUIFont        = 17
	idcArrow              = 32512
	colorBtnFace          = 15
	errorClassExists      = syscall.Errno(1410)
	ofnExplorer           = 0x00080000
	ofnFileMustExist      = 0x00001000
	ofnPathMustExist      = 0x00000800
	ofnHideReadOnly       = 0x00000004
	ofnNoChangeDir        = 0x00000008
	maxPathWide           = 32768
)

type rect struct{ Left, Top, Right, Bottom int32 }
type size struct{ CX, CY int32 }

type openFileNameW struct {
	StructSize    uint32
	Owner         uintptr
	Instance      uintptr
	Filter        *uint16
	CustomFilter  *uint16
	MaxCustFilter uint32
	FilterIndex   uint32
	File          *uint16
	MaxFile       uint32
	FileTitle     *uint16
	MaxFileTitle  uint32
	InitialDir    *uint16
	Title         *uint16
	Flags         uint32
	FileOffset    uint16
	FileExtension uint16
	DefExt        *uint16
	CustData      uintptr
	Hook          uintptr
	TemplateName  *uint16
	Reserved      uintptr
	ReservedFlags uint32
	FlagsEx       uint32
}

type metrics struct {
	dpi  int32
	font uintptr
}

func (m metrics) s(v int32) int32 { return v * m.dpi / 96 }

var (
	gdi32                = windows.NewLazySystemDLL("gdi32.dll")
	comdlg32             = windows.NewLazySystemDLL("comdlg32.dll")
	enableWindow         = user32.NewProc("EnableWindow")
	isWindow             = user32.NewProc("IsWindow")
	isDialogMessage      = user32.NewProc("IsDialogMessageW")
	sendMessage          = user32.NewProc("SendMessageW")
	setWindowPos         = user32.NewProc("SetWindowPos")
	getWindowRect        = user32.NewProc("GetWindowRect")
	systemParameters     = user32.NewProc("SystemParametersInfoW")
	loadCursor           = user32.NewProc("LoadCursorW")
	getDpiForSystem      = user32.NewProc("GetDpiForSystem")
	adjustWindowRectEx   = user32.NewProc("AdjustWindowRectEx")
	getDC                = user32.NewProc("GetDC")
	releaseDC            = user32.NewProc("ReleaseDC")
	getStockObject       = gdi32.NewProc("GetStockObject")
	selectObject         = gdi32.NewProc("SelectObject")
	getTextExtentPoint32 = gdi32.NewProc("GetTextExtentPoint32W")
	getOpenFileName      = comdlg32.NewProc("GetOpenFileNameW")
	settingsCallback     = windows.NewCallback(settingsWindowProc)
	keysCallback         = windows.NewCallback(keysWindowProc)
	passwordCallback     = windows.NewCallback(passwordWindowProc)
)

func systemDPI() int32 {
	if err := getDpiForSystem.Find(); err == nil {
		if result, _, _ := getDpiForSystem.Call(); result != 0 {
			return int32(result)
		}
	}
	return 96
}

func newMetrics() metrics {
	font, _, _ := getStockObject.Call(defaultGUIFont)
	return metrics{dpi: systemDPI(), font: font}
}

func registerDialogClass(name string, proc uintptr) error {
	instance, _, callErr := getModuleHandle.Call(0)
	if instance == 0 {
		return winErr(callErr)
	}
	class, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	cursor, _, _ := loadCursor.Call(0, idcArrow)
	wc := wndClassEx{
		Size:       uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:    proc,
		Instance:   instance,
		Cursor:     cursor,
		Background: colorBtnFace + 1,
		ClassName:  class,
	}
	if result, _, callErr := registerClass.Call(uintptr(unsafe.Pointer(&wc))); result == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == errorClassExists {
			return nil
		}
		return winErr(callErr)
	}
	return nil
}

func createPopup(class, title string, width, height int32, owner, proc uintptr) (uintptr, error) {
	if err := registerDialogClass(class, proc); err != nil {
		return 0, err
	}
	instance, _, callErr := getModuleHandle.Call(0)
	if instance == 0 {
		return 0, winErr(callErr)
	}
	classPtr, err := windows.UTF16PtrFromString(class)
	if err != nil {
		return 0, err
	}
	titlePtr, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return 0, err
	}
	style := uint32(wsPopup | wsCaption | wsSysMenu | wsClipSiblings)
	ex := uint32(wsExDlgModalFrame | wsExControlParent | wsExWindowEdge)
	area := rect{Right: width, Bottom: height}
	adjustWindowRectEx.Call(uintptr(unsafe.Pointer(&area)), uintptr(style), 0, uintptr(ex))
	hwnd, _, callErr := createWindow.Call(
		uintptr(ex),
		uintptr(unsafe.Pointer(classPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(style),
		cwUseDefault, cwUseDefault,
		uintptr(area.Right-area.Left), uintptr(area.Bottom-area.Top),
		owner, 0, instance, 0,
	)
	if hwnd == 0 {
		return 0, winErr(callErr)
	}
	return hwnd, nil
}

func createChild(class, title string, style, ex uint32, x, y, w, h int32, parent, id uintptr) (uintptr, error) {
	instance, _, callErr := getModuleHandle.Call(0)
	if instance == 0 {
		return 0, winErr(callErr)
	}
	classPtr, err := windows.UTF16PtrFromString(class)
	if err != nil {
		return 0, err
	}
	var titlePtr *uint16
	if title != "" {
		titlePtr, err = windows.UTF16PtrFromString(title)
		if err != nil {
			return 0, err
		}
	}
	hwnd, _, callErr := createWindow.Call(
		uintptr(ex),
		uintptr(unsafe.Pointer(classPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(style|wsChild|wsVisible),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		parent, id, instance, 0,
	)
	if hwnd == 0 {
		return 0, winErr(callErr)
	}
	return hwnd, nil
}

func setChildFont(hwnd, font uintptr) {
	if hwnd != 0 && font != 0 {
		sendMessage.Call(hwnd, wmSetFont, font, 1)
	}
}

func runModal(owner, hwnd uintptr) {
	if owner != 0 {
		enableWindow.Call(owner, 0)
		defer enableWindow.Call(owner, 1)
	}
	centerWindow(hwnd)
	showWindow.Call(hwnd, swShow)
	updateWindow.Call(hwnd)
	setForegroundWindow.Call(hwnd)
	var msg message
	for {
		alive, _, _ := isWindow.Call(hwnd)
		if alive == 0 {
			return
		}
		result, _, callErr := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(result) == -1 {
			_ = callErr
			return
		}
		if result == 0 {
			destroyWindow.Call(hwnd)
			postQuitMessage.Call(0)
			return
		}
		if handled, _, _ := isDialogMessage.Call(hwnd, uintptr(unsafe.Pointer(&msg))); handled != 0 {
			continue
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func centerWindow(hwnd uintptr) {
	var wr, ar rect
	if result, _, _ := getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr))); result == 0 {
		return
	}
	if result, _, _ := systemParameters.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&ar)), 0); result == 0 {
		return
	}
	width := wr.Right - wr.Left
	height := wr.Bottom - wr.Top
	x := ar.Left + (ar.Right-ar.Left-width)/2
	y := ar.Top + (ar.Bottom-ar.Top-height)/2
	setWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpNoZOrder)
}

func windowText(hwnd uintptr) string {
	if hwnd == 0 {
		return ""
	}
	n, _, _ := sendMessage.Call(hwnd, wmGetTextLength, 0, 0)
	buf := make([]uint16, n+1)
	sendMessage.Call(hwnd, wmGetText, n+1, uintptr(unsafe.Pointer(&buf[0])))
	return windows.UTF16ToString(buf)
}

func setWindowText(hwnd uintptr, text string) {
	ptr, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	sendMessage.Call(hwnd, wmSetText, 0, uintptr(unsafe.Pointer(ptr)))
}

func isChecked(hwnd uintptr) bool {
	result, _, _ := sendMessage.Call(hwnd, bmGetCheck, 0, 0)
	return result == bstChecked
}

func setChecked(hwnd uintptr, checked bool) {
	value := uintptr(bstUnchecked)
	if checked {
		value = bstChecked
	}
	sendMessage.Call(hwnd, bmSetCheck, value, 0)
}

func comboSelect(hwnd uintptr, index int) {
	sendMessage.Call(hwnd, cbSetCurSel, uintptr(index), 0)
}

func comboIndex(hwnd uintptr) int {
	result, _, _ := sendMessage.Call(hwnd, cbGetCurSel, 0, 0)
	return int(int32(result))
}

func comboAdd(hwnd uintptr, text string) {
	ptr, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	sendMessage.Call(hwnd, cbAddString, 0, uintptr(unsafe.Pointer(ptr)))
}

func listAdd(hwnd uintptr, text string) {
	ptr, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	sendMessage.Call(hwnd, lbAddString, 0, uintptr(unsafe.Pointer(ptr)))
}

func listReset(hwnd uintptr) {
	sendMessage.Call(hwnd, lbResetContent, 0, 0)
	sendMessage.Call(hwnd, lbSetHorizontalExtent, 0, 0)
}

func listIndex(hwnd uintptr) int {
	result, _, _ := sendMessage.Call(hwnd, lbGetCurSel, 0, 0)
	return int(int32(result))
}

func listSetHorizontalExtent(hwnd uintptr, extent int32) {
	sendMessage.Call(hwnd, lbSetHorizontalExtent, uintptr(extent), 0)
}

func measureTextWidth(hwnd, font uintptr, text string) int32 {
	if text == "" {
		return 0
	}
	dc, _, _ := getDC.Call(hwnd)
	if dc == 0 {
		return 0
	}
	defer releaseDC.Call(hwnd, dc)
	var previous uintptr
	if font != 0 {
		previous, _, _ = selectObject.Call(dc, font)
	}
	encoded, err := windows.UTF16FromString(text)
	if err != nil {
		return 0
	}
	var measured size
	getTextExtentPoint32.Call(
		dc,
		uintptr(unsafe.Pointer(&encoded[0])),
		uintptr(len(encoded)-1),
		uintptr(unsafe.Pointer(&measured)),
	)
	if previous != 0 {
		selectObject.Call(dc, previous)
	}
	return measured.CX
}

func listUpdateHorizontalExtent(hwnd, font uintptr, items []string) {
	var widest int32
	for _, item := range items {
		if width := measureTextWidth(hwnd, font, item); width > widest {
			widest = width
		}
	}
	if widest > 0 {
		// Extra padding so the last characters clear the vertical scrollbar.
		widest += 16
	}
	listSetHorizontalExtent(hwnd, widest)
}

func showError(owner uintptr, title string, err error) {
	if err == nil {
		return
	}
	titlePtr, _ := windows.UTF16PtrFromString(title)
	textPtr, _ := windows.UTF16PtrFromString(err.Error())
	messageBox.Call(owner, uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(titlePtr)), mbOK|mbIconError)
}

func utf16Filter(pairs ...string) []uint16 {
	var out []uint16
	for _, part := range pairs {
		encoded, err := windows.UTF16FromString(part)
		if err != nil {
			continue
		}
		out = append(out, encoded...)
	}
	return append(out, 0)
}

func pickOpenFile(owner uintptr, title, initialDir, initialFile string, filter []string, mustExist bool) (string, bool) {
	buf := make([]uint16, maxPathWide)
	if initialFile != "" {
		copyUTF16(buf, initialFile)
	}
	filterUTF16 := utf16Filter(filter...)
	ofn := openFileNameW{
		StructSize: uint32(unsafe.Sizeof(openFileNameW{})),
		Owner:      owner,
		Filter:     &filterUTF16[0],
		File:       &buf[0],
		MaxFile:    uint32(len(buf)),
		Flags:      ofnExplorer | ofnHideReadOnly | ofnNoChangeDir | ofnPathMustExist,
	}
	if mustExist {
		ofn.Flags |= ofnFileMustExist
	}
	if title != "" {
		ofn.Title, _ = windows.UTF16PtrFromString(title)
	}
	if initialDir != "" {
		ofn.InitialDir, _ = windows.UTF16PtrFromString(initialDir)
	}
	result, _, _ := getOpenFileName.Call(uintptr(unsafe.Pointer(&ofn)))
	runtime.KeepAlive(filterUTF16)
	runtime.KeepAlive(buf)
	if result == 0 {
		return "", false
	}
	return windows.UTF16ToString(buf), true
}

func commandID(wParam uintptr) uintptr { return uintptr(uint16(wParam)) }

var errDialogCanceled = errors.New("canceled")
