package backend

import "golang.org/x/crypto/ssh/agent"

// Backend is the protocol-level contract used by compatibility interfaces.
type Backend interface {
	agent.ExtendedAgent
}
