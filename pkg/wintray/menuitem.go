//go:build windows
// +build windows

package wintray

import (
	"fmt"
	"sync/atomic"
	"unsafe"

	"github.com/cwchiu/go-winapi"
)

// MenuItem is used to keep track each menu item of systray.
// Don't create it directly, use the one systray.AddMenuItem() returned
type MenuItem struct {
	// ClickedCh is the channel which will be notified when the menu item is clicked
	ClickedCh chan struct{}

	// trayIcon points to the owning tray icon
	trayIcon *TrayIcon
	// id uniquely identify a menu item, not supposed to be modified
	id uint32
	// icon is the icon to use when drawing the menu
	icon winapi.HBITMAP
	// title is the text shown on menu item
	title string
	// tooltip is the text shown when pointing to menu item
	tooltip string
	// disabled menu item is grayed out and has no effect when clicked
	disabled bool
	// checked menu item has a tick before the title
	checked bool
	// has the menu item a checkbox (Linux)
	isCheckable bool
	// parent item, for sub menus
	parent *MenuItem
}

func newMenuItem(ti *TrayIcon, title, tooltip string, parent *MenuItem) *MenuItem {
	return &MenuItem{
		ClickedCh:   make(chan struct{}),
		trayIcon:    ti,
		id:          atomic.AddUint32(&ti.currentMenuID, 1),
		title:       title,
		tooltip:     tooltip,
		disabled:    false,
		checked:     false,
		isCheckable: false,
		parent:      parent,
	}
}

func newSeparatorMenuItem(ti *TrayIcon, parent *MenuItem) error {
	menuItemId := atomic.AddUint32(&ti.currentMenuID, 1)
	mi := winapi.MENUITEMINFO{
		FMask: winapi.MIIM_FTYPE | winapi.MIIM_ID | winapi.MIIM_STATE,
		FType: winapi.MFT_SEPARATOR,
		WID:   menuItemId,
	}
	mi.CbSize = uint32(unsafe.Sizeof(mi))

	ti.menusLock.RLock()
	menu := ti.menus[parent.id]
	ti.menusLock.RUnlock()
	if !winapi.InsertMenuItem(menu, 0, false, &mi) {
		return fmt.Errorf("%d", winapi.GetLastError())
	}

	return nil
}

// update propagates changes on a menu item to systray
func (item *MenuItem) update() {
	ti := item.trayIcon
	ti.menuItemsLock.Lock()
	ti.menuItems[item.id] = item
	ti.menuItemsLock.Unlock()
	ti.updateMenuItem(item)
}

func (item *MenuItem) parentID() uint32 {
	if item.parent != nil {
		return item.parent.id
	}

	return 0
}

// Disabled checks if the menu item is disabled
func (item *MenuItem) Disabled() bool {
	return item.disabled
}

// Enable a menu item regardless if it's previously enabled or not
func (item *MenuItem) Enable() {
	item.disabled = false
	item.update()
}

// Disable a menu item regardless if it's previously disabled or not
func (item *MenuItem) Disable() {
	item.disabled = true
	item.update()
}

// Checked returns if the menu item has a check mark
func (item *MenuItem) Checked() bool {
	return item.checked
}

// Check a menu item regardless if it's previously checked or not
func (item *MenuItem) Check() {
	item.checked = true
	item.update()
}

// Uncheck a menu item regardless if it's previously unchecked or not
func (item *MenuItem) Uncheck() {
	item.checked = false
	item.update()
}
