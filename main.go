package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/masahide/OmniSSHAgent/pkg/pageant"
	"github.com/masahide/OmniSSHAgent/pkg/store"
	"github.com/masahide/OmniSSHAgent/pkg/store/local"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

const (
	AppName = "OmniSSHAgent"
)

//go:embed frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

func getExeName() string {
	return filepath.Base(os.Args[0])
}

func checkAlreadyRunning() {
	b, err := pageant.AlreadyRunning()
	if err != nil {
		return
	}
	//respLen := binary.BigEndian.Uint32(b[:4])
	if string(b[4:]) == AppName {
		os.Exit(0)
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// Create an instance of the app structure

	checkAlreadyRunning()

	app := NewApp()
	app.settings = store.NewSettings(getExeName(), local.NewLocalCred(AppName))
	if err := app.settings.Load(); err != nil {
		log.Fatal(err.Error())
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:             AppName,
		Width:             900,
		Height:            900,
		MinWidth:          720,
		MinHeight:         570,
		MaxWidth:          1280,
		MaxHeight:         900,
		DisableResize:     false,
		Fullscreen:        false,
		Frameless:         false,
		StartHidden:       app.settings.StartHidden,
		HideWindowOnClose: true,
		RGBA:              &options.RGBA{R: 33, G: 37, B: 43, A: 255},
		Assets:            assets,
		LogLevel:          logger.DEBUG,
		OnStartup:         app.startup,
		OnDomReady:        app.domReady,
		OnShutdown:        app.shutdown,
		Bind: []interface{}{
			app,
		},
		// Windows platform specific options
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "My Application",
				Message: "",
				Icon:    icon,
			},
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}
