package names

import "testing"

func TestValidID(t *testing.T) {
	ok := []string{"golden", "agent-001", "A1", "sales.east", "a_b"}
	bad := []string{"", " ", "../x", "agent/1", ".hidden", "con", "CON.txt", "nul", "ends.", "has space"}
	for _, s := range ok {
		if err := ValidID(s); err != nil {
			t.Errorf("ValidID(%q) unexpected error: %v", s, err)
		}
	}
	for _, s := range bad {
		if err := ValidID(s); err == nil {
			t.Errorf("ValidID(%q) expected error", s)
		}
	}
}

func TestParseExtRef(t *testing.T) {
	name, ver, pinned, err := ParseExtRef("hello")
	if err != nil || name != "hello" || ver != "" || pinned {
		t.Fatalf("bare ref: %s %s %v %v", name, ver, pinned, err)
	}
	name, ver, pinned, err = ParseExtRef("hello@1.2.3")
	if err != nil || name != "hello" || ver != "1.2.3" || !pinned {
		t.Fatalf("pinned ref: %s %s %v %v", name, ver, pinned, err)
	}
	if _, _, _, err := ParseExtRef("Hello"); err == nil {
		t.Fatal("expected lowercase name")
	}
	if _, _, _, err := ParseExtRef("hello@"); err == nil {
		t.Fatal("expected version error")
	}
}

func TestValidEmailAndZone(t *testing.T) {
	if err := ValidEmail("ada@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := ValidEmail("not-an-email"); err == nil {
		t.Fatal("expected email error")
	}
	if err := ValidTimezone("America/New_York"); err != nil {
		t.Fatal(err)
	}
	if err := ValidTimezone(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidTimezone("Not/AZone"); err == nil {
		t.Fatal("expected zone error")
	}
	if err := ValidLocale("en-US"); err != nil {
		t.Fatal(err)
	}
}
