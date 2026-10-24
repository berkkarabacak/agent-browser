package profile

import (
	"fmt"
	"sort"
	"time"
)

// Slot is one browser counted against the concurrent limit.
type Slot struct {
	Kind         string
	ID           string
	PID          int
	LastActivity time.Time
}

// Evict chooses a running profile to close when the concurrent limit is full.
// It returns an empty id when there is spare capacity.
// Templates are never chosen. A profile whose PID is not known yet is not chosen.
func Evict(running []Slot, max int, auto bool) (string, error) {
	if max < 1 {
		return "", fmt.Errorf("max concurrent must be at least 1")
	}
	if len(running) < max {
		return "", nil
	}
	if !auto {
		return "", fmt.Errorf("already at the %d profile limit; close one with \"abrowser close <id>\" or enable auto-close with \"abrowser config set auto-close-lru true\"", max)
	}
	candidates := make([]Slot, 0, len(running))
	for _, slot := range running {
		if slot.Kind == "profile" && slot.PID > 0 && slot.ID != "" {
			candidates = append(candidates, slot)
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("already at the %d profile limit and no running profile can be closed automatically; close a template with \"abrowser template close <name>\"", max)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].LastActivity.Equal(candidates[j].LastActivity) {
			return candidates[i].LastActivity.Before(candidates[j].LastActivity)
		}
		return candidates[i].ID < candidates[j].ID
	})
	return candidates[0].ID, nil
}
