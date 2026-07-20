package sshutil

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/masahide/OmniSSHAgent/pkg/sshkey"
	"github.com/masahide/OmniSSHAgent/pkg/store"
)

func TestCheckKeyType(t *testing.T) {
	tests := []struct {
		filePath  string
		pass      string
		fileType  string
		keyType   string
		encrypted bool
		err       string
	}{
		{"test/dsa.ppk", "", "ppk", "ssh-dss", false, "ssh: unsupported DSA key size 2048"},
		{"test/dsa.p.ppk", "", "ppk", "ssh-dss", true, ""},
		{"test/rsa.ppk", "", "ppk", "ssh-rsa", false, ""},
		{"test/rsa.p.ppk", "", "ppk", "ssh-rsa", true, ""},
		{"test/ecdsa_putty.ppk", "", "ppk", "ecdsa-sha2-nistp256", false, ""},
		{"test/ecdsa_putty.p.ppk", "", "ppk", "ecdsa-sha2-nistp256", true, ""},
		{"test/ed_putty.ppk", "", "ppk", "ssh-ed25519", false, ""},
		{"test/ed_putty.p.ppk", "", "ppk", "ssh-ed25519", true, ""},
		{"test/id_rsa", "", "OpenSSH", "ssh-rsa", false, ""},
		{"test/id_rsa.p", "", "OpenSSH", "", true, ""},
		{"test/id_ecdsa", "", "OpenSSH", "ecdsa-sha2-nistp256", false, ""},
		{"test/id_ecdsa.p", "", "OpenSSH", "", true, ""},
		{"test/id_ed25519", "", "OpenSSH", "ssh-ed25519", false, ""},
		{"test/id_ed25519.p", "", "OpenSSH", "", true, ""},

		{"test/id_rsa", "hoge", "OpenSSH", "ssh-rsa", false, "ssh: key is not password protected"},
		{"test/id_rsa.p", "aaa", "OpenSSH", "", true, "x509: decryption password incorrect"},
		{"test/id_rsa.p", "abc", "OpenSSH", "ssh-rsa", true, ""},

		{"test/id_ecdsa", "hoge", "OpenSSH", "ssh-rsa", false, "ssh: key is not password protected"},
		{"test/id_ecdsa.p", "aaa", "OpenSSH", "", true, "x509: decryption password incorrect"},
		{"test/id_ecdsa.p", "abc", "OpenSSH", "ecdsa-sha2-nistp256", true, ""},

		{"test/id_ed25519", "hoge", "OpenSSH", "ssh-rsa", false, "ssh: key is not password protected"},
		{"test/id_ed25519.p", "aaa", "OpenSSH", "", true, "x509: decryption password incorrect"},
		{"test/id_ed25519.p", "abc", "OpenSSH", "ssh-ed25519", true, ""},

		{"test/id_dsa", "", "", "", false, "ssh: unhandled key type"},
		{"test/id_dsa.p", "", "OpenSSH", "", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			pkey, err := CheckKeyType(tt.filePath, tt.pass)
			if err != nil {
				if err.Error() != tt.err {
					t.Errorf("err(%v)!=tt.err(%v)", err, tt.err)
				}
				return

			}
			if len(tt.err) > 0 {
				t.Errorf("err(%v)!=tt.err(%v)", err, tt.err)
				return
			}
			if pkey.FileType != tt.fileType {
				t.Errorf("pkey.Type(%q)!=tt.fileType(%q)", pkey.FileType, tt.fileType)
			}
			if pkey.PublicKey.Type != tt.keyType {
				t.Errorf("pkey.Type(%q)!=tt.fileType(%q)", pkey.PublicKey.Type, tt.keyType)
			}
			if pkey.Encryption != tt.encrypted {
				t.Errorf("pkey.Encryption(%v)!=tt.encrypted(%v)", pkey.Encryption, tt.encrypted)
			}
			//t.Log(pkey.PublicKey.String)
		})
	}
}

