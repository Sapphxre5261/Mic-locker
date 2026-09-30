# Mic Locker

A Windows 10/11 desktop utility that keeps your microphone's system volume at the level you choose. Built with Go, Wails v2, and a local HTML/CSS/JavaScript interface.

The interface supports **English and Thai**, with a blue/cyan glass theme, background blur, and reduced-motion support.

## Features

- Select a specific microphone or follow the Windows default microphone.
- Set a target volume from 0 to 100%, with optional automatic volume restoration.
- Choose a check interval of 100, 250, 500, or 1,000 milliseconds.
- View the current volume, mute state, input peak, and restoration count.
- Keep running in System tray, start hidden, or launch when you sign in to Windows.
- Save settings and your preferred interface language locally.
- Reconnect when the selected microphone becomes available again.

The app does not record audio or send microphone data over the network. The peak meter reads Windows audio-meter information; it does not record samples.

> Locking restores the Windows microphone volume after another application changes it; it does not prevent the initial change. Automatic gain control inside Discord, conferencing software, or a device driver may need to be disabled separately. Muting is never automatically undone.

## Requirements

Build this project on **Windows 10 or 11**. The native audio implementation is Windows-specific; Linux and macOS builds are not supported by this repository.

| Tool | Required for |
| --- | --- |
| Git | Cloning the repository. You can also download a source archive. |
| Go 1.27.1 or newer | Building the application; see `go.mod` for the declared version. |
| Microsoft Edge WebView2 Runtime | Running the desktop interface. Install it if it is missing. |
| Node.js 22 or newer | Running frontend syntax checks and tests; not needed to build or run the packaged application. |

The build below uses the pinned **Wails v2.16.0** CLI through `go run`, so installing a global `wails` command is optional. This frontend is plain static HTML/CSS/JavaScript: **there is no `npm install` or frontend bundling step**. The first build needs network access to download Go modules and build tools.

The verified build target is **Windows amd64 (x64)**. Other architectures have not been validated.

## Clone and build

Run these commands in **PowerShell**. Paste this repository's actual clone URL at the prompt; no particular hosting service is assumed.

```powershell
$RepositoryUrl = Read-Host "Repository clone URL"
git clone $RepositoryUrl Mic-locker
Set-Location .\Mic-locker

go version
go mod download
go run github.com/wailsapp/wails/v2/cmd/wails@v2.16.0 build -platform windows/amd64
```

The executable is written to:

```text
build\bin\MicLocker.exe
```

Start it with:

```powershell
Start-Process .\build\bin\MicLocker.exe
```

The executable embeds the frontend and tray icon. End users do not need Go or Node.js, but they do need WebView2 Runtime.

Close a previously running copy with **Quit / ออกจากโปรแกรม** in its tray menu before replacing the executable. Closing the window may only hide it in the tray. Mic Locker is single-instance, so launching another copy normally brings the running copy forward rather than loading a different build.

To build alongside an existing executable without overwriting it:

```powershell
go run github.com/wailsapp/wails/v2/cmd/wails@v2.16.0 build -platform windows/amd64 -o MicLocker-dev.exe
```

This changes the output filename, not the single-instance behavior. Avoid `build -clean` if you want to keep other files in `build/bin`.

## Modify and rebuild

- `frontend/index.html` — layout and accessible controls.
- `frontend/style.css` — theme, glass effects, animations, and responsive styles.
- `frontend/app.js` — frontend state, controls, and calls to the Go backend.
- `frontend/i18n.js` — English and Thai interface translations.
- `app.go` — Wails bindings and settings application.
- `tray.go` and `localization.go` — tray menu and backend message translations.
- `internal/core/` — microphone worker, settings types, and lock policy.
- `internal/audio/` — native Windows Core Audio adapter.
- `internal/settings/` — local JSON settings storage.
- `internal/startup/` — per-user Windows startup registration.
- `main.go` — application entry point and window configuration.
- `wails.json` — Wails build configuration and executable metadata.

After editing source files, run the build command again. Opening `frontend/index.html` directly in a browser shows a disconnected preview with microphone controls disabled; actual audio control requires the Wails desktop app.

Keep `go.mod` and `go.sum` together when sharing the source. Also keep `assets/icon.ico`, `build/appicon.png`, and **all files in `build/windows/`**: these are build inputs, not disposable executable output.

## Tests and checks

From the repository root on Windows:

```powershell
go test ./...
go vet ./...
node --check frontend/app.js
node --check frontend/i18n.js
node --test tests/frontend.test.cjs
```

