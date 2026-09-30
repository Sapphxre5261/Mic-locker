package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"mic-locker/internal/core"
)

type fakeEndpoint struct {
	level  float32
	muted  bool
	setErr error
	sets   int
}

func (e *fakeEndpoint) Read() (float32, bool, float32, error) {
	return e.level, e.muted, 0, nil
}
func (e *fakeEndpoint) SetLevel(v float32) error {
	if e.setErr != nil {
		return e.setErr
	}
	e.sets++
	e.level = v
	return nil
}
func (e *fakeEndpoint) Close() {}

type fakeAudio struct {
	devices   []core.Device
	endpoints map[string]*fakeEndpoint
}

func (a *fakeAudio) Devices() ([]core.Device, error) { return a.devices, nil }
func (a *fakeAudio) Open(id string) (core.Endpoint, string, string, error) {
	if id == "" {
		for _, d := range a.devices {
			if d.Default {
				id = d.ID
			}
		}
	}
	ep, ok := a.endpoints[id]
	if !ok {
		return nil, "", "", errors.New("ไม่พบไมโครโฟน")
	}
	return ep, id, "fake " + id, nil
}
func (a *fakeAudio) Close() {}

type fakeStore struct {
	mu      sync.Mutex
	saved   []core.Settings
	err     error
	enter   chan struct{}
	release chan struct{}
}

func (s *fakeStore) Save(c core.Settings) error {
	if s.enter != nil {
		s.enter <- struct{}{}
		<-s.release
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.saved = append(s.saved, c)
	return nil
}

func (s *fakeStore) last() (core.Settings, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.saved) == 0 {
		return core.Settings{}, false
	}
	return s.saved[len(s.saved)-1], true
}

func newTestApp(t *testing.T, ep *fakeEndpoint, cfg core.Settings) (*App, *fakeStore, *[]core.Settings) {
	t.Helper()
	fa := &fakeAudio{
		devices:   []core.Device{{ID: "mic1", Name: "Mic 1", Default: true}},
		endpoints: map[string]*fakeEndpoint{"mic1": ep},
	}
	w := core.NewWorker(func() (core.Audio, error) { return fa, nil }, cfg)
	w.Start()
	store := &fakeStore{}
	startupCalls := &[]core.Settings{}
	var mu sync.Mutex
	app := NewApp()
	app.startupWriter = func(s core.Settings) error {
		mu.Lock()
		*startupCalls = append(*startupCalls, s)
		mu.Unlock()
		return nil
	}
	app.init(store, w, "")
	app.win = windowAPI{
		show: func(context.Context) {},
		hide: func(context.Context) {},
		quit: func(context.Context) {},
	}
	t.Cleanup(w.Stop)
	return app, store, startupCalls
}

func TestSaveSerializedConcurrent(t *testing.T) {
	app, store, _ := newTestApp(t, &fakeEndpoint{level: 0.5}, core.DefaultSettings())
	store.enter = make(chan struct{}, 2)
	store.release = make(chan struct{}, 2)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		n := core.DefaultSettings()
		n.Target = 60
		_, errs[0] = app.SaveSettings(n)
	}()
	<-store.enter
	secondDone := make(chan struct{})
	go func() {
		defer wg.Done()
		n := core.DefaultSettings()
		n.Target = 40
		_, errs[1] = app.SaveSettings(n)
		close(secondDone)
	}()
	select {
	case <-store.enter:
		t.Fatal("second save entered store while first in flight")
	case <-time.After(100 * time.Millisecond):
	}
	store.release <- struct{}{}
	<-store.enter
	store.release <- struct{}{}
	select {
	case <-secondDone:
	case <-time.After(3 * time.Second):
		t.Fatal("second save did not complete")
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Fatalf("save %d failed: %v", i, e)
		}
	}
	last, _ := store.last()
	if last.Target != 40 || app.worker.Status().Settings.Target != 40 {
		t.Fatal("saves must be serialized in order")
	}
}

func TestSaveDiskFailureRestoresWorker(t *testing.T) {
	app, store, _ := newTestApp(t, &fakeEndpoint{level: 0.5}, core.DefaultSettings())
	store.err = errors.New("disk full")
	n := core.DefaultSettings()
	n.Target = 60
	if _, err := app.SaveSettings(n); err == nil {
		t.Fatal("expected save error")
	}
	if app.worker.Status().Settings.Target != 75 {
		t.Fatal("worker settings must be rolled back")
	}
	if _, ok := store.last(); ok {
		t.Fatal("store must not record failed save")
	}
}

