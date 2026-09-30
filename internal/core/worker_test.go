package core

import (
	"errors"
	"testing"
	"time"
)

type fakeEndpoint struct {
	level   float32
	muted   bool
	peak    float32
	readErr error
	setErr  error
	sets    int
	closed  bool
}

func (e *fakeEndpoint) Read() (float32, bool, float32, error) {
	return e.level, e.muted, e.peak, e.readErr
}
func (e *fakeEndpoint) SetLevel(v float32) error {
	if e.setErr != nil {
		return e.setErr
	}
	e.sets++
	e.level = v
	return nil
}
func (e *fakeEndpoint) Close() { e.closed = true }

type fakeAudio struct {
	devices   []Device
	endpoints map[string]*fakeEndpoint
	openErr   error
	enumFail  bool
	opened    []string
	closed    bool
}

func (a *fakeAudio) Devices() ([]Device, error) {
	if a.enumFail {
		return nil, errors.New("enumerate failed")
	}
	return a.devices, nil
}

func (a *fakeAudio) Open(id string) (Endpoint, string, string, error) {
	a.opened = append(a.opened, id)
	if a.openErr != nil {
		return nil, "", "", a.openErr
	}
	if id == "" {
		for _, d := range a.devices {
			if d.Default {
				id = d.ID
				break
			}
		}
		if id == "" {
			return nil, "", "", errors.New("ไม่มีอุปกรณ์เริ่มต้น")
		}
	}
	ep, ok := a.endpoints[id]
	if !ok {
		return nil, "", "", errors.New("ไม่พบไมโครโฟน")
	}
	name := id
	for _, d := range a.devices {
		if d.ID == id {
			name = d.Name
		}
	}
	return ep, id, name, nil
}

func (a *fakeAudio) Close() { a.closed = true }

func newTestWorker(t *testing.T, a *fakeAudio, s Settings) *Worker {
	t.Helper()
	w := NewWorker(func() (Audio, error) { return a, nil }, s)
	w.initAudio()
	return w
}

func tickAt(w *Worker, ms int) {
	w.tick(time.Unix(0, 0).Add(time.Duration(ms) * time.Millisecond))
}

func TestWorkerLockedExternalChangeRestored(t *testing.T) {
	a := &fakeAudio{
		devices:   []Device{{ID: "mic1", Name: "Mic 1", Default: true}},
		endpoints: map[string]*fakeEndpoint{"mic1": {level: 0.75}},
	}
	s := DefaultSettings()
	s.Locked = true
	w := newTestWorker(t, a, s)
	tickAt(w, 0)
	tickAt(w, 250)
	a.endpoints["mic1"].level = 0.30
	tickAt(w, 550)
	if got := a.endpoints["mic1"].level; got != 0.75 {
		t.Fatalf("expected restore to 0.75, got %v", got)
	}
	st := w.snapshot()
	if st.Restorations != 1 {
		t.Fatalf("expected 1 restoration, got %d", st.Restorations)
	}
}

func TestWorkerUnlockedLeavesLevel(t *testing.T) {
	a := &fakeAudio{
		devices:   []Device{{ID: "mic1", Name: "Mic 1", Default: true}},
		endpoints: map[string]*fakeEndpoint{"mic1": {level: 0.30}},
	}
	w := newTestWorker(t, a, DefaultSettings())
	tickAt(w, 0)
	a.endpoints["mic1"].level = 0.10
	tickAt(w, 250)
	if got := a.endpoints["mic1"].level; got != 0.10 {
		t.Fatalf("unlocked must not write, got %v", got)
	}
	if a.endpoints["mic1"].sets != 0 {
		t.Fatal("unlocked must not call SetLevel")
	}
}

func TestWorkerMuteNotChanged(t *testing.T) {
	a := &fakeAudio{
		devices:   []Device{{ID: "mic1", Name: "Mic 1", Default: true}},
		endpoints: map[string]*fakeEndpoint{"mic1": {level: 0.75, muted: true}},
	}
	s := DefaultSettings()
	s.Locked = true
	w := newTestWorker(t, a, s)
	tickAt(w, 0)
	tickAt(w, 500)
	if !a.endpoints["mic1"].muted {
		t.Fatal("worker must never unmute")
	}
	if !w.snapshot().Muted {
		t.Fatal("status must report muted")
	}
}

