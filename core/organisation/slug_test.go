package organisation

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Acme Pty Ltd", "acme-pty-ltd"},
		{"  Hello---World  ", "hello-world"},
		{"!!!", "org"},
		{"ACME", "acme"},
	}
	for _, tt := range tests {
		if got := Slugify(tt.in); got != tt.want {
			t.Fatalf("Slugify(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
