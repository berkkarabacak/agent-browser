package desktop

import (
	"strings"

	"abrowser/internal/profile"
)

// ProfileLine is one row in the profile list.
type ProfileLine struct {
	ID          string
	DisplayName string
	Email       string
	Template    string
	Groups      []string
	Add         []string
	Remove      []string
	Running     bool
	RSS         uint64
	KnownRAM    bool
}

// Title is the name a person typed, or the profile id when they did not.
func (p ProfileLine) Title() string {
	if strings.TrimSpace(p.DisplayName) != "" {
		return p.DisplayName
	}
	return p.ID
}

// StateLabel is the running-state word used by the list and the filter.
func (p ProfileLine) StateLabel() string {
	if p.Running {
		return "running"
	}
	return "stopped"
}

// RAMLabel is the working-set text. A stopped profile has no sample.
func (p ProfileLine) RAMLabel() string {
	if !p.Running {
		return "—"
	}
	if !p.KnownRAM {
		return "…"
	}
	return profile.FormatRSS(p.RSS)
}

// GroupsLabel joins assigned groups for the row.
func (p ProfileLine) GroupsLabel() string {
	if len(p.Groups) == 0 {
		return "—"
	}
	return strings.Join(p.Groups, ", ")
}

// MetaLine is the second row: template, groups, and extension overrides.
func (p ProfileLine) MetaLine() string {
	template := p.Template
	if template == "" {
		template = "—"
	}
	return template + "   ·   " + p.GroupsLabel() + "   ·   " + overrideBrief(p.Add, p.Remove)
}

func overrideBrief(add, remove []string) string {
	plus := "—"
	if len(add) > 0 {
		plus = strings.Join(add, ", ")
	}
	minus := "—"
	if len(remove) > 0 {
		minus = strings.Join(remove, ", ")
	}
	return "+ " + plus + "    − " + minus
}

func (p ProfileLine) haystack() string {
	parts := []string{
		p.ID,
		p.DisplayName,
		p.Title(),
		p.Email,
		p.Template,
		strings.Join(p.Groups, " "),
		strings.Join(p.Add, " "),
		strings.Join(p.Remove, " "),
		p.StateLabel(),
		p.RAMLabel(),
	}
	return strings.ToLower(strings.Join(parts, "\n"))
}

// FilterProfiles keeps rows whose name, id, email, template, groups, extensions,
// running state, or RAM text contain query. The match is case-insensitive.
// An empty query returns every row.
func FilterProfiles(in []ProfileLine, query string) []ProfileLine {
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]ProfileLine, 0, len(in))
	for _, p := range in {
		if q == "" || strings.Contains(p.haystack(), q) {
			out = append(out, p)
		}
	}
	return out
}

// changedStrings is the group edit diff. add are values wanted but missing.
// remove are values currently assigned that the edit dropped.
func changedStrings(have, want []string) (add, remove []string) {
	inHave := map[string]bool{}
	inWant := map[string]bool{}
	for _, s := range have {
		inHave[s] = true
	}
	for _, s := range want {
		inWant[s] = true
	}
	for _, s := range want {
		if !inHave[s] {
			add = append(add, s)
		}
	}
	for _, s := range have {
		if !inWant[s] {
			remove = append(remove, s)
		}
	}
	return add, remove
}
