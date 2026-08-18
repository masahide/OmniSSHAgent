//go:build windows

package tray

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/masahide/OmniSSHAgent/internal/config"
)

const (
	settingsClass  = "OmniSSHAgentSettingsWindow"
	idGroupCompat  = 2101
	idPageant      = 2102
	idCygwin       = 2103
	idSocketLabel  = 2104
	idSocket       = 2105
	idBrowse       = 2106
	idGroupBackend = 2107
	idPipeLabel    = 2108
	idPipe         = 2109
	idTimeoutLabel = 2110
	idTimeout      = 2111
	idGroupApp     = 2112
	idAutoStart    = 2113
	idLogLabel     = 2114
	idLogLevel     = 2115
	idRestartNote  = 2116
	idManageKeys   = 2117
	idApply        = 2118
)

var logLevels = []string{"debug", "info", "warn", "error"}

type settingsValues struct {
	PageantEnabled bool
	CygwinEnabled  bool
	SocketPath     string
	Pipe           string
	ConnectTimeout string
	LogLevel       string
	AutoStart      bool
}

type settingsDialog struct {
	tray      *Tray
	m         metrics
	hwnd      uintptr
	pageant   uintptr
	cygwin    uintptr
	socket    uintptr
	pipe      uintptr
	timeout   uintptr
	autoStart uintptr
	logLevel  uintptr
}

func mergeSettings(cfg config.Config, v settingsValues) config.Config {
	cfg.Interfaces.Pageant.Enabled = v.PageantEnabled
	cfg.Interfaces.Cygwin.Enabled = v.CygwinEnabled
	cfg.Interfaces.Cygwin.SocketPath = strings.TrimSpace(v.SocketPath)
	cfg.Backend.Pipe = strings.TrimSpace(v.Pipe)
	cfg.Backend.ConnectTimeout = strings.TrimSpace(v.ConnectTimeout)
	cfg.Logging.Level = strings.ToLower(strings.TrimSpace(v.LogLevel))
	return cfg
}

func validateSettings(cfg config.Config, configPath, userProfile string) error {
	_, err := config.Validate(cfg, configPath, "", userProfile)
	return err
}

func (t *Tray) openSettings() {
	if settingsDlg != nil && settingsDlg.hwnd != 0 {
		setForegroundWindow.Call(settingsDlg.hwnd)
		return
	}
	if _, err := createDefaultConfig(t.configPath); err != nil {
		ShowFatal("Settings", err)
		return
	}
	d := &settingsDialog{tray: t, m: newMetrics()}
	settingsDlg = d
	defer func() {
		if settingsDlg == d {
			settingsDlg = nil
		}
	}()
	if err := d.create(t.window); err != nil {
		ShowFatal("Settings", err)
		return
	}
	runModal(t.window, d.hwnd)
}

var settingsDlg *settingsDialog

func (d *settingsDialog) create(owner uintptr) error {
	m := d.m
	width := m.s(492)
	height := m.s(458)
	hwnd, err := createPopup(settingsClass, "OmniSSHAgent Settings", width, height, owner, settingsCallback)
	if err != nil {
		return err
	}
	d.hwnd = hwnd
	if err := d.createControls(); err != nil {
		destroyWindow.Call(hwnd)
		d.hwnd = 0
		return err
	}
	if err := d.load(); err != nil {
		destroyWindow.Call(hwnd)
		d.hwnd = 0
		return err
	}
	return nil
}

