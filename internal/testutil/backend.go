package testutil

import (
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Backend is an in-memory ExtendedAgent suitable for component contract tests.
type Backend struct{ agent.ExtendedAgent }

func NewBackend() *Backend {
	return &Backend{ExtendedAgent: agent.NewKeyring().(agent.ExtendedAgent)}
}

// PublicKey converts a crypto public key for test requests.
func PublicKey(key any) (ssh.PublicKey, error) { return ssh.NewPublicKey(key) }
