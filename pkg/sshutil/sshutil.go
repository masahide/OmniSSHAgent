package sshutil

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/kayrus/putty"
	"github.com/masahide/OmniSSHAgent/pkg/namedpipe"
	"github.com/masahide/OmniSSHAgent/pkg/sshkey"
	"github.com/masahide/OmniSSHAgent/pkg/store"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	LocalStore = "LocalStore"
)

func publicKeyString(k ssh.PublicKey, comment string) string {
	s := k.Type() + " " + base64.StdEncoding.EncodeToString(k.Marshal())
	return s + map[bool]string{true: " " + comment, false: ""}[len(comment) > 0]

}

// KeyRing saves the state of ssh-agent
type KeyRing struct {
	keyring        agent.ExtendedAgent
	settings       *store.Settings
	NotifyCallback func(action string, data interface{})
}

// AddKeySettings saves PrivateKeyFile informatio in the store
func (k *KeyRing) AddKeySettings(key sshkey.PrivateKeyFile) (string, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}
	key.ID = id.String()
	pss := key.Passphrase
	key.Passphrase = ""
	k.settings.Keys = append(k.settings.Keys, key)
	err = k.settings.SecretStore.Set(id.String(), pss)
	if err != nil {
		return "", err
	}
	err = k.settings.Save()
	if err != nil {
		return "", err
	}
	log.Printf("AddKeySettings:%s", JSONDump(key))
	return id.String(), nil
}

// DeleteKeySettings Delete PrivateKeyFile informatio in the store
func (k *KeyRing) DeleteKeySettings(sha256 string) error {
	id := ""
	newKeys := make([]sshkey.PrivateKeyFile, 0, len(k.settings.Keys)-1)
	for i := range k.settings.Keys {
		if k.settings.Keys[i].PublicKey.SHA256 == sha256 {
			id = k.settings.Keys[i].ID
			continue
		}
		newKeys = append(newKeys, k.settings.Keys[i])
	}
	k.settings.Keys = newKeys
	if len(id) == 0 {
		return nil
	}
	k.settings.SecretStore.Remove(id)
	err := k.settings.Save()
	return err
}

func (k *KeyRing) getKey(keyID string) (sshkey.PrivateKeyFile, error) {
	for _, key := range k.settings.Keys {
		if key.ID == keyID {
			return key, nil
		}
	}
	return sshkey.PrivateKeyFile{}, fmt.Errorf("Not found key ID:%s", keyID)
}

func (k *KeyRing) AddKeys() error {
	for _, key := range k.settings.Keys {
		if err := k.AddKey(key.ID); err != nil {
			return err
		}
	}
	return nil
}

// AddKey loads PrivateKeyFile into ssh-agent
func (k *KeyRing) AddKey(keyID string) error {
	pkf, err := k.getKey(keyID)
	if err != nil {
		return err
	}
	agentKeys, err := k.listPublickeys()
	if err != nil {
		return err
	}
	for _, agentKey := range agentKeys {
		if pkf.PublicKey.SHA256 == agentKey.SHA256 {
			return nil
		}
	}
	pf, err := k.settings.SecretStore.Get(keyID)
	if err != nil {
		return err
	}
	_, addkey, err := LoadKeyfile(pkf.FilePath, pf)
	if err != nil {
		return err
	}
	return k.Add(*addkey)
}

func getkeyInfo(s ssh.Signer, comment string) sshkey.PublicKey {
	pubkey := s.PublicKey()
	return sshkey.PublicKey{
		MD5:     ssh.FingerprintLegacyMD5(pubkey),
		SHA256:  ssh.FingerprintSHA256(pubkey),
		Type:    pubkey.Type(),
		Comment: comment,
		String:  publicKeyString(s.PublicKey(), comment),
	}
}

