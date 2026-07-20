package config

import "fmt"

type BooleanSetting uint8

const (
	PageantEnabled BooleanSetting = iota
	CygwinEnabled
	ShowSignNotifications
)

type BooleanSettings struct {
	PageantEnabled        bool
	CygwinEnabled         bool
	ShowSignNotifications bool
}

func LoadBooleanSettings(path string) (BooleanSettings, error) {
	cfg, err := Load(path)
	if err != nil {
		return BooleanSettings{}, err
	}
	return booleanSettings(cfg), nil
}

// ToggleBooleanSetting flips one Boolean TOML setting and returns its new value.
func ToggleBooleanSetting(path string, setting BooleanSetting) (bool, error) {
	cfg, err := Load(path)
	if err != nil {
		return false, err
	}
	var enabled bool
	switch setting {
	case PageantEnabled:
		cfg.Interfaces.Pageant.Enabled = !cfg.Interfaces.Pageant.Enabled
		enabled = cfg.Interfaces.Pageant.Enabled
	case CygwinEnabled:
		cfg.Interfaces.Cygwin.Enabled = !cfg.Interfaces.Cygwin.Enabled
		enabled = cfg.Interfaces.Cygwin.Enabled
	case ShowSignNotifications:
		cfg.Tray.ShowSignNotifications = !cfg.Tray.ShowSignNotifications
		enabled = cfg.Tray.ShowSignNotifications
	default:
		return false, fmt.Errorf("unknown Boolean setting %d", setting)
	}
	if err := Save(path, cfg); err != nil {
		return false, err
	}
	return enabled, nil
}

func booleanSettings(cfg Config) BooleanSettings {
	return BooleanSettings{
		PageantEnabled:        cfg.Interfaces.Pageant.Enabled,
		CygwinEnabled:         cfg.Interfaces.Cygwin.Enabled,
		ShowSignNotifications: cfg.Tray.ShowSignNotifications,
	}
}