func TestWorkerSetErrorNoRestorationAndVisible(t *testing.T) {
	ep := &fakeEndpoint{level: 0.30, setErr: errors.New("set failed")}
	a := &fakeAudio{
		devices:   []Device{{ID: "mic1", Name: "Mic 1", Default: true}},
		endpoints: map[string]*fakeEndpoint{"mic1": ep},
	}
	s := DefaultSettings()
	s.Locked = true
	w := newTestWorker(t, a, s)
	tickAt(w, 0)
	tickAt(w, 500)
	st := w.snapshot()
	if st.Restorations != 0 {
		t.Fatal("failed set must not increment restorations")
	}
	if st.Error == "" {
		t.Fatal("set error must be visible in status")
	}
}

func TestWorkerMissingExplicitDeviceNeverFallsBack(t *testing.T) {
	a := &fakeAudio{
		devices: []Device{
			{ID: "mic1", Name: "Mic 1", Default: true},
			{ID: "gone", Name: "Gone Mic"},
		},
		endpoints: map[string]*fakeEndpoint{"mic1": {level: 0.5}, "gone": {level: 0.5}},
	}
	s := DefaultSettings()
	s.DeviceID = "gone"
	w := newTestWorker(t, a, s)
	a.devices = []Device{{ID: "mic1", Name: "Mic 1", Default: true}}
	tickAt(w, 2000)
	st := w.snapshot()
	if st.Connected {
		t.Fatal("missing explicit device must show disconnected")
	}
	if st.ActiveDeviceID == "mic1" {
		t.Fatal("must never fall back to another mic")
	}
}

func TestWorkerReconnectRetry(t *testing.T) {
	ep := &fakeEndpoint{level: 0.5}
	a := &fakeAudio{
		devices:   []Device{{ID: "mic1", Name: "Mic 1", Default: true}},
		endpoints: map[string]*fakeEndpoint{"mic1": ep},
	}
	s := DefaultSettings()
	s.DeviceID = "mic1"
	w := newTestWorker(t, a, s)
	a.devices = nil
	tickAt(w, 2000)
	if w.snapshot().Connected {
		t.Fatal("device removed must disconnect")
	}
	a.devices = []Device{{ID: "mic1", Name: "Mic 1", Default: true}}
	tickAt(w, 4000)
	st := w.snapshot()
	if !st.Connected || st.ActiveDeviceID != "mic1" {
		t.Fatal("worker must retry and reconnect missing device")
	}
}

func TestWorkerApplyTargetOnceUnlocked(t *testing.T) {
	ep := &fakeEndpoint{level: 0.30}
	a := &fakeAudio{
		devices:   []Device{{ID: "mic1", Name: "Mic 1", Default: true}},
		endpoints: map[string]*fakeEndpoint{"mic1": ep},
	}
	w := newTestWorker(t, a, DefaultSettings())
	next := w.settings
	next.Target = 60
	w.apply(next)
	if ep.sets != 1 || ep.level != 0.60 {
		t.Fatalf("target change must apply once, sets=%d level=%v", ep.sets, ep.level)
	}
	tickAt(w, 0)
	tickAt(w, 250)
	if ep.sets != 1 {
		t.Fatal("unlocked ticks must not write again")
	}
}

func TestWorkerInitFailThenRecoversViaRefresh(t *testing.T) {
	calls := 0
	a := &fakeAudio{
		devices:   []Device{{ID: "mic1", Name: "Mic 1", Default: true}},
		endpoints: map[string]*fakeEndpoint{"mic1": {level: 0.5}},
	}
	w := NewWorker(func() (Audio, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("com init failed")
		}
		return a, nil
	}, DefaultSettings())
	w.Start()
	defer w.Stop()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if w.Status().Error != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if w.Status().Error == "" {
		t.Fatal("init failure must surface in status")
	}
	if err := w.Refresh(); err != nil {
		t.Fatalf("refresh after init recovery must succeed: %v", err)
	}
	st := w.Status()
	if !st.Connected || st.Error != "" {
		t.Fatalf("expected connected clean status, got %+v", st)
	}
}

func TestWorkerRefreshWhileAlwaysFailing(t *testing.T) {
	w := NewWorker(func() (Audio, error) {
		return nil, errors.New("no audio")
	}, DefaultSettings())
	w.Start()
	defer w.Stop()
	if err := w.Refresh(); err == nil {
		t.Fatal("refresh with failed init must return error")
	}
}

