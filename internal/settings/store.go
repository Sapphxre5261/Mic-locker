package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"mic-locker/internal/core"
)

type Store struct {
	path string
}

func New(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Path() string {
	return s.path
}

func DefaultPath() (string, error) {
	dir := os.Getenv("APPDATA")
	if dir == "" {
		return "", errors.New("ไม่พบโฟลเดอร์ APPDATA")
	}
	return filepath.Join(dir, "MicLocker", "settings.json"), nil
}

func (s *Store) Load() (core.Settings, string) {
	def := core.DefaultSettings()
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return def, ""
		}
		return def, "อ่านไฟล์ตั้งค่าไม่สำเร็จ ใช้ค่าเริ่มต้น"
	}
	next := def
	if err := json.Unmarshal(data, &next); err != nil {
		return def, "ไฟล์ตั้งค่าเสียหาย ใช้ค่าเริ่มต้น (ไฟล์เดิมยังอยู่)"
	}
	if next.Language == "" {
		next.Language = "th"
	}
	if err := next.Validate(); err != nil {
		return def, "ค่าตั้งค่าไม่ถูกต้อง ใช้ค่าเริ่มต้น"
	}
	return next, ""
}

func (s *Store) Save(cfg core.Settings) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("สร้างโฟลเดอร์ตั้งค่าไม่สำเร็จ: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), "settings-*.tmp")
	if err != nil {
		return fmt.Errorf("บันทึกตั้งค่าไม่สำเร็จ: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("บันทึกตั้งค่าไม่สำเร็จ: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("บันทึกตั้งค่าไม่สำเร็จ: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("บันทึกตั้งค่าไม่สำเร็จ: %w", err)
	}
	return nil
}
