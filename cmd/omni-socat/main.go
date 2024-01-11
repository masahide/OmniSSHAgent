package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kelseyhightower/envconfig"
	"github.com/masahide/OmniSSHAgent/pkg/namedpipe"
	"github.com/masahide/OmniSSHAgent/pkg/npipe2stdin"
	"github.com/masahide/OmniSSHAgent/pkg/store"
	"github.com/masahide/OmniSSHAgent/pkg/store/local"

	"github.com/apenwarr/fixconsole"
)

type specification struct {
	Debug   bool
	LogFile string `default:"omni-socat.log"`
	List    bool   `default:"false"`
}

const (
	appName = "OmniSSHAgent"
)

func proxy(name string, s specification) {
	ctx := context.Background()
	fixconsole.FixConsoleIfNeeded()
	p := &npipe2stdin.Npipe2Stdin{
		Name:  name,
		Debug: s.Debug,
	}
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
	flag.BoolVar(&s.Debug, "debug", s.Debug, "Output debug log")
	flag.BoolVar(&s.List, "l", s.List, "list ssh-agent keys")
	flag.Parse()
	if s.Debug {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		f, err := os.OpenFile(filepath.Join(home, s.LogFile), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			log.Fatal(err)
		}
		log.SetOutput(f)
		defer f.Close()
	}
	if s.List {
		listMode()
		return
	}
	settings := store.NewSettings(getExeName(), local.NewLocalCred(appName))
	if err := settings.Load(); err != nil {
		log.Fatal(err.Error())
	}
	//	fmt.Println(jsonDump(settings))
	if settings.NamedPipeAgent || settings.ProxyModeOfNamedPipe {
		proxy("", s)
		return
	}
	log.Fatal("Failed to connect to OmniSSHAgent. Enable the Named pipe interface for OmniSSHAgent.")
}

func listMode() {
	k := &namedpipe.NamedPipeClient{}
	keys, err := k.List()
	if err != nil {
		fmt.Println(err)
	}
	for i, k := range keys {
		fmt.Printf("#%d: %s\n", i+1, k)
	}
}

func jsonDump(data interface{}) string {
	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(b)
}
