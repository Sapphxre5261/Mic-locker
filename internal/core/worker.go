package core

import (
	"errors"
	"math"
	"sync"
	"time"
)

var ErrStopped = errors.New("ตัวควบคุมไมโครโฟนปิดแล้ว")

type Endpoint interface {
	Read() (level float32, muted bool, peak float32, err error)
	SetLevel(level float32) error
	Close()
}

type Audio interface {
	Devices() ([]Device, error)
	Open(deviceID string) (Endpoint, string, string, error)
	Close()
}

type NewAudio func() (Audio, error)

type requestKind int

const (
	reqApply requestKind = iota
	reqRefresh
	reqStop
)

type request struct {
	kind     requestKind
	settings Settings
	reply    chan error
}

const (
	tickEvery   = 100 * time.Millisecond
	rescanEvery = 2 * time.Second
)

type Worker struct {
	newAudio NewAudio

	audio        Audio
	ep           Endpoint
	epID         string
	epName       string
	epOn         bool
	settings     Settings
	devices      []Device
	current      float32
	muted        bool
	peak         float32
	restorations uint64
	lastErr      string
	initErr      string

	lastRestore time.Time
	lastScan    time.Time
	lastInitTry time.Time

	mu    sync.Mutex
	snap  Status
	reqCh chan request
	done  chan struct{}
}

func NewWorker(newAudio NewAudio, initial Settings) *Worker {
	return &Worker{
		newAudio: newAudio,
		settings: initial,
		snap:     Status{Settings: initial, Devices: []Device{}},
		reqCh:    make(chan request),
		done:     make(chan struct{}),
	}
}

func (w *Worker) Start() {
	go w.run()
}

func (w *Worker) send(req request) error {
	select {
	case w.reqCh <- req:
	case <-w.done:
		return ErrStopped
	}
	select {
	case err := <-req.reply:
		return err
	case <-w.done:
		return ErrStopped
	}
}

func (w *Worker) Stop() {
	_ = w.send(request{kind: reqStop, reply: make(chan error, 1)})
	<-w.done
}

func (w *Worker) Apply(next Settings) error {
	if err := next.Validate(); err != nil {
		return err
	}
	return w.send(request{kind: reqApply, settings: next, reply: make(chan error, 1)})
}

func (w *Worker) Refresh() error {
	return w.send(request{kind: reqRefresh, reply: make(chan error, 1)})
}

func (w *Worker) Status() Status {
	return w.snapshot()
}

func (w *Worker) snapshot() Status {
	w.mu.Lock()
	defer w.mu.Unlock()
	s := w.snap
	s.Devices = append([]Device(nil), s.Devices...)
	return s
}

func (w *Worker) publish() {
	w.mu.Lock()
	w.snap = Status{
		Settings:         w.settings,
		Devices:          append([]Device(nil), w.devices...),
		ActiveDeviceID:   w.epID,
		ActiveDeviceName: w.epName,
		Connected:        w.epOn,
		Current:          int(math.Round(float64(w.current) * 100)),
		Muted:            w.muted,
		Peak:             w.peak,
		Restorations:     w.restorations,
		Error:            firstErr(w.initErr, w.lastErr),
	}
	w.mu.Unlock()
}

func firstErr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func (w *Worker) run() {
	w.initAudio()
	ticker := time.NewTicker(tickEvery)
	defer ticker.Stop()
	for {
		select {
		case req := <-w.reqCh:
			w.process(req)
			if req.kind == reqStop {
				w.shutdownAudio()
				close(w.done)
				return
			}
		case <-ticker.C:
			w.tick(time.Now())
		}
	}
}

func (w *Worker) initAudio() {
	a, err := w.newAudio()
	if err != nil {
		w.initErr = err.Error()
		w.publish()
		return
	}
	w.initErr = ""
	w.audio = a
	if err := w.scanDevices(); err == nil {
		w.rebind()
	}
	w.publish()
}

func (w *Worker) shutdownAudio() {
	w.closeEndpoint()
	if w.audio != nil {
		w.audio.Close()
		w.audio = nil
	}
}

func (w *Worker) closeEndpoint() {
	if w.ep != nil {
		w.ep.Close()
		w.ep = nil
	}
	w.epOn = false
	w.epID = ""
	w.epName = ""
	w.peak = 0
}