func TestSaveStartupFailureRollsBack(t *testing.T) {
	app, store, calls := newTestApp(t, &fakeEndpoint{level: 0.5}, core.DefaultSettings())
	app.startupWriter = func(s core.Settings) error {
		*calls = append(*calls, s)
		return errors.New("registry denied")
	}
	n := core.DefaultSettings()
	n.RunOnStartup = true
	if _, err := app.SaveSettings(n); err == nil {
		t.Fatal("expected startup error")
	}
	if app.worker.Status().Settings.RunOnStartup {
		t.Fatal("worker must roll back runOnStartup")
	}
	last, ok := store.last()
	if !ok || last.RunOnStartup {
		t.Fatal("store must be rolled back to previous settings")
	}
	if len(*calls) != 2 {
		t.Fatalf("startup writer must be attempted then reverted, got %d calls", len(*calls))
	}
	if app.getWarn() == "" {
		t.Fatal("failed revert must record warning")
	}
}

func TestSuccessfulSaveClearsWarning(t *testing.T) {
	app, store, _ := newTestApp(t, &fakeEndpoint{level: 0.5}, core.DefaultSettings())
	app.setWarn("ไฟล์ตั้งค่าเสียหาย ใช้ค่าเริ่มต้น")
	n := core.DefaultSettings()
	n.Target = 60
	if _, err := app.SaveSettings(n); err != nil {
		t.Fatal(err)
	}
	if app.getWarn() != "" {
		t.Fatal("successful save must clear corrupt-load warning")
	}
	st := app.GetStatus()
	if st.Error != "" {
		t.Fatalf("status must be clean, got %q", st.Error)
	}
	_ = store
}

func TestInvalidSaveRejected(t *testing.T) {
	app, store, _ := newTestApp(t, &fakeEndpoint{level: 0.5}, core.DefaultSettings())
	n := core.DefaultSettings()
	n.IntervalMS = 999
	if _, err := app.SaveSettings(n); err == nil {
		t.Fatal("invalid settings must be rejected")
	}
	if _, ok := store.last(); ok {
		t.Fatal("invalid settings must not reach store")
	}
}

func TestAutoHideOnceTrayBeforeDom(t *testing.T) {
	cfg := core.DefaultSettings()
	cfg.StartHidden = true
	cfg.CloseToTray = true
	app, _, _ := newTestApp(t, &fakeEndpoint{level: 0.5}, cfg)
	hides := 0
	app.win.hide = func(context.Context) { hides++ }
	app.tray = &trayMenu{done: make(chan struct{})}
	app.trayReady.Store(true)
	app.maybeHideOnStart()
	if hides != 0 {
		t.Fatal("must not hide before DOM ready")
	}
	app.ctx = context.Background()
	app.onDomReady(context.Background())
	if hides != 1 {
		t.Fatalf("expected exactly one hide, got %d", hides)
	}
	app.maybeHideOnStart()
	if hides != 1 {
		t.Fatal("auto-hide must fire only once")
	}
}

func TestAutoHideOnceDomBeforeTray(t *testing.T) {
	cfg := core.DefaultSettings()
	cfg.StartHidden = true
	cfg.CloseToTray = true
	app, _, _ := newTestApp(t, &fakeEndpoint{level: 0.5}, cfg)
	hides := 0
	app.win.hide = func(context.Context) { hides++ }
	app.tray = &trayMenu{done: make(chan struct{})}
	app.ctx = context.Background()
	app.onDomReady(context.Background())
	if hides != 0 {
		t.Fatal("must not hide before tray ready")
	}
	app.trayReady.Store(true)
	app.maybeHideOnStart()
	if hides != 1 {
		t.Fatalf("expected one hide after tray ready, got %d", hides)
	}
}

func TestShowCancelsPendingHide(t *testing.T) {
	cfg := core.DefaultSettings()
	cfg.StartHidden = true
	cfg.CloseToTray = true
	app, _, _ := newTestApp(t, &fakeEndpoint{level: 0.5}, cfg)
	hides := 0
	app.win.hide = func(context.Context) { hides++ }
	app.tray = &trayMenu{done: make(chan struct{})}
	app.ctx = context.Background()
	app.onDomReady(context.Background())
	app.ShowWindow()
	app.trayReady.Store(true)
	app.maybeHideOnStart()
	if hides != 0 {
		t.Fatal("ShowWindow must cancel pending auto-hide")
	}
}
