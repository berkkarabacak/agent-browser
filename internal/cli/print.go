package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"abrowser/internal/app"
	"abrowser/internal/extensions"
	"abrowser/internal/profile"
)

func writeTable(w io.Writer, headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i := 0; i < len(headers) && i < len(row); i++ {
			if len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
	}
	line := func(row []string) {
		for i := 0; i < len(headers); i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			if i > 0 {
				fmt.Fprint(w, "  ")
			}
			if i == len(headers)-1 {
				fmt.Fprint(w, cell)
				continue
			}
			fmt.Fprintf(w, "%-*s", widths[i], cell)
		}
		fmt.Fprintln(w)
	}
	line(headers)
	for _, row := range rows {
		line(row)
	}
}

func joinOrDash(items []string) string {
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, ",")
}

func zoneOrHost(z string) string {
	if z == "" {
		return "(host)"
	}
	return z
}

func formatAge(at, now time.Time) string {
	if at.IsZero() {
		return "-"
	}
	d := now.Sub(at)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd", int(d.Hours())/24)
	}
}

func (c *CLI) printConfig(view app.ConfigView, asJSON bool) int {
	if asJSON {
		return c.printJSON(view)
	}
	browser := view.BrowserPath
	if browser == "" {
		browser = "(not set)"
	}
	exists := "no"
	if view.BrowserExists {
		exists = "yes"
	}
	fmt.Fprintf(c.IO.Out, "root:             %s\n", view.Root)
	fmt.Fprintf(c.IO.Out, "browser:          %s\n", browser)
	if view.BrowserExpanded != "" && view.BrowserExpanded != view.BrowserPath {
		fmt.Fprintf(c.IO.Out, "browser expanded: %s\n", view.BrowserExpanded)
	}
	fmt.Fprintf(c.IO.Out, "browser exists:   %s\n", exists)
	fmt.Fprintf(c.IO.Out, "max-concurrent:   %d\n", view.MaxConcurrent)
	fmt.Fprintf(c.IO.Out, "auto-close-lru:   %t\n", view.AutoCloseLRU)
	if len(view.ExtraArgs) == 0 {
		fmt.Fprintf(c.IO.Out, "extra-args:       (none)\n")
	} else {
		fmt.Fprintf(c.IO.Out, "extra-args:       %s\n", strings.Join(view.ExtraArgs, " "))
	}
	return 0
}

func (c *CLI) printExtensions(list []extensions.Extension, asJSON bool) int {
	if asJSON {
		return c.printJSON(list)
	}
	if len(list) == 0 {
		fmt.Fprintln(c.IO.Out, "No extensions in the store.")
		fmt.Fprintln(c.IO.Out, "Add one with: abrowser ext add <name> --from <unpacked-dir>")
		return 0
	}
	rows := make([][]string, 0, len(list))
	for _, ext := range list {
		rows = append(rows, []string{ext.Name, ext.Version, ext.ManifestName, ext.Path})
	}
	writeTable(c.IO.Out, []string{"NAME", "VERSION", "MANIFEST NAME", "PATH"}, rows)
	return 0
}

func (c *CLI) printGroups(g extensions.GroupsFile, asJSON bool) int {
	if asJSON {
		return c.printJSON(g)
	}
	if len(g) == 0 {
		fmt.Fprintln(c.IO.Out, "No extension groups.")
		fmt.Fprintln(c.IO.Out, "Create one with: abrowser group create <name>")
		return 0
	}
	names := make([]string, 0, len(g))
	for name := range g {
		names = append(names, name)
	}
	sortStrings(names)
	rows := make([][]string, 0, len(names))
	for _, name := range names {
		rows = append(rows, []string{name, joinOrDash(g[name])})
	}
	writeTable(c.IO.Out, []string{"GROUP", "EXTENSIONS"}, rows)
	return 0
}

func (c *CLI) printInventory(inv app.Inventory, asJSON bool, what string) int {
	if asJSON {
		if what == "templates" {
			return c.printJSON(inv.Templates)
		}
		return c.printJSON(inv)
	}
	if len(inv.Templates) == 0 {
		fmt.Fprintln(c.IO.Out, "No templates yet. Create one with: abrowser template create <name>")
	} else {
		fmt.Fprintln(c.IO.Out, "TEMPLATES")
		rows := make([][]string, 0, len(inv.Templates))
		for _, t := range inv.Templates {
			frozen := "no"
			if t.Frozen {
				frozen = "yes"
			}
			state := "stopped"
			if t.Running {
				state = "running"
			}
			rows = append(rows, []string{t.Name, frozen, t.Locale, zoneOrHost(t.Timezone), joinOrDash(t.Groups), state})
		}
		writeTable(c.IO.Out, []string{"NAME", "FROZEN", "LOCALE", "TIMEZONE", "GROUPS", "STATE"}, rows)
	}
	if what == "templates" {
		return 0
	}
	fmt.Fprintln(c.IO.Out)
	if len(inv.Profiles) == 0 {
		fmt.Fprintln(c.IO.Out, "No profiles yet. Clone a frozen template with: abrowser clone <template> --as <id> --email <email>")
		return 0
	}
	fmt.Fprintln(c.IO.Out, "PROFILES")
	rows := make([][]string, 0, len(inv.Profiles))
	for _, p := range inv.Profiles {
		state := "stopped"
		if p.Running {
			state = "running"
		}
		rows = append(rows, []string{p.ID, p.Email, p.Template, p.Locale, zoneOrHost(p.Timezone), joinOrDash(p.Groups), state})
	}
	writeTable(c.IO.Out, []string{"ID", "EMAIL", "TEMPLATE", "LOCALE", "TIMEZONE", "GROUPS", "STATE"}, rows)
	return 0
}

