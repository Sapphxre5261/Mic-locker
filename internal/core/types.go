package core

import (
	"errors"
	"math"
)

type Settings struct {
	DeviceID     string `json:"deviceId"`
	Target       int    `json:"target"`
	Locked       bool   `json:"locked"`
	IntervalMS   int    `json:"intervalMs"`
	CloseToTray  bool   `json:"closeToTray"`
	StartHidden  bool   `json:"startHidden"`
	RunOnStartup bool   `json:"runOnStartup"`
	Language     string `json:"language"`
}

type Device struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Default bool   `json:"default"`
}

type Status struct {
	Settings         Settings `json:"settings"`
	Devices          []Device `json:"devices"`
	ActiveDeviceID   string   `json:"activeDeviceId"`
	ActiveDeviceName string   `json:"activeDeviceName"`
	Connected        bool     `json:"connected"`
	Current          int      `json:"current"`
	Muted            bool     `json:"muted"`
	Peak             float32  `json:"peak"`
	Restorations     uint64   `json:"restorations"`
	Error            string   `json:"error"`
}

func DefaultSettings() Settings {
	return Settings{Target: 75, IntervalMS: 250, CloseToTray: true, Language: "th"}
}

func (s Settings) Locale() string {
	if s.Language == "en" {
		return "en"
	}
	return "th"
}

func (s Settings) Validate() error {
	if s.Target < 0 || s.Target > 100 {
		return errors.New("ระดับเสียงต้องอยู่ระหว่าง 0–100")
	}
	switch s.IntervalMS {
	case 100, 250, 500, 1000:
	default:
		return errors.New("ช่วงเวลาตรวจสอบไม่ถูกต้อง")
	}
	if s.StartHidden && !s.CloseToTray {
		return errors.New("เปิด System tray ก่อนเริ่มแบบซ่อนหน้าต่าง")
	}
	switch s.Language {
	case "", "th", "en":
	default:
		return errors.New("ภาษาที่เลือกไม่รองรับ")
	}
	return nil
}

func ShouldRestore(locked bool, target int, current float32) bool {
	return locked && math.Abs(float64(current)-float64(target)/100) > 0.005
}
