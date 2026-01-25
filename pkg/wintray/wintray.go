package wintray

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/cwchiu/go-winapi"
	"github.com/google/uuid"
	"golang.org/x/sys/windows"
)

const (
	ID                 = "OmniSSHAgent"
	TrayIconMsg        = winapi.WM_APP + 1
	trayCommandMessage = winapi.WM_APP + 2

	NOTIFYICON_VERSION_4 = winapi.NOTIFYICON_VERSION + 1
	NIN_BALLOONSHOW      = 0x0402
	NIN_BALLOONTIMEOUT   = 0x0404
	NIN_BALLOONUSERCLICK = 0x0405

	// NotifyIcon flags
	NIF_MESSAGE  = 0x00000001
	NIF_ICON     = 0x00000002
	NIF_TIP      = 0x00000004
	NIF_GUID     = 0x00000020
	NIF_REALTIME = 0x00000040
	NIF_SHOWTIP  = 0x00000080
)

type trayCommand struct {
	fn func()
}

type notifyIconData struct {
	CbSize            uint32
	HWnd              winapi.HWND
	UID               uint32
	UFlags            uint32
	UCallbackMessage  uint32
	HIcon             winapi.HICON
	SzTip             [128]uint16
	DwState           uint32
	DwStateMask       uint32
	SzInfo            [256]uint16
	UVersionOrTimeout uint32
	SzInfoTitle       [64]uint16
	DwInfoFlags       uint32
	GuidItem          winapi.GUID
	HBalloonIcon      winapi.HICON
}

func (nid *notifyIconData) Notify(message uint32) bool {
	return winapi.Shell_NotifyIcon(message, (*winapi.NOTIFYICONDATA)(unsafe.Pointer(nid)))
}

func currentThreadID() uint32 {
	return windows.GetCurrentThreadId()
}

func reportTrayFatalError(reason string) {
	title := windows.StringToUTF16Ptr("OmniSSHAgent")
	message := windows.StringToUTF16Ptr(reason)
	winapi.MessageBox(0, message, title, winapi.MB_OK|winapi.MB_ICONERROR)
	os.Exit(1)
}

// enqueueCommand keeps Win32 actions on the tray thread (see doc/dev/issue-tasktry.md for context).
func (ti *TrayIcon) enqueueCommand(fn func()) {
	ti.commandMu.Lock()
	if ti.shuttingDown || ti.commandCh == nil {
		ti.commandMu.Unlock()
		return
	}
	ch := ti.commandCh
	ti.commandMu.Unlock()
	if fn == nil {
		return
	}
	ch <- trayCommand{fn: fn}
	if ti.hwnd != 0 {
		winapi.PostMessage(ti.hwnd, trayCommandMessage, 0, 0)
	}
}

func (ti *TrayIcon) handleCommands() {
	for {
		select {
		case cmd := <-ti.commandCh:
			if cmd.fn != nil {
				cmd.fn()
			}
		default:
			return
		}
	}
}

func (ti *TrayIcon) stopCommandQueue() {
	ti.commandMu.Lock()
	ti.shuttingDown = true
	ti.commandMu.Unlock()
}

func (ti *TrayIcon) wndProc(hWnd winapi.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case TrayIconMsg:
		switch nmsg := winapi.LOWORD(uint32(lParam)); nmsg {
		case NIN_BALLOONUSERCLICK:
			ti.BalloonClickFunc()
		case winapi.WM_LBUTTONDOWN:
			ti.TrayClickFunc()
		case winapi.WM_RBUTTONUP:
			if err := ti.showMenu(); err != nil {
				log.Printf("wintray: showMenu error: %v", err)
			}
		default:
		}
	case winapi.WM_COMMAND:
		menuItemId := int32(wParam)
		// https://docs.microsoft.com/en-us/windows/win32/menurc/wm-command#menus
		if menuItemId != -1 {
			ti.menuSelected(uint32(wParam))
		}
	case trayCommandMessage:
		ti.handleCommands()
	case winapi.WM_DESTROY:
		winapi.PostQuitMessage(0)
	default:
		r := winapi.DefWindowProc(hWnd, msg, wParam, lParam)
		return r
	}
	return 0
}

