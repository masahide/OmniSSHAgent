//go:build windows

package tray

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/masahide/OmniSSHAgent/internal/config"
	"github.com/masahide/OmniSSHAgent/internal/testutil"
	"golang.org/x/crypto/ssh"
)

func TestMergeSettingsValidatesAndPreservesReservedFields(t *testing.T) {
	cfg := config.Default()
	cfg.Tray.ShowSignNotifications = true
	cfg = mergeSettings(cfg, settingsValues{
		PageantEnabled: false,
		CygwinEnabled:  true,
		SocketPath:     "",
		BackendType:    config.BackendWindowsOpenSSH,
		Pipe:           "openssh-ssh-agent",
		ConnectTimeout: "3s",
		LogLevel:       "DEBUG",
	})
	if cfg.Interfaces.Pageant.Enabled {
		t.Fatal("pageant should be disabled")
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("level=%q", cfg.Logging.Level)
	}
	if cfg.Backend.ConnectTimeout != "3s" {
		t.Fatalf("timeout=%q", cfg.Backend.ConnectTimeout)
	}
	if !cfg.Tray.ShowSignNotifications {
		t.Fatal("reserved tray field was lost")
	}
	if cfg.Backend.Type != config.BackendWindowsOpenSSH {
		t.Fatalf("type=%q", cfg.Backend.Type)
	}
	if err := validateSettings(cfg, `C:\cfg.toml`, `C:\Users\tester`); err != nil {
		t.Fatal(err)
	}
}

func TestMergeSettingsRejectsInvalidValues(t *testing.T) {
	cfg := mergeSettings(config.Default(), settingsValues{
		PageantEnabled: true,
		CygwinEnabled:  true,
		SocketPath:     "relative.sock",
		BackendType:    config.BackendWindowsOpenSSH,
		Pipe:           "openssh-ssh-agent",
		ConnectTimeout: "5s",
		LogLevel:       "info",
	})
	if err := validateSettings(cfg, `C:\cfg.toml`, `C:\Users\tester`); err == nil {
		t.Fatal("expected relative socket path to fail")
	}
}

func TestMergeSettingsSelectsEmbeddedBackend(t *testing.T) {
	cfg := mergeSettings(config.Default(), settingsValues{
		PageantEnabled: true,
		CygwinEnabled:  true,
		BackendType:    config.BackendEmbedded,
		Pipe:           "",
		ConnectTimeout: "ignored",
		LogLevel:       "info",
	})
	if cfg.Backend.Type != config.BackendEmbedded {
		t.Fatalf("type=%q", cfg.Backend.Type)
	}
	if err := validateSettings(cfg, `C:\cfg.toml`, `C:\Users\tester`); err != nil {
		t.Fatal(err)
	}
}

func TestAddKeyFromFile(t *testing.T) {
	dir := t.TempDir()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, encPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	plainPath := filepath.Join(dir, "id_ed25519")
	if err := writePrivateKey(plainPath, priv, nil); err != nil {
		t.Fatal(err)
	}
	encryptedPath := filepath.Join(dir, "id_ed25519_enc")
	if err := writePrivateKey(encryptedPath, encPriv, []byte("secret")); err != nil {
		t.Fatal(err)
	}

	b := testutil.NewBackend()
	if err := addKeyFromFile(b, plainPath, nil); err != nil {
		t.Fatal(err)
	}
	keys, err := b.List()
	if err != nil || len(keys) != 1 {
		t.Fatalf("keys=%d err=%v", len(keys), err)
	}
	if got := formatListedKey(keys[0]); got == "" || keys[0].Comment != "id_ed25519" {
		t.Fatalf("listed=%q comment=%q", got, keys[0].Comment)
	}

	asked := false
	if err := addKeyFromFile(b, encryptedPath, func() ([]byte, bool) {
		asked = true
		return []byte("secret"), true
	}); err != nil {
		t.Fatal(err)
	}
	if !asked {
		t.Fatal("passphrase was not requested")
	}
	if err := addKeyFromFile(b, filepath.Join(dir, "id.ppk"), nil); err == nil {
		t.Fatal("expected ppk rejection")
	}

	if err := b.Remove(keys[0]); err != nil {
		t.Fatal(err)
	}
	keys, err = b.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("after remove remaining=%d", len(keys))
	}
}

func TestAddKeyFromFileCanceledPassphrase(t *testing.T) {
	dir := t.TempDir()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "id_ed25519")
	if err := writePrivateKey(path, priv, []byte("secret")); err != nil {
		t.Fatal(err)
	}
	b := testutil.NewBackend()
	err = addKeyFromFile(b, path, func() ([]byte, bool) { return nil, false })
	if !errors.Is(err, errDialogCanceled) {
		t.Fatalf("err=%v", err)
	}
	keys, err := b.List()
	if err != nil || len(keys) != 0 {
		t.Fatalf("keys=%d err=%v", len(keys), err)
	}
}

func writePrivateKey(path string, key any, passphrase []byte) error {
	var block *pem.Block
	var err error
	if len(passphrase) == 0 {
		block, err = ssh.MarshalPrivateKey(key, "")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(key, "", passphrase)
	}
	if err != nil {
		return err
	}
	return os.WriteFile(path, pem.EncodeToMemory(block), 0600)
}

func TestSettingsCommandOpensDialogHook(t *testing.T) {
	old := openSettingsDialog
	defer func() { openSettingsDialog = old }()
	opened := 0
	openSettingsDialog = func(*Tray) { opened++ }
	tray := New(`C:\Config\config.toml`, `C:\Logs`, nil)
	tray.command(menuSettings)
	if opened != 1 {
		t.Fatalf("opened=%d", opened)
	}
}