// LoadKeyfile Read privateKey file from local filesystem
func LoadKeyfile(filePath string, passPhrase string) (*sshkey.PrivateKeyFile, *agent.AddedKey, error) {
	puttyKey, err := putty.NewFromFile(filePath)
	if err == nil {
		fileType := "ppk"
		algo := puttyKey.Algo
		if len(passPhrase) == 0 && puttyKey.Encryption != "none" {
			return &sshkey.PrivateKeyFile{FilePath: filePath,
				FileType: fileType, StoreType: LocalStore,
				PublicKey:  sshkey.PublicKey{Type: algo},
				Encryption: puttyKey.Encryption != "none"}, nil, nil
		}
		pk, err := puttyKey.ParseRawPrivateKey([]byte(passPhrase))
		if err != nil {
			return nil, nil, errors.New("faild decrypto")
		}
		signer, err := ssh.NewSignerFromKey(pk)
		if err != nil {
			return nil, nil, err
		}
		pkinfo := getkeyInfo(signer, puttyKey.Comment)
		addkey := &agent.AddedKey{PrivateKey: pk, Comment: puttyKey.Comment}
		return &sshkey.PrivateKeyFile{FilePath: filePath, FileType: fileType, StoreType: LocalStore,
			PublicKey:  pkinfo,
			Encryption: puttyKey.Encryption != "none"}, addkey, nil

	}
	pemBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, err
	}
	if len(passPhrase) == 0 {
		fileType := "OpenSSH"
		key, err := ssh.ParseRawPrivateKey(pemBytes)
		if err == nil {
			signer, err := ssh.NewSignerFromKey(key)
			if err != nil {
				return nil, nil, err
			}
			addkey := &agent.AddedKey{PrivateKey: key}
			pkinfo := getkeyInfo(signer, "")
			return &sshkey.PrivateKeyFile{FilePath: filePath, FileType: fileType, StoreType: LocalStore,
				PublicKey:  pkinfo,
				Encryption: false}, addkey, nil
		}
		switch err.(type) {
		case *ssh.PassphraseMissingError:
			return &sshkey.PrivateKeyFile{FilePath: filePath, FileType: fileType, StoreType: LocalStore, Encryption: true}, nil, nil
		default:
			return nil, nil, err
		}
	}

	key, err := ssh.ParseRawPrivateKeyWithPassphrase(pemBytes, []byte(passPhrase))
	if err != nil {
		return nil, nil, err
	}
	fileType := "OpenSSH"
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, nil, err
	}
	addkey := &agent.AddedKey{PrivateKey: key}
	pkinfo := getkeyInfo(signer, "")
	return &sshkey.PrivateKeyFile{FilePath: filePath, FileType: fileType, StoreType: LocalStore, PublicKey: pkinfo, Encryption: true}, addkey, nil
}

// CheckKeyType Check the information in the PrivateKey file
func CheckKeyType(filePath string, passPhrase string) (*sshkey.PrivateKeyFile, error) {
	pf, _, err := LoadKeyfile(filePath, passPhrase)
	return pf, err
}

// NewKeyRing an Agent that holds keys in memory.
func NewKeyRing(s *store.Settings) *KeyRing {
	k := &KeyRing{settings: s}
	if s.ProxyModeOfNamedPipe {
		k.keyring = &namedpipe.NamedPipeClient{}
		return k
	}
	a := agent.NewKeyring()
	if extendedAgent, ok := a.(agent.ExtendedAgent); ok {
		k.keyring = extendedAgent
		return k
	}
	return nil
}

func fpMD5(blob []byte) string {
	fp := md5.Sum(blob)
	return hex.EncodeToString(fp[:])
}
func fpSHA256(blob []byte) string {
	fp := sha256.Sum256(blob)
	return hex.EncodeToString(fp[:])
}

func (k *KeyRing) listPublickeys() ([]sshkey.PublicKey, error) {
	list, err := k.List()
	if err != nil {
		return nil, err
	}
	res := make([]sshkey.PublicKey, len(list))
	for i, k := range list {
		res[i].Comment = k.Comment
		res[i].Type = k.Type()
		res[i].MD5 = ssh.FingerprintLegacyMD5(k)
		res[i].SHA256 = ssh.FingerprintSHA256(k)
		res[i].String = k.String()
	}
	return res, nil
}

func (k *KeyRing) RemoveKey(sha256 string) error {
	list, err := k.List()
	if err != nil {
		return err
	}
	for _, key := range list {
		if ssh.FingerprintSHA256(key) == sha256 {
			pubkey, err := ssh.ParsePublicKey(key.Marshal())
			if err != nil {
				return err
			}
			return k.Remove(pubkey)
		}
	}
	return nil
}

