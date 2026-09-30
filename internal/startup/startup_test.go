package startup

import (
	"testing"
)

type fakeKey struct {
	values map[string]string
	getErr error
	setErr error
	delErr error
}

func newFakeKey() *fakeKey {
	return &fakeKey{values: map[string]string{"OtherApp": `"C:\Apps\other.exe"`}}
}

func (f *fakeKey) GetString(name string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	v, ok := f.values[name]
	if !ok {
		return "", ErrNotExist
	}
	return v, nil
}
func (f *fakeKey) SetString(name, value string) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.values[name] = value
	return nil
}
func (f *fakeKey) DeleteValue(name string) error {
	if f.delErr != nil {
		return f.delErr
	}
	if _, ok := f.values[name]; !ok {
		return ErrNotExist
	}
	delete(f.values, name)
	return nil
}

func TestEnableQuotesPathsWithSpaces(t *testing.T) {
	k := newFakeKey()
	if err := Apply(k, `C:\Program Files\Mic Locker\MicLocker.exe`, true, false); err != nil {
		t.Fatal(err)
	}
	got := k.values[ValueName]
	want := `"C:\Program Files\Mic Locker\MicLocker.exe"`
	if got != want {
		t.Fatalf("want %q got %q", want, got)
	}
	on, err := Enabled(k)
	if err != nil || !on {
		t.Fatal("must report enabled")
	}
	if k.values["OtherApp"] == "" {
		t.Fatal("other values must be preserved")
	}
}

func TestStartHiddenAppendsBackground(t *testing.T) {
	k := newFakeKey()
	if err := Apply(k, `E:\Mic-locker\bin\MicLocker.exe`, true, true); err != nil {
		t.Fatal(err)
	}
	want := `"E:\Mic-locker\bin\MicLocker.exe" --background`
	if k.values[ValueName] != want {
		t.Fatalf("want %q got %q", want, k.values[ValueName])
	}
}

func TestDisableRemovesOnlyOwnValue(t *testing.T) {
	k := newFakeKey()
	k.values[ValueName] = `"x.exe"`
	if err := Apply(k, "", false, false); err != nil {
		t.Fatal(err)
	}
	if _, ok := k.values[ValueName]; ok {
		t.Fatal("own value must be removed")
	}
	if k.values["OtherApp"] == "" {
		t.Fatal("other values must be preserved")
	}
}

func TestDisableWhenAbsentIsOK(t *testing.T) {
	k := newFakeKey()
	if err := Apply(k, "", false, false); err != nil {
		t.Fatal("removing absent value must be a no-op")
	}
}

func TestEnabledReflectsActualValue(t *testing.T) {
	k := newFakeKey()
	on, _ := Enabled(k)
	if on {
		t.Fatal("must report disabled when value absent")
	}
	k.values[ValueName] = `"something"`
	on, _ = Enabled(k)
	if !on {
		t.Fatal("must report enabled when value present")
	}
}
