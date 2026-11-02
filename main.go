//go:build windows

package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// main opens the WebView window and binds the cleaner API.
func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:            "WinCleaner",
		Width:            1120,
		Height:           760,
		MinWidth:         900,
		MinHeight:        620,
		Frameless:        true,
		BackgroundColour: &options.RGBA{R: 244, G: 246, B: 248, A: 255},
		Windows: &windows.Options{
			Theme: windows.Dark,
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