func guid() winapi.GUID {
	u := uuid.NewSHA1(
		uuid.MustParse("4443722f-bc9a-4ba0-8cb3-bfb877b42add"),
		[]byte(ID),
	)
	buf, _ := u.MarshalBinary()
	return *(*winapi.GUID)(unsafe.Pointer(&buf[0]))
}

type TrayIcon struct {
	hwnd             winapi.HWND
	guid             winapi.GUID
	BalloonClickFunc func()
	TrayClickFunc    func()

	currentMenuID uint32
	menuItems     map[uint32]*MenuItem
	menuItemsLock sync.RWMutex
	// menus keeps track of the submenus keyed by the menu item ID, plus 0
	// which corresponds to the main popup menu.
	menus     map[uint32]winapi.HMENU
	menusLock sync.RWMutex
	// menuOf keeps track of the menu each menu item belongs to.
	menuOf       map[uint32]winapi.HMENU
	menuOfLock   sync.RWMutex
	icon         winapi.HICON
	commandCh    chan trayCommand
	commandMu    sync.Mutex
	shuttingDown bool
}

func (ti *TrayIcon) createMainWindow() winapi.HWND {
	hInstance := winapi.GetModuleHandle(nil)

	wndClass := windows.StringToUTF16Ptr(ID)

	var wcex winapi.WNDCLASSEX

	wcex.CbSize = uint32(unsafe.Sizeof(wcex))
	wcex.LpfnWndProc = windows.NewCallback(ti.wndProc)
	wcex.HInstance = hInstance
	wcex.LpszClassName = wndClass
	winapi.RegisterClassEx(&wcex)

	hwnd := winapi.CreateWindowEx(
		0,
		wndClass,
		windows.StringToUTF16Ptr("Tray Icons "+ID),
		winapi.WS_OVERLAPPEDWINDOW,
		winapi.CW_USEDEFAULT,
		winapi.CW_USEDEFAULT,
		winapi.CW_USEDEFAULT, //400,
		winapi.CW_USEDEFAULT, //300,
		0,
		0,
		hInstance,
		nil)
	if hwnd == 0 {
		reportTrayFatalError(fmt.Sprintf("CreateWindowEx failed: %d", winapi.GetLastError()))
	}

	return hwnd
}

func (ti *TrayIcon) createMenu() error {
	menuHandle := winapi.CreatePopupMenu()
	if menuHandle == 0 {
		err := fmt.Errorf("%d", winapi.GetLastError())
		log.Printf("wintray: CreatePopupMenu failed: %v", err)
		return err
	}
	ti.menuItems = map[uint32]*MenuItem{}
	ti.menus = map[uint32]winapi.HMENU{}
	ti.menus[0] = menuHandle
	ti.menuOf = map[uint32]winapi.HMENU{}

	mi := winapi.MENUINFO{
		FMask: winapi.MIM_APPLYTOSUBMENUS,
	}
	mi.CbSize = uint32(unsafe.Sizeof(mi))

	if !winapi.SetMenuInfo(ti.menus[0], &mi) {
		err := fmt.Errorf("%d", winapi.GetLastError())
		log.Printf("wintray: SetMenuInfo failed: %v", err)
		return err
	}

	return nil
}

func (ti *TrayIcon) initData() *notifyIconData {
	data := notifyIconData{
		UFlags:           NIF_GUID | winapi.NIF_MESSAGE,
		HWnd:             ti.hwnd,
		GuidItem:         ti.guid,
		UCallbackMessage: TrayIconMsg,
	}
	data.CbSize = uint32(unsafe.Sizeof(data))
	return &data
}

func (ti *TrayIcon) Dispose() {
	ti.initData().Notify(winapi.NIM_DELETE)
}

func (ti *TrayIcon) SetIcon(icon winapi.HICON) {
	ti.enqueueCommand(func() {
		data := ti.initData()
		data.UFlags |= winapi.NIF_ICON
		data.HIcon = icon
		data.Notify(winapi.NIM_MODIFY)
	})
}

func (ti *TrayIcon) SetTooltip(tooltip string) {
	ti.enqueueCommand(func() {
		data := ti.initData()
		data.UFlags |= winapi.NIF_TIP
		copy(data.SzTip[:], windows.StringToUTF16(tooltip))
		data.Notify(winapi.NIM_MODIFY)
	})
}

