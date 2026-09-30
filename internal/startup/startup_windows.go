//go:build windows

package startup

import (
	"golang.org/x/sys/windows/registry"
)

func OpenRunKey() (Key, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return nil, err
	}
	return &runKeyHandle{k: k}, nil
}

type runKeyHandle struct {
	k registry.Key
}

func (h *runKeyHandle) GetString(name string) (string, error) {
	v, _, err := h.k.GetStringValue(name)
	if err == registry.ErrNotExist {
		return "", ErrNotExist
	}
	return v, err
}

func (h *runKeyHandle) SetString(name, value string) error {
	return h.k.SetStringValue(name, value)
}

func (h *runKeyHandle) DeleteValue(name string) error {
	err := h.k.DeleteValue(name)
	if err == registry.ErrNotExist {
		return ErrNotExist
	}
	return err
}

func (h *runKeyHandle) Close() {
	h.k.Close()
}
