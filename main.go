package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// appVersion is stamped at build time via
// `-ldflags "-X main.appVersion=<git describe>"` (see Makefile build-desktop).
var appVersion = "dev"

// Main window minimum; also restored after the quick-attach picker shrinks it.
const appMinW, appMinH = 900, 600

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "VibeCodeWay",
		Width:     1280,
		Height:    800,
		MinWidth:  appMinW,
		MinHeight: appMinH,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 18, A: 1},
		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		OnShutdown:       app.shutdown,
		OnBeforeClose:    app.beforeClose,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}
