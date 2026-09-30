package main

import (
	_ "embed"
	"runtime"
	"sync"

	"github.com/getlantern/systray"
)

//go:embed assets/icon.ico
var trayIcon []byte

type trayMenu struct {
	mu   sync.Mutex
	show *systray.MenuItem
	lock *systray.MenuItem
	quit *systray.MenuItem
	done chan struct{}
}

func (a *App) runTray() {
	a.tray = &trayMenu{done: make(chan struct{})}
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		systray.Run(a.onTrayReady, func() {})
	}()
}

func (a *App) onTrayReady() {
	systray.SetIcon(trayIcon)
	systray.SetTitle("Mic Locker")
	systray.SetTooltip("Mic Locker")
	t := a.tray
	t.mu.Lock()
	labels := trayLabels(a.worker.Status().Settings.Locale())
	t.show = systray.AddMenuItem(labels.Show, labels.ShowTip)
	t.lock = systray.AddMenuItemCheckbox(labels.Lock, labels.LockTip, false)
	systray.AddSeparator()
	t.quit = systray.AddMenuItem(labels.Quit, labels.QuitTip)
	if a.worker.Status().Settings.Locked {
		t.lock.Check()
	}
	showCh := t.show.ClickedCh
	lockCh := t.lock.ClickedCh
	quitCh := t.quit.ClickedCh
	done := t.done
	t.mu.Unlock()

	a.trayReady.Store(true)
	a.maybeHideOnStart()

	go func() {
		for {
			select {
			case <-showCh:
				a.ShowWindow()
			case <-lockCh:
				if _, err := a.trayToggleLock(); err != nil {
					a.setWarn(err.Error())
					a.traySync()
					a.ShowWindow()
				}
			case <-quitCh:
				a.Quit()
			case <-done:
				return
			}
		}
	}()
}

func (a *App) trayToggleLock() (bool, error) {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	next := a.worker.Status().Settings
	next.Locked = !next.Locked
	_, err := a.saveSettingsLocked(next)
	return next.Locked, err
}

func (a *App) traySync() {
	t := a.tray
	if t == nil || !a.trayReady.Load() {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lock == nil || t.show == nil || t.quit == nil {
		return
	}
	settings := a.worker.Status().Settings
	labels := trayLabels(settings.Locale())
	t.show.SetTitle(labels.Show)
	t.show.SetTooltip(labels.ShowTip)
	t.lock.SetTitle(labels.Lock)
	t.lock.SetTooltip(labels.LockTip)
	t.quit.SetTitle(labels.Quit)
	t.quit.SetTooltip(labels.QuitTip)
	if settings.Locked {
		t.lock.Check()
	} else {
		t.lock.Uncheck()
	}
}

func (a *App) stopTray() {
	a.trayReady.Store(false)
	if a.tray != nil {
		close(a.tray.done)
	}
	systray.Quit()
}
