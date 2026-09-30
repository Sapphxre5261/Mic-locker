package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"mic-locker/internal/audio"
	"mic-locker/internal/core"
	"mic-locker/internal/settings"
	"mic-locker/internal/startup"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	checkAudio := flag.Bool("check-audio", false, "read microphone status and exit")
	_ = flag.Bool("background", false, "start hidden in system tray")
	flag.Parse()

	if *checkAudio {
		os.Exit(runCheckAudio())
	}

	settingsPath, err := settings.DefaultPath()
	if err != nil {
		settingsPath = "settings.json"
	}
	store := settings.New(settingsPath)
	cfg, warn := store.Load()

	if k, err := startup.OpenRunKey(); err == nil {
		if on, err := startup.Enabled(k); err == nil {
			cfg.RunOnStartup = on
		}
		if c, ok := k.(interface{ Close() }); ok {
			c.Close()
		}
	}

	worker := core.NewWorker(audio.New, cfg)

	app := NewApp()
	app.init(store, worker, warn)

	frontend, err := fs.Sub(assets, "frontend")
	if err != nil {
		log.Fatal(err)
	}

	err = wails.Run(&options.App{
		Title:     "Mic Locker",
		Width:     1120,
		Height:    780,
		MinWidth:  850,
		MinHeight: 650,
		AssetServer: &assetserver.Options{
			Assets: frontend,
		},
		BackgroundColour: options.NewRGBA(5, 10, 22, 255),
		OnStartup:        app.onStartup,
		OnDomReady:       app.onDomReady,
		OnBeforeClose: func(ctx context.Context) bool {
			return app.beforeClose()
		},
		OnShutdown: func(ctx context.Context) {
			app.shutdown()
		},
		Bind: []interface{}{app},
		Windows: &windows.Options{
			Theme: windows.Dark,
			CustomTheme: &windows.ThemeSettings{
				DarkModeTitleBar:          0x00160a05,
				DarkModeTitleBarInactive:  0x00160a05,
				DarkModeTitleText:         0x00f7efe8,
				DarkModeTitleTextInactive: 0x00a79680,
				DarkModeBorder:            0x00513221,
				DarkModeBorderInactive:    0x00301c12,
			},
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "mic-locker-6dfaa4f1-5f7b-43c4-86d2-7c6e91122440",
			OnSecondInstanceLaunch: func(data options.SecondInstanceData) {
				app.ShowWindow()
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

func (a *App) beforeClose() bool {
	if a.quitting.Load() {
		return false
	}
	st := a.worker.Status()
	ctx := a.runtimeContext()
	if st.Settings.CloseToTray && a.trayReady.Load() && ctx != nil {
		a.startupHidden.Store(true)
		a.win.hide(ctx)
		return true
	}
	return false
}

func (a *App) shutdown() {
	a.quitting.Store(true)
	a.worker.Stop()
	a.stopTray()
}
