package settings

import (
	"os"
	"path/filepath"
	"testing"

	"mic-locker/internal/core"
)

func TestRoundtrip(t *testing.T) {
	dir := t.TempDir()
	s := New(filepath.Join(dir, "sub", "settings.json"))
	cfg := core.DefaultSettings()
	cfg.Target = 42
	cfg.DeviceID = "micX"
	cfg.Locked = true
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	got, warn := s.Load()
	if warn != "" {
		t.Fatalf("unexpected warning: %s", warn)
	}
	if got != cfg {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
}

func TestMissingFileDefaults(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "settings.json"))
	got, warn := s.Load()
	if warn != "" || got != core.DefaultSettings() {
		t.Fatal("missing file must return defaults without warning")
	}
}

func TestCorruptPreserved(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(p, []byte("{bad json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := New(p)
	got, warn := s.Load()
	if warn == "" {
		t.Fatal("corrupt file must produce warning")
	}
	if got.Locked {
		t.Fatal("corrupt fallback must be unlocked defaults")
	}
	data, _ := os.ReadFile(p)
	if string(data) != "{bad json" {
		t.Fatal("corrupt file must be preserved")
	}
}

func TestInvalidValuesFallback(t *testing.T) {
	for _, body := range []string{
		`{"target": 500, "intervalMs": 250}`,
		`{"target": 50, "intervalMs": 123}`,
		`{"target": 50, "intervalMs": 250, "startHidden": true, "closeToTray": false}`,
	} {
		p := filepath.Join(t.TempDir(), "settings.json")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		got, warn := New(p).Load()
		if warn == "" {
			t.Fatalf("invalid settings %s must warn", body)
		}
		if got != core.DefaultSettings() {
			t.Fatalf("invalid settings must fall back to defaults: %s", body)
		}
	}
}

func TestLanguageRoundtrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	s := New(p)
	cfg := core.DefaultSettings()
	cfg.Language = "en"
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	got, warn := s.Load()
	if warn != "" || got.Language != "en" {
		t.Fatalf("language must roundtrip, got %q warn %q", got.Language, warn)
	}
}

func TestLegacyNoLanguageMigratesThai(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	body := `{"deviceId":"micX","target":62,"locked":true,"intervalMs":250,"closeToTray":true}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, warn := New(p).Load()
	if warn != "" {
		t.Fatalf("legacy file must load clean, got %q", warn)
	}
	if got.Target != 62 || !got.Locked || got.DeviceID != "micX" {
		t.Fatalf("legacy fields must be preserved: %+v", got)
	}
	if got.Language != "th" {
		t.Fatalf("missing language must default th, got %q", got.Language)
	}
}

func TestExplicitEmptyLanguageThai(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	body := `{"deviceId":"micX","target":50,"intervalMs":250,"closeToTray":true,"language":""}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, warn := New(p).Load()
	if warn != "" || got.Language != "th" {
		t.Fatalf("explicit empty language must migrate to th: %+v warn %q", got, warn)
	}
}

func TestInvalidLanguageWarnsDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	body := `{"target":50,"intervalMs":250,"closeToTray":true,"language":"fr"}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, warn := New(p).Load()
	if warn == "" {
		t.Fatal("invalid language must warn")
	}
	if got != core.DefaultSettings() {
		t.Fatalf("invalid language must fall back to defaults: %+v", got)
	}
}

func TestWriteFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := New(filepath.Join(blocker, "settings.json"))
	if err := s.Save(core.DefaultSettings()); err == nil {
		t.Fatal("save into uncreatable dir must fail")
	}
}

func TestAtomicReplaceExisting(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	s := New(p)
	cfg := core.DefaultSettings()
	if err := s.Save(cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Target = 10
	if err := s.Save(cfg); err != nil {
		t.Fatalf("rename over existing file must succeed: %v", err)
	}
	got, _ := s.Load()
	if got.Target != 10 {
		t.Fatal("second save must replace first")
	}
}
