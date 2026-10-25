package names

import "testing"

func TestSlugID(t *testing.T) {
	cases := map[string]string{
		"Mom":         "mom",
		"  Mom  ":     "mom",
		"Mary Jane":   "mary-jane",
		"Mary  Jane":  "mary-jane",
		"John O'Neil": "john-o-neil",
		"Mom!":        "mom",
		"agent-001":   "agent-001",
		"123":         "123",
	}
	for in, want := range cases {
		got, err := SlugID(in)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
	for _, bad := range []string{"", "   ", "...", "___", "李"} {
		if _, err := SlugID(bad); err == nil {
			t.Fatalf("%q should not become an id", bad)
		}
	}
	long := ""
	for i := 0; i < 80; i++ {
		long += "a"
	}
	got, err := SlugID(long)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > 64 || len(got) == 0 {
		t.Fatalf("long slug %q", got)
	}
}

func TestUniqueID(t *testing.T) {
	taken := map[string]bool{"mom": true, "mom-2": true}
	got, err := UniqueID("mom", func(id string) bool { return taken[id] })
	if err != nil {
		t.Fatal(err)
	}
	if got != "mom-3" {
		t.Fatalf("got %q", got)
	}
	got, err = UniqueID("ada", func(string) bool { return false })
	if err != nil || got != "ada" {
		t.Fatalf("got %q %v", got, err)
	}
}
