//go:build windows

package tray

import (
	"crypto/subtle"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/masahide/OmniSSHAgent/internal/backend"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	keysClass     = "OmniSSHAgentKeysWindow"
	passwordClass = "OmniSSHAgentPasswordWindow"
	idKeyList     = 2201
	idAddKey      = 2202
	idRemoveKey   = 2203
	idPassEdit    = 2204
)

var errPassphraseRequired = errors.New("passphrase required")

type keysDialog struct {
	tray    *Tray
	owner   uintptr
	m       metrics
	hwnd    uintptr
	list    uintptr
	backend backend.Backend
	keys    []*agent.Key
}

type passwordDialog struct {
	hwnd   uintptr
	edit   uintptr
	ok     bool
	secret []byte
}

var (
	keysDlg     *keysDialog
	passwordDlg *passwordDialog
)

func parsePrivateKey(data, passphrase []byte) (any, error) {
	if len(passphrase) == 0 {
		key, err := ssh.ParseRawPrivateKey(data)
		var missing *ssh.PassphraseMissingError
		if errors.As(err, &missing) {
			return nil, errPassphraseRequired
		}
		return key, err
	}
	return ssh.ParseRawPrivateKeyWithPassphrase(data, passphrase)
}

func formatListedKey(k *agent.Key) string {
	if k == nil {
		return ""
	}
	text := k.Type() + " " + ssh.FingerprintSHA256(k)
	if comment := strings.TrimSpace(k.Comment); comment != "" {
		text += " " + comment
	}
	return text
}

func clearBytes(b []byte) {
	if len(b) == 0 {
		return
	}
	zeros := make([]byte, len(b))
	subtle.ConstantTimeCopy(1, b, zeros)
}