func TestWorkerCallsAfterStop(t *testing.T) {
	a := &fakeAudio{
		devices:   []Device{{ID: "mic1", Name: "Mic 1", Default: true}},
		endpoints: map[string]*fakeEndpoint{"mic1": {level: 0.5}},
	}
	w := NewWorker(func() (Audio, error) { return a, nil }, DefaultSettings())
	w.Start()
	done := make(chan struct{})
	go func() { w.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop hung")
	}
	if err := w.Apply(DefaultSettings()); err != ErrStopped {
		t.Fatalf("Apply after Stop must be ErrStopped, got %v", err)
	}
	if err := w.Refresh(); err != ErrStopped {
		t.Fatalf("Refresh after Stop must be ErrStopped, got %v", err)
	}
	st := w.Status()
	if st.Restorations != 0 {
		t.Fatal("Status after Stop must return promptly")
	}
}

func TestWorkerApplyInvalidRejected(t *testing.T) {
	a := &fakeAudio{
		devices:   []Device{{ID: "mic1", Name: "Mic 1", Default: true}},
		endpoints: map[string]*fakeEndpoint{"mic1": {level: 0.5}},
	}
	w := newTestWorker(t, a, DefaultSettings())
	bad := w.settings
	bad.Target = 200
	if err := w.apply(bad); err == nil {
		t.Fatal("invalid settings must be rejected")
	}
	if w.settings.Target != 75 {
		t.Fatal("invalid apply must not change settings")
	}
}

func TestWorkerApplySetErrorRollsBack(t *testing.T) {
	ep := &fakeEndpoint{level: 0.30, setErr: errors.New("set failed")}
	a := &fakeAudio{
		devices:   []Device{{ID: "mic1", Name: "Mic 1", Default: true}},
		endpoints: map[string]*fakeEndpoint{"mic1": ep},
	}
	w := newTestWorker(t, a, DefaultSettings())
	next := w.settings
	next.Target = 60
	if err := w.apply(next); err == nil {
		t.Fatal("set failure must return error")
	}
	if w.settings.Target != 75 {
		t.Fatal("failed apply must restore previous settings")
	}
	if ep.level != 0.30 {
		t.Fatal("failed set must not change fake level")
	}
	st := w.snapshot()
	if st.Error == "" || st.Settings.Target != 75 {
		t.Fatal("status must show error and previous settings")
	}
}

func TestWorkerApplySuccessReadsBack(t *testing.T) {
	ep := &fakeEndpoint{level: 0.30}
	a := &fakeAudio{
		devices:   []Device{{ID: "mic1", Name: "Mic 1", Default: true}},
		endpoints: map[string]*fakeEndpoint{"mic1": ep},
	}
	w := newTestWorker(t, a, DefaultSettings())
	next := w.settings
	next.Target = 60
	if err := w.apply(next); err != nil {
		t.Fatal(err)
	}
	if ep.level != 0.60 {
		t.Fatalf("target must be applied, got %v", ep.level)
	}
	if w.snapshot().Current != 60 {
		t.Fatal("status must reflect readback level")
	}
}

func TestWorkerLockedDeviceSwitchAppliesImmediately(t *testing.T) {
	ep1 := &fakeEndpoint{level: 0.75}
	ep2 := &fakeEndpoint{level: 0.20}
	a := &fakeAudio{
		devices: []Device{
			{ID: "mic1", Name: "Mic 1", Default: true},
			{ID: "mic2", Name: "Mic 2"},
		},
		endpoints: map[string]*fakeEndpoint{"mic1": ep1, "mic2": ep2},
	}
	s := DefaultSettings()
	s.Locked = true
	w := newTestWorker(t, a, s)
	next := w.settings
	next.DeviceID = "mic2"
	if err := w.apply(next); err != nil {
		t.Fatal(err)
	}
	if ep2.sets != 1 || ep2.level != 0.75 {
		t.Fatalf("locked device switch must apply target, sets=%d level=%v", ep2.sets, ep2.level)
	}
}

