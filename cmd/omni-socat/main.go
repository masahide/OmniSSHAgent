package main

import (
	"context"
	"log"

	"github.com/kelseyhightower/envconfig"
	"github.com/masahide/OmniSSHAgent/pkg/npipe2stdin"
	"github.com/masahide/OmniSSHAgent/pkg/store"
	"github.com/masahide/OmniSSHAgent/pkg/store/local"

	"github.com/apenwarr/fixconsole"
)

type specification struct {
	Debug bool
}

const (
	appName = "OmniSSHAgent"
)

func proxy(name string) {
	ctx := context.Background()
	fixconsole.FixConsoleIfNeeded()
	p := &npipe2stdin.Npipe2Stdin{Name: name}
	if err := p.Proxy(ctx); err != nil {
		log.Fatal(err)
	}
}

func getExeName() string {
	return appName + ".exe"
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var s specification
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatal(err)
	}
	settings := store.NewSettings(getExeName(), local.NewLocalCred(appName))
	if err := settings.Load(); err != nil {
		log.Fatal(err.Error())
	}

	if settings.NamedPipeAgent {
		proxy("")
		return
	}
}
