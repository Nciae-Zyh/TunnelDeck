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

func main() {
	if isNativeMessagingLaunch(os.Args) {
		if err := runNativeMessagingHost(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "TunnelDeck native host:", err)
			os.Exit(1)
		}
		return
	}

	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "TunnelDeck",
		Width:     1180,
		Height:    780,
		MinWidth:  940,
		MinHeight: 650,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 7, G: 12, B: 18, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		fmt.Fprintln(os.Stderr, "TunnelDeck:", err)
	}
}
