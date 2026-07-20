package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/masahide/OmniSSHAgent/internal/config"
)

func TestVersionAndUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	result := Run([]string{"version"}, &out, &errOut)
	if result.Code != 0 || !strings.Contains(out.String(), "GOOS:") {
		t.Fatalf("%+v %q", result, out.String())
	}
	result = Run([]string{"bad"}, &out, &errOut)
	if result.Code != ExitUsage {
		t.Fatal(result.Code)
	}
}

func TestNoArgsStartsApplication(t *testing.T) {
	if !Run(nil, &bytes.Buffer{}, &bytes.Buffer{}).StartApplication {
		t.Fatal("not started")
	}
}

func TestConfigPathAndCheckConfig(t *testing.T) {
	var out, errOut bytes.Buffer
	result := Run([]string{"config-path"}, &out, &errOut)
	if result.Code != 0 || !strings.Contains(strings.ToLower(out.String()), "config.toml") {
		t.Fatalf("result=%+v out=%q err=%q", result, out.String(), errOut.String())
	}

	path := filepath.Join(t.TempDir(), "config.toml")
	if _, err := config.CreateDefault(path); err != nil {
		t.Fatal(err)
	}
	t.Setenv("USERPROFILE", `C:\Users\tester`)
	out.Reset()
	errOut.Reset()
	result = Run([]string{"check-config", "--config", path}, &out, &errOut)
	if result.Code != 0 {
		t.Fatalf("result=%+v err=%q", result, errOut.String())
	}
	if err := os.WriteFile(path, []byte("version = 999\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result = Run([]string{"check-config", "--config", path}, &out, &errOut)
	if result.Code != ExitConfiguration {
		t.Fatalf("code=%d err=%q", result.Code, errOut.String())
	}
}
