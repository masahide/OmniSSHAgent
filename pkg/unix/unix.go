package unix

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/masahide/OmniSSHAgent/pkg/agentlistener"
	"golang.org/x/crypto/ssh/agent"
)

type DomainSock struct {
	agent.ExtendedAgent
	Debug bool
	Path  string
}

func (a *DomainSock) RunAgent(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if len(a.Path) == 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("userHomeDir err:%w", err)
		}
		a.Path = filepath.Join(home, "OmniSSHAgent.sock")
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
	if err := agentlistener.Serve(ctx, sock, func(ctx context.Context, conn net.Conn) {
		if a.Debug {
			log.Println("start DomainSock handler")
		}
		a.handle(ctx, conn)
	}); err != nil {
		return fmt.Errorf("pipe.Accept err: %w", err)
	}
	return nil
}

func (a *DomainSock) handle(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	if ctx == nil {
		ctx = context.Background()
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
		}
	}()
	defer close(done)
	// Set a read deadline to prevent indefinite blocking on stale connections
	conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	err := agent.ServeAgent(a, conn)
	if err != nil {
		if err == io.EOF {
			return
		}
		log.Printf("DomainSock agent.ServeAgent err:%s", err)
	}
}
