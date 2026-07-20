//go:build windows

package main

import (
	"errors"

	"github.com/Microsoft/go-winio"
	"github.com/davidmz/go-pageant"
	"golang.org/x/crypto/ssh/agent"
)

const (
	sshAgentPipe = `\\.\pipe\openssh-ssh-agent`
)

func NewPageant() (sshAgent, error) {
	ok := pageant.Available()
	if !ok {
		return nil, errors.New("pageant is not available")
	}
	p := pageant.New()
	return &Agent{ExtendedAgent: &exAgent{p}}, nil
}

func (a *Agent) Close() error {
	if a.Conn != nil {
		a.Conn.Close()
	}
	return nil
}

func NewNamedPipe() (sshAgent, error) {
	conn, err := winio.DialPipe(sshAgentPipe, nil)
	if err != nil {
		return nil, err
	}
	a := agent.NewClient(conn)
	return &Agent{ExtendedAgent: &exAgent{a}, Conn: conn}, nil
}

func newAgent() (sshAgent, error) {
	return NewNamedPipe()
}