func (c *CLI) printProfile(view app.ProfileView, asJSON bool) int {
	if asJSON {
		return c.printJSON(view)
	}
	p := view.Profile
	state := "stopped"
	if view.Running {
		state = fmt.Sprintf("running (pid %d)", view.PID)
	}
	notes := p.Notes
	if notes == "" {
		notes = "(none)"
	}
	fmt.Fprintf(c.IO.Out, "Profile %s\n", p.ID)
	fmt.Fprintf(c.IO.Out, "  email:      %s\n", p.Email)
	fmt.Fprintf(c.IO.Out, "  template:   %s\n", p.Template)
	fmt.Fprintf(c.IO.Out, "  timezone:   %s\n", zoneOrHost(p.Timezone))
	fmt.Fprintf(c.IO.Out, "  locale:     %s\n", emptyDash(p.Locale))
	fmt.Fprintf(c.IO.Out, "  notes:      %s\n", notes)
	fmt.Fprintf(c.IO.Out, "  state:      %s\n", state)
	fmt.Fprintf(c.IO.Out, "  user data:  %s\n", view.UserDataDir)
	fmt.Fprintln(c.IO.Out)
	fmt.Fprintf(c.IO.Out, "Groups: %s\n", joinOrDash(p.Groups))
	fmt.Fprintf(c.IO.Out, "Overrides:\n")
	fmt.Fprintf(c.IO.Out, "  add:    %s\n", joinOrDash(p.AddExtensions))
	fmt.Fprintf(c.IO.Out, "  remove: %s\n", joinOrDash(p.RemoveExtensions))
	fmt.Fprintln(c.IO.Out)
	if len(view.Resolved) == 0 {
		fmt.Fprintln(c.IO.Out, "Resolved extensions: (none)")
	} else {
		fmt.Fprintln(c.IO.Out, "Resolved extensions:")
		for _, ext := range view.Resolved {
			fmt.Fprintf(c.IO.Out, "  %s\n    %s\n", ext.Ref(), ext.Path)
		}
	}
	fmt.Fprintf(c.IO.Out, "\nLaunch also loads a profile badge extension (toolbar badge %s).\n", profile.BadgeText(p.ID))
	return 0
}

func (c *CLI) printLaunch(res app.LaunchResult, asJSON bool) int {
	if asJSON {
		return c.printJSON(res)
	}
	if res.DryRun {
		fmt.Fprintf(c.IO.Out, "Dry run for %s %s\n", res.Kind, res.ID)
		fmt.Fprintln(c.IO.Out, "browser:")
		if res.Browser == "" {
			fmt.Fprintln(c.IO.Out, "  (not set)")
		} else {
			fmt.Fprintf(c.IO.Out, "  %s\n", res.Browser)
		}
		fmt.Fprintln(c.IO.Out, "args:")
		for _, arg := range res.Args {
			fmt.Fprintf(c.IO.Out, "  %s\n", arg)
		}
		if len(res.Env) == 0 {
			fmt.Fprintln(c.IO.Out, "env:")
			fmt.Fprintln(c.IO.Out, "  (host environment)")
		} else {
			fmt.Fprintln(c.IO.Out, "env:")
			for _, e := range res.Env {
				fmt.Fprintf(c.IO.Out, "  %s\n", e)
			}
		}
		return 0
	}
	for _, id := range res.Closed {
		fmt.Fprintf(c.IO.Out, "Closed least recently launched profile %s to stay within the concurrent limit.\n", id)
	}
	fmt.Fprintf(c.IO.Out, "Launched %s %s\n", res.Kind, res.ID)
	fmt.Fprintf(c.IO.Out, "  pid:        %d\n", res.PID)
	fmt.Fprintf(c.IO.Out, "  browser:    %s\n", res.Browser)
	fmt.Fprintf(c.IO.Out, "  extensions: %s\n", strings.Join(res.Extensions, ", "))
	fmt.Fprintf(c.IO.Out, "  log:        %s\n", res.LogPath)
	return 0
}

func (c *CLI) printStatus(st app.Status, asJSON bool) int {
	if asJSON {
		return c.printJSON(st)
	}
	auto := "off"
	if st.AutoCloseLRU {
		auto = "on"
	}
	if len(st.Running) == 0 {
		fmt.Fprintf(c.IO.Out, "No browsers are running (0 / %d). auto-close-lru %s.\n", st.MaxConcurrent, auto)
		return 0
	}
	fmt.Fprintf(c.IO.Out, "Running %d / %d    auto-close-lru %s    approx RAM %s\n\n",
		len(st.Running), st.MaxConcurrent, auto, profile.FormatRSS(st.TotalRSS))
	now := time.Now()
	rows := make([][]string, 0, len(st.Running))
	for _, e := range st.Running {
		email := e.Email
		if email == "" {
			email = "-"
		}
		rows = append(rows, []string{
			e.Kind,
			e.ID,
			email,
			fmt.Sprintf("%d", e.PID),
			profile.FormatRSS(e.RSS),
			formatAge(e.StartedAt, now),
		})
	}
	writeTable(c.IO.Out, []string{"KIND", "ID", "EMAIL", "PID", "RSS", "SINCE"}, rows)
	return 0
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		j := i
		for j > 0 && s[j] < s[j-1] {
			s[j], s[j-1] = s[j-1], s[j]
			j--
		}
	}
}