func (d *settingsDialog) createControls() error {
	m := d.m
	pad := m.s(12)
	labelW := m.s(92)
	editH := m.s(23)
	btnW := m.s(88)
	btnH := m.s(24)
	innerX := pad + m.s(10)
	innerW := m.s(448)
	editX := innerX + labelW
	editW := innerW - labelW - m.s(96)
	browseW := m.s(84)

	compatTop := pad
	compatH := m.s(118)
	if _, err := d.child("Button", "Compatibility", bsGroupBox, 0, pad, compatTop, m.s(468), compatH, idGroupCompat); err != nil {
		return err
	}
	var err error
	if d.pageant, err = d.child("Button", "Enable Pageant interface", bsAutoCheckBox|wsTabStop, 0, innerX, compatTop+m.s(22), innerW, editH, idPageant); err != nil {
		return err
	}
	if d.cygwin, err = d.child("Button", "Enable Cygwin/MSYS2 interface", bsAutoCheckBox|wsTabStop, 0, innerX, compatTop+m.s(46), innerW, editH, idCygwin); err != nil {
		return err
	}
	if _, err = d.child("Static", "Socket path:", 0, 0, innerX, compatTop+m.s(78), labelW, editH, idSocketLabel); err != nil {
		return err
	}
	if d.socket, err = d.child("Edit", "", esAutoHScroll|wsTabStop, wsExClientEdge, editX, compatTop+m.s(74), editW, editH, idSocket); err != nil {
		return err
	}
	if _, err = d.child("Button", "Browse...", bsPushButton|wsTabStop, 0, editX+editW+m.s(8), compatTop+m.s(74), browseW, btnH, idBrowse); err != nil {
		return err
	}

	backendTop := compatTop + compatH + m.s(10)
	backendH := m.s(86)
	if _, err = d.child("Button", "Backend", bsGroupBox, 0, pad, backendTop, m.s(468), backendH, idGroupBackend); err != nil {
		return err
	}
	if _, err = d.child("Static", "Named pipe:", 0, 0, innerX, backendTop+m.s(24), labelW, editH, idPipeLabel); err != nil {
		return err
	}
	if d.pipe, err = d.child("Edit", "", esAutoHScroll|wsTabStop, wsExClientEdge, editX, backendTop+m.s(20), innerW-labelW, editH, idPipe); err != nil {
		return err
	}
	if _, err = d.child("Static", "Timeout:", 0, 0, innerX, backendTop+m.s(52), labelW, editH, idTimeoutLabel); err != nil {
		return err
	}
	if d.timeout, err = d.child("Edit", "", esAutoHScroll|wsTabStop, wsExClientEdge, editX, backendTop+m.s(48), m.s(80), editH, idTimeout); err != nil {
		return err
	}

	appTop := backendTop + backendH + m.s(10)
	appH := m.s(96)
	if _, err = d.child("Button", "Application", bsGroupBox, 0, pad, appTop, m.s(468), appH, idGroupApp); err != nil {
		return err
	}
	if d.autoStart, err = d.child("Button", "Start with Windows", bsAutoCheckBox|wsTabStop, 0, innerX, appTop+m.s(22), innerW, editH, idAutoStart); err != nil {
		return err
	}
	if _, err = d.child("Static", "Log level:", 0, 0, innerX, appTop+m.s(50), labelW, editH, idLogLabel); err != nil {
		return err
	}
	if d.logLevel, err = d.child("ComboBox", "", cbsDropDownList|wsVScroll|wsTabStop, 0, editX, appTop+m.s(46), m.s(120), m.s(140), idLogLevel); err != nil {
		return err
	}
	for _, level := range logLevels {
		comboAdd(d.logLevel, level)
	}
	if _, err = d.child("Static", "Interface and backend changes apply after restart.", 0, 0, innerX, appTop+m.s(74), innerW, editH, idRestartNote); err != nil {
		return err
	}

	buttonY := appTop + appH + m.s(12)
	if _, err = d.child("Button", "Manage keys...", bsPushButton|wsTabStop, 0, pad, buttonY, m.s(120), btnH, idManageKeys); err != nil {
		return err
	}
	if _, err = d.child("Button", "OK", bsDefPushButton|wsTabStop, 0, m.s(216), buttonY, btnW, btnH, idOK); err != nil {
		return err
	}
	if _, err = d.child("Button", "Cancel", bsPushButton|wsTabStop, 0, m.s(310), buttonY, btnW, btnH, idCancel); err != nil {
		return err
	}
	if _, err = d.child("Button", "Apply", bsPushButton|wsTabStop, 0, m.s(404), buttonY, btnW, btnH, idApply); err != nil {
		return err
	}
	return nil
}

func (d *settingsDialog) child(class, title string, style, ex uint32, x, y, w, h int32, id uintptr) (uintptr, error) {
	hwnd, err := createChild(class, title, style, ex, x, y, w, h, d.hwnd, id)
	if err != nil {
		return 0, err
	}
	setChildFont(hwnd, d.m.font)
	return hwnd, nil
}

