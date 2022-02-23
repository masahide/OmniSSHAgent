package sshutil

import (
	"testing"
)

func TestCheckKeyType(t *testing.T) {
	tests := []struct {
		filePath  string
		pass      string
		keyType   string
		encrypted bool
		err       string
	}{
		{"test/dsa.ppk", "", "ppk:ssh-dss", false, ""},
		{"test/dsa.p.ppk", "", "ppk:ssh-dss", true, ""},
		{"test/rsa.ppk", "", "ppk:ssh-rsa", false, ""},
		{"test/rsa.p.ppk", "", "ppk:ssh-rsa", true, ""},
		{"test/ecdsa_putty.ppk", "", "ppk:ecdsa-sha2-nistp256", false, ""},
		{"test/ecdsa_putty.p.ppk", "", "ppk:ecdsa-sha2-nistp256", true, ""},
		{"test/ed_putty.ppk", "", "ppk:ssh-ed25519", false, ""},
		{"test/ed_putty.p.ppk", "", "ppk:ssh-ed25519", true, ""},
		{"test/id_rsa", "", "OpenSSH:ssh-rsa", false, ""},
		{"test/id_rsa.p", "", "OpenSSH", true, ""},
		{"test/id_ecdsa", "", "OpenSSH:ecdsa-sha2-nistp256", false, ""},
		{"test/id_ecdsa.p", "", "OpenSSH", true, ""},
		{"test/id_ed25519", "", "OpenSSH:ssh-ed25519", false, ""},
		{"test/id_ed25519.p", "", "OpenSSH", true, ""},

		{"test/id_rsa", "hoge", "OpenSSH:ssh-rsa", false, "ssh: key is not password protected"},
		{"test/id_rsa.p", "aaa", "OpenSSH", true, "x509: decryption password incorrect"},
		{"test/id_rsa.p", "abc", "OpenSSH:ssh-rsa", true, ""},

		{"test/id_ecdsa", "hoge", "OpenSSH:ssh-rsa", false, "ssh: key is not password protected"},
		{"test/id_ecdsa.p", "aaa", "OpenSSH", true, "x509: decryption password incorrect"},
		{"test/id_ecdsa.p", "abc", "OpenSSH:ecdsa-sha2-nistp256", true, ""},

		{"test/id_ed25519", "hoge", "OpenSSH:ssh-rsa", false, "ssh: key is not password protected"},
		{"test/id_ed25519.p", "aaa", "OpenSSH", true, "x509: decryption password incorrect"},
		{"test/id_ed25519.p", "abc", "OpenSSH:ssh-ed25519", true, ""},

		{"test/id_dsa", "", "", false, "ssh: unhandled key type"},
		{"test/id_dsa.p", "", "OpenSSH", true, ""},
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
			if pkey.Type != tt.keyType {
				t.Errorf("pkey.Type(%q)!=tt.keyType(%q)", pkey.Type, tt.keyType)
			}
			if pkey.Encryption != tt.encrypted {
				t.Errorf("pkey.Encryption(%v)!=tt.encrypted(%v)", pkey.Encryption, tt.encrypted)
			}
		})
	}
}