func jsonDump(x interface{}) string {
	b, err := json.Marshal(x)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func TestMergeKeyList(t *testing.T) {
	tests := []struct {
		name      string
		k         *KeyRing
		agentKeys []sshkey.PublicKey
		res       []sshkey.PrivateKeyFile
		err       string
	}{
		{
			name: "1",
			k: &KeyRing{
				settings: &store.Settings{
					SaveData: store.SaveData{
						Keys: []sshkey.PrivateKeyFile{
							{PublicKey: sshkey.PublicKey{SHA256: "cc"}},
						},
					},
				},
			},
			agentKeys: []sshkey.PublicKey{
				{SHA256: "aa"},
				{SHA256: "bb"},
			},
			res: []sshkey.PrivateKeyFile{
				{PublicKey: sshkey.PublicKey{SHA256: "aa"}},
				{PublicKey: sshkey.PublicKey{SHA256: "bb"}},
				{PublicKey: sshkey.PublicKey{SHA256: "cc"}},
			},
			err: "",
		},
		{
			name: "2",
			k: &KeyRing{
				settings: &store.Settings{
					SaveData: store.SaveData{
						Keys: []sshkey.PrivateKeyFile{},
					},
				},
			},
			agentKeys: []sshkey.PublicKey{
				{SHA256: "aa"},
				{SHA256: "bb"},
			},
			res: []sshkey.PrivateKeyFile{
				{PublicKey: sshkey.PublicKey{SHA256: "aa"}},
				{PublicKey: sshkey.PublicKey{SHA256: "bb"}},
			},
			err: "",
		},
		{
			name: "3",
			k: &KeyRing{
				settings: &store.Settings{
					SaveData: store.SaveData{
						Keys: []sshkey.PrivateKeyFile{
							{PublicKey: sshkey.PublicKey{SHA256: "cc"}},
							{PublicKey: sshkey.PublicKey{SHA256: "dd"}},
						},
					},
				},
			},
			agentKeys: []sshkey.PublicKey{},
			res: []sshkey.PrivateKeyFile{
				{PublicKey: sshkey.PublicKey{SHA256: "cc"}},
				{PublicKey: sshkey.PublicKey{SHA256: "dd"}},
			},
			err: "",
		},
		{
			name: "4",
			k: &KeyRing{
				settings: &store.Settings{
					SaveData: store.SaveData{
						Keys: []sshkey.PrivateKeyFile{
							{PublicKey: sshkey.PublicKey{SHA256: "cc"}},
							{PublicKey: sshkey.PublicKey{SHA256: "dd"}},
						},
					},
				},
			},
			agentKeys: []sshkey.PublicKey{
				{SHA256: "cc"},
				{SHA256: "dd"},
			},
			res: []sshkey.PrivateKeyFile{
				{PublicKey: sshkey.PublicKey{SHA256: "cc"}},
				{PublicKey: sshkey.PublicKey{SHA256: "dd"}},
			},
			err: "",
		},
		{
			name: "5",
			k: &KeyRing{
				settings: &store.Settings{
					SaveData: store.SaveData{
						Keys: []sshkey.PrivateKeyFile{
							{PublicKey: sshkey.PublicKey{SHA256: "cc"}},
							{PublicKey: sshkey.PublicKey{SHA256: "dd"}},
						},
					},
				},
			},
			agentKeys: []sshkey.PublicKey{
				{SHA256: "dd"},
				{SHA256: "aa"},
			},
			res: []sshkey.PrivateKeyFile{
				{PublicKey: sshkey.PublicKey{SHA256: "dd"}},
				{PublicKey: sshkey.PublicKey{SHA256: "aa"}},
				{PublicKey: sshkey.PublicKey{SHA256: "cc"}},
			},
			err: "",
		},
		{
			name: "6",
			k: &KeyRing{
				settings: &store.Settings{
					SaveData: store.SaveData{
						Keys: []sshkey.PrivateKeyFile{
							{PublicKey: sshkey.PublicKey{SHA256: "aa"}},
							{PublicKey: sshkey.PublicKey{SHA256: "bb"}},
						},
					},
				},
			},
			agentKeys: []sshkey.PublicKey{
				{SHA256: "bb"},
				{SHA256: "aa"},
			},
			res: []sshkey.PrivateKeyFile{
				{PublicKey: sshkey.PublicKey{SHA256: "bb"}},
				{PublicKey: sshkey.PublicKey{SHA256: "aa"}},
			},
			err: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.k.mergeKeyList(tt.agentKeys)
			if err != nil && len(tt.err) > 0 {
				if err.Error() != tt.err {
					t.Errorf("err(%s)!=tt.err(%s)", err, tt.err)
				}
				return
			}
			if diff := cmp.Diff(res, tt.res); diff != "" {
				t.Errorf("mismatch (-res +tt.res):\n%s", diff)
			}
			//t.Log(jsonDump(res))
		})
	}
}