// KeyList wails function to display the key list with
func (k *KeyRing) KeyList() ([]sshkey.PrivateKeyFile, error) {
	agentKeys, err := k.listPublickeys()
	if err != nil {
		return nil, err
	}
	return k.mergeKeyList(agentKeys)
}

func JSONDump(data interface{}) string {
	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(b)
}

// KeyList wails function to display the key list with
func (k *KeyRing) mergeKeyList(agentKeys []sshkey.PublicKey) ([]sshkey.PrivateKeyFile, error) {
	res := make([]sshkey.PrivateKeyFile, 0, len(agentKeys)+len(k.settings.Keys))
	for i := range agentKeys {
		key := getKey(k.settings.Keys, agentKeys[i])
		if key == nil {
			res = append(res, sshkey.PrivateKeyFile{PublicKey: agentKeys[i]})
			continue
		}
		res = append(res, *key)
	}
	for i := range k.settings.Keys {
		if !hasKey(agentKeys, k.settings.Keys[i]) {
			res = append(res, k.settings.Keys[i])
			log.Printf("mergeKeyList-key:%s", JSONDump(k.settings.Keys[i]))
		}
	}
	return res, nil
}

func getKey(pkfiles []sshkey.PrivateKeyFile, pubkey sshkey.PublicKey) *sshkey.PrivateKeyFile {
	for i := range pkfiles {
		if pkfiles[i].PublicKey.SHA256 == pubkey.SHA256 {
			return &pkfiles[i]
		}
	}
	return nil
}
func hasKey(pubkeys []sshkey.PublicKey, pkfile sshkey.PrivateKeyFile) bool {
	for _, pubkey := range pubkeys {
		if pubkey.SHA256 == pkfile.PublicKey.SHA256 {
			return true
		}
	}
	return false
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

func (k *KeyRing) notice(action string, data interface{}) {
	//log.Printf("%s:%s", action, JSONDump(data))
	if k.NotifyCallback == nil {
		return
	}
	k.NotifyCallback(action, data)
}

func (k *KeyRing) List() ([]*agent.Key, error) {
	k.notice("List", nil)
	defer k.notice("Listed", nil)
	return k.keyring.List()
}
func (k *KeyRing) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	//k.notice("Sign", map[string]interface{}{"publickey": key.Marshal(), "data": data})
	k.notice("Sign", key)
	defer k.notice("Signed", key)
	return k.keyring.Sign(key, data)
}
func (k *KeyRing) Add(key agent.AddedKey) error {
	k.notice("Add", key)
	defer k.notice("Added", key)
	return k.keyring.Add(key)
}
func (k *KeyRing) Remove(key ssh.PublicKey) error {
	keyContents := key.Marshal()
	k.notice("Remove", keyContents)
	defer k.notice("Removed", keyContents)
	return k.keyring.Remove(key)
}
func (k *KeyRing) RemoveAll() error {
	k.notice("RemoveAll", "")
	defer k.notice("RemovedAll", "")
	return k.keyring.RemoveAll()
}
func (k *KeyRing) Lock(passphrase []byte) error {
	k.notice("Lock", "")
	defer k.notice("Locked", "")
	return k.keyring.Lock(passphrase)
}
func (k *KeyRing) Unlock(passphrase []byte) error {
	k.notice("UnLock", "")
	defer k.notice("UnLocked", "")
	return k.keyring.Unlock(passphrase)
}
func (k *KeyRing) Signers() ([]ssh.Signer, error) {
	k.notice("Signers", nil)
	defer k.notice("Signers", nil)
	return k.Signers()
}
func (k *KeyRing) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	//k.notice("SignWithFlags", map[string]interface{}{"publickey": key.Marshal(), "data": data, "flags": flags})
	k.notice("SignWithFlags", key)
	defer k.notice("SignedWithFlags", key)
	return k.keyring.SignWithFlags(key, data, flags)
}
func (k *KeyRing) Extension(extensionType string, contents []byte) ([]byte, error) {
	//k.notice("Extension", map[string]interface{}{"extensionType": extensionType, "contents": contents})
	k.notice("Extension", nil)
	return k.keyring.Extension(extensionType, contents)
}