func (w *Worker) process(req request) {
	switch req.kind {
	case reqApply:
		req.reply <- w.apply(req.settings)
	case reqRefresh:
		if w.audio == nil {
			w.initAudio()
			if w.audio == nil {
				req.reply <- errors.New(w.initErr)
				return
			}
			req.reply <- nil
			return
		}
		if err := w.scanDevices(); err != nil {
			w.publish()
			req.reply <- err
			return
		}
		w.verifyEndpointStillValid()
		if !w.epOn {
			w.rebind()
		}
		w.publish()
		req.reply <- nil
	case reqStop:
		req.reply <- nil
	}
}

func (w *Worker) apply(next Settings) error {
	if err := next.Validate(); err != nil {
		return err
	}
	prev := w.settings
	devChanged := next.DeviceID != prev.DeviceID
	targetChanged := next.Target != prev.Target
	lockEnabled := next.Locked && !prev.Locked
	w.settings = next
	if devChanged {
		w.rebind()
	}
	if w.epOn && (targetChanged || lockEnabled || (devChanged && next.Locked)) {
		if err := w.ep.SetLevel(float32(next.Target) / 100); err != nil {
			w.settings = prev
			if devChanged {
				w.rebind()
			}
			w.lastErr = err.Error()
			w.publish()
			return err
		}
		w.readBack()
	}
	w.publish()
	return nil
}

func (w *Worker) readBack() {
	level, muted, peak, err := w.ep.Read()
	if err != nil {
		w.lastErr = err.Error()
		return
	}
	w.current = level
	w.muted = muted
	w.peak = peak
	w.lastErr = ""
}

func (w *Worker) tick(now time.Time) {
	if w.audio == nil {
		if now.Sub(w.lastInitTry) >= rescanEvery {
			w.lastInitTry = now
			w.initAudio()
		}
		w.publish()
		return
	}
	if now.Sub(w.lastScan) >= rescanEvery {
		w.lastScan = now
		if err := w.scanDevices(); err == nil {
			w.verifyEndpointStillValid()
			if !w.epOn {
				w.rebind()
			}
		}
	}
	if w.epOn {
		level, muted, peak, err := w.ep.Read()
		if err != nil {
			w.lastErr = err.Error()
			w.closeEndpoint()
		} else {
			w.current = level
			w.muted = muted
			w.peak = peak
			w.lastErr = ""
			if now.Sub(w.lastRestore) >= time.Duration(w.settings.IntervalMS)*time.Millisecond {
				w.lastRestore = now
				if ShouldRestore(w.settings.Locked, w.settings.Target, w.current) {
					if err := w.ep.SetLevel(float32(w.settings.Target) / 100); err != nil {
						w.lastErr = err.Error()
					} else {
						w.restorations++
						w.readBack()
					}
				}
			}
		}
	}
	w.publish()
}

func (w *Worker) scanDevices() error {
	devices, err := w.audio.Devices()
	if err != nil {
		w.lastErr = err.Error()
		w.devices = nil
		w.closeEndpoint()
		return err
	}
	w.devices = devices
	return nil
}

func (w *Worker) verifyEndpointStillValid() {
	if !w.epOn {
		return
	}
	if w.settings.DeviceID == "" {
		for _, d := range w.devices {
			if d.Default {
				if d.ID != w.epID {
					w.rebind()
				}
				return
			}
		}
		w.closeEndpoint()
		return
	}
	for _, d := range w.devices {
		if d.ID == w.settings.DeviceID {
			return
		}
	}
	w.closeEndpoint()
}

func (w *Worker) rebind() {
	w.closeEndpoint()
	if w.audio == nil {
		return
	}
	if w.settings.DeviceID != "" {
		found := false
		for _, d := range w.devices {
			if d.ID == w.settings.DeviceID {
				found = true
				break
			}
		}
		if !found {
			w.lastErr = "ไม่พบไมโครโฟนที่เลือก"
			return
		}
	}
	ep, id, name, err := w.audio.Open(w.settings.DeviceID)
	if err != nil {
		w.lastErr = err.Error()
		return
	}
	w.ep = ep
	w.epID = id
	w.epName = name
	w.epOn = true
	level, muted, _, err := w.ep.Read()
	if err != nil {
		w.lastErr = err.Error()
		w.closeEndpoint()
		return
	}
	w.current = level
	w.muted = muted
	w.lastErr = ""
}
