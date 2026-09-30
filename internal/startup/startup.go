package startup

const ValueName = "MicLocker"

var ErrNotExist = errNotExist{}

type errNotExist struct{}

func (errNotExist) Error() string { return "value not found" }

type Key interface {
	GetString(name string) (string, error)
	SetString(name, value string) error
	DeleteValue(name string) error
}

func CommandLine(exePath string, startHidden bool) string {
	q := `"` + exePath + `"`
	if startHidden {
		return q + " --background"
	}
	return q
}

func Enabled(k Key) (bool, error) {
	_, err := k.GetString(ValueName)
	if err != nil {
		if err == ErrNotExist {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func Apply(k Key, exePath string, enabled bool, startHidden bool) error {
	if enabled {
		return k.SetString(ValueName, CommandLine(exePath, startHidden))
	}
	err := k.DeleteValue(ValueName)
	if err == ErrNotExist {
		return nil
	}
	return err
}
