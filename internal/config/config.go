package config

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const (
	ApplicationName = "OmniSSHAgent"
	SchemaVersion   = 1
)

type Config struct {
	Version    int             `toml:"version"`
	Backend    BackendConfig   `toml:"backend"`
	Interfaces InterfaceConfig `toml:"interfaces"`
	Tray       TrayConfig      `toml:"tray"`
	Logging    LoggingConfig   `toml:"logging"`
}

type BackendConfig struct {
	Type           string `toml:"type"`
	Pipe           string `toml:"pipe"`
	ConnectTimeout string `toml:"connect_timeout"`
}

type InterfaceConfig struct {
	Pageant PageantConfig `toml:"pageant"`
	Cygwin  CygwinConfig  `toml:"cygwin"`
}

type PageantConfig struct {
	Enabled bool `toml:"enabled"`
}

type CygwinConfig struct {
	Enabled    bool   `toml:"enabled"`
	SocketPath string `toml:"socket_path"`
}

type TrayConfig struct {
	ShowSignNotifications bool `toml:"show_sign_notifications"`
}

type LoggingConfig struct {
	Level string `toml:"level"`
}

type RuntimeConfig struct {
	ConfigPath            string
	LogDirectory          string
	BackendPipePath       string
	ConnectTimeout        time.Duration
	CygwinPath            string
	PageantEnabled        bool
	CygwinEnabled         bool
	ShowSignNotifications bool
	LogLevel              slog.Level
}

func Default() Config {
	return Config{
		Version: SchemaVersion,
		Backend: BackendConfig{
			Type:           "windows-openssh",
			Pipe:           "openssh-ssh-agent",
			ConnectTimeout: "5s",
		},
		Interfaces: InterfaceConfig{
			Pageant: PageantConfig{Enabled: true},
			Cygwin:  CygwinConfig{Enabled: true},
		},
		Tray:    TrayConfig{ShowSignNotifications: false},
		Logging: LoggingConfig{Level: "info"},
	}
}

func Decode(r io.Reader) (Config, error) {
	var cfg Config
	decoder := toml.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode configuration: %w", err)
	}
	return cfg, nil
}

func Load(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open configuration: %w", err)
	}
	defer f.Close()
	return Decode(f)
}

func Validate(cfg Config, configPath, logDirectory, userProfile string) (RuntimeConfig, error) {
	if cfg.Version != SchemaVersion {
		return RuntimeConfig{}, fmt.Errorf("unsupported version %d", cfg.Version)
	}
	if cfg.Backend.Type != "windows-openssh" {
		return RuntimeConfig{}, fmt.Errorf("unsupported backend.type %q", cfg.Backend.Type)
	}
	pipe := strings.TrimSpace(cfg.Backend.Pipe)
	if pipe == "" {
		return RuntimeConfig{}, fmt.Errorf("backend.pipe must not be empty")
	}
	if !strings.HasPrefix(strings.ToLower(pipe), `\\.\pipe\`) {
		if strings.ContainsAny(pipe, `\/`) {
			return RuntimeConfig{}, fmt.Errorf("backend.pipe must be a pipe name or \\\\.\\pipe\\ path")
		}
		pipe = `\\.\pipe\` + pipe
	}
	timeout, err := time.ParseDuration(cfg.Backend.ConnectTimeout)
	if err != nil || timeout <= 0 {
		return RuntimeConfig{}, fmt.Errorf("backend.connect_timeout must be a positive duration")
	}
	cygwinPath := cfg.Interfaces.Cygwin.SocketPath
	if cygwinPath == "" {
		if userProfile == "" {
			return RuntimeConfig{}, fmt.Errorf("USERPROFILE is unavailable")
		}
		cygwinPath = filepath.Join(userProfile, ".ssh", "omnisshagent-cygwin.sock")
	} else if !filepath.IsAbs(cygwinPath) {
		return RuntimeConfig{}, fmt.Errorf("interfaces.cygwin.socket_path must be absolute")
	}
	var level slog.Level
	switch strings.ToLower(cfg.Logging.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return RuntimeConfig{}, fmt.Errorf("logging.level must be debug, info, warn, or error")
	}
	return RuntimeConfig{
		ConfigPath:            configPath,
		LogDirectory:          logDirectory,
		BackendPipePath:       pipe,
		ConnectTimeout:        timeout,
		CygwinPath:            filepath.Clean(cygwinPath),
		PageantEnabled:        cfg.Interfaces.Pageant.Enabled,
		CygwinEnabled:         cfg.Interfaces.Cygwin.Enabled,
		ShowSignNotifications: cfg.Tray.ShowSignNotifications,
		LogLevel:              level,
	}, nil
}
