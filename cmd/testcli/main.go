package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/getlantern/systray"
	"github.com/kelseyhightower/envconfig"
	"github.com/masahide/OmniSSHAgent/cmd/testcli/icon"
	"github.com/masahide/OmniSSHAgent/pkg/namedpipe"
	"github.com/masahide/OmniSSHAgent/pkg/npipe2stdin"
	"github.com/masahide/OmniSSHAgent/pkg/pageant"
	"github.com/masahide/OmniSSHAgent/pkg/unix"
	"github.com/masahide/OmniSSHAgent/pkg/wintray"

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

	wintray.RunTray()
	os.Exit(0)

	if proxyFlag {
		proxy(s.PipeName)
		return
	}
	systray.Run(onReady, onExit)

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

func onReady() {
	systray.SetTemplateIcon(icon.Data, icon.Data)
	systray.SetTitle("OmniSSHAgent")
	systray.SetTooltip("ssh-agent")
	mQuitOrig := systray.AddMenuItem("Exit OmniSSHAgent", "Exit the app")
	go func() {
		<-mQuitOrig.ClickedCh
		systray.Quit()
	}()
}

func onExit() {

}
