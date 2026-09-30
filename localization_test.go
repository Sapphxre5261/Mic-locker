package main

import (
	"errors"
	"fmt"
	"testing"

	"mic-locker/internal/core"
)

func TestLocalizeTextEnglish(t *testing.T) {
	if got := localizeText("en", "ระดับเสียงต้องอยู่ระหว่าง 0–100"); got != "Volume must be between 0 and 100" {
		t.Fatalf("got %q", got)
	}
	if got := localizeText("en", "ไฟล์ตั้งค่าเสียหาย ใช้ค่าเริ่มต้น (ไฟล์เดิมยังอยู่)"); got != "Settings file is damaged. Using defaults; the original file is preserved." {
		t.Fatalf("longest warning phrase must win, got %q", got)
	}
	if got := localizeText("en", "บันทึกตั้งค่าไม่สำเร็จ: access denied (0x80070005)"); got != "Could not save settings: access denied (0x80070005)" {
		t.Fatalf("native suffix must be preserved, got %q", got)
	}
}

func TestLocalizeTextThaiPassthrough(t *testing.T) {
	raw := "บันทึกตั้งค่าไม่สำเร็จ: x"
	if got := localizeText("th", raw); got != raw {
		t.Fatal("th must return the original text")
	}
}

func TestLocalizeErrorPreservesUnwrap(t *testing.T) {
	sentinel := errors.New("sentinel")
	err := localizeError("en", fmt.Errorf("บันทึกตั้งค่าไม่สำเร็จ: %w", sentinel))
	if err == nil {
		t.Fatal("expected wrapped error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatal("errors.Is must still match the wrapped error")
	}
	if err.Error() == sentinel.Error() {
		t.Fatal("message must be translated")
	}
	if got := localizeError("en", nil); got != nil {
		t.Fatal("nil must stay nil")
	}
	raw := errors.New("native detail (0x80070005)")
	if got := localizeError("en", raw); got != raw {
		t.Fatal("unmatched native errors must be returned unchanged")
	}
}

func TestTrayLabels(t *testing.T) {
	en := trayLabels("en")
	if en.Show != "Show Mic Locker" || en.ShowTip != "Open Mic Locker" ||
		en.Lock != "Lock microphone level" || en.LockTip != "Keep the microphone at the target level" ||
		en.Quit != "Quit" || en.QuitTip != "Close Mic Locker" {
		t.Fatalf("unexpected en labels: %+v", en)
	}
	th := trayLabels("th")
	if th.Show != "แสดง Mic Locker" || th.Lock != "ล็อกระดับเสียง" || th.Quit != "ออกจากโปรแกรม" {
		t.Fatalf("unexpected th labels: %+v", th)
	}
}

func TestSaveLanguagePersistsWithoutSideEffects(t *testing.T) {
	ep := &fakeEndpoint{level: 0.5}
	app, store, calls := newTestApp(t, ep, core.DefaultSettings())
	n := core.DefaultSettings()
	n.Language = "en"
	st, err := app.SaveSettings(n)
	if err != nil {
		t.Fatal(err)
	}
	last, ok := store.last()
	if !ok || last.Language != "en" {
		t.Fatalf("store must persist en: %+v", last)
	}
	if len(*calls) != 0 {
		t.Fatal("language change must not touch startup writer")
	}
	if ep.sets != 0 {
		t.Fatal("language change must not write audio level")
	}
	if st.Settings.Target != 75 || st.Settings.DeviceID != "" {
		t.Fatalf("target/device must be unchanged: %+v", st.Settings)
	}
}

func TestStatusWarningLocalizedByLanguage(t *testing.T) {
	app, _, _ := newTestApp(t, &fakeEndpoint{level: 0.5}, core.DefaultSettings())
	app.setWarn("ไฟล์ตั้งค่าเสียหาย ใช้ค่าเริ่มต้น")
	st := app.GetStatus()
	if st.Error != "ไฟล์ตั้งค่าเสียหาย ใช้ค่าเริ่มต้น" {
		t.Fatalf("th status must keep raw warning, got %q", st.Error)
	}
	n := core.DefaultSettings()
	n.Language = "en"
	st, err := app.SaveSettings(n)
	if err != nil {
		t.Fatal(err)
	}
	if st.Error != "" {
		t.Fatalf("successful save clears warning, got %q", st.Error)
	}
	app.setWarn("อ่านไฟล์ตั้งค่าไม่สำเร็จ ใช้ค่าเริ่มต้น")
	st = app.GetStatus()
	if st.Error != "Could not read settings. Using defaults." {
		t.Fatalf("en status must translate warning, got %q", st.Error)
	}
	n.Language = "th"
	st, err = app.SaveSettings(n)
	if err != nil {
		t.Fatal(err)
	}
	app.setWarn("อ่านไฟล์ตั้งค่าไม่สำเร็จ ใช้ค่าเริ่มต้น")
	st = app.GetStatus()
	if st.Error != "อ่านไฟล์ตั้งค่าไม่สำเร็จ ใช้ค่าเริ่มต้น" {
		t.Fatalf("th status must show original warning, got %q", st.Error)
	}
}

func TestSaveSettingsErrorLocalizedAtBoundary(t *testing.T) {
	app, store, _ := newTestApp(t, &fakeEndpoint{level: 0.5}, core.DefaultSettings())
	store.err = errors.New("disk full")
	n := core.DefaultSettings()
	n.Language = "en"
	_, err := app.SaveSettings(n)
	if err == nil {
		t.Fatal("expected error")
	}
}
