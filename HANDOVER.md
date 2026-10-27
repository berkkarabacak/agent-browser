# Agent Browser handover

This is the state of the project at version **0.3.0**. Read this before changing the installer, the window, or the launch path.

## Goal

Agent Browser (`abrowser`) is a Windows profile manager for Chromium. Each email account or AI agent gets its own browser profile. Agents such as Claude or ChatGPT run as ordinary extensions inside that profile. There is no automation API: no Chrome DevTools Protocol, Puppeteer, or Playwright.

The target machine is one Windows PC with 16 GB of RAM and on the order of 100 profiles on disk. Only a handful should run at once. The people who install it include non-technical family (Berk's mom and grandfather), so the installer has to feel like installing Chrome: one download, no administrator, no Go, no MSYS, and no PowerShell.

Stock Google Chrome ignores `--load-extension`. The supported browsers are Chrome for Testing and ungoogled-chromium.

## What is already built

Three milestones are on `main`:

| Commit | What landed |
| --- | --- |
| `ea5bc3d` | CLI: templates, clone, extension store, groups, launch, close, locks, RAM cap |
| `b7668af` | Fyne desktop window in the same binary (`abrowser gui`) |
| `fb8cb1c` | Per-user `AgentBrowserSetup.exe`, first-run wizard, simpler Home screen, Web Store install |

Version string: `internal/version/version.go` (`Current = "0.3.0"`). The CLI, the window title line, and the NSIS installer all read that constant. The Makefile passes it to NSIS as `-DVERSION`.

## Architecture

One Go module, `abrowser`, Go 1.22. The only direct dependency is Fyne v2.6.3. Everything else in `go.mod` is pulled in by Fyne. The CLI and the window are the same executable.

```text
cmd/abrowser/           process entry, Windows icon resource, console attach
internal/cli/           commands and help
internal/desktop/       Fyne window, wizard, Home, serialized calls into the core
internal/app/           data-root lock, manifests, clone, launch, close
internal/extensions/    unpacked store, CRX extract, group resolution
internal/profile/       user-data copy, Chromium args, badge extension, eviction
internal/proc/          process tree, RSS, kill (Linux, Windows, other)
internal/browserfetch/  Chrome for Testing download and unzip
internal/webstore/      Chrome Web Store link or id, CRX download
internal/update/        GitHub releases check (notify only)
internal/memory/        free RAM and the suggested concurrent cap
internal/layout/        paths under the data root
internal/manifest/      profile and template JSON
internal/names/         id, email, locale, time zone, display-name slugs
installer/agentbrowser.nsi
assets/agentbrowser.ico
```

### One process, two faces

`abrowser` with no command, and `abrowser gui`, open the window. Any other command stays on the CLI. `abrowser gui --help` prints help and does not open a window.

The Windows build uses `-H windowsgui`, so a double-click has no console. From `cmd.exe`, `cmd/abrowser/console_windows.go` calls `AttachConsole` and reopens `CONOUT$` / `CONIN$`, so `abrowser list` still prints. `desktop.initConsole` calls `FreeConsole` when the window starts; that is harmless when there is no console.

### Desktop calls the core through one lock

`internal/app` takes a PID file lock (`.abrowser.lock`) for each operation. The lock records this process, and `proc.Alive` is true for the current PID, so two `App` methods in the same process deadlock if they overlap. `internal/desktop.Service` holds a Go mutex and runs every core call off the UI thread. Fyne updates from those goroutines go through `fyne.Do`. Do not call a `Service` method that locks from inside a method that already holds that mutex. Use the `Locked` helpers in `internal/desktop/setup.go`.

### Launch

Each profile is a real Chromium process with its own `--user-data-dir`. `internal/profile.BuildArgs` always passes:

- `--user-data-dir=<absolute path>`
- `--no-first-run`
- `--no-default-browser-check`
- `--disable-sync`
- `--disable-default-apps`
- `--new-window`
- `--disable-extensions-except` and `--load-extension` when any extension is resolved (comma-separated absolute paths; a comma inside a path is an error)
- `--lang` and `--accept-lang` from the profile locale
- on Linux only, `--class=abrowser-<id>`
- `config.json` `extraArgs` last

A small MV3 badge extension is appended at launch so the toolbar shows the last four alphanumerics of the profile id (`agent-001` becomes `T001`). It is not part of the resolved extension list.

A second launch of a profile that still has a live PID returns an error and does not start another window. `running.json` next to the profile records kind, id, pid, and start time. Close kills the process tree and removes that file. Closing something that is not running returns `*app.NotRunningError`; the window treats that as success.

Do not point the browser path at `/usr/local/bin/chrome` or `/usr/local/bin/google-chrome` on the Linux dev machine. Those wrappers force a shared user-data dir and a remote-debugging port.

### Templates and profiles

A golden template is a Chromium user-data directory plus a manifest. Freeze it before cloning. Clone copies the user data (caches stripped), then writes `profiles/<id>/manifest.json`. The wizard creates group `common` and a frozen template named `golden` (locale `en-US`, host time zone, group `common`).

Home creates a person from a display name and an email. The id is a slug of the name (`Mom` becomes `mom`). Collisions become `mom-2`, `mom-3`, and so on. The typed name is stored as `displayName`. Cards show that name. Advanced search matches it too.

### Extensions

The shared store is unpacked directories named `extensions/<name>@<version>/`. Names are lowercase. A bare name means the latest dotted version. `internal/extensions.Resolve` applies groups in order, then profile adds, then removals. A removal wins.

The Web Store path accepts a 32-character id (`[a-p]{32}`) or a `chromewebstore.google.com` / `chrome.google.com` URL. It downloads a CRX from Google's update service, unpacks it, slugs the manifest name, and either pins `name@version` on the `common` group or adds it to one profile. A live check downloaded Dark Reader `eimadpbcbfnmbkopoojfekhnkhdbieeh` as `dark-reader@4.9.133`. A store listing that does not publish a CRX still has to be installed from a file on Advanced → Extensions.

### How many Chromes may run

`internal/memory.SuggestMaxConcurrent` uses free RAM, about one running profile per 1.5 GiB. If free is unknown and total is known, free is treated as half of total. Machines with 18 GiB or less are capped at 8, so a 16 GB PC stays at 8 even when a lot of memory is free. Larger machines are capped at 20. The result is at least 1. The wizard writes that number into `maxConcurrent`. The hard CLI ceiling remains 1–500. The default before the wizard runs is 20.

Auto-close, when enabled, closes the profile with the oldest `lastActivityAt`. That timestamp is set at launch and is not updated from tabs, input, or network. Templates are never closed automatically.

### Data root

Windows: `%LOCALAPPDATA%\AgentBrowser`. Elsewhere: `$XDG_DATA_HOME/AgentBrowser` or `~/.local/share/AgentBrowser`. Override with `--root` (before the command, and also after `gui`) or `ABROWSER_ROOT`. `--root` wins.

Program files are separate: `%LOCALAPPDATA%\Programs\Agent Browser`. Uninstall can delete the program and keep the data root. The downloaded browser lives inside the data root, so keeping people also keeps Chrome.

```text
%LOCALAPPDATA%\AgentBrowser\
  config.json                 browserPath, maxConcurrent, autoCloseLRU, extraArgs, setupComplete
  groups.json                 map of group name to name@version pins
  registry.json               index rebuilt from the manifests
  .abrowser.lock              PID lock for the data root
  browser\chrome-win64\       Chrome for Testing, when the wizard downloaded it
  extensions\<name>@<version>\ unpacked extension, manifest.json at the root
  templates\<name>\
    manifest.json
    user-data\
    running.json              only while that template browser is running
    badge\                    generated at launch
    launch.log
  profiles\<id>\
    manifest.json             id, displayName, email, template, groups,
                              addExtensions, removeExtensions, timezone, locale,
                              notes, createdAt, updatedAt
    user-data\
    running.json
    badge\
    launch.log
```

Each profile manifest is the source of truth. `registry.json` is rebuilt from those files.

The first-run wizard appears when `setupComplete` is false and there are no profiles. A data root that already has profiles skips the wizard and sets the flag so it does not nag.

### Installer

NSIS, compiled on Linux with `makensis` (`Target amd64-unicode`). Inno Setup was not used because it needs Wine to compile. The script is `installer/agentbrowser.nsi`.

- `RequestExecutionLevel user`
- Install directory `%LOCALAPPDATA%\Programs\Agent Browser`
- Shortcuts: `$DESKTOP\Agent Browser.lnk` and `$SMPROGRAMS\Agent Browser\Agent Browser.lnk`, target `abrowser.exe`, no arguments, icon `agentbrowser.ico`
- Uninstall key `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\AgentBrowser` so it shows in Apps
- Uninstall checkbox **Keep my people and their browsers**, checked by default
- Silent install: `AgentBrowserSetup.exe /S` (keeps profiles)
- Silent uninstall: `Uninstall.exe /S` keeps `%LOCALAPPDATA%\AgentBrowser`
- `Uninstall.exe /S /DELETEPROFILES` deletes that data root

The Windows exe icon comes from `cmd/abrowser/windows.rc` via `x86_64-w64-mingw32-windres`. The generated `*.syso` is gitignored. The green "Ab" icon is `assets/agentbrowser.png` / `.ico`, and the same PNG is embedded for the Fyne window from `internal/desktop/icon.png`.

Chrome is not inside the installer. The wizard downloads the Stable Chrome for Testing zip for the current OS and architecture (Windows amd64 unpacks to `browser/chrome-win64/chrome.exe`) and saves that path. `ABROWSER_BROWSER`, when it points at an existing file, skips the download. `ABROWSER_CFT_MANIFEST` overrides the version-list URL.

### Update check

On startup the window GETs `https://api.github.com/repos/berkkarabacak/agent-browser/releases/latest` with a 5 second timeout (`internal/update`). `ABROWSER_UPDATE_URL` replaces that URL. HTTP 404 is ignored. A newer dotted version shows a line in the window. Nothing is downloaded or installed.

That GitHub repository did not exist when this handover was written (the public API returned 404). Until Berk creates it and publishes a release, the check stays quiet.

## Build from scratch

Linux, for the CLI, the window, and the installer:

```bash
sudo apt install gcc pkg-config libgl1-mesa-dev libxcursor-dev libxrandr-dev \
  libxinerama-dev libxi-dev libxxf86vm-dev gcc-mingw-w64-x86-64 nsis make
go test ./...
make build       # dist/abrowser          (cgo, Linux window)
make installer   # dist/AgentBrowserSetup.exe
```

`make installer` runs `make build-windows` first: `CGO_ENABLED=1`, `CC=x86_64-w64-mingw32-gcc`, `GOOS=windows`, `GOARCH=amd64`, ldflags `-s -w -H windowsgui`.

On Windows, for a local exe without NSIS, install MSYS2 UCRT64 gcc and put `C:\msys64\ucrt64\bin` on `PATH`:

```powershell
go test ./...
go build -ldflags "-s -w -H windowsgui" -o abrowser.exe ./cmd/abrowser
```

The icon resource step on Windows still needs `windres` (from mingw) if you want the exe icon. `make installer` does that on Linux.

`dist/`, `*.exe`, and `*.syso` are gitignored. The installer people download is a build product, not a committed file.

GitHub Actions: `.github/workflows/installer.yml` on pushes to `main` installs mingw and NSIS on `ubuntu-latest`, runs `make installer`, and uploads `dist/AgentBrowserSetup.exe` as the `AgentBrowserSetup` artifact. That workflow has not been observed running, because this history was not on github.com yet.

## How to test

Unit tests, no browser required:

```bash
go test ./...
```

They cover extension resolution (group order, latest version, add, remove-wins), CRX unpack, clone cache exclusion, launch arguments, the concurrent limit, profile filtering, name slugs, the RAM formula, Chrome zip unpack and zip-slip rejection, Web Store URL parsing, the update check, the first-run starter, and a CLI walkthrough that uses a fake browser process. The walkthrough creates a template, adds an extension from a folder and from a CRX, clones three profiles, hits the limit, and auto-closes the oldest launch.

Live Chromium, Linux only, not part of `go test ./...`:

```bash
go test -tags integration -count=1 -timeout 12m ./internal/app/ -run TestFiveLiveProfiles
```

Needs `DISPLAY` and Chrome for Testing. Set `ABROWSER_BROWSER`, or place the binary at `/tmp/cft/chrome-linux64/chrome`. The run that passed here used Chrome for Testing 156.0.8078.47. It checked five separate user-data dirs, the shared extension plus the per-profile extension on each command line, a localStorage value that stayed in one profile, the double-launch lock, and close.

Five real windows on that machine were about **1.3 GB RSS each**, about **6.7 GB** together. That is process-tree working set, not private bytes, so shared Chrome libraries are counted more than once. The practical cap on a 16 GB PC is **8** running profiles when free RAM would allow more (`SuggestMaxConcurrent`). The five-profile test itself set the cap to 5 so it could prove concurrency without filling the machine. Do not bake `--no-sandbox` into the Windows product. The Linux container needs extra args (`--no-sandbox`, `--disable-dev-shm-usage`, and the SwiftShader flags) only because of that container.

The exact Windows PowerShell steps for the same five profiles are in the README section "Five profiles on a Windows PC". They were not executed on Windows in this environment.

Installer, Wine only (Wine 9, 64-bit prefix):

- Welcome page shows "Install Agent Browser" and says it does not need an administrator.
- `AgentBrowserSetup.exe /S` installs `abrowser.exe`, `agentbrowser.ico`, and `Uninstall.exe` under the per-user Programs folder.
- Desktop and Start menu shortcuts target `abrowser.exe` with no `cmd.exe`.
- `HKCU\...\Uninstall\AgentBrowser` has DisplayName `Agent Browser`, DisplayVersion `0.3.0`.
- `Uninstall.exe /S` removes the program and leaves a file planted in the data root.
- `Uninstall.exe /S /DELETEPROFILES` removes the data root too.

NSIS uninstall relaunches itself and the parent exits first. Wait until the Wine process actually finishes before checking the files.

GUI, Linux Fyne on the dev display (Fyne under Wine was not attempted):

- Empty data root shows the wizard. **Get started** created `common` and frozen `golden`, then a profile from the name Ada and `ada@example.com` (id `ada`).
- Pasting the Dark Reader Web Store link installed it onto `common`.
- **Open** started Chrome for Testing with `--load-extension` pointing at `dark-reader@4.9.133`. The Dark Reader help page loaded.
- **Close** then **Open** stopped that process and started a new one.

On that run `ABROWSER_BROWSER` pointed at a Chrome for Testing binary already on disk, so the wizard skipped the download. The download itself is covered by `internal/browserfetch` tests against a local zip. A full first-run download on Windows is still untested.

## Release process

Intended home: a **private** GitHub repository at `https://github.com/berkkarabacak/agent-browser`.

1. Tag the commit that matches `internal/version.Current`. For this milestone the tag is `v0.3.0` (the checker strips a leading `v`).
2. Build `make installer` and attach `dist/AgentBrowserSetup.exe` to that GitHub release. The update check reads `releases/latest` and compares the tag to the running version. It only notifies.
3. Code signing is not done. Do not claim the installer is signed. Family members will see SmartScreen until an Authenticode certificate signs both `AgentBrowserSetup.exe` and `abrowser.exe`.
4. The README SmartScreen steps are the current user-facing workaround: **More info**, then **Run anyway**.

Creating that GitHub repository, pushing this history there, and publishing the `v0.3.0` release were not possible from the environment that wrote this file. `gh` was not logged in, there was no `GH_TOKEN`, SSH to `git@github.com` was denied, and the GitHub connection available to the agent failed during tool discovery. The public API for `berkkarabacak/agent-browser` returned 404.

## Known limitations

- The installer and `abrowser.exe` are unsigned. SmartScreen says Windows protected the PC. **More info** then **Run anyway** is required.
- The installer was exercised with Wine, not on a Windows PC. Shortcut targets, the per-user path, the uninstall registry key, and keep-versus-delete were checked in a Wine prefix.
- The five-profile RAM figure (~1.3 GB each) is from Linux Chrome for Testing, not from a 16 GB Windows laptop.
- Per-profile time zone is the `TZ` environment variable. Windows Chromium uses the system time zone and may ignore `TZ`. Locale is `--lang` and `--accept-lang`.
- Auto-close picks the oldest launch, not the idle profile. The field is named `lastActivityAt`, but nothing writes it after start.
- The update check only shows a line. It does not download or replace the exe. It also does nothing useful until `berkkarabacak/agent-browser` has a release.
- The first-run Chrome download was not watched end to end on Windows. The wizard path that skips the download (browser already present, or `ABROWSER_BROWSER`) was.
- No per-window title rewrite on Windows. The taskbar title stays the active tab. Identity is the badge and the profile name inside Agent Browser.
- Web Store installs need Google to return a CRX. Some listings will not.
- No sync between machines, no remote control, no automation API.
- RAM in the header is the sum of process-tree working sets.
- The data-root lock is a PID file, not a kernel mutex. A recycled PID could confuse it until that other process exits. A dead PID is reclaimed.
- Rewriting Chromium `Preferences` deletes `Secure Preferences` when the display name or clean-exit flag changes, so Chromium regenerates the HMAC. Protected settings that lived only in the signed file can be dropped on that rewrite.
- Home is a simple list. It has not been tried with 100 cards. Advanced uses a virtualized Fyne list.
- Linux container launches need `--no-sandbox` and related flags in `extraArgs`. Those must stay off the Windows default.

## Next steps

In the order that unblocks family use, then scale.

1. Create the private GitHub repo `berkkarabacak/agent-browser`, push `main` with this history, and publish GitHub release `v0.3.0` with `AgentBrowserSetup.exe` attached. That is the URL the update check already calls, and it is the download link to send.
2. Run the README five-profile test on a real 16 GB Windows PC: isolation, extensions, double-launch lock, close, and a RAM reading. Adjust the "about 1.3 GB" note if Windows differs.
3. On that same PC, install `AgentBrowserSetup.exe` as a normal user, walk through SmartScreen, finish the wizard including the Chrome download, create a person, open and close, install one Web Store extension, then uninstall once keeping profiles and once deleting them.
4. Sign `AgentBrowserSetup.exe` and `abrowser.exe` with an Authenticode certificate so SmartScreen stops blocking family members. Document the timestamp server and where the certificate lives. Do not commit the certificate.
5. After signing, ship the signed exe as a new release (`v0.3.1` or later) and bump `internal/version.Current` in the same commit as the tag. Confirm the running 0.3.0 window shows the "newer version" line.
6. Make 100 stored profiles comfortable: Home search, a calmer card for a long list, and a note of disk use per profile. The running cap can stay at 8 on 16 GB.
7. If auto-close should follow real use, define an idle signal that does not become an automation API, and stop using launch time as `lastActivityAt`.
8. Decide what to do about Windows time zones. If Chromium will not honor `TZ`, say so in the wizard and stop offering a per-profile zone on Windows, or find a flag that this Chrome for Testing build actually honors.
9. Only after the signed installer is in family hands: an update that downloads the new setup exe and tells the person to run it. Keep that separate from the notify-only check until signing exists.
