package config

import (
	"path/filepath"
	"testing"
)

func TestToggleBooleanSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if _, err := CreateDefault(path); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		setting BooleanSetting
		want    bool
	}{
		{PageantEnabled, false},
		{CygwinEnabled, false},
		{ShowSignNotifications, true},
	} {
		got, err := ToggleBooleanSetting(path, test.setting)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Fatalf("ToggleBooleanSetting(%d)=%v, want %v", test.setting, got, test.want)
		}
	}

	got, err := LoadBooleanSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	want := BooleanSettings{
		PageantEnabled:        false,
		CygwinEnabled:         false,
		ShowSignNotifications: true,
	}
	if got != want {
		t.Fatalf("settings=%+v, want %+v", got, want)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("saved configuration is invalid: %v", err)
	}
}

func TestToggleBooleanSettingRejectsUnknownSetting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if _, err := CreateDefault(path); err != nil {
		t.Fatal(err)
	}
	if _, err := ToggleBooleanSetting(path, BooleanSetting(255)); err == nil {
		t.Fatal("expected unknown setting error")
	}
}
