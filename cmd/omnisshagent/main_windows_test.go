//go:build windows

package main

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/masahide/OmniSSHAgent/internal/config"
)

func TestEmbeddedBackendCompositionIncludesAllInterfaces(t *testing.T) {
	runtimeConfig := config.RuntimeConfig{
		BackendType:    config.BackendEmbedded,
		ConnectTimeout: time.Second,
		CygwinPath:     `C:\Users\tester\.ssh\test.sock`,
		PageantEnabled: true,
		CygwinEnabled:  true,
	}
	b, err := newBackend(runtimeConfig)
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	components := newComponents(runtimeConfig, b, logger)
	want := []string{"openssh", "pageant", "cygwin"}
	if len(components) != len(want) {
		t.Fatalf("components=%d, want %d", len(components), len(want))
	}
	for i, name := range want {
		if components[i].Name() != name {
			t.Fatalf("component[%d]=%q, want %q", i, components[i].Name(), name)
		}
	}
}

func TestWindowsOpenSSHBackendDoesNotOwnOpenSSHPipe(t *testing.T) {
	runtimeConfig := config.RuntimeConfig{
		BackendType:     config.BackendWindowsOpenSSH,
		BackendPipePath: `\\.\pipe\test-agent`,
		ConnectTimeout:  time.Second,
		PageantEnabled:  true,
	}
	b, err := newBackend(runtimeConfig)
	if err != nil {
		t.Fatal(err)
	}
	components := newComponents(runtimeConfig, b, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if len(components) != 1 || components[0].Name() != "pageant" {
		t.Fatalf("unexpected components: %#v", components)
	}
}
