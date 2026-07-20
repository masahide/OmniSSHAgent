package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

func DefaultConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve APPDATA: %w", err)
	}
	return filepath.Join(dir, ApplicationName, "config.toml"), nil
}

func DefaultLogDirectory() (string, error) {
	dir := os.Getenv("LOCALAPPDATA")
	if dir == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("resolve LOCALAPPDATA: %w", err)
		}
		dir = cache
	}
	return filepath.Join(dir, ApplicationName, "logs"), nil
}

func CreateDefault(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("inspect configuration: %w", err)
	}
	data, err := toml.Marshal(Default())
	if err != nil {
		return false, fmt.Errorf("encode default configuration: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return false, fmt.Errorf("create configuration directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return false, fmt.Errorf("create temporary configuration: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return false, fmt.Errorf("protect temporary configuration: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return false, fmt.Errorf("write temporary configuration: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return false, fmt.Errorf("flush temporary configuration: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return false, fmt.Errorf("close temporary configuration: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return false, fmt.Errorf("install configuration: %w", err)
	}
	return true, nil
}

func LoadRuntime(path string) (RuntimeConfig, error) {
	logDir, err := DefaultLogDirectory()
	if err != nil {
		return RuntimeConfig{}, err
	}
	cfg, err := Load(path)
	if err != nil {
		return RuntimeConfig{}, err
	}
	return Validate(cfg, path, logDir, os.Getenv("USERPROFILE"))
}
