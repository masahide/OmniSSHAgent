package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/masahide/OmniSSHAgent/pkg/sshkey"
)

const (
	filename = "settings.json"
)

type Store interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Remove(key string) error
}

type SaveData struct {
	Keys             []sshkey.PrivateKeyFile
	StartHidden      bool
	PageantAgent     bool
	NamedPipeAgent   bool
	UnixSocketAgent  bool
	CygWinAgent      bool
	UnixSocketPath   string
	CygWinSocketPath string
}

type Settings struct {
	SecretStore Store
	AppName     string
	SaveData
}

func initSetting() SaveData {
	home, _ := os.UserHomeDir()
	return SaveData{
		Keys:             []sshkey.PrivateKeyFile{},
		PageantAgent:     true,
		NamedPipeAgent:   true,
		UnixSocketAgent:  true,
		CygWinAgent:      true,
		UnixSocketPath:   filepath.Join(home, "OmniSSHAgent.sock"),
		CygWinSocketPath: filepath.Join(home, "OmniSSHCygwin.sock"),
	}
}

func NewSettings(AppName string, SecretStore Store) *Settings {
	return &Settings{
		SecretStore: SecretStore,
		AppName:     AppName,
	}
}

func isDir(dir string) bool {
	f, err := os.Stat(dir)
	if err != nil {
		return false
	}
	if !f.IsDir() {
		return false
	}
	return true
}

func (s *Settings) Save() error {
	dir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	dir = filepath.Join(dir, s.AppName)
	if !isDir(dir) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("mkdir [%s] err:%w", dir, err)
		}
	}
	b, err := json.Marshal(s.SaveData)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(dir, filename), b, 0600)
	return err
}
func (s *Settings) Load() error {
	dir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(filepath.Join(dir, s.AppName, filename))
	if err != nil {
		s.SaveData = initSetting()
		return s.Save()
	}
	data := SaveData{}
	err = json.Unmarshal(b, &data)
	if err != nil {
		return err
	}
	s.SaveData = data
	return nil
}