func TestWorkerDefaultChangeFollowed(t *testing.T) {
	ep1 := &fakeEndpoint{level: 0.5}
	ep2 := &fakeEndpoint{level: 0.4}
	a := &fakeAudio{
		devices: []Device{
			{ID: "mic1", Name: "Mic 1", Default: true},
			{ID: "mic2", Name: "Mic 2"},
		},
		endpoints: map[string]*fakeEndpoint{"mic1": ep1, "mic2": ep2},
	}
	w := newTestWorker(t, a, DefaultSettings())
	a.devices = []Device{
		{ID: "mic1", Name: "Mic 1"},
		{ID: "mic2", Name: "Mic 2", Default: true},
	}
	tickAt(w, 2000)
	st := w.snapshot()
	if !st.Connected || st.ActiveDeviceID != "mic2" {
		t.Fatal("auto mode must follow new default endpoint")
	}
}

func TestWorkerEnumFailureNoWrites(t *testing.T) {
	ep := &fakeEndpoint{level: 0.30}
	a := &fakeAudio{
		devices:   []Device{{ID: "mic1", Name: "Mic 1", Default: true}},
		endpoints: map[string]*fakeEndpoint{"mic1": ep},
	}
	s := DefaultSettings()
	s.Locked = true
	w := newTestWorker(t, a, s)
	a.enumFail = true
	tickAt(w, 2000)
	st := w.snapshot()
	if st.Connected {
		t.Fatal("failed enumeration must disconnect")
	}
	if len(st.Devices) != 0 {
		t.Fatal("failed enumeration must clear stale devices")
	}
	if ep.sets != 0 {
		t.Fatal("no writes allowed after failed enumeration")
	}
	a.enumFail = false
	a.devices = []Device{{ID: "mic1", Name: "Mic 1", Default: true}}
	tickAt(w, 4000)
	if !w.snapshot().Connected {
		t.Fatal("must recover after enumeration succeeds")
	}
}

func TestWorkerLanguageOnlyApplyNoSet(t *testing.T) {
	ep := &fakeEndpoint{level: 0.31}
	a := &fakeAudio{
		devices:   []Device{{ID: "mic1", Name: "Mic 1", Default: true}},
		endpoints: map[string]*fakeEndpoint{"mic1": ep},
	}
	w := newTestWorker(t, a, DefaultSettings())
	tickAt(w, 0)
	next := w.settings
	next.Language = "en"
	if err := w.apply(next); err != nil {
		t.Fatal(err)
	}
	if ep.sets != 0 {
		t.Fatal("language-only apply must not call SetLevel")
	}
	if ep.level != 0.31 {
		t.Fatalf("endpoint level must stay .31, got %v", ep.level)
	}
	st := w.snapshot()
	if st.Settings.Target != 75 || st.Settings.Language != "en" {
		t.Fatalf("settings must update language only: %+v", st.Settings)
	}
}

func TestWorkerLanguageApplyLockedNoExtraSet(t *testing.T) {
	ep := &fakeEndpoint{level: 0.75}
	a := &fakeAudio{
		devices:   []Device{{ID: "mic1", Name: "Mic 1", Default: true}},
		endpoints: map[string]*fakeEndpoint{"mic1": ep},
	}
	s := DefaultSettings()
	s.Locked = true
	w := newTestWorker(t, a, s)
	tickAt(w, 0)
	next := w.settings
	next.Language = "en"
	if err := w.apply(next); err != nil {
		t.Fatal(err)
	}
	if ep.sets != 0 {
		t.Fatalf("locked language-only apply must not add sets, got %d", ep.sets)
	}
}

func TestWorkerDeviceChangeUnlockedNoWrites(t *testing.T) {
	ep1 := &fakeEndpoint{level: 0.5}
	ep2 := &fakeEndpoint{level: 0.2}
	a := &fakeAudio{
		devices: []Device{
			{ID: "mic1", Name: "Mic 1", Default: true},
			{ID: "mic2", Name: "Mic 2"},
		},
		endpoints: map[string]*fakeEndpoint{"mic1": ep1, "mic2": ep2},
	}
	w := newTestWorker(t, a, DefaultSettings())
	next := w.settings
	next.DeviceID = "mic2"
	w.apply(next)
	st := w.snapshot()
	if !st.Connected || st.ActiveDeviceID != "mic2" {
		t.Fatal("device switch must rebind")
	}
	if ep2.sets != 0 {
		t.Fatal("unlocked device switch must not write")
	}
	if !ep1.closed {
		t.Fatal("old endpoint must be released")
	}
	if st.Current != 20 {
		t.Fatalf("expected read of new device level 20, got %d", st.Current)
	}
}
