# Mic Locker

Windows 10/11 desktop app (Wails v2 + embedded vanilla JS frontend) that locks a
chosen microphone's Windows capture volume at a target level.

## Build

```
go run github.com/wailsapp/wails/v2/cmd/wails@v2.16.0 build -clean
# output: build/bin/MicLocker.exe (requires WebView2 runtime on target machine)
```

## Test / verify

```
go test ./...
go vet ./...
node --check frontend/app.js
node --check frontend/i18n.js
node --test tests/frontend.test.cjs
.\build\bin\MicLocker.exe --check-audio   # read-only mic enumeration JSON
```

## Safety notes

- Local only: no recording, no network, no telemetry.
- `--check-audio` is read-only; it never sets volume or writes files/registry.
- Settings live in `%APPDATA%\MicLocker\settings.json` (written only on explicit save).
- Run-on-startup touches only HKCU `...\Run\MicLocker` when the user toggles it.
- Tests use fakes; never call live audio/registry mutation paths in tests.
- UI uses a blue/cyan glass theme (backdrop-filter); language is stored in settings with `th` as the legacy default.