The automated audio and startup tests use fakes. They do not change your real microphone volume or Windows startup registry.

The executable also supports `--check-audio`, a read-only microphone enumeration/status check. Because it is a Windows GUI executable, capture its output when using PowerShell:

```powershell
$AudioReport = Join-Path $env:TEMP ("MicLocker-audio-" + [guid]::NewGuid().ToString() + ".json")
$Check = Start-Process .\build\bin\MicLocker.exe -ArgumentList "--check-audio" -Wait -PassThru -RedirectStandardOutput $AudioReport
Get-Content $AudioReport
$Check.ExitCode
```

This diagnostic does not write microphone volume, application settings, or startup entries. It reports an error if no usable default microphone is available.

## Settings and normal use

1. Select your microphone or leave the device on Automatic.
2. Set the target volume, then enable the lock if you want ongoing restoration.
3. Choose English or ภาษาไทย in the top-right language selector.
4. Configure tray behavior, hidden startup, and launch-on-sign-in as desired.

Changing the target applies it once even when the lock is disabled. Changing only the interface language does not change audio settings.

Preferences are stored in:

```text
%APPDATA%\MicLocker\settings.json
```

Run on startup uses only the current user's registry value:

```text
HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run\MicLocker
```

Enabling Run on startup records the executable's current path. If you move or rename the executable, disable and re-enable that option in the new location. Starting hidden requires tray behavior to be enabled; if the window is hidden, use the tray menu to show it again.

## Generated files and Git

`.gitignore` excludes executable output (`build/bin/`), generated Wails JavaScript bindings (`frontend/wailsjs/`), temporary Windows resource objects, and local test output.

These files are regenerated by the build and do not need to be included in a source checkout. The current frontend uses Wails' injected `window.go` bridge directly rather than importing the generated wrappers. Tests and Windows build resources intentionally remain part of the source distribution.

Adding `.gitignore` does not delete local files, nor does it automatically remove files already tracked by Git. Review `git status` before committing. Do not commit local settings, credentials, or unrelated development artifacts.

## Troubleshooting

- **Go version mismatch:** install Go 1.27.1 or newer and check `go version` and `go.mod`.
- **No GUI / WebView2 missing:** install Microsoft Edge WebView2 Runtime, then launch again.
- **The old interface still opens:** quit the running copy through its tray menu before launching the new build.
- **Executable cannot be overwritten:** quit that executable before rebuilding, or use `-o` with a different output filename.
- **No microphone:** check the Windows input-device configuration and reconnect the selected device. A missing explicitly selected microphone is not silently replaced with another one.
- **Volume still changes inside another app:** disable that application's automatic gain control; the lock controls the Windows endpoint level, not every application's internal processing.

## สรุปภาษาไทย

ติดตั้ง Go 1.27.1 ขึ้นไปและ WebView2 Runtime จากนั้น clone โปรเจกต์และเปิด PowerShell ในโฟลเดอร์ซอร์ส:

```powershell
go mod download
go run github.com/wailsapp/wails/v2/cmd/wails@v2.16.0 build -platform windows/amd64
Start-Process .\build\bin\MicLocker.exe
```

ไม่ต้องรัน `npm install` และไม่ต้องติดตั้ง Wails CLI แยก หากต้องการรันทดสอบฝั่ง frontend ให้ติดตั้ง Node.js 22 ขึ้นไป ส่วนการแก้ UI และระบบเสียงดูหัวข้อ Modify and rebuild ด้านบน

ก่อนเปิดเวอร์ชันที่ build ใหม่ ให้เลือก **ออกจากโปรแกรม** จาก System tray ของตัวเก่าก่อน การปิดหน้าต่างอาจเป็นเพียงการซ่อนโปรแกรม การเพิ่ม `.gitignore` จะไม่ลบไฟล์เดิมของคุณ

## License

Copyright (C) 2026 Sapphxre.

Mic Locker is licensed under the **GNU General Public License, version 3 only** (`GPL-3.0-only`). See [LICENSE](LICENSE) for the complete terms.

You may use, modify, and redistribute this software, including commercially, subject to GPL v3. If you distribute a covered executable or modified version, comply with the license's requirements for notices, licensing, and providing the corresponding source. Private modifications that you do not distribute do not by themselves require public source publication.

This program is distributed **without any warranty**, including the implied warranties of merchantability or fitness for a particular purpose. Third-party dependencies retain their own licenses; this license does not replace their notices or terms.
