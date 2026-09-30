//go:build windows

package main

import (
	"encoding/json"
	"os"

	"mic-locker/internal/audio"
	"mic-locker/internal/core"
)

type audioCheckResult struct {
	OK       bool          `json:"ok"`
	Error    string        `json:"error,omitempty"`
	Devices  []core.Device `json:"devices"`
	ActiveID string        `json:"activeDeviceId"`
	Name     string        `json:"activeDeviceName"`
	Current  float32       `json:"current"`
	Muted    bool          `json:"muted"`
	Peak     float32       `json:"peak"`
}

func runCheckAudio() int {
	done := make(chan int, 1)
	go func() {
		done <- checkAudioLocked()
	}()
	return <-done
}

func checkAudioLocked() int {
	res := audioCheckResult{Devices: []core.Device{}}
	a, err := audio.New()
	if err != nil {
		res.Error = err.Error()
		writeCheck(res)
		return 1
	}
	defer a.Close()
	devices, err := a.Devices()
	if err != nil {
		res.Error = err.Error()
		writeCheck(res)
		return 1
	}
	res.Devices = devices
	ep, id, name, err := a.Open("")
	if err != nil {
		res.Error = err.Error()
		writeCheck(res)
		return 1
	}
	defer ep.Close()
	level, muted, peak, err := ep.Read()
	if err != nil {
		res.Error = err.Error()
		writeCheck(res)
		return 1
	}
	res.OK = true
	res.ActiveID = id
	res.Name = name
	res.Current = level
	res.Muted = muted
	res.Peak = peak
	writeCheck(res)
	return 0
}

func writeCheck(res audioCheckResult) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(res)
}