func (d *settingsDialog) load() error {
	cfg, err := config.Load(d.tray.configPath)
	if err != nil {
		return err
	}
	setChecked(d.pageant, cfg.Interfaces.Pageant.Enabled)
	setChecked(d.cygwin, cfg.Interfaces.Cygwin.Enabled)
	socket := cfg.Interfaces.Cygwin.SocketPath
	if socket == "" {
		if profile := os.Getenv("USERPROFILE"); profile != "" {
			socket = filepath.Join(profile, ".ssh", "omnisshagent-cygwin.sock")
		}
	}
	setWindowText(d.socket, socket)
	setWindowText(d.pipe, cfg.Backend.Pipe)
	setWindowText(d.timeout, cfg.Backend.ConnectTimeout)
	level := strings.ToLower(cfg.Logging.Level)
	selected := 1
	for i, name := range logLevels {
		if name == level {
			selected = i
			break
		}
	}
	comboSelect(d.logLevel, selected)
	enabled, err := autoStartEnabled()
	if err != nil {
		return err
	}
	setChecked(d.autoStart, enabled)
	return nil
}

func (d *settingsDialog) values() settingsValues {
	level := "info"
	if i := comboIndex(d.logLevel); i >= 0 && i < len(logLevels) {
		level = logLevels[i]
	}
	return settingsValues{
		PageantEnabled: isChecked(d.pageant),
		CygwinEnabled:  isChecked(d.cygwin),
		SocketPath:     windowText(d.socket),
		Pipe:           windowText(d.pipe),
		ConnectTimeout: windowText(d.timeout),
		LogLevel:       level,
		AutoStart:      isChecked(d.autoStart),
	}
}

func (d *settingsDialog) apply() bool {
	if _, err := createDefaultConfig(d.tray.configPath); err != nil {
		showError(d.hwnd, "Settings", err)
		return false
	}
	cfg, err := config.Load(d.tray.configPath)
	if err != nil {
		showError(d.hwnd, "Settings", err)
		return false
	}
	cfg = mergeSettings(cfg, d.values())
	if err := validateSettings(cfg, d.tray.configPath, os.Getenv("USERPROFILE")); err != nil {
		showError(d.hwnd, "Settings", err)
		return false
	}
	if err := config.Save(d.tray.configPath, cfg); err != nil {
		showError(d.hwnd, "Settings", err)
		return false
	}
	if err := setAutoStartEnabled(d.values().AutoStart); err != nil {
		showError(d.hwnd, "Auto-start setting failed", err)
		return false
	}
	_ = d.tray.applyMenuChecks()
	return true
}

func (d *settingsDialog) browseSocket() {
	initialDir := os.Getenv("USERPROFILE")
	current := strings.TrimSpace(windowText(d.socket))
	initialFile := "omnisshagent-cygwin.sock"
	if current != "" {
		initialDir = filepath.Dir(current)
		initialFile = filepath.Base(current)
	} else if initialDir != "" {
		initialDir = filepath.Join(initialDir, ".ssh")
	}
	path, ok := pickOpenFile(
		d.hwnd,
		"Cygwin socket path",
		initialDir,
		initialFile,
		[]string{"Socket files", "*.sock", "All files", "*.*"},
		false,
	)
	if ok {
		setWindowText(d.socket, path)
	}
}

func (d *settingsDialog) close() {
	if d.hwnd != 0 {
		destroyWindow.Call(d.hwnd)
	}
}

func settingsWindowProc(window uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	d := settingsDlg
	if d == nil || (d.hwnd != 0 && d.hwnd != window) {
		result, _, _ := defWindowProc.Call(window, uintptr(msg), wParam, lParam)
		return result
	}
	switch msg {
	case wmCommand:
		switch commandID(wParam) {
		case idOK:
			if d.apply() {
				d.close()
			}
			return 0
		case idCancel:
			d.close()
			return 0
		case idApply:
			d.apply()
			return 0
		case idBrowse:
			d.browseSocket()
			return 0
		case idManageKeys:
			d.tray.openKeys(d.hwnd)
			return 0
		}
	case wmClose:
		d.close()
		return 0
	case wmDestroy:
		d.hwnd = 0
		if settingsDlg == d {
			settingsDlg = nil
		}
		return 0
	}
	result, _, _ := defWindowProc.Call(window, uintptr(msg), wParam, lParam)
	return result
}
