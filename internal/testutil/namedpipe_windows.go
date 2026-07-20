//go:build windows

package testutil

import (
	"fmt"
	"net"
	"os"

	"github.com/Microsoft/go-winio"
	"golang.org/x/crypto/ssh/agent"
)

type NamedPipeServer struct {
	Path     string
	listener net.Listener
	done     chan struct{}
}

func StartNamedPipeServer(suffix string, backend agent.Agent) (*NamedPipeServer, error) {
	path := fmt.Sprintf(`\\.\pipe\OmniSSHAgent-Test-%d-%s`, os.Getpid(), suffix)
	listener, err := winio.ListenPipe(path, nil)
	if err != nil {
		return nil, err
	}
	server := &NamedPipeServer{Path: path, listener: listener, done: make(chan struct{})}
	go func() {
		defer close(server.done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = agent.ServeAgent(backend, conn)
			}()
		}
	}()
	return server, nil
}

func (s *NamedPipeServer) Close() error {
	err := s.listener.Close()
	<-s.done
	return err
}
