// Package manifest holds the per-profile and per-template JSON documents.
package manifest

import "time"

// Profile is stored at profiles/<id>/manifest.json.
type Profile struct {
	ID               string    `json:"id"`
	Email            string    `json:"email"`
	Template         string    `json:"template"`
	Groups           []string  `json:"groups"`
	AddExtensions    []string  `json:"addExtensions"`
	RemoveExtensions []string  `json:"removeExtensions"`
	Timezone         string    `json:"timezone"`
	Locale           string    `json:"locale"`
	Notes            string    `json:"notes"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// Template is stored at templates/<name>/manifest.json.
type Template struct {
	Name             string     `json:"name"`
	Frozen           bool       `json:"frozen"`
	FrozenAt         *time.Time `json:"frozenAt,omitempty"`
	Groups           []string   `json:"groups"`
	AddExtensions    []string   `json:"addExtensions"`
	RemoveExtensions []string   `json:"removeExtensions"`
	Timezone         string     `json:"timezone"`
	Locale           string     `json:"locale"`
	Notes            string     `json:"notes"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// Normalize replaces nil slices so callers can encode stable JSON.
func (p *Profile) Normalize() {
	p.Groups = fill(p.Groups)
	p.AddExtensions = fill(p.AddExtensions)
	p.RemoveExtensions = fill(p.RemoveExtensions)
}

// Normalize replaces nil slices so callers can encode stable JSON.
func (t *Template) Normalize() {
	t.Groups = fill(t.Groups)
	t.AddExtensions = fill(t.AddExtensions)
	t.RemoveExtensions = fill(t.RemoveExtensions)
}

func fill(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
