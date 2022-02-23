package sshutil

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/kayrus/putty"
	"github.com/masahide/ssh-agent-win/pkg/store"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type PrivateKeyFile struct {
	FilePath   string `json:"filePath"`
	Type       string `json:"type"`
	Algo       string `json:"algo"`
	Encryption bool   `json:"encryption"`
	Passphrase string `json:"passphrase"`
}

type KeyRing struct {
	agent.Agent
	settings *store.Settings
}

func (k *KeyRing) AddKeySettings(key PrivateKeyFile) error {
	id, err := uuid.NewUUID()
	if err != nil {
		return err
	}
	data := store.KeyInfo{
		ID:         id.String(),
		Type:       key.Type,
		Encryption: key.Encryption,
		FilePath:   key.FilePath,
	}
	k.settings.Keys = append(k.settings.Keys, data)
	err = k.settings.SecretStore.Set(id.String(), key.Passphrase)
	if err != nil {
		return err
	}
	err = k.settings.Save()
	if err != nil {
		return err
	}
	return nil
}

func (k *KeyRing) AddKey(key PrivateKeyFile) error {
	return nil
}

func LoadKeyfile(filePath string, passPhrase string) (*PrivateKeyFile, *agent.AddedKey, error) {
	puttyKey, err := putty.NewFromFile(filePath)
	if err == nil {
		kt := "ppk"
		algo := puttyKey.Algo
		pf := map[bool][]byte{
			true:  nil,
			false: []byte(passPhrase),
		}[len(passPhrase) == 0]
		pk, err := puttyKey.ParseRawPrivateKey(pf)
		if err != nil {
			return nil, nil, errors.New("faild decrypto")
		}
		addkey := &agent.AddedKey{PrivateKey: pk, Comment: puttyKey.Comment}
		return &PrivateKeyFile{FilePath: filePath, Type: kt,
			Algo: algo, Encryption: puttyKey.Encryption != "none"}, addkey, nil

	}
	pemBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, err
	}
	if len(passPhrase) == 0 {
		kt := "OpenSSH"
		key, err := ssh.ParseRawPrivateKey(pemBytes)
		if err == nil {
			signer, err := ssh.NewSignerFromKey(key)
			if err == nil {
				return nil, nil, fmt.Errorf("NewSignerFromKey err:%w", err)
			}
			addkey := &agent.AddedKey{PrivateKey: key}
			algo := signer.PublicKey().Type()
			return &PrivateKeyFile{FilePath: filePath, Type: kt,
				Algo: algo, Encryption: false}, addkey, nil
		}
		switch err.(type) {
		case *ssh.PassphraseMissingError:
			return &PrivateKeyFile{FilePath: filePath, Type: kt, Algo: "", Encryption: true}, nil, nil
		default:
			return nil, nil, err
		}
	}

	key, err := ssh.ParseRawPrivateKeyWithPassphrase(pemBytes, []byte(passPhrase))
	if err != nil {
		return nil, nil, err
	}
	kt := "OpenSSH"
	signer, err := ssh.NewSignerFromKey(key)
	if err == nil {
		return nil, nil, fmt.Errorf("NewSignerFromKey err:%w", err)
	}
	algo := signer.PublicKey().Type()
	addkey := &agent.AddedKey{PrivateKey: key}
	return &PrivateKeyFile{FilePath: filePath, Type: kt, Algo: algo, Encryption: false}, addkey, nil
}

func CheckKeyType(filePath string, passPhrase string) (*PrivateKeyFile, error) {
	puttyKey, err := putty.NewFromFile(filePath)
	if err == nil {
		kt := "ppk"
		algo := puttyKey.Algo
		if len(passPhrase) == 0 {
			return &PrivateKeyFile{FilePath: filePath, Type: kt, Algo: algo, Encryption: puttyKey.Encryption != "none"}, nil
		}
		_, err := puttyKey.ParseRawPrivateKey([]byte(passPhrase))
		if err != nil {
			return nil, errors.New("faild decrypto")
		}
		return &PrivateKeyFile{FilePath: filePath, Type: kt, Algo: algo, Encryption: true}, nil

	}
	pemBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	if len(passPhrase) == 0 {
		signer, err := ssh.ParsePrivateKey(pemBytes)
		kt := "OpenSSH"
		if err == nil {
			algo := signer.PublicKey().Type()
			return &PrivateKeyFile{FilePath: filePath, Type: kt, Algo: algo, Encryption: false}, nil
		}
		switch err.(type) {
		case *ssh.PassphraseMissingError:
			return &PrivateKeyFile{FilePath: filePath, Type: kt, Algo: "", Encryption: true}, nil
		default:
			return nil, err
		}
	}

	signer, err := ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(passPhrase))
	if err != nil {
		return nil, err
	}
	kt := "OpenSSH"
	algo := signer.PublicKey().Type()
	return &PrivateKeyFile{FilePath: filePath, Type: kt, Algo: algo, Encryption: true}, nil
}

type Key struct {
	MD5       string
	SHA256    string
	Type      string
	Comment   string
	PublicKey string
}

func NewKeyRing(s *store.Settings) *KeyRing {
	k := &KeyRing{settings: s}
	k.Agent = agent.NewKeyring()
	return k
}

func fpMD5(blob []byte) string {
	fp := md5.Sum(blob)
	return hex.EncodeToString(fp[:])
}
func fpSHA256(blob []byte) string {
	fp := sha256.Sum256(blob)
	return hex.EncodeToString(fp[:])
}

func (k *KeyRing) KeyList() ([]Key, error) {
	list, err := k.List()
	if err != nil {
		return nil, err
	}
	res := make([]Key, len(list))
	for i, k := range list {
		res[i].Comment = k.Comment
		res[i].Type = k.Type()
		res[i].MD5 = ssh.FingerprintLegacyMD5(k)
		res[i].SHA256 = ssh.FingerprintSHA256(k)
		res[i].PublicKey = k.String()
	}
	return res, nil
}

func generatePrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	// Private Key generation
	privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, err
	}
	// Validate Private Key
	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

/*
func (a *App) KeyList() ([]Key, error) {
	rsa, err := generatePrivateKey(4096)
	if err != nil {
		return nil, err
	}
	keyring := agent.NewKeyring()
	keyring.Add(agent.AddedKey{PrivateKey: rsa})
	list, err := keyring.List()
	if err != nil {
		return nil, err
	}
	res := make([]Key, len(list))
	for i := range list {
		res[i].Key = list[i]
		res[i].MD5 = fpMD5(list[i].Blob)
		res[i].SHA256 = fpSHA256(list[i].Blob)
	}
	return res, nil
}
*/
