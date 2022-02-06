package namedpipe

import (
	"fmt"
	"io"
	"log"
	"net"

	"github.com/Microsoft/go-winio"
	"golang.org/x/crypto/ssh/agent"
)

type NamedPipe struct {
	agent.Agent
	Debug bool
}

func (a *NamedPipe) RunAgent() error {
	namedPipeName := "openssh-ssh-agent"
	pipePath := `\\.\pipe\` + namedPipeName

	cfg := &winio.PipeConfig{}
	pipe, err := winio.ListenPipe(pipePath, cfg)
	if err != nil {
		return fmt.Errorf("Failed open named pipe %s, err: '%w'\n", pipePath, err)
	}
	defer pipe.Close()
	for {
		conn, err := pipe.Accept()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("pipe.Accept err: %w", err)
		}
		if a.Debug {
			log.Println("start namedPipe handler")
		}
		go a.handle(conn)
	}
}

func (a *NamedPipe) handle(conn net.Conn) {
	defer conn.Close()
	err := agent.ServeAgent(a, conn)
	if err != nil {
		if err == io.EOF {
			return
		}
		log.Printf("NamedPipe agent.ServeAgent err:%s", err)
	}
}
