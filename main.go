package main

import (
	"embed"
	"fmt"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"io"
	"log"
	"os"
	"path/filepath"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if len(os.Args) == 3 && os.Args[1] == "--self-test-embedding" {
		if err := embeddingDiagnostic(os.Args[2]); err != nil {
			os.Exit(1)
		}
		return
	}

	if err := run(); err != nil {
		log.Print(err)
		showStartupError(err.Error())
		os.Exit(1)
	}
}

func run() error {
	app, err := NewApp()
	if err != nil {
		return fmt.Errorf("初始化本地数据失败：\n%w", err)
	}
	defer app.service.Close()
	if file, err := os.OpenFile(filepath.Join(app.dataDir, "startup.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600); err == nil {
		defer file.Close()
		log.SetOutput(io.MultiWriter(os.Stderr, file))
	}
	log.Printf("SmartNote starting; data directory: %s", app.dataDir)
	err = wails.Run(&options.App{Title: "SmartNote · 智能笔记", Width: 1440, Height: 920, MinWidth: 1000, MinHeight: 680, AssetServer: &assetserver.Options{Assets: assets}, BackgroundColour: &options.RGBA{R: 247, G: 248, B: 250, A: 255}, OnStartup: app.startup, OnShutdown: app.shutdown, Bind: []interface{}{app}, Windows: &windows.Options{WebviewUserDataPath: filepath.Join(app.dataDir, "webview")}})
	return err
}
