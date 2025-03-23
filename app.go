package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/masahide/OmniSSHAgent/pkg/cygwinsocket"
	"github.com/masahide/OmniSSHAgent/pkg/namedpipe"
	"github.com/masahide/OmniSSHAgent/pkg/pageant"
	"github.com/masahide/OmniSSHAgent/pkg/sshkey"
	"github.com/masahide/OmniSSHAgent/pkg/sshutil"
	"github.com/masahide/OmniSSHAgent/pkg/store"
	"github.com/masahide/OmniSSHAgent/pkg/unix"
	"github.com/masahide/OmniSSHAgent/pkg/winopen"
	"github.com/masahide/OmniSSHAgent/pkg/wintray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App application struct
type App struct {
	ctx      context.Context
	ti       *wintray.TrayIcon
	keyRing  *sshutil.KeyRing
	settings *store.Settings
	wg       sync.WaitGroup
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

func (a *App) setTrayTooltip() {
	tooltip := AppName
	if keys, err := a.keyRing.KeyList(); err == nil {
		tooltip = fmt.Sprintf("%s - %d keys loaded", tooltip, len(keys))
	}
	a.ti.SetTooltip(tooltip)
}

func (a *App) systrayOnReady() {
	a.ti.SetTitle(AppName)
	a.setTrayTooltip()
	mShowWindow := a.ti.AddMenuItem("ShowWindow", "Show main window")
	mQuit := a.ti.AddMenuItem("Quit", "Quit the whole app")
	mLogCheckBox := a.ti.AddMenuItemCheckbox("Debug log", "Enable debug log file output", false)
	mLogDirOpen := a.ti.AddMenuItem("Open log directory", "open log directory")
	mLogDirOpen.Disable()
	go func() {
		for {
			select {
			case <-mShowWindow.ClickedCh:
				a.showWindow()
			case <-mLogCheckBox.ClickedCh:
				if mLogCheckBox.Checked() {
					mLogCheckBox.Uncheck()
					log.Print("Disable debug log")
					Logger.SetEnable(false)
					mLogDirOpen.Disable()
				} else {
					mLogCheckBox.Check()
					Logger.SetEnable(true)
					log.Print("Enable debug log")
					mLogDirOpen.Enable()
				}
			case <-mQuit.ClickedCh:
				log.Print("Quit was clicked on the menu")
				a.Quit()
				return
			case <-mLogDirOpen.ClickedCh:
				dir := filepath.Dir(Logger.FilePath)
				winopen.Open(dir)
			}
		}
	}()
}
func (a *App) systrayOnExit() {
	//log.Print("systrayOnExit")
	Logger.Close()
	a.wg.Done()
}

// startup is called at application startup
func (a *App) startup(ctx context.Context) {
	// Perform your setup here
	a.ctx = ctx
	a.ti = wintray.NewTrayIcon()
	a.ti.BalloonClickFunc = a.showWindow
	a.ti.TrayClickFunc = a.showWindow

	a.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("recover:%v", r)
			}
			a.Quit()
		}()
		a.ti.Run(a.systrayOnReady, a.systrayOnExit)
	}()

	debug := false
	a.keyRing = sshutil.NewKeyRing(a.settings)
	if err := a.keyRing.AddKeys(); err != nil {
		log.Printf("KeyRing.AddKeys err: %s", err)
	}
	a.keyRing.NotifyCallback = a.notice
	pa := &pageant.Pageant{
		ExtendedAgent: a.keyRing,
		AppName:       AppName,
		Debug:         debug,
		CheckFunc:     a.showWindow,
	}
	if a.settings.PageantAgent {
		go pa.RunAgent()
	}
	log.Println("Starting pageant...")
	if a.settings.NamedPipeAgent {
		pipeName := ""
		na := &namedpipe.NamedPipe{ExtendedAgent: a.keyRing, Debug: debug, Name: pipeName}
		log.Println("Starting NamedPipe agent..")
		go na.RunAgent()
	}
	if a.settings.UnixSocketAgent {
		ua := &unix.DomainSock{ExtendedAgent: a.keyRing, Debug: debug, Path: a.settings.UnixSocketPath}
		go ua.RunAgent()
		log.Println("Start Unix domain socket agent..")
	}
	if a.settings.CygWinAgent {
		ca := &cygwinsocket.CygwinSock{ExtendedAgent: a.keyRing, Debug: debug, Path: a.settings.CygWinSocketPath}
		go ca.RunAgent()
		log.Println("Starting Cygwin unix domain socket agent..")
	}
}

