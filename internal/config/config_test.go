package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultValidate(t *testing.T) {
	rt, err := Validate(Default(), `C:\cfg.toml`, `C:\logs`, `C:\Users\tester`)
	if err != nil {
		t.Fatal(err)
	}
	if rt.BackendPipePath != `\\.\pipe\openssh-ssh-agent` {
		t.Fatalf("pipe = %q", rt.BackendPipePath)
	}
	if !rt.PageantEnabled || !rt.CygwinEnabled {
		t.Fatal("interfaces should be enabled")
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	_, err := Decode(strings.NewReader("version=1\nunknown=true\n"))
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestValidationFailures(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
	}{
		{"version", func(c *Config) { c.Version = 2 }},
		{"backend", func(c *Config) { c.Backend.Type = "local" }},
		{"pipe", func(c *Config) { c.Backend.Pipe = "" }},
		{"timeout", func(c *Config) { c.Backend.ConnectTimeout = "0s" }},
		{"path", func(c *Config) { c.Interfaces.Cygwin.SocketPath = "relative" }},
		{"level", func(c *Config) { c.Logging.Level = "trace" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			tt.edit(&cfg)
			if _, err := Validate(cfg, `C:\cfg`, `C:\logs`, `C:\Users\x`); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestCreateDefaultDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.toml")
	created, err := CreateDefault(path)
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	created, err = CreateDefault(path)
	if err != nil || created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "sentinel" {
		t.Fatal("existing file overwritten")
	}
}
