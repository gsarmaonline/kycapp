package billing

import "testing"

func TestEffectiveKeys(t *testing.T) {
	got := EffectiveKeys(
		[]string{"api_access", "sso"},
		map[string]string{"sso": "deny", "extra": "grant"},
	)
	want := []string{"api_access", "extra"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	if !Has(got, "api_access") || Has(got, "sso") {
		t.Fatal("Has mismatch")
	}
}
