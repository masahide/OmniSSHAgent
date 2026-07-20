//go:build windows

package openssh

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/masahide/OmniSSHAgent/internal/backend"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type Dialer func(context.Context, string) (io.ReadWriteCloser, error)

type Client struct {
	pipePath string
	timeout  time.Duration
	dial     Dialer
}

func New(pipePath string, timeout time.Duration) *Client {
	return NewWithDialer(pipePath, timeout, func(ctx context.Context, path string) (io.ReadWriteCloser, error) {
		return winio.DialPipeContext(ctx, path)
	})
}

func NewWithDialer(pipePath string, timeout time.Duration, dial Dialer) *Client {
	return &Client{pipePath: NormalizePipePath(pipePath), timeout: timeout, dial: dial}
}

func NormalizePipePath(path string) string {
	if len(path) >= len(`\\.\pipe\`) && path[:len(`\\.\pipe\`)] == `\\.\pipe\` {
		return path
	}
	return `\\.\pipe\` + path
}

func (c *Client) connect() (agent.ExtendedAgent, io.Closer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	conn, err := c.dial(ctx, c.pipePath)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, nil, &backend.Error{Kind: backend.ErrorTimeout, Operation: "connect", Err: err}
		}
		return nil, nil, &backend.Error{Kind: backend.ErrorUnavailable, Operation: "connect", Err: err}
	}
	return agent.NewClient(conn), conn, nil
}

func (c *Client) List() ([]*agent.Key, error) {
	a, conn, err := c.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return a.List()
}
func (c *Client) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	a, conn, err := c.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return a.Sign(key, data)
}
func (c *Client) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	a, conn, err := c.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return a.SignWithFlags(key, data, flags)
}
func (c *Client) Add(key agent.AddedKey) error {
	a, conn, err := c.connect()
	if err != nil {
		return err
	}
	defer conn.Close()
	return a.Add(key)
}
func (c *Client) Remove(key ssh.PublicKey) error {
	a, conn, err := c.connect()
	if err != nil {
		return err
	}
	defer conn.Close()
	return a.Remove(key)
}
func (c *Client) RemoveAll() error {
	a, conn, err := c.connect()
	if err != nil {
		return err
	}
	defer conn.Close()
	return a.RemoveAll()
}
func (c *Client) Lock(passphrase []byte) error {
	a, conn, err := c.connect()
	if err != nil {
		return err
	}
	defer conn.Close()
	return a.Lock(passphrase)
}
func (c *Client) Unlock(passphrase []byte) error {
	a, conn, err := c.connect()
	if err != nil {
		return err
	}
	defer conn.Close()
	return a.Unlock(passphrase)
}
func (c *Client) Signers() ([]ssh.Signer, error) {
	a, conn, err := c.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return a.Signers()
}
func (c *Client) Extension(extensionType string, contents []byte) ([]byte, error) {
	a, conn, err := c.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return a.Extension(extensionType, contents)
}
