package namedpipe

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/masahide/OmniSSHAgent/pkg/agentlistener"
	"golang.org/x/crypto/ssh/agent"
)

type NamedPipe struct {
	agent.ExtendedAgent
	Debug bool
	Name  string
}

func (a *NamedPipe) RunAgent(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	pipePath := map[bool]string{
		true:  `\\.\pipe\openssh-ssh-agent`,
		false: `\\.\pipe\` + a.Name,
	}[len(a.Name) == 0]

	pipe, err := winio.ListenPipe(pipePath, &winio.PipeConfig{})
	if err != nil {
		return fmt.Errorf("Failed open named-pipe %s, err:%w", pipePath, err)
	}
	defer pipe.Close()
	if a.Debug {
		log.Printf("Open named-pipe: %s", pipePath)
	}
	if err := agentlistener.Serve(ctx, pipe, func(ctx context.Context, conn net.Conn) {
		if a.Debug {
			log.Println("start namedPipe handler")
		}
		a.handle(ctx, conn)
	}); err != nil {
		return fmt.Errorf("pipe.Accept err: %w", err)
	}
	return nil
}

func (a *NamedPipe) handle(ctx context.Context, conn net.Conn) {
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
		log.Printf("NamedPipe agent.ServeAgent err:%s", err)
	}
}
