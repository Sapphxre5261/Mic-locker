package core

import "testing"

func TestShouldRestoreUnlockedNeverRestores(t *testing.T) {
	if ShouldRestore(false, 75, 0.1) {
		t.Fatal("unlocked must never restore")
	}
}

func TestShouldRestoreLockedEqual(t *testing.T) {
	if ShouldRestore(true, 75, 0.75) {
		t.Fatal("equal level must not restore")
	}
}

func TestShouldRestoreLockedDrift(t *testing.T) {
	if !ShouldRestore(true, 75, 0.74) {
		t.Fatal("drift of 1 percent must restore")
	}
}

func TestShouldRestoreBoundaries(t *testing.T) {
	if !ShouldRestore(true, 0, 0.5) {
		t.Fatal("target 0 vs 0.5 must restore")
	}
	if !ShouldRestore(true, 100, 0.5) {
		t.Fatal("target 100 vs 0.5 must restore")
	}
	if ShouldRestore(true, 0, 0) || ShouldRestore(true, 100, 1) {
		t.Fatal("exact boundary levels must not restore")
	}
}

func TestValidateTargetRange(t *testing.T) {
	s := DefaultSettings()
	s.Target = -1
	if s.Validate() == nil {
		t.Fatal("-1 must be invalid")
	}
	s.Target = 101
	if s.Validate() == nil {
		t.Fatal("101 must be invalid")
	}
	for _, v := range []int{0, 50, 100} {
		s.Target = v
		if err := s.Validate(); err != nil {
			t.Fatalf("target %d must be valid: %v", v, err)
		}
	}
}

func TestValidateIntervals(t *testing.T) {
	for _, ms := range []int{100, 250, 500, 1000} {
		s := DefaultSettings()
		s.IntervalMS = ms
		if err := s.Validate(); err != nil {
			t.Fatalf("interval %d must be valid: %v", ms, err)
		}
	}
	s := DefaultSettings()
	s.IntervalMS = 300
	if s.Validate() == nil {
		t.Fatal("interval 300 must be invalid")
	}
}

func TestValidateLanguage(t *testing.T) {
	for _, lang := range []string{"", "th", "en"} {
		s := DefaultSettings()
		s.Language = lang
		if err := s.Validate(); err != nil {
			t.Fatalf("language %q must be valid: %v", lang, err)
		}
	}
	s := DefaultSettings()
	s.Language = "fr"
	if s.Validate() == nil {
		t.Fatal("language fr must be rejected")
	}
}

func TestDefaultLanguageThai(t *testing.T) {
	if DefaultSettings().Language != "th" {
		t.Fatal("defaults must keep Thai for existing installs")
	}
}

func TestLocaleMapping(t *testing.T) {
	if (Settings{Language: "en"}).Locale() != "en" {
		t.Fatal("en must map to en")
	}
	for _, lang := range []string{"", "th", "fr"} {
		if (Settings{Language: lang}).Locale() != "th" {
			t.Fatalf("language %q must map to th", lang)
		}
	}
}

func TestValidateHiddenRequiresTray(t *testing.T) {
	s := DefaultSettings()
	s.CloseToTray = false
	s.StartHidden = true
	if s.Validate() == nil {
		t.Fatal("startHidden without closeToTray must be invalid")
	}
	s.CloseToTray = true
	if err := s.Validate(); err != nil {
		t.Fatalf("startHidden with closeToTray must be valid: %v", err)
	}
}
