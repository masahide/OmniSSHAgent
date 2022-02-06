package main

import (
	"log"

	"github.com/kelseyhightower/envconfig"
	"github.com/masahide/ssh-agent-win/pkg/namedpipe"
	"github.com/masahide/ssh-agent-win/pkg/pageant"

	"golang.org/x/crypto/ssh/agent"
)

type specification struct {
	Debug bool
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var s specification
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatal(err)
	}
	keys := agent.NewKeyring()
	pa := &pageant.Pageant{Agent: keys, Debug: s.Debug}
	na := &namedpipe.NamedPipe{Agent: keys, Debug: s.Debug}

	log.Println("start agents..")
	go pa.RunAgent()
	err = na.RunAgent()
	log.Println(err)
}
