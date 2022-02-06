package unix

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/crypto/ssh/agent"
)

type DomainSock struct {
	agent.Agent
	Debug bool
	Path  string
}

func (a *DomainSock) RunAgent() error {

	if len(a.Path) == 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("userHomeDir err:%w", err)
		}
		a.Path = filepath.Join(home, "ssh-agent-win.sock")
	}
	_, err := os.Stat(a.Path)
	if err == nil || !os.IsNotExist(err) {
		err = syscall.Unlink(a.Path)
		if err != nil {
			return fmt.Errorf("Failed remove socket %s, err:%w", a.Path, err)
		}
	}
	sock, err := net.Listen("unix", a.Path)
	if err != nil {
		return fmt.Errorf("Failed open named unix domain socket: %s, err: %w", a.Path, err)
	}
	defer sock.Close()
	if a.Debug {
		log.Printf("Open unix domain socket: %s", a.Path)
	}
	for {
		conn, err := sock.Accept()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("pipe.Accept err: %w", err)
		}
		if a.Debug {
			log.Println("start DomainSock handler")
		}
		go a.handle(conn)
	}
}

func (a *DomainSock) handle(conn net.Conn) {
	defer conn.Close()
	err := agent.ServeAgent(a, conn)
	if err != nil {
		if err == io.EOF {
			return
		}
		log.Printf("DomainSock agent.ServeAgent err:%s", err)
	}
}