func (a *App) notice(action string, data interface{}) {
	switch action {
	case "Add", "Remove", "RemoveAll":
		//a.ti.ShowBalloonNotification(action, sshutil.JSONDump(data))
		runtime.EventsEmit(a.ctx, "LoadKeysEvent")

	case "Added", "Removed", "RemovedAll":
		a.setTrayTooltip()

	case "Sign", "SignWithFlags":
		switch t := data.(type) {
		case *agent.Key:
			if err := a.onSign(t); err != nil {
				log.Printf("cannot find key to print: %v\n", err)
			}
		case ssh.PublicKey:
			log.Printf("unexpected ssh.PublicKey\n")
		}
	}
}

func (a *App) onSign(pubkey *agent.Key) error {
	privkey := a.keyRing.FindPrivKey(pubkey)
	if privkey == nil {
		return errors.New("private key not found")
	}

	a.ti.ShowBalloonNotification(wintray.ID,
		fmt.Sprintf("SSH Key '%s' was used", privkey.PublicKey.Comment))
	return nil
}

func (a *App) OpenFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select a private key file",
	})
}

// domReady is called after the front-end dom has been loaded
func (a *App) domReady(ctx context.Context) {
	// Add your action here
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s!", name)
}

func (a *App) showWindow() {
	//runtime.LogDebug(a.ctx, "showWindow")
	runtime.WindowShow(a.ctx)
}

func (a *App) Quit() {
	log.Print("call a.Quit")
	runtime.Quit(a.ctx)
}

// shutdown はruntime.Quitから呼ばれる
func (a *App) shutdown(ctx context.Context) {
	log.Print("shutdown")
	a.ti.Quit()
	a.wg.Wait()
}

func (a *App) AddLocalFile(pk sshkey.PrivateKeyFile) error {
	pk.Name = filepath.Base(pk.FilePath)
	pk.StoreType = sshutil.LocalStore
	//log.Printf("AddLocalFile:%s", sshutil.JSONDump(pk))
	id, err := a.keyRing.AddKeySettings(pk)
	if err != nil {
		return err
	}
	if err := a.keyRing.AddKey(id); err != nil {
		return err
	}
	return nil
}

func (a *App) DeleteKey(sha256 string) error {
	c, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:    runtime.QuestionDialog,
		Title:   "Delete?",
		Message: "Do you really want to delete this key?",
	})
	if err != nil {
		return err
	}
	//runtime.LogDebug(a.ctx, c)
	if c != "Yes" {
		return errors.New("cancel")
	}
	if err := a.keyRing.RemoveKey(sha256); err != nil {
		return err
	}
	return a.keyRing.DeleteKeySettings(sha256)
}

func (a *App) KeyList() ([]sshkey.PrivateKeyFile, error) {
	return a.keyRing.KeyList()
}

func (a *App) CheckKeyType(filePath, passphrase string) (*sshkey.PrivateKeyFile, error) {
	return sshutil.CheckKeyType(filePath, passphrase)
}

func (a *App) GetSettings() store.SaveData {
	return a.settings.SaveData
}
func (a *App) Save(s store.SaveData) error {
	a.settings.SaveData.StartHidden = s.StartHidden
	a.settings.SaveData.PageantAgent = s.PageantAgent
	a.settings.SaveData.NamedPipeAgent = s.NamedPipeAgent
	a.settings.SaveData.UnixSocketAgent = s.UnixSocketAgent
	a.settings.SaveData.UnixSocketPath = s.UnixSocketPath
	a.settings.SaveData.CygWinAgent = s.CygWinAgent
	a.settings.SaveData.CygWinSocketPath = s.CygWinSocketPath
	a.settings.SaveData.ProxyModeOfNamedPipe = s.ProxyModeOfNamedPipe
	return a.settings.Save()
}