func (ti *TrayIcon) SetTitle(title string) {
}

func (ti *TrayIcon) ShowBalloonNotification(title, text string) {
	ti.enqueueCommand(func() {
		data := ti.initData()
		data.UFlags |= winapi.NIF_INFO | NIF_REALTIME
		if title != "" {
			copy(data.SzInfoTitle[:], windows.StringToUTF16(title))
		}
		copy(data.SzInfo[:], windows.StringToUTF16(text))
		if !data.Notify(winapi.NIM_MODIFY) {
			log.Printf("cannot show balloon: %d", winapi.GetLastError())
		}
	})
}

func (ti *TrayIcon) AddMenuItem(title, tooltip string) *MenuItem {
	item := newMenuItem(ti, title, tooltip, nil)
	item.update()
	return item
}

func (ti *TrayIcon) AddMenuItemCheckbox(title, tooltip string, checked bool) *MenuItem {
	item := newMenuItem(ti, title, tooltip, nil)
	item.isCheckable = true
	item.checked = checked
	item.update()
	return item
}

// AddSeparator adds a separator bar to the menu
func (ti *TrayIcon) AddSeparator() {
	newSeparatorMenuItem(ti, nil)
}

func (ti *TrayIcon) updateMenuItem(item *MenuItem) error {
	titlePtr := winapi.StringToBSTR(item.title)
	if titlePtr == nil {
		return fmt.Errorf("%d", winapi.GetLastError())
	}

	mi := winapi.MENUITEMINFO{
		FMask:      winapi.MIIM_FTYPE | winapi.MIIM_STRING | winapi.MIIM_ID | winapi.MIIM_STATE,
		FType:      winapi.MFT_STRING,
		WID:        item.id,
		DwTypeData: titlePtr,
		Cch:        uint32(len(item.title)),
	}
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	if item.disabled {
		mi.FState |= winapi.MFS_DISABLED
	}
	if item.checked {
		mi.FState |= winapi.MFS_CHECKED
	}

	if item.icon != 0 {
		mi.FMask |= winapi.MIIM_BITMAP
		mi.HbmpItem = item.icon
	}

	ti.menusLock.RLock()
	menu, exists := ti.menus[item.parentID()]
	ti.menusLock.RUnlock()
	if exists {
		// We set the menu item info based on the menuID
		if winapi.SetMenuItemInfo(menu, item.id, false, &mi) {
			return nil
		}
	} else {
		// Create the parent menu
		var err error
		menu, err = ti.updateSubMenuItem(item.parent)
		if err != nil {
			return err
		}
		ti.menusLock.Lock()
		ti.menus[item.parentID()] = menu
		ti.menusLock.Unlock()
	}

	// Menu item does not already exist, create it
	ti.menusLock.RLock()
	submenu, exists := ti.menus[item.id]
	ti.menusLock.RUnlock()
	if exists {
		mi.FMask |= winapi.MIIM_SUBMENU
		mi.HSubMenu = submenu
	}
	if !winapi.InsertMenuItem(menu, 0, false, &mi) {
		return fmt.Errorf("%d", winapi.GetLastError())
	}
	ti.menuOfLock.Lock()
	ti.menuOf[item.id] = menu
	ti.menuOfLock.Unlock()
	return nil
}

func (ti *TrayIcon) updateSubMenuItem(item *MenuItem) (winapi.HMENU, error) {
	menu := winapi.CreateMenu()
	if menu == 0 {
		return menu, fmt.Errorf("%d", winapi.GetLastError())
	}

	mi := winapi.MENUITEMINFO{
		FMask:    winapi.MIIM_SUBMENU,
		HSubMenu: menu,
	}
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	itemID := uint32(0)
	if item != nil {
		itemID = item.id
	}
	ti.menuOfLock.RLock()
	hMenu := ti.menuOf[itemID]
	ti.menuOfLock.RUnlock()
	if !winapi.SetMenuItemInfo(hMenu, itemID, false, &mi) {
		return winapi.HMENU(0), fmt.Errorf("%d", winapi.GetLastError())
	}
	ti.menusLock.Lock()
	ti.menus[itemID] = menu
	ti.menusLock.Unlock()
	return menu, nil
}

