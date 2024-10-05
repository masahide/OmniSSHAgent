//go:build unix

package main

import (
	"log"
	"net"
	"os"

	"golang.org/x/crypto/ssh/agent"
)

func listKeys() {
	socketPath := os.Getenv("SSH_AUTH_SOCK")
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Fatal(err)
	}
	agentClient := agent.NewClient(conn)
	list, err := agentClient.List()
	if err != nil {
		log.Fatal(err)
	}

	for _, key := range list {
		log.Println(key.String())
	}
}

func NewUnixDomain() (sshAgent, error) {
	socketPath := os.Getenv("SSH_AUTH_SOCK")
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Fatal(err)
	}
	a := agent.NewClient(conn)
	return &Agent{ExtendedAgent: &exAgent{a}, Conn: conn}, nil
}

func newAgent() (sshAgent, error) {
	return NewUnixDomain()
}
