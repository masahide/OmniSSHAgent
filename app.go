package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/masahide/ssh-agent-win/pkg/namedpipe"
	"github.com/masahide/ssh-agent-win/pkg/pageant"
	"github.com/masahide/ssh-agent-win/pkg/sshutil"
	"github.com/masahide/ssh-agent-win/pkg/store"
	"github.com/masahide/ssh-agent-win/pkg/store/local"
	"github.com/masahide/ssh-agent-win/pkg/unix"
	"github.com/masahide/ssh-agent-win/pkg/wintray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App application struct
type App struct {
	ctx      context.Context
	ti       *wintray.TrayIcon
	keyRing  *sshutil.KeyRing
	settings *store.Settings
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

func getExeName() string {
	return filepath.Base(os.Args[0])
}

// startup is called at application startup
func (a *App) startup(ctx context.Context) {
	// Perform your setup here
	a.ctx = ctx
	a.ti = wintray.NewTrayIcon()
	a.ti.BalloonClickFunc = a.showWindow
	a.ti.TrayClickFunc = a.showWindow
	go a.ti.RunTray()
	a.settings = store.NewSettings(getExeName(), local.NewLocalCred(AppName))
	err := a.settings.Load()
	if err != nil {
		runtime.LogFatal(ctx, err.Error())
	}
	debug := false
	a.keyRing = sshutil.NewKeyRing(a.settings)
	pa := &pageant.Pageant{Agent: a.keyRing, Debug: debug}
	go pa.RunAgent()
	runtime.LogInfo(ctx, "Start pageant...")
	if a.settings.NamedPipeAgent {
		pipeName := ""
		na := &namedpipe.NamedPipe{Agent: a.keyRing, Debug: debug, Name: pipeName}
		runtime.LogInfo(ctx, "Start NamedPipe agent..")
		go na.RunAgent()
	}
	if a.settings.UnixSocketAgent {
		ua := &unix.DomainSock{Agent: a.keyRing, Debug: debug, Path: a.settings.UnixSocketPath}
		go ua.RunAgent()
		runtime.LogInfo(ctx, "Start Unix domain socket agent..")
	}

}

func (a *App) OpenFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select private key file",
	})
}

// domReady is called after the front-end dom has been loaded
func (a *App) domReady(ctx context.Context) {
	// Add your action here
}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	a.ti.Dispose()
	// Perform your teardown here
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s!", name)
}

func (a *App) showWindow() {
	runtime.WindowShow(a.ctx)
}

func (a *App) Quit() {
	a.ti.Dispose()
	runtime.Quit(a.ctx)
}

func (a *App) AddKey(pk sshutil.PrivateKeyFile) error {
	return nil
}

func (a *App) KeyList() ([]sshutil.Key, error) {
	return a.keyRing.KeyList()
}

func (a *App) CheckKeyType(filePath, passphrase string) (*sshutil.PrivateKeyFile, error) {
	return sshutil.CheckKeyType(filePath, passphrase)
}