func addKeyFromFile(b backend.Backend, path string, askPass func() ([]byte, bool)) error {
	if b == nil {
		return errors.New("the SSH agent backend is unavailable")
	}
	if strings.EqualFold(filepath.Ext(path), ".ppk") {
		return errors.New("PuTTY .ppk keys are not supported")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	key, err := parsePrivateKey(data, nil)
	if errors.Is(err, errPassphraseRequired) {
		if askPass == nil {
			return err
		}
		pass, ok := askPass()
		if !ok {
			return errDialogCanceled
		}
		defer clearBytes(pass)
		key, err = parsePrivateKey(data, pass)
	}
	if err != nil {
		return err
	}
	return b.Add(agent.AddedKey{PrivateKey: key, Comment: filepath.Base(path)})
}

func (t *Tray) openKeys(owner uintptr) {
	if keysDlg != nil && keysDlg.hwnd != 0 {
		setForegroundWindow.Call(keysDlg.hwnd)
		return
	}
	b := t.backend()
	if b == nil {
		showError(owner, "Manage keys", errors.New("the SSH agent backend is unavailable until the configuration is valid"))
		return
	}
	d := &keysDialog{tray: t, owner: owner, m: newMetrics(), backend: b}
	keysDlg = d
	defer func() {
		if keysDlg == d {
			keysDlg = nil
		}
	}()
	if err := d.create(); err != nil {
		showError(owner, "Manage keys", err)
		return
	}
	runModal(owner, d.hwnd)
}

func (t *Tray) backend() backend.Backend {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.agent
}

func (t *Tray) SetBackend(b backend.Backend) {
	t.mu.Lock()
	t.agent = b
	t.mu.Unlock()
}

func (d *keysDialog) create() error {
	m := d.m
	hwnd, err := createPopup(keysClass, "Manage keys", m.s(492), m.s(360), d.owner, keysCallback)
	if err != nil {
		return err
	}
	d.hwnd = hwnd
	pad := m.s(12)
	btnW := m.s(88)
	btnH := m.s(24)
	listH := m.s(240)
	if d.list, err = d.child("ListBox", "", lbsNotify|lbsNoIntegralHeight|wsVScroll|wsHScroll|wsTabStop, wsExClientEdge, pad, pad, m.s(468), listH, idKeyList); err != nil {
		destroyWindow.Call(hwnd)
		d.hwnd = 0
		return err
	}
	buttonY := pad + listH + m.s(12)
	if _, err = d.child("Button", "Add...", bsPushButton|wsTabStop, 0, pad, buttonY, btnW, btnH, idAddKey); err != nil {
		destroyWindow.Call(hwnd)
		d.hwnd = 0
		return err
	}
	if _, err = d.child("Button", "Remove", bsPushButton|wsTabStop, 0, pad+btnW+m.s(8), buttonY, btnW, btnH, idRemoveKey); err != nil {
		destroyWindow.Call(hwnd)
		d.hwnd = 0
		return err
	}
	if _, err = d.child("Button", "Close", bsDefPushButton|wsTabStop, 0, m.s(392), buttonY, btnW, btnH, idCancel); err != nil {
		destroyWindow.Call(hwnd)
		d.hwnd = 0
		return err
	}
	if err := d.refresh(); err != nil {
		showError(d.hwnd, "Manage keys", err)
	}
	return nil
}

func (d *keysDialog) child(class, title string, style, ex uint32, x, y, w, h int32, id uintptr) (uintptr, error) {
	hwnd, err := createChild(class, title, style, ex, x, y, w, h, d.hwnd, id)
	if err != nil {
		return 0, err
	}
	setChildFont(hwnd, d.m.font)
	return hwnd, nil
}

func (d *keysDialog) refresh() error {
	keys, err := d.backend.List()
	if err != nil {
		return err
	}
	d.keys = keys
	listReset(d.list)
	labels := make([]string, 0, len(keys))
	for _, key := range keys {
		label := formatListedKey(key)
		labels = append(labels, label)
		listAdd(d.list, label)
	}
	listUpdateHorizontalExtent(d.list, d.m.font, labels)
	return nil
}

func (d *keysDialog) add() {
	initialDir := os.Getenv("USERPROFILE")
	if initialDir != "" {
		initialDir = filepath.Join(initialDir, ".ssh")
	}
	path, ok := pickOpenFile(
		d.hwnd,
		"Add private key",
		initialDir,
		"",
		[]string{"Private key files", "*.*", "All files", "*.*"},
		true,
	)
	if !ok {
		return
	}
	err := addKeyFromFile(d.backend, path, func() ([]byte, bool) {
		return promptPassphrase(d.hwnd, filepath.Base(path))
	})
	if err != nil {
		if errors.Is(err, errDialogCanceled) {
			return
		}
		showError(d.hwnd, "Add key", err)
		return
	}
	if err := d.refresh(); err != nil {
		showError(d.hwnd, "Manage keys", err)
	}
}

func (d *keysDialog) remove() {
	index := listIndex(d.list)
	if index < 0 || index >= len(d.keys) {
		showError(d.hwnd, "Remove key", errors.New("select a key to remove"))
		return
	}
	if err := d.backend.Remove(d.keys[index]); err != nil {
		showError(d.hwnd, "Remove key", err)
		return
	}
	if err := d.refresh(); err != nil {
		showError(d.hwnd, "Manage keys", err)
	}
}

func (d *keysDialog) close() {
	if d.hwnd != 0 {
		destroyWindow.Call(d.hwnd)
	}
}

func keysWindowProc(window uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	d := keysDlg
	if d == nil || (d.hwnd != 0 && d.hwnd != window) {
		result, _, _ := defWindowProc.Call(window, uintptr(msg), wParam, lParam)
		return result
	}
	switch msg {
	case wmCommand:
		switch commandID(wParam) {
		case idAddKey:
			d.add()
			return 0
		case idRemoveKey:
			d.remove()
			return 0
		case idCancel, idOK:
			d.close()
			return 0
		}
	case wmClose:
		d.close()
		return 0
	case wmDestroy:
		d.hwnd = 0
		if keysDlg == d {
			keysDlg = nil
		}
		return 0
	}
	result, _, _ := defWindowProc.Call(window, uintptr(msg), wParam, lParam)
	return result
}

func promptPassphrase(owner uintptr, name string) ([]byte, bool) {
	m := newMetrics()
	d := &passwordDialog{}
	passwordDlg = d
	defer func() {
		if passwordDlg == d {
			passwordDlg = nil
		}
	}()
	hwnd, err := createPopup(passwordClass, "Passphrase", m.s(380), m.s(168), owner, passwordCallback)
	if err != nil {
		showError(owner, "Passphrase", err)
		return nil, false
	}
	d.hwnd = hwnd
	pad := m.s(12)
	label := "Enter the passphrase for " + name + "."
	if _, err := createChild("Static", label, 0, 0, pad, pad, m.s(348), m.s(36), hwnd, 0); err != nil {
		destroyWindow.Call(hwnd)
		return nil, false
	}
	edit, err := createChild("Edit", "", esAutoHScroll|esPassword|wsTabStop, wsExClientEdge, pad, m.s(52), m.s(348), m.s(23), hwnd, idPassEdit)
	if err != nil {
		destroyWindow.Call(hwnd)
		return nil, false
	}
	d.edit = edit
	btnY := m.s(92)
	if _, err := createChild("Button", "OK", bsDefPushButton|wsTabStop, 0, m.s(176), btnY, m.s(88), m.s(24), hwnd, idOK); err != nil {
		destroyWindow.Call(hwnd)
		return nil, false
	}
	if _, err := createChild("Button", "Cancel", bsPushButton|wsTabStop, 0, m.s(272), btnY, m.s(88), m.s(24), hwnd, idCancel); err != nil {
		destroyWindow.Call(hwnd)
		return nil, false
	}
	setChildFont(hwnd, m.font)
	for _, child := range []uintptr{edit} {
		setChildFont(child, m.font)
	}
	runModal(owner, hwnd)
	if !d.ok {
		clearBytes(d.secret)
		return nil, false
	}
	return d.secret, true
}

func (d *passwordDialog) accept() {
	text := windowText(d.edit)
	d.secret = []byte(text)
	d.ok = true
	setWindowText(d.edit, "")
	destroyWindow.Call(d.hwnd)
}

func (d *passwordDialog) cancel() {
	d.ok = false
	setWindowText(d.edit, "")
	destroyWindow.Call(d.hwnd)
}

func passwordWindowProc(window uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	d := passwordDlg
	if d == nil || (d.hwnd != 0 && d.hwnd != window) {
		result, _, _ := defWindowProc.Call(window, uintptr(msg), wParam, lParam)
		return result
	}
	switch msg {
	case wmCommand:
		switch commandID(wParam) {
		case idOK:
			d.accept()
			return 0
		case idCancel:
			d.cancel()
			return 0
		}
	case wmClose:
		d.cancel()
		return 0
	case wmDestroy:
		d.hwnd = 0
		if passwordDlg == d {
			passwordDlg = nil
		}
		return 0
	}
	result, _, _ := defWindowProc.Call(window, uintptr(msg), wParam, lParam)
	return result
}
