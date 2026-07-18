package emailtemplates

import (
	"strings"
	"testing"
)

func TestValidKey(t *testing.T) {
	if !ValidKey("welcome") || !ValidKey("payment_thank_you") {
		t.Fatal("expected valid keys")
	}
	if ValidKey("") || ValidKey("Welcome") || ValidKey("bad-key") {
		t.Fatal("expected invalid keys")
	}
}

func TestValidateCreate(t *testing.T) {
	_, err := ValidateCreate(CreateFields{Key: "x", Name: "X", Subject: "Hi"})
	if err == nil {
		t.Fatal("expected body required")
	}
	out, err := ValidateCreate(CreateFields{
		Key: "  Reminder_1 ", Name: " Reminder ", Subject: " Ping ", BodyText: "Hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Key != "reminder_1" || out.Name != "Reminder" {
		t.Fatalf("got %+v", out)
	}
}

func TestRender(t *testing.T) {
	got := Render("Hi {{display_name}} from {{org_name}}", map[string]string{
		"display_name": "Pat",
		"org_name":     "Acme",
	})
	want := "Hi Pat from Acme"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if Render("Hi {{missing}}", nil) != "Hi {{missing}}" {
		t.Fatal("missing vars should stay")
	}
}

func TestDefaults(t *testing.T) {
	defs := Defaults()
	if len(defs) < 3 {
		t.Fatalf("want at least 3 defaults, got %d", len(defs))
	}
	for _, d := range defs {
		if !ValidKey(d.Key) || d.Subject == "" || d.BodyText == "" {
			t.Fatalf("bad default %+v", d)
		}
	}
}

func TestWrapBranding(t *testing.T) {
	got := Wrap(`<p>Hello</p>`, Branding{
		OrgName: "Acme <Corp>", LogoURL: "https://example.com/l.png",
		PrimaryColor: "#1f4d3a", AccentColor: "#16382a", Footer: "Thanks",
	})
	for _, want := range []string{
		"<!DOCTYPE html>", "Hello", "Acme &lt;Corp&gt;", "https://example.com/l.png",
		"#1f4d3a", "Thanks",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestWrapSkipsFullDocument(t *testing.T) {
	in := "<!DOCTYPE html><html><body>x</body></html>"
	if Wrap(in, Branding{OrgName: "A"}) != in {
		t.Fatal("full documents should not be wrapped")
	}
}

func TestNormalizeColor(t *testing.T) {
	c, err := NormalizeColor(" #ABC ")
	if err != nil || c != "#abc" {
		t.Fatalf("got %q %v", c, err)
	}
	if _, err := NormalizeColor("red"); err == nil {
		t.Fatal("expected error")
	}
}

func TestWrapFont(t *testing.T) {
	got := Wrap(`<p>Hi</p>`, Branding{OrgName: "Acme", Font: "georgia"})
	if !strings.Contains(got, "Georgia") {
		t.Fatalf("expected georgia stack in:\n%s", got)
	}
}

func TestNormalizeFont(t *testing.T) {
	k, err := NormalizeFont(" Georgia ")
	if err != nil || k != "georgia" {
		t.Fatalf("got %q %v", k, err)
	}
	if _, err := NormalizeFont("comic-sans"); err == nil {
		t.Fatal("expected error")
	}
}
