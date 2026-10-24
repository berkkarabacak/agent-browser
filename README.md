# Agent Browser

`abrowser` is a profile manager and launcher for Chromium. It keeps one isolated browser profile per email account or AI agent, on a single Windows PC, without an automation API. Agents such as Claude or ChatGPT run as normal browser extensions inside each profile.

Milestone 1 is the CLI: golden templates, cloned user-data directories, a shared extension store, and a cap on how many profiles can run at once. Milestone 2 adds `abrowser gui`, a desktop window on the same binary and the same data root.

Stock Google Chrome ignores `--load-extension`. Point `abrowser` at a Chromium build that still accepts it. [Ungoogled Chromium](https://github.com/ungoogled-software/ungoogled-chromium-windows/releases) is the reliable choice. [Chrome for Testing](https://googlechromelabs.github.io/chrome-for-testing/) does too. The Linux walkthrough below used Chrome for Testing 156.0.8078.47. Do not use the consumer Google Chrome from google.com/chrome.

## Build

Requires Go 1.22 or newer. The desktop window uses [Fyne](https://fyne.io), so the build needs cgo and a C compiler. One executable contains both the CLI and the GUI.

On Windows, install [MSYS2](https://www.msys2.org/) and the UCRT64 gcc toolchain, then put `C:\msys64\ucrt64\bin` on `PATH` so `gcc` is visible to cgo:

```powershell
go test ./...
go build -ldflags "-s -w" -o abrowser.exe ./cmd/abrowser
```

The executable is a console program, so `abrowser list` still prints in `cmd.exe`. `abrowser gui` detaches that console and leaves the window up.

From Linux, install a C compiler, the GL/X11 headers, and mingw-w64, then cross-compile:

```bash
sudo apt install gcc pkg-config libgl1-mesa-dev libxcursor-dev libxrandr-dev \
  libxinerama-dev libxi-dev libxxf86vm-dev gcc-mingw-w64-x86-64
make test
make build          # dist/abrowser
make build-windows  # dist/abrowser.exe
```

`make build-windows` sets `CGO_ENABLED=1` and `CC=x86_64-w64-mingw32-gcc`. It does not pass `-H windowsgui`, so the CLI keeps a console.

Put `abrowser.exe` on `PATH`. The tool does not need Node.

## Data root

On Windows the default root is `%LOCALAPPDATA%\AgentBrowser`. Elsewhere it is `$XDG_DATA_HOME/AgentBrowser` or `~/.local/share/AgentBrowser`.

Override it with `--root` or the `ABROWSER_ROOT` environment variable. `--root` wins.

```text
%LOCALAPPDATA%\AgentBrowser\
  config.json                 browser path, max concurrent, extra Chromium flags
  groups.json                 named extension groups
  registry.json               index rebuilt from the manifests
  extensions\<name>@<version>\ unpacked extension
  templates\<name>\
    manifest.json
    user-data\                Chromium user-data directory
  profiles\<id>\
    manifest.json             id, email, template, groups, overrides, timezone, locale, notes
    user-data\
    running.json              present only while that browser is running
```

Each profile manifest is the source of truth. `registry.json` is an index rebuilt from those files.

## Get a browser

1. Download a Windows 64-bit build of [ungoogled-chromium](https://github.com/ungoogled-software/ungoogled-chromium-windows/releases), or the `chrome-win64` zip from [Chrome for Testing](https://googlechromelabs.github.io/chrome-for-testing/).
2. Unzip it. Note the path to `chrome.exe`.
3. Tell `abrowser` where it is:

```powershell
abrowser init
abrowser config set browser "$env:LOCALAPPDATA\ungoogled-chromium\chrome.exe"
abrowser config show
```

`config show` prints the root, whether the executable exists, the concurrent limit, and any extra Chromium flags.

A signed-in AI tab is often 400–800 MB. On a 16 GB PC the operating system already uses several gigabytes, so a practical `max-concurrent` is often 8–12 rather than the default ceiling of 20. The default is a safety cap, not a promise that 20 Chrome windows will fit.

## End-to-end example

This creates a golden template, registers two extensions and two groups, clones three profiles, and launches them.

```powershell
abrowser init
abrowser config set browser "$env:LOCALAPPDATA\ungoogled-chromium\chrome.exe"
abrowser config set max-concurrent 12
abrowser config set auto-close-lru true

abrowser ext add ublock-origin --from C:\ext\ublock-origin
abrowser ext add sales-helper --from C:\ext\sales-helper
abrowser ext list

abrowser group create common
abrowser group add common ublock-origin
abrowser group create sales
abrowser group add sales sales-helper
abrowser group list

abrowser template create golden --timezone America/New_York --locale en-US --group common
abrowser template launch golden
# Set extension options, bookmarks, and the pages each agent should inherit.
# Close the window when you are done (or: abrowser template close golden).
abrowser template freeze golden

abrowser clone golden --as agent-001 --email ada@example.com --group common --group sales
abrowser clone golden --as agent-002 --email grace@example.com
abrowser clone golden --as agent-003 --email lin@example.com --group common --group sales

abrowser profile show agent-001
abrowser launch agent-001 --dry-run
abrowser launch agent-001
abrowser launch agent-002
abrowser launch agent-003
abrowser status
abrowser close agent-001
```

`agent-002` omits `--group`, so it inherits the template's groups (`common` only). Passing `--group` replaces that list instead of appending to it.

`profile show` prints the resolved extension list: group members first, then `+ext` overrides, then `-ext` removals. A removal wins over a group and over an add.

```powershell
abrowser profile ext add agent-003 extra-tool
abrowser profile ext remove agent-003 ublock-origin
abrowser profile show agent-003
```

`profile ext remove` records a `-ext` override. It does not delete the extension from the shared store. `abrowser ext remove ublock-origin@1.52.2 --yes` does that, and it refuses while a group or an override still references it.

## What launch does

Each launch gets its own `--user-data-dir`. The resolved extensions, plus a small badge extension, are passed as:

```text
--load-extension=<paths>
--disable-extensions-except=<same paths>
```

`--disable-extensions-except` disables extensions that were installed inside the profile but are not in the resolved set. Manage extensions from the shared store so every profile stays on the same versions.

Also set on every launch:

- `--lang` and `--accept-lang` from the profile locale
- `TZ` and `LANGUAGE` in the process environment when a timezone or locale is set
- `--class=abrowser-<id>` on Linux, so window managers can tell profiles apart
- the Chromium profile display name (`Default/Preferences` and `Local State`)
- a toolbar badge whose text is the last four letters or digits of the profile id, and whose tooltip is `<id> · <email>`

Chromium on Windows uses the system time zone and may ignore `TZ`. Locale flags still apply. The window title bar stays the tab title; the profile name is the toolbar avatar and the badge. `abrowser` sets `profile.exit_type` to `Normal` before launch so a previous `abrowser close` does not stop on the "restore pages" bubble.

A profile that is already running is refused. The lock is `running.json` plus the recorded PID. If that PID is dead, the next launch treats the lock as stale. Chromium also refuses a second process on the same user-data directory.

`abrowser close <id>` stops that browser's process tree. `abrowser delete <id>` asks you to type the profile id, unless you pass `--yes`. It will not delete a running profile.

## Extensions and groups

```powershell
abrowser ext add ublock-origin --from C:\src\uBlock
abrowser ext add something --crx C:\downloads\something.crx
abrowser ext list
abrowser group create common
abrowser group add common ublock-origin
abrowser group remove common ublock-origin
```

`group add` pins `name@version`. If you pass a bare name, the newest dotted version in the store is pinned (`1.10.0` beats `1.9.0`). Adding the same name again replaces the pin. Profiles store group names, so the next launch picks up the new pin. Copies that were already cloned do not keep a private copy of the member list.

Unpacked extension IDs are derived from the absolute path unless `manifest.json` contains a `"key"`. Keep the data root stable, and put a public key in the manifest if settings must survive a version bump. A new directory name is a new extension id.

CRX2 and CRX3 files are unpacked into the store. A plain zip is rejected; unpack it and use `--from`.

## Limits and RAM

```powershell
abrowser config set max-concurrent 12
abrowser config set auto-close-lru true
abrowser status
```

`status` lists running profiles and templates, the PID, and the resident memory of each process tree. Shared libraries are counted once per process, so the total is an approximation.

With `auto-close-lru` on, a launch that would pass the cap closes the least recently **launched** profile first. Templates that you are configuring are not closed automatically. This is not input-idle or network-idle detection.

Extra Chromium flags are appended after Agent Browser's own flags:

```powershell
abrowser config set extra-args --disable-renderer-backgrounding --disable-background-timer-throttling
```

Run `config set extra-args` with no flags to clear them.

## Templates

| Command | What it does |
| --- | --- |
| `template create` | Empty user-data directory and manifest |
| `template launch` | Opens the golden profile so you can configure it |
| `template close` | Stops that browser |
| `template freeze` | Marks user data as the clone source and prints the slim size |
| `template unfreeze` | Allows another configure session |
| `template delete --yes` | Removes a template no profile still references |

Freeze refuses to run while the template browser is open. Close the window normally when you can. `template close` kills the process; the next launch clears the crashed-session flag.

Cloning skips caches and runtime files, including `Cache`, `Code Cache`, `GPUCache`, shader caches, Crashpad, BrowserMetrics, service-worker cache storage, HTTP cache, singleton locks, and `LOG`. It keeps bookmarks, cookies, local storage, IndexedDB, extension settings, and session files. Extension settings are kept even if a folder inside them is named `Cache`.

Changing a template later does not update profiles that were already cloned.

## Command reference

```text
abrowser init
abrowser version
abrowser config show [--json]
abrowser config set browser <path>
abrowser config set max-concurrent <n>
abrowser config set auto-close-lru <true|false>
abrowser config set extra-args [flags...]

abrowser ext list [--json]
abrowser ext add <name> (--from <dir> | --crx <file>) [--version <v>]
abrowser ext remove <name[@version]> [--yes]

abrowser group list [--json]
abrowser group create <name>
abrowser group add <group> <extension>
abrowser group remove <group> <extension>

abrowser template create|launch|close|freeze|unfreeze|list|delete|set
abrowser template group add|remove <template> <group>
abrowser template ext add|remove|reset <template> <extension>

abrowser clone <template> --as <id> --email <email>
            [--timezone TZ] [--locale LOC] [--notes TEXT]
            [--group G]... [--add-ext EXT]... [--remove-ext EXT]...

abrowser list [--json]
abrowser launch <id> [--dry-run] [--json]
abrowser close <id>
abrowser delete <id> [--yes]
abrowser status [--json]

abrowser profile show <id> [--json]
abrowser profile set <id> [--email E] [--timezone TZ] [--locale LOC] [--notes TEXT]
abrowser profile group add|remove <id> <group>
abrowser profile ext add|remove|reset <id> <extension>

abrowser gui [--root PATH]
```

`--json` prints the same data as a JSON document. Global flags `--root` and `--json` must come before the command (`abrowser --root PATH gui` works too). `abrowser <command> --help` prints that command's usage.

Ids are 1–64 characters: letters, digits, `.`, `_`, and `-`. Extension and group names are lowercase. Emails are checked lightly (`ada@example.com`). Time zones are IANA names (`America/New_York`).

## Desktop window

`abrowser gui` opens the window for the same data root as the CLI.

- **Profiles** lists id, email, template, groups, extension overrides, running state, and RAM. The filter matches any of those fields. Each row has Launch, Close, and Edit. Checkboxes plus Launch selected / Close selected act on many profiles. Clone asks for a frozen template, an id, an email, and groups.
- **Edit** changes that profile's groups and its +ext / −ext overrides. Add pins an extra extension, Block records a removal (a removal wins), and Clear override drops both. The toolbar badge is still added automatically at launch.
- **Extensions** lists the shared store and can add an unpacked folder or a `.crx`, or remove a version that nothing references.
- **Groups** creates a group and pins or removes `name@version` members.
- The header is always `Running N / max` plus approximate RAM.
- **Settings** stores the browser executable, max concurrent (1–500), and auto-close. Extra Chromium flags stay on `abrowser config set extra-args`.

Templates are still created and frozen from the CLI. The window only clones from a template that is already frozen.

## Five profiles on a Windows PC

This is the same check that ran here against Chrome for Testing: one shared extension in the `common` group, one extra extension per profile, five concurrent windows, separate user-data directories, a localStorage value that stays in one profile, the double-launch lock, then close.

1. Install Go 1.22 or newer and the MSYS2 UCRT64 gcc toolchain described above. Build `abrowser.exe` from this repo.
2. Download Chrome for Testing for Windows and unzip it. The build used here was [156.0.8078.47 win64](https://storage.googleapis.com/chrome-for-testing-public/156.0.8078.47/win64/chrome-win64.zip). A current Stable Chrome for Testing zip from [chrome-for-testing](https://googlechromelabs.github.io/chrome-for-testing/) is fine. Note the path to `chrome.exe` inside `chrome-win64`. Do not point this at stock Google Chrome.
3. In PowerShell, from any directory:

```powershell
$browser = "$env:USERPROFILE\chrome-win64\chrome.exe"
abrowser init
abrowser config set browser $browser
abrowser config set max-concurrent 5
abrowser config set auto-close-lru false

$src = "$env:TEMP\abrowser-ext"
New-Item -ItemType Directory -Force "$src\common-marker" | Out-Null
[System.IO.File]::WriteAllText("$src\common-marker\manifest.json", '{"manifest_version":3,"name":"Common Marker","version":"1.0.0","description":"Shared extension for every agent profile."}')
abrowser ext add common-marker --from "$src\common-marker"

1..5 | ForEach-Object {
  $n = "{0:d3}" -f $_
  $dir = "$src\note-$n"
  New-Item -ItemType Directory -Force $dir | Out-Null
  $json = '{"manifest_version":3,"name":"Note ' + $n + '","version":"1.0.0","description":"Per-profile extension for agent-' + $n + '."}'
  [System.IO.File]::WriteAllText("$dir\manifest.json", $json)
  abrowser ext add "note-$n" --from $dir
}

abrowser group create common
abrowser group add common common-marker
abrowser template create golden --timezone America/New_York --locale en-US --group common
abrowser template freeze golden
abrowser gui
```

`WriteAllText` avoids a UTF-8 BOM, which would make `manifest.json` invalid.

4. In the window, Settings should already show that `chrome.exe`, max concurrent `5`, and auto-close off. Save only if you change them.
5. Profiles, Clone, five times:

| ID | Email | Groups |
| --- | --- | --- |
| agent-001 | agent-001@example.com | common |
| agent-002 | agent-002@example.com | common |
| agent-003 | agent-003@example.com | common |
| agent-004 | agent-004@example.com | common |
| agent-005 | agent-005@example.com | common |

The template's `common` group is checked for you. Leave it checked.

6. Edit each row. Choose `note-00N@1.0.0` and click Add (+). agent-001 gets note-001, agent-002 gets note-002, and so on. Resolved should list `common-marker@1.0.0` and that profile's note.
7. Select shown, then Launch selected. The header should read `Running 5 / 5` and five Chrome for Testing windows should open. In PowerShell, the five main processes have different `--user-data-dir` values and the matching extensions:

```powershell
Get-CimInstance Win32_Process -Filter "Name = 'chrome.exe'" |
  Where-Object { $_.CommandLine -like '*--user-data-dir=*' -and $_.CommandLine -notlike '*--type=*' } |
  ForEach-Object { $_.CommandLine }
```

Each line should contain `profiles\agent-00N\user-data`, `common-marker@`, and `note-00N@`, and it should not contain another profile's `note-00M@`.

8. Click Launch on the agent-001 row again. The window reports that agent-001 is already running, and the other four stay up. That is the double-launch lock.
9. Close selected. The header returns to `Running 0 / 5`.
10. After they are closed, write a page that only agent-001 will open:

```powershell
$page = "$env:TEMP\abrowser-iso.html"
[System.IO.File]::WriteAllText($page, @"
<!doctype html><meta charset="utf-8"><title>isolation</title>
<script>
localStorage.setItem("abrowserIsolation", "only-agent-001");
document.cookie = "abrowserIsolation=only-agent-001; path=/";
</script><p>only-agent-001</p>
"@)
$ud = "$env:LOCALAPPDATA\AgentBrowser\profiles\agent-001\user-data"
& $browser --user-data-dir="$ud" --no-first-run --no-default-browser-check "file:///$($page -replace '\\','/')"
```

Close that window, then search both profiles. agent-001's user-data should contain `only-agent-001`. agent-002's should not:

```powershell
$root = "$env:LOCALAPPDATA\AgentBrowser\profiles"
Get-ChildItem "$root\agent-001\user-data" -Recurse -File -ErrorAction SilentlyContinue |
  Select-String -Pattern "only-agent-001" -SimpleMatch -List
Get-ChildItem "$root\agent-002\user-data" -Recurse -File -ErrorAction SilentlyContinue |
  Select-String -Pattern "only-agent-001" -SimpleMatch -List
```

The second command should print nothing. The value lives in that profile's Local Storage (or Cookies) only.

On Linux the same scenario is `go test -tags integration -count=1 -timeout 12m ./internal/app/ -run TestFiveLiveProfiles`. It expects `DISPLAY` and Chrome for Testing. Set `ABROWSER_BROWSER` to the chrome binary, or place it at `/tmp/cft/chrome-linux64/chrome`. That test is not part of `go test ./...`.

## Tests

```bash
go test ./...
```

The tests cover extension resolution (group order, latest version, add, and remove-wins), CRX unpacking, cache exclusion while cloning, launch arguments, the concurrent limit, profile-list filtering, and a CLI walkthrough that creates a template, adds extensions from a folder and a `.crx`, clones three profiles, launches them, hits the limit, and auto-closes the oldest. The walkthrough uses a fake browser process, not a real Chromium. `TestFiveLiveProfiles` (build tag `integration`) launches five real Chrome for Testing processes and checks user-data dirs, loaded extensions, the double-launch lock, and a localStorage marker.

## Layout of this repo

```text
cmd/abrowser/          main
internal/cli/          commands and help
internal/desktop/      Fyne window (abrowser gui)
internal/app/          root lock, manifests, launch, clone
internal/extensions/   store, CRX, resolution
internal/profile/      user-data copy, preferences, badge, process start
internal/proc/         process tree, RSS, kill (Linux, Windows, other)
internal/layout/       paths under the data root
internal/manifest/     profile and template JSON
internal/names/        id, email, locale, and time zone checks
internal/version/      release string shared by the CLI and the window
```

## Left out

- No Chrome DevTools Protocol, Puppeteer, Playwright, or other automation API. Extensions run inside the profile as a person would use them.
- No download or update from the Chrome Web Store. Extensions are local unpacked directories or `.crx` files you already have.
- No per-window title rewrite on Windows. Identity is the Chromium toolbar name plus the badge extension. The title bar remains the active tab's title.
- Time zone is the `TZ` environment variable. Windows Chromium uses the system time zone and may ignore it. Locale is `--lang` and `--accept-lang`.
- "Least recently used" is least recently launched. There is no idle timer based on input or network.
- No sync between machines and no remote control. `abrowser gui` is a local window on the same data root as the CLI.
- RAM is the sum of each browser's process-tree working set, not a private-bytes accounting of shared Chrome libraries.
- A recycled PID could theoretically confuse a lock until that other process exits. A dead PID is reclaimed.
- Editing `Preferences` deletes `Secure Preferences` only when the display name or clean-exit flag actually changes, so Chromium regenerates the HMAC. Protected settings that lived only in that signed file can be dropped on that one rewrite.
- The single-instance lock is a PID file under the data root, not a cross-process kernel mutex. It is meant for one person on one PC.
