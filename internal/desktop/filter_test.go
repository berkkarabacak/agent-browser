package desktop

import "testing"

func TestFilterProfiles(t *testing.T) {
	rows := []ProfileLine{
		{ID: "agent-001", Email: "ada@example.com", Template: "golden", Groups: []string{"common", "sales"}, Add: []string{"note-001@1.0.0"}, Running: true, RSS: 80 << 20, KnownRAM: true},
		{ID: "agent-002", Email: "grace@example.com", Template: "golden", Groups: []string{"common"}, Running: false},
	}
	if got := FilterProfiles(rows, "  "); len(got) != 2 {
		t.Fatalf("empty query: %d rows", len(got))
	}
	if got := FilterProfiles(rows, "ADA@"); len(got) != 1 || got[0].ID != "agent-001" {
		t.Fatalf("email filter: %+v", got)
	}
	if got := FilterProfiles(rows, "sales"); len(got) != 1 || got[0].ID != "agent-001" {
		t.Fatalf("group filter: %+v", got)
	}
	if got := FilterProfiles(rows, "note-001"); len(got) != 1 {
		t.Fatalf("extension filter: %+v", got)
	}
	if got := FilterProfiles(rows, "stopped"); len(got) != 1 || got[0].ID != "agent-002" {
		t.Fatalf("state filter: %+v", got)
	}
	if got := FilterProfiles(rows, "80.0 MB"); len(got) != 1 || got[0].ID != "agent-001" {
		t.Fatalf("ram filter: %+v (label %q)", got, rows[0].RAMLabel())
	}
	if got := FilterProfiles(rows, "missing"); len(got) != 0 {
		t.Fatalf("miss: %+v", got)
	}
	if rows[0].MetaLine() == "" || rows[1].RAMLabel() != "—" {
		t.Fatalf("labels: %q %q", rows[0].MetaLine(), rows[1].RAMLabel())
	}
}

func TestChangedStrings(t *testing.T) {
	add, remove := changedStrings([]string{"common", "sales"}, []string{"common", "notes"})
	if len(add) != 1 || add[0] != "notes" || len(remove) != 1 || remove[0] != "sales" {
		t.Fatalf("add %v remove %v", add, remove)
	}
	add, remove = changedStrings(nil, nil)
	if add != nil || remove != nil {
		t.Fatalf("empty diff: %v %v", add, remove)
	}
}
