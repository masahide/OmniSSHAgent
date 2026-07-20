//go:build windows

package tray

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"testing"

	"github.com/masahide/OmniSSHAgent/internal/app"
	"github.com/masahide/OmniSSHAgent/internal/config"
)

func TestTrayUsesOriginalOmniSSHAgentIcon(t *testing.T) {
	icon, err := assets.ReadFile("assets/tray.ico")
	if err != nil {
		t.Fatal(err)
	}
	const want = "f1510437cf0c19fc0496ad43f39c76d6e4dd2c7cf43a6d6fb5e844536554268a"
	if got := fmt.Sprintf("%x", sha256.Sum256(icon)); got != want {
		t.Fatalf("tray icon SHA-256=%s, want original OmniSSHAgent icon %s", got, want)
	}
}

func TestRequiredMenu(t *testing.T) {
	var labels []string
	for _, item := range requiredMenuEntries() {
		if item.flags != mfSeparator {
			labels = append(labels, item.text)
		}
	}
	want := []string{
		"Status: Degraded",
		"Open configuration",
		"Open configuration directory",
		"Open log directory",
		"Settings (restart required)",
		"Enable Pageant interface",
		"Enable Cygwin/MSYS2 interface",
		"Show signing notifications",
		"Start with Windows",
		"Quit",
	}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("labels=%v", labels)
	}
}

func TestStateLabels(t *testing.T) {
	for state, want := range map[app.State]string{
		app.StateRunning:            "Running",
		app.StateDegraded:           "Degraded",
		app.StateConfigurationError: "Configuration error",
	} {
		if got := stateLabel(state); got != want {
			t.Fatalf("%s=%q", state, got)
		}
	}
}

func TestCommandsAndShutdownRejection(t *testing.T) {
	oldOpenPath := openPath
	oldOpenConfiguration := openConfiguration
	oldCreateDefault := createDefaultConfig
	oldAutoStartEnabled := autoStartEnabled
	oldSetAutoStartEnabled := setAutoStartEnabled
	oldLoadBooleanSettings := loadBooleanSettings
	oldToggleBooleanSetting := toggleBooleanSetting
	defer func() {
		openPath = oldOpenPath
		openConfiguration = oldOpenConfiguration
		createDefaultConfig = oldCreateDefault
		autoStartEnabled = oldAutoStartEnabled
		setAutoStartEnabled = oldSetAutoStartEnabled
		loadBooleanSettings = oldLoadBooleanSettings
		toggleBooleanSetting = oldToggleBooleanSetting
	}()
	var opened []string
	openPath = func(path string) error { opened = append(opened, path); return nil }
	openConfiguration = func(path string) error { opened = append(opened, path); return nil }
	createDefaultConfig = func(path string) (bool, error) {
		opened = append(opened, "create:"+path)
		return true, nil
	}
	autoStart := false
	autoStartEnabled = func() (bool, error) { return autoStart, nil }
	setAutoStartEnabled = func(enabled bool) error {
		autoStart = enabled
		return nil
	}
	booleanSettings := config.BooleanSettings{
		PageantEnabled: true,
		CygwinEnabled:  true,
	}
	loadBooleanSettings = func(string) (config.BooleanSettings, error) {
		return booleanSettings, nil
	}
	toggleBooleanSetting = func(_ string, setting config.BooleanSetting) (bool, error) {
		switch setting {
		case config.PageantEnabled:
			booleanSettings.PageantEnabled = !booleanSettings.PageantEnabled
			return booleanSettings.PageantEnabled, nil
		case config.CygwinEnabled:
			booleanSettings.CygwinEnabled = !booleanSettings.CygwinEnabled
			return booleanSettings.CygwinEnabled, nil
		case config.ShowSignNotifications:
			booleanSettings.ShowSignNotifications = !booleanSettings.ShowSignNotifications
			return booleanSettings.ShowSignNotifications, nil
		}
		return false, nil
	}
	quit := make(chan struct{}, 2)
	tray := New(`C:\Config\config.toml`, `C:\Logs`, func() { quit <- struct{}{} })
	tray.command(menuOpenConfig)
	tray.command(menuOpenConfigDir)
	tray.command(menuOpenLogDir)
	tray.command(menuPageant)
	tray.command(menuCygwin)
	tray.command(menuSignNotify)
	if booleanSettings.PageantEnabled || booleanSettings.CygwinEnabled || !booleanSettings.ShowSignNotifications {
		t.Fatalf("Boolean settings were not toggled: %+v", booleanSettings)
	}
	tray.command(menuAutoStart)
	if !autoStart {
		t.Fatal("auto-start was not enabled")
	}
	tray.command(menuAutoStart)
	if autoStart {
		t.Fatal("auto-start was not disabled")
	}
	tray.command(menuQuit)
	<-quit
	wantOpened := []string{
		`create:C:\Config\config.toml`,
		`C:\Config\config.toml`,
		`C:\Config`,
		`C:\Logs`,
		`create:C:\Config\config.toml`,
		`create:C:\Config\config.toml`,
		`create:C:\Config\config.toml`,
	}
	if !reflect.DeepEqual(opened, wantOpened) {
		t.Fatalf("opened=%v, want %v", opened, wantOpened)
	}
	tray.shuttingDown = true
	tray.command(menuQuit)
	select {
	case <-quit:
		t.Fatal("command accepted during shutdown")
	default:
	}
}
