package userattributes

import "testing"

func TestDefaults(t *testing.T) {
	defs := Defaults()
	if len(defs) < 7 {
		t.Fatalf("want at least 7 defaults, got %d", len(defs))
	}
	seen := map[string]bool{}
	for _, d := range defs {
		if d.Key == "" || d.Label == "" || d.ValueType == "" {
			t.Fatalf("bad default %+v", d)
		}
		if seen[d.Key] {
			t.Fatalf("duplicate key %q", d.Key)
		}
		seen[d.Key] = true
	}
	if !seen["phone"] || !seen["country"] {
		t.Fatalf("missing expected keys: %v", seen)
	}
	for _, d := range defs {
		if d.Key == "country" && len(d.EnumValues) < 5 {
			t.Fatal("country needs dropdown options")
		}
	}
}
