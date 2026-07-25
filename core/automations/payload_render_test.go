package automations

import (
	"encoding/json"
	"testing"
)

func TestBuildWebhookBodyDefaultDump(t *testing.T) {
	raw, err := BuildWebhookBody("", "org1", map[string]any{"email": "a@b.c", "trigger": "app_user.created"})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["organisation_id"] != "org1" {
		t.Fatalf("org=%v", m["organisation_id"])
	}
	payload, ok := m["payload"].(map[string]any)
	if !ok || payload["email"] != "a@b.c" {
		t.Fatalf("payload=%v", m["payload"])
	}
}

func TestRenderJSONTemplateTypedAndNested(t *testing.T) {
	tmpl := `{
		"org": "{{organisation_id}}",
		"user": "{{app_user.email}}",
		"country": "{{app_user.country}}",
		"note": "hello {{app_user.email}}"
	}`
	raw, err := BuildWebhookBody(tmpl, "org1", map[string]any{
		"email":      "a@b.c",
		"attributes": map[string]any{"country": "AU"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["org"] != "org1" || m["user"] != "a@b.c" || m["country"] != "AU" {
		t.Fatalf("got %#v", m)
	}
	if m["note"] != "hello a@b.c" {
		t.Fatalf("note=%v", m["note"])
	}
}

func TestValidateJSONTemplate(t *testing.T) {
	if err := ValidateJSONTemplate(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSONTemplate(`{"a":1}`); err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSONTemplate(`{`); err == nil {
		t.Fatal("want error")
	}
}

func TestRenderStringTemplate(t *testing.T) {
	data := map[string]any{
		"organisation_id": "org1",
		"organisation":    map[string]any{"id": "org1", "name": "Acme"},
		"app_user": map[string]any{
			"display_name": "Pat",
			"email":        "pat@example.com",
			"attributes":   map[string]any{"country": "AU"},
		},
	}
	got := RenderStringTemplate(
		"Hi {{app_user.display_name}} ({{app_user.email}}) at {{organisation.name}} / {{app_user.country}}",
		data,
	)
	want := "Hi Pat (pat@example.com) at Acme / AU"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if RenderStringTemplate("{{org_name}}", data) != "Acme" {
		t.Fatal("org_name alias")
	}
}
