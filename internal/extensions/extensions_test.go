package extensions

import (
	"testing"
)

func ext(name, version string) Extension {
	return Extension{Name: name, Version: version, Path: "/ext/" + name + "@" + version}
}

func TestCompareVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.2", "1.2.0", 0},
		{"1.9.0", "1.10.0", -1},
		{"1.10.0", "1.9.0", 1},
		{"2.0.0", "1.9.9", 1},
		{"1.0.0", "1.0.1", -1},
	}
	for _, tc := range cases {
		if got := CompareVersion(tc.a, tc.b); got != tc.want {
			t.Errorf("CompareVersion(%s, %s) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestResolveOrderAddRemoveAndLatest(t *testing.T) {
	catalog := []Extension{
		ext("alpha", "1.0.0"),
		ext("alpha", "1.10.0"),
		ext("alpha", "1.9.0"),
		ext("beta", "2.0.0"),
		ext("gamma", "1.0.0"),
		ext("delta", "3.1.0"),
	}
	groups := map[string][]string{
		"common": {"alpha@1.0.0", "beta@2.0.0"},
		"sales":  {"beta", "gamma@1.0.0"},
	}
	got, err := Resolve(catalog, groups,
		[]string{"common", "sales"},
		[]string{"delta", "beta@2.0.0"},
		[]string{"alpha@1.0.0"},
	)
	if err != nil {
		t.Fatal(err)
	}
	var refs []string
	for _, e := range got {
		refs = append(refs, e.Ref())
	}
	want := []string{"beta@2.0.0", "gamma@1.0.0", "delta@3.1.0"}
	if len(refs) != len(want) {
		t.Fatalf("got %v want %v", refs, want)
	}
	for i := range want {
		if refs[i] != want[i] {
			t.Fatalf("got %v want %v", refs, want)
		}
	}
}

func TestResolveBareNamePicksLatest(t *testing.T) {
	catalog := []Extension{ext("alpha", "1.9.0"), ext("alpha", "1.10.0"), ext("alpha", "1.2.0")}
	got, err := Resolve(catalog, map[string][]string{"g": {"alpha"}}, []string{"g"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Ref() != "alpha@1.10.0" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveRemoveWinsOverAdd(t *testing.T) {
	catalog := []Extension{ext("alpha", "1.0.0"), ext("beta", "1.0.0")}
	got, err := Resolve(catalog, nil, nil, []string{"alpha", "beta"}, []string{"alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "beta" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveRemoveUnknown(t *testing.T) {
	catalog := []Extension{ext("alpha", "1.0.0")}
	if _, err := Resolve(catalog, nil, nil, nil, []string{"missing"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveRemoveKnownButUnselected(t *testing.T) {
	catalog := []Extension{ext("alpha", "1.0.0"), ext("beta", "1.0.0")}
	got, err := Resolve(catalog, map[string][]string{"g": {"alpha"}}, []string{"g"}, nil, []string{"beta"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "alpha" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveErrors(t *testing.T) {
	catalog := []Extension{ext("alpha", "1.0.0")}
	if _, err := Resolve(catalog, map[string][]string{}, []string{"missing"}, nil, nil); err == nil {
		t.Fatal("expected missing group")
	}
	groups := map[string][]string{"g": {"nope@1.0.0"}}
	if _, err := Resolve(catalog, groups, []string{"g"}, nil, nil); err == nil {
		t.Fatal("expected missing extension")
	}
	if _, err := Resolve(catalog, nil, nil, []string{"alpha@9.9.9"}, nil); err == nil {
		t.Fatal("expected missing add")
	}
}

func TestResolveEmpty(t *testing.T) {
	got, err := Resolve(nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}
