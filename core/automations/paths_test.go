package automations

import "testing"

func TestNormalizeFieldPath(t *testing.T) {
	cases := map[string]string{
		"email":               "app_user.email",
		"attributes.country":  "app_user.country",
		"app_user.email":      "app_user.email",
		"app_user.country":    "app_user.country",
		"organisation_id":     "organisation_id",
		"attributes":          "app_user.attributes",
	}
	for in, want := range cases {
		if got := NormalizeFieldPath(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestLookupAppUserPaths(t *testing.T) {
	payload := map[string]any{
		"email":      "a@b.c",
		"status":     "active",
		"attributes": map[string]any{"country": "AU"},
	}
	cases := map[string]any{
		"app_user.email":     "a@b.c",
		"app_user.country":   "AU",
		"email":              "a@b.c",
		"attributes.country": "AU",
	}
	for path, want := range cases {
		got, ok := Lookup(payload, path)
		if !ok || got != want {
			t.Fatalf("%q: got %v ok=%v want %v", path, got, ok, want)
		}
	}
}
