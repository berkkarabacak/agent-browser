package profile

import (
	"testing"
	"time"
)

func TestEvictUnderLimit(t *testing.T) {
	id, err := Evict([]Slot{{Kind: "profile", ID: "a", PID: 1}}, 2, false)
	if err != nil || id != "" {
		t.Fatalf("id %q err %v", id, err)
	}
}

func TestEvictRequiresAuto(t *testing.T) {
	_, err := Evict([]Slot{
		{Kind: "profile", ID: "a", PID: 1},
		{Kind: "profile", ID: "b", PID: 2},
	}, 2, false)
	if err == nil {
		t.Fatal("expected limit error")
	}
}

func TestEvictOldestProfileSkipsTemplates(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	id, err := Evict([]Slot{
		{Kind: "template", ID: "golden", PID: 9, LastActivity: older},
		{Kind: "profile", ID: "b", PID: 2, LastActivity: newer},
		{Kind: "profile", ID: "a", PID: 1, LastActivity: older},
		{Kind: "profile", ID: "starting", PID: 0, LastActivity: older.Add(-time.Hour)},
	}, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if id != "a" {
		t.Fatalf("evict %s", id)
	}
}

func TestEvictTieBreaksByID(t *testing.T) {
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	id, err := Evict([]Slot{
		{Kind: "profile", ID: "m", PID: 1, LastActivity: when},
		{Kind: "profile", ID: "a", PID: 2, LastActivity: when},
	}, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if id != "a" {
		t.Fatalf("evict %s", id)
	}
}

func TestEvictOnlyTemplates(t *testing.T) {
	_, err := Evict([]Slot{{Kind: "template", ID: "golden", PID: 3, LastActivity: time.Now()}}, 1, true)
	if err == nil {
		t.Fatal("expected error")
	}
}
