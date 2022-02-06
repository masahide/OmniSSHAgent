package main

import (
	"context"
	"flag"
	"log"

	"github.com/kelseyhightower/envconfig"
	"github.com/masahide/ssh-agent-win/pkg/namedpipe"
	"github.com/masahide/ssh-agent-win/pkg/npipe2stdin"
	"github.com/masahide/ssh-agent-win/pkg/pageant"
	"github.com/masahide/ssh-agent-win/pkg/unix"

	"github.com/apenwarr/fixconsole"
	"golang.org/x/crypto/ssh/agent"
)

type specification struct {
	PipeName   string
	SocketPath string
	Debug      bool
}

var (
	proxyFlag bool
)

func init() {
	flag.BoolVar(&proxyFlag, "p", false, "")
	flag.Parse()
}

func proxy(name string) {
	ctx := context.Background()
	fixconsole.FixConsoleIfNeeded()
	p := &npipe2stdin.Npipe2Stdin{Name: name}
	if err := p.Proxy(ctx); err != nil {
		log.Fatal(err)
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var s specification
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatal(err)
	}
	if proxyFlag {
		proxy(s.PipeName)
		return
	}

	keys := agent.NewKeyring()
	pa := &pageant.Pageant{Agent: keys, Debug: s.Debug}
	na := &namedpipe.NamedPipe{Agent: keys, Debug: s.Debug, Name: s.PipeName}
	ua := &unix.DomainSock{Agent: keys, Debug: s.Debug, Path: s.SocketPath}

	log.Println("start agents..")
	go pa.RunAgent()
	go ua.RunAgent()
	err = na.RunAgent()
	log.Println(err)
}
