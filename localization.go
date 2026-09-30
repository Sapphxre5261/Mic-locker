package main

import (
	"strings"
)

var enReplacer = strings.NewReplacer(
	"ไฟล์ตั้งค่าเสียหาย ใช้ค่าเริ่มต้น (ไฟล์เดิมยังอยู่)", "Settings file is damaged. Using defaults; the original file is preserved.",
	"ไฟล์ตั้งค่าเสียหาย ใช้ค่าเริ่มต้น", "Settings file is damaged. Using defaults.",
	"อ่านไฟล์ตั้งค่าไม่สำเร็จ ใช้ค่าเริ่มต้น", "Could not read settings. Using defaults.",
	"ค่าตั้งค่าไม่ถูกต้อง ใช้ค่าเริ่มต้น", "Invalid settings. Using defaults.",
	"สร้างโฟลเดอร์ตั้งค่าไม่สำเร็จ", "Could not create the settings folder",
	"บันทึกตั้งค่าไม่สำเร็จ", "Could not save settings",
	"ไม่พบโฟลเดอร์ APPDATA", "APPDATA folder is unavailable",
	"ระดับเสียงต้องอยู่ระหว่าง 0–100", "Volume must be between 0 and 100",
	"ช่วงเวลาตรวจสอบไม่ถูกต้อง", "Invalid check interval",
	"เปิด System tray ก่อนเริ่มแบบซ่อนหน้าต่าง", "Enable System tray before starting hidden",
	"ภาษาที่เลือกไม่รองรับ", "Unsupported language",
	"ตัวควบคุมไมโครโฟนปิดแล้ว", "Microphone controller has stopped",
	"ไม่มีไมโครโฟนเริ่มต้น", "No default microphone is available",
	"ไม่พบไมโครโฟนที่เลือก", "The selected microphone is unavailable",
	"เปิดรีจิสทรีไม่สำเร็จ", "Could not open the startup registry",
	"ตั้งค่าเริ่มพร้อมระบบไม่สำเร็จ", "Could not update run-on-startup settings",
	"ยังไม่พร้อมใช้งาน System tray", "System tray is not ready",
	"หน้าต่างยังไม่พร้อม", "The window is not ready",
)

func localizeText(lang, text string) string {
	if lang != "en" {
		return text
	}
	return enReplacer.Replace(text)
}

type localizedError struct {
	msg string
	err error
}

func (e *localizedError) Error() string { return e.msg }
func (e *localizedError) Unwrap() error { return e.err }

func localizeError(lang string, err error) error {
	if err == nil {
		return nil
	}
	msg := localizeText(lang, err.Error())
	if msg == err.Error() {
		return err
	}
	return &localizedError{msg: msg, err: err}
}

type trayText struct {
	Show    string
	ShowTip string
	Lock    string
	LockTip string
	Quit    string
	QuitTip string
}

func trayLabels(lang string) trayText {
	if lang == "en" {
		return trayText{
			Show:    "Show Mic Locker",
			ShowTip: "Open Mic Locker",
			Lock:    "Lock microphone level",
			LockTip: "Keep the microphone at the target level",
			Quit:    "Quit",
			QuitTip: "Close Mic Locker",
		}
	}
	return trayText{
		Show:    "แสดง Mic Locker",
		ShowTip: "เปิดหน้าต่าง Mic Locker",
		Lock:    "ล็อกระดับเสียง",
		LockTip: "คงระดับเสียงไมโครโฟนไว้ตามเป้าหมาย",
		Quit:    "ออกจากโปรแกรม",
		QuitTip: "ปิด Mic Locker",
	}
}
