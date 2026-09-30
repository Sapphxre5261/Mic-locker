//go:build windows

package audio

import (
	"errors"
	"runtime"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"

	"mic-locker/internal/core"
)

type wcaAudio struct {
	enum *wca.IMMDeviceEnumerator
}

func New() (core.Audio, error) {
	runtime.LockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		runtime.UnlockOSThread()
		return nil, err
	}
	unk, err := ole.CreateInstance(wca.CLSID_MMDeviceEnumerator, wca.IID_IMMDeviceEnumerator)
	if err != nil {
		ole.CoUninitialize()
		runtime.UnlockOSThread()
		return nil, err
	}
	return &wcaAudio{enum: (*wca.IMMDeviceEnumerator)(unsafe.Pointer(unk))}, nil
}

func (a *wcaAudio) Close() {
	if a.enum != nil {
		a.enum.Release()
		a.enum = nil
	}
	ole.CoUninitialize()
	runtime.UnlockOSThread()
}

func deviceName(m *wca.IMMDevice) string {
	var ps *wca.IPropertyStore
	if err := m.OpenPropertyStore(wca.STGM_READ, &ps); err != nil {
		return ""
	}
	defer ps.Release()
	var pv wca.PROPVARIANT
	if err := ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
		return ""
	}
	return pv.String()
}

func (a *wcaAudio) Devices() ([]core.Device, error) {
	var dc *wca.IMMDeviceCollection
	if err := a.enum.EnumAudioEndpoints(wca.ECapture, wca.DEVICE_STATE_ACTIVE, &dc); err != nil {
		return nil, err
	}
	defer dc.Release()
	var count uint32
	if err := dc.GetCount(&count); err != nil {
		return nil, err
	}
	defaultID := ""
	var dm *wca.IMMDevice
	if err := a.enum.GetDefaultAudioEndpoint(wca.ECapture, wca.EConsole, &dm); err == nil {
		var id string
		if err := dm.GetId(&id); err == nil {
			defaultID = id
		}
		dm.Release()
	}
	devices := make([]core.Device, 0, count)
	for i := uint32(0); i < count; i++ {
		var m *wca.IMMDevice
		if err := dc.Item(i, &m); err != nil {
			continue
		}
		var id string
		if err := m.GetId(&id); err != nil {
			m.Release()
			continue
		}
		name := deviceName(m)
		if name == "" {
			name = "ไมโครโฟน"
		}
		devices = append(devices, core.Device{ID: id, Name: name, Default: id == defaultID})
		m.Release()
	}
	return devices, nil
}

type wcaEndpoint struct {
	vol   *wca.IAudioEndpointVolume
	meter *wca.IAudioMeterInformation
	dev   *wca.IMMDevice
}

func (a *wcaAudio) Open(deviceID string) (core.Endpoint, string, string, error) {
	var m *wca.IMMDevice
	if deviceID == "" {
		if err := a.enum.GetDefaultAudioEndpoint(wca.ECapture, wca.EConsole, &m); err != nil {
			return nil, "", "", errors.New("ไม่มีไมโครโฟนเริ่มต้น")
		}
	} else {
		var dc *wca.IMMDeviceCollection
		if err := a.enum.EnumAudioEndpoints(wca.ECapture, wca.DEVICE_STATE_ACTIVE, &dc); err != nil {
			return nil, "", "", err
		}
		var count uint32
		if err := dc.GetCount(&count); err == nil {
			for i := uint32(0); i < count; i++ {
				var item *wca.IMMDevice
				if err := dc.Item(i, &item); err != nil {
					continue
				}
				var id string
				if err := item.GetId(&id); err == nil && id == deviceID {
					m = item
					break
				}
				item.Release()
			}
		}
		dc.Release()
		if m == nil {
			return nil, "", "", errors.New("ไม่พบไมโครโฟนที่เลือก")
		}
	}
	var id string
	if err := m.GetId(&id); err != nil {
		m.Release()
		return nil, "", "", err
	}
	name := deviceName(m)
	var vol *wca.IAudioEndpointVolume
	if err := m.Activate(wca.IID_IAudioEndpointVolume, ole.CLSCTX_ALL, nil, &vol); err != nil {
		m.Release()
		return nil, "", "", err
	}
	var meter *wca.IAudioMeterInformation
	if err := m.Activate(wca.IID_IAudioMeterInformation, ole.CLSCTX_ALL, nil, &meter); err != nil {
		meter = nil
	}
	return &wcaEndpoint{vol: vol, meter: meter, dev: m}, id, name, nil
}

func (e *wcaEndpoint) Read() (float32, bool, float32, error) {
	var level float32
	if err := e.vol.GetMasterVolumeLevelScalar(&level); err != nil {
		return 0, false, 0, err
	}
	var muted bool
	if err := e.vol.GetMute(&muted); err != nil {
		return 0, false, 0, err
	}
	var peak float32
	if e.meter != nil {
		_ = e.meter.GetPeakValue(&peak)
	}
	return level, muted, peak, nil
}

func (e *wcaEndpoint) SetLevel(level float32) error {
	return e.vol.SetMasterVolumeLevelScalar(level, nil)
}

func (e *wcaEndpoint) Close() {
	if e.meter != nil {
		e.meter.Release()
		e.meter = nil
	}
	if e.vol != nil {
		e.vol.Release()
		e.vol = nil
	}
	if e.dev != nil {
		e.dev.Release()
		e.dev = nil
	}
}