func (ti *TrayIcon) insertSeparator(parentID uint32) {
	ti.menusLock.RLock()
	parentMenu := ti.menus[parentID]
	ti.menusLock.RUnlock()
	if parentMenu == 0 {
		log.Printf("wintray: insertSeparator parent menu not found id=%d", parentID)
		return
	}

	menuItemId := atomic.AddUint32(&ti.currentMenuID, 1)
	mi := winapi.MENUITEMINFO{
		FMask: winapi.MIIM_FTYPE | winapi.MIIM_ID | winapi.MIIM_STATE,
		FType: winapi.MFT_SEPARATOR,
		WID:   menuItemId,
	}
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	if !winapi.InsertMenuItem(parentMenu, 0, false, &mi) {
		log.Printf("wintray: InsertMenuItem failed: %d", winapi.GetLastError())
		return
	}
}

func (ti *TrayIcon) showMenu() error {
	p := winapi.POINT{}
	if !winapi.GetCursorPos(&p) {
		err := fmt.Errorf("%d", winapi.GetLastError())
		log.Printf("wintray: GetCursorPos failed: %v", err)
		return err
	}
	if !winapi.SetForegroundWindow(ti.hwnd) {
		err := fmt.Errorf("%d", winapi.GetLastError())
		log.Printf("wintray: SetForegroundWindow failed: %v", err)
		return err
	}

	if winapi.TrackPopupMenu(ti.menus[0], winapi.TPM_BOTTOMALIGN|winapi.TPM_LEFTALIGN, p.X, p.Y, 0, ti.hwnd, nil) == 0 {
		errCode := winapi.GetLastError()
		if errCode != 0 {
			err := fmt.Errorf("%d", errCode)
			log.Printf("wintray: TrackPopupMenu failed: %v", err)
			return err
		}
	}

	return nil
}

func (ti *TrayIcon) registerIcon() bool {
	data := ti.initData()
	data.UFlags |= NIF_MESSAGE | NIF_SHOWTIP
	data.UCallbackMessage = TrayIconMsg
	if !data.Notify(winapi.NIM_ADD) {
		reportTrayFatalError(fmt.Sprintf("Shell_NotifyIcon(NIM_ADD) failed: %d", winapi.GetLastError()))
		return false
	}
	if data.Notify(winapi.NIM_MODIFY) {
		// nothing to log on success
	} else {
		log.Printf("wintray: NIM_MODIFY failed (post NIM_ADD): %d", winapi.GetLastError())
	}
	return true
}

func (ti *TrayIcon) unregisterIcon() {
	data := ti.initData()
	if !data.Notify(winapi.NIM_DELETE) {
		log.Printf("wintray: NIM_DELETE failed: %d", winapi.GetLastError())
		return
	}
}

func (ti *TrayIcon) menuSelected(id uint32) error {
	ti.menuItemsLock.RLock()
	item, ok := ti.menuItems[id]
	ti.menuItemsLock.RUnlock()
	if !ok {
		return fmt.Errorf("no menu item with ID %v", id)
	}

	select {
	case item.ClickedCh <- struct{}{}:
	// in case no one waiting for the channel
	default:
	}

	return nil
}

func NewTrayIcon() *TrayIcon {
	ti := &TrayIcon{
		guid:      guid(),
		commandCh: make(chan trayCommand, 128),
	}
	return ti
}

func (ti *TrayIcon) Run(onReady, onExit func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ti.hwnd = ti.createMainWindow()
	ti.createMenu()
	ti.icon = winapi.LoadIcon(winapi.GetModuleHandle(nil), winapi.MAKEINTRESOURCE(3))
	ti.registerIcon()
	ti.SetIcon(ti.icon)

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

	//winapi.ShowWindow(ti.hwnd, winapi.SW_SHOW)
	//winapi.ShowWindow(ti.hwnd, winapi.SW_HIDE)
	onReady()
	defer onExit()

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

func (ti *TrayIcon) Quit() {
	ti.stopCommandQueue()
	winapi.PostMessage(
		ti.hwnd,
		winapi.WM_CLOSE,
		0,
		0,
	)
}
