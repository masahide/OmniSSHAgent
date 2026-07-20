//go:build windows

package tray

import (
	"reflect"
	"testing"

	"github.com/masahide/OmniSSHAgent/internal/app"
)

func TestRequiredMenu(t *testing.T) {
	var labels []string
	for _, item := range requiredMenuEntries() {
		if item.flags != mfSeparator {
			labels = append(labels, item.text)
		}
	}
	want := []string{"Status: Degraded", "Open configuration", "Open configuration directory", "Open log directory", "Quit"}
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
	defer func() {
		openPath = oldOpenPath
		openConfiguration = oldOpenConfiguration
		createDefaultConfig = oldCreateDefault
	}()
	var opened []string
	openPath = func(path string) error { opened = append(opened, path); return nil }
	openConfiguration = func(path string) error { opened = append(opened, path); return nil }
	createDefaultConfig = func(path string) (bool, error) {
		opened = append(opened, "create:"+path)
		return true, nil
	}
	quit := make(chan struct{}, 2)
	tray := New(`C:\Config\config.toml`, `C:\Logs`, func() { quit <- struct{}{} })
	tray.command(menuOpenConfig)
	tray.command(menuOpenConfigDir)
	tray.command(menuOpenLogDir)
	tray.command(menuQuit)
	<-quit
	if len(opened) != 4 {
		t.Fatalf("opened=%v", opened)
	}
	tray.shuttingDown = true
	tray.command(menuQuit)
	select {
	case <-quit:
		t.Fatal("command accepted during shutdown")
	default:
	}
}
