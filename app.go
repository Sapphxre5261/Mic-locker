package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"mic-locker/internal/core"
	"mic-locker/internal/startup"
)

type SettingsStore interface {
	Save(core.Settings) error
}

type windowAPI struct {
	show func(ctx context.Context)
	hide func(ctx context.Context)
	quit func(ctx context.Context)
}

type App struct {
	lifeMu   sync.Mutex
	ctx      context.Context
	domReady bool

	worker        *core.Worker
	store         SettingsStore
	startupWriter func(core.Settings) error
	win           windowAPI

	warnMu sync.Mutex
	warn   string

	saveMu        sync.Mutex
	tray          *trayMenu
	trayReady     atomic.Bool
	quitting      atomic.Bool
	startupHidden atomic.Bool
}

func NewApp() *App {
	a := &App{}
	a.win = windowAPI{
		show: func(ctx context.Context) {
			wailsRuntime.WindowUnminimise(ctx)
			wailsRuntime.WindowShow(ctx)
		},
		hide: func(ctx context.Context) { wailsRuntime.WindowHide(ctx) },
		quit: func(ctx context.Context) { wailsRuntime.Quit(ctx) },
	}
	a.startupWriter = a.applyStartup
	return a
}

func (a *App) init(store SettingsStore, worker *core.Worker, warn string) {
	a.store = store
	a.worker = worker
	a.setWarn(warn)
}

func (a *App) runtimeContext() context.Context {
	a.lifeMu.Lock()
	defer a.lifeMu.Unlock()
	if !a.domReady {
		return nil
	}
	return a.ctx
}

func (a *App) onStartup(ctx context.Context) {
	a.lifeMu.Lock()
	a.ctx = ctx
	a.lifeMu.Unlock()
	a.worker.Start()
	a.runTray()
}

func (a *App) onDomReady(ctx context.Context) {
	a.lifeMu.Lock()
	a.domReady = true
	a.lifeMu.Unlock()
	a.maybeHideOnStart()
}

func (a *App) maybeHideOnStart() {
	ctx := a.runtimeContext()
	if ctx == nil || !a.trayReady.Load() || a.quitting.Load() {
		return
	}
	s := a.worker.Status().Settings
	if s.StartHidden && s.CloseToTray && a.startupHidden.CompareAndSwap(false, true) {
		a.win.hide(ctx)
	}
}

func (a *App) setWarn(w string) {
	a.warnMu.Lock()
	a.warn = w
	a.warnMu.Unlock()
}

func (a *App) getWarn() string {
	a.warnMu.Lock()
	defer a.warnMu.Unlock()
	return a.warn
}

func (a *App) status() core.Status {
	st := a.worker.Status()
	if w := a.getWarn(); w != "" {
		if st.Error != "" {
			st.Error = w + " • " + st.Error
		} else {
			st.Error = w
		}
	}
	st.Error = localizeText(st.Settings.Locale(), st.Error)
	return st
}

func (a *App) GetStatus() core.Status {
	return a.status()
}

func (a *App) SaveSettings(next core.Settings) (core.Status, error) {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	st, err := a.saveSettingsLocked(next)
	return st, localizeError(st.Settings.Locale(), err)
}

func (a *App) saveSettingsLocked(next core.Settings) (core.Status, error) {
	if next.Language == "" {
		next.Language = a.worker.Status().Settings.Locale()
	}
	if err := next.Validate(); err != nil {
		return a.status(), err
	}
	prev := a.worker.Status().Settings
	if err := a.worker.Apply(next); err != nil {
		return a.status(), err
	}
	if err := a.store.Save(next); err != nil {
		return a.rollback(prev, err, false)
	}
	startupChanged := next.RunOnStartup != prev.RunOnStartup ||
		(next.RunOnStartup && next.StartHidden != prev.StartHidden)
	if startupChanged {
		if err := a.startupWriter(next); err != nil {
			return a.rollback(prev, err, true)
		}
	}
	a.setWarn("")
	a.traySync()
	return a.status(), nil
}

func (a *App) rollback(prev core.Settings, primary error, startupAttempted bool) (core.Status, error) {
	errs := []error{primary}
	if err := a.worker.Apply(prev); err != nil {
		errs = append(errs, err)
	}
	if err := a.store.Save(prev); err != nil {
		errs = append(errs, err)
	}
	if startupAttempted {
		if err := a.startupWriter(prev); err != nil {
			errs = append(errs, err)
		}
	}
	combined := errors.Join(errs...)
	if len(errs) > 1 {
		a.setWarn(combined.Error())
	}
	return a.status(), combined
}

func (a *App) applyStartup(s core.Settings) error {
	k, err := startup.OpenRunKey()
	if err != nil {
		return fmt.Errorf("เปิดรีจิสทรีไม่สำเร็จ: %w", err)
	}
	if c, ok := k.(interface{ Close() }); ok {
		defer c.Close()
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := startup.Apply(k, exe, s.RunOnStartup, s.StartHidden); err != nil {
		return fmt.Errorf("ตั้งค่าเริ่มพร้อมระบบไม่สำเร็จ: %w", err)
	}
	return nil
}

func (a *App) RefreshDevices() (core.Status, error) {
	st, err := func() (core.Status, error) {
		if err := a.worker.Refresh(); err != nil {
			return a.status(), err
		}
		return a.status(), nil
	}()
	return st, localizeError(st.Settings.Locale(), err)
}

func (a *App) HideToTray() error {
	if !a.trayReady.Load() {
		return localizeError(a.status().Settings.Locale(), errors.New("ยังไม่พร้อมใช้งาน System tray"))
	}
	ctx := a.runtimeContext()
	if ctx == nil {
		return localizeError(a.status().Settings.Locale(), errors.New("หน้าต่างยังไม่พร้อม"))
	}
	a.startupHidden.Store(true)
	a.win.hide(ctx)
	return nil
}

func (a *App) ShowWindow() {
	a.startupHidden.Store(true)
	if ctx := a.runtimeContext(); ctx != nil {
		a.win.show(ctx)
	}
}

func (a *App) Quit() {
	a.quitting.Store(true)
	if ctx := a.runtimeContext(); ctx != nil {
		a.win.quit(ctx)
	}
}
