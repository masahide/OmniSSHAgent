package embedded

import (
	"errors"
	"fmt"

	"github.com/masahide/OmniSSHAgent/internal/backend"
	"golang.org/x/crypto/ssh/agent"
)

var ErrConfirmBeforeUseUnsupported = errors.New("embedded backend does not support confirm-before-use")

// Backend is an ephemeral, in-process SSH agent backend. Keys are retained
// only by this instance and are never persisted by this package.
type Backend struct {
	agent.ExtendedAgent
}

var _ backend.Backend = (*Backend)(nil)

func New() (*Backend, error) {
	keyring := agent.NewKeyring()
	extended, ok := keyring.(agent.ExtendedAgent)
	if !ok {
		return nil, fmt.Errorf("embedded keyring does not implement agent.ExtendedAgent")
	}
	return &Backend{ExtendedAgent: extended}, nil
}

// Add rejects confirmation constraints instead of silently weakening their
// security semantics. The underlying keyring handles lifetime constraints.
func (b *Backend) Add(key agent.AddedKey) error {
	if key.ConfirmBeforeUse {
		return ErrConfirmBeforeUseUnsupported
	}
	return b.ExtendedAgent.Add(key)
}
