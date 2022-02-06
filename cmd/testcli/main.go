package main

import (
	"log"

	"github.com/kelseyhightower/envconfig"
	"github.com/masahide/ssh-agent-win/pkg/pageant"

	"golang.org/x/crypto/ssh/agent"
)

type specification struct {
	Keyfile string
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var s specification
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatal(err)
	}

	p := &pageant.Pageant{
		Agent: agent.NewKeyring(),
	}

	keys, err := p.List()
	if err != nil {
		log.Fatal(err)
	}
	for _, k := range keys {
		log.Printf("key:%s", k.String())
	}
	log.Println("start pageant..")
	p.RunAgent()
}
