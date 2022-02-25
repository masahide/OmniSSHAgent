package sshkey

/*
type KeyInfo struct {
	ID         string `json:"id"`
	FileType   string `json:"fileType"`
	Encryption bool   `json:"encryption"`
	FilePath   string `json:"filePath"`
}
*/

// PrivateKeyFile Private key file information interface with wails.
type PrivateKeyFile struct {
	ID         string    `json:"id"`
	FilePath   string    `json:"filePath"`
	FileType   string    `json:"fileType"`
	Encryption bool      `json:"encryption"`
	Passphrase string    `json:"passphrase"`
	PublicKey  PublicKey `json:"publickey"`
}

// PublicKey Publick key file information interface with wails.
type PublicKey struct {
	MD5     string `json:"md5"`
	SHA256  string `json:"sha256"`
	Type    string `json:"type"`
	Comment string `json:"comment"`
	String  string `json:"string"`
}
