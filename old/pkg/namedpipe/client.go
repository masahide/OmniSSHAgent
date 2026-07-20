package namedpipe

import (
	"fmt"
	"io"
	"sync"

	"github.com/Microsoft/go-winio"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	pipePath = `\\.\pipe\openssh-ssh-agent`
)

type NamedPipeClient struct {
	Debug bool
	conn  io.ReadWriteCloser
	mu    sync.Mutex
}

func (a *NamedPipeClient) dial() (agent.ExtendedAgent, error) {
	if a.conn == nil {
		var err error
		a.conn, err = winio.DialPipe(pipePath, nil)
		if err != nil {
			a.conn = nil
			return nil, fmt.Errorf("Failed open named-pipe %s, err:%w", pipePath, err)
		}
	}
	return agent.NewClient(a.conn), nil
}

func (a *NamedPipeClient) Close() error {
	if a.conn != nil {
		err := a.conn.Close()
		a.conn = nil
		return err
	}
	return nil
}

func (a *NamedPipeClient) List() ([]*agent.Key, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ea, err := a.dial()
	if err != nil {
		return nil, err
	}
	defer a.Close()
	return ea.List()
}

// Sign has the agent sign the data using a protocol 2 key as defined
// in [PROTOCOL.agent] section 2.6.2.
func (a *NamedPipeClient) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ea, err := a.dial()
	if err != nil {
		return nil, err
	}
	defer a.Close()
	return ea.Sign(key, data)
}

// Add adds a private key to the agent.
func (a *NamedPipeClient) Add(key agent.AddedKey) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	ea, err := a.dial()
	if err != nil {
		return err
	}
	defer a.Close()
	return ea.Add(key)
}

// Remove removes all identities with the given public key.
func (a *NamedPipeClient) Remove(key ssh.PublicKey) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	ea, err := a.dial()
	if err != nil {
		return err
	}
	defer a.Close()
	return ea.Remove(key)
}

// RemoveAll removes all identities.
func (a *NamedPipeClient) RemoveAll() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	ea, err := a.dial()
	if err != nil {
		return err
	}
	defer a.Close()
	return ea.RemoveAll()
}

// Lock locks the agent. Sign and Remove will fail, and List will empty an empty list.
func (a *NamedPipeClient) Lock(passphrase []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	ea, err := a.dial()
	if err != nil {
		return err
	}
	defer a.Close()
	return ea.Lock(passphrase)
}

// Unlock undoes the effect of Lock
func (a *NamedPipeClient) Unlock(passphrase []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	ea, err := a.dial()
	if err != nil {
		return err
	}
	defer a.Close()
	return ea.Unlock(passphrase)
}

// Signers returns signers for all the known keys.
func (a *NamedPipeClient) Signers() ([]ssh.Signer, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ea, err := a.dial()
	if err != nil {
		return nil, err
	}
	defer a.Close()
	return ea.Signers()
}
func (a *NamedPipeClient) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ea, err := a.dial()
	if err != nil {
		return nil, err
	}
	defer a.Close()
	return ea.SignWithFlags(key, data, flags)
}
func (a *NamedPipeClient) Extension(extensionType string, contents []byte) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ea, err := a.dial()
	if err != nil {
		return nil, err
	}
	defer a.Close()
	return ea.Extension(extensionType, contents)
}
