package cli

const rootHelp = `Agent Browser (abrowser) runs isolated Chromium profiles for AI agents.

Usage:
  abrowser [--root PATH] [--json] <command> [arguments]

Data root:
  %LOCALAPPDATA%\AgentBrowser on Windows.
  ~/.local/share/AgentBrowser elsewhere, or $XDG_DATA_HOME/AgentBrowser.
  Override with --root or the ABROWSER_ROOT environment variable.

Commands:
  init                          Create the data root
  config show|set               Browser path, concurrent limit, extra Chromium flags
  ext add|list|remove           Shared unpacked extension store
  group create|add|remove|list  Named extension groups
  template ...                  Create, configure, and freeze a golden profile
  clone                         Copy a frozen template into a new profile
  list                          List templates and profiles
  launch <id>                   Start a profile in its own user-data directory
  close <id>                    Stop a profile
  delete <id>                   Delete a profile and its user data
  status                        Running browsers and approximate RAM
  profile show|set|group|ext    Inspect or edit a profile
  version                       Print the version

Run "abrowser <command> --help" for command details.
Stock Google Chrome ignores --load-extension. Point "browser" at Chromium
(ungoogled-chromium or Chrome for Testing). See the README.

`

const configHelp = `Usage:
  abrowser config show [--json]
  abrowser config set browser <path>
  abrowser config set max-concurrent <n>
  abrowser config set auto-close-lru <true|false>
  abrowser config set extra-args [chromium flags...]

browser
  Path to a Chromium executable that still honors --load-extension.
  Quote paths that contain spaces. The value may use environment variables.

max-concurrent
  How many browsers may run at once. Default 20. Range 1–500.

auto-close-lru
  When a launch would pass the limit, close the least recently launched
  profile first. Templates are never closed automatically. "Least recently
  used" means least recently launched; this milestone does not see tab activity.

extra-args
  Extra Chromium arguments appended after Agent Browser's own flags.
  With no arguments, the list is cleared.
  Example: abrowser config set extra-args --disable-renderer-backgrounding

`

const extHelp = `Usage:
  abrowser ext list [--json]
  abrowser ext add <name> (--from <unpacked-dir> | --crx <file>) [--version <v>]
  abrowser ext remove <name[@version]> [--yes]

Extensions are copied into <root>/extensions/<name>@<version>/.
<name> is lowercase. The version comes from manifest.json unless --version
matches it. .crx files may be CRX2 or CRX3.

"ext remove" deletes one version from the store. It refuses while a group
or a +ext / -ext override still references that extension.

`

const groupHelp = `Usage:
  abrowser group list [--json]
  abrowser group create <name>
  abrowser group add <group> <extension>
  abrowser group remove <group> <extension>

Group names are lowercase (common, sales). Adding an extension pins the
current version, or the version in name@version. Adding the same name again
replaces the pinned version. A bare name on remove drops every version of
that extension from the group.

Profiles and templates store group names, not a private copy of the member
list. Changing a group changes what those profiles load the next time they launch.

`

const templateHelp = `Usage:
  abrowser template create <name> [--timezone TZ] [--locale LOC] [--notes TEXT] [--group G]...
  abrowser template launch <name> [--dry-run]
  abrowser template close <name>
  abrowser template freeze <name>
  abrowser template unfreeze <name>
  abrowser template list [--json]
  abrowser template delete <name> [--yes]
  abrowser template set <name> [--timezone TZ] [--locale LOC] [--notes TEXT]
  abrowser template group add <name> <group>
  abrowser template group remove <name> <group>
  abrowser template ext add <name> <extension>
  abrowser template ext remove <name> <extension>
  abrowser template ext reset <name> <extension>

A template is a golden Chromium user-data directory. Launch it, configure
bookmarks, extension options, and startup pages, close it, then freeze it.
clone copies that directory and skips caches. A frozen template cannot be
launched until you unfreeze it. Clones already made are independent copies.

`

const cloneHelp = `Usage:
  abrowser clone <template> --as <id> --email <email> [options]

Options:
  --timezone TZ     IANA time zone. Default: the template's zone.
  --locale LOC      Browser locale, for example en-US. Default: the template's locale.
  --notes TEXT      Replaces the template notes when set.
  --group G         Repeatable. When set, replaces the template's groups.
  --add-ext EXT     Repeatable. When set, replaces the template's +ext overrides.
  --remove-ext EXT  Repeatable. When set, replaces the template's -ext overrides.

The template must be frozen and not running. Caches, crash dumps, and
Chromium singleton locks are not copied. The new profile gets its own
display name.

Example:
  abrowser clone golden --as agent-001 --email ada@example.com --group common --group sales

`

const launchHelp = `Usage:
  abrowser launch <id> [--dry-run] [--json]
  abrowser close <id>
  abrowser delete <id> [--yes]
  abrowser list [--json]
  abrowser status [--json]

launch starts Chromium with that profile's user-data directory, locale, and
the resolved extension set (--load-extension and --disable-extensions-except).
The same profile cannot be launched twice. A lock file records the PID; a
dead PID is treated as a stale lock.

--dry-run prints the browser path, arguments, and TZ/LANGUAGE overrides.

close stops that profile's process tree. delete removes the profile directory
and asks you to type the profile id, unless --yes is set.

status prints running browsers and the sum of each process tree's resident
memory. Shared libraries are counted once per process, so the total is approximate.

`

const profileHelp = `Usage:
  abrowser profile show <id> [--json]
  abrowser profile set <id> [--email E] [--timezone TZ] [--locale LOC] [--notes TEXT]
  abrowser profile group add <id> <group>
  abrowser profile group remove <id> <group>
  abrowser profile ext add <id> <extension>
  abrowser profile ext remove <id> <extension>
  abrowser profile ext reset <id> <extension>

profile show prints the manifest and the extension list resolved from groups,
then +ext, then -ext. A removal wins over a group and over an add.
The toolbar badge extension is added at launch and is not part of that list.

"profile ext remove" records a -ext override. It does not delete the extension
from the shared store. "profile ext reset" clears both overrides for that extension.

`
