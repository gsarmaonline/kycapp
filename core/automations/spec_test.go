package automations

import (
	"encoding/json"
	"testing"
)

func TestValidateAndMatch(t *testing.T) {
	spec, err := ValidateCreate(
		"app_user.created",
		json.RawMessage(`{"all":[{"field":"attributes.country","op":"eq","value":"AU"}]}`),
		json.RawMessage(`[{"type":"send_email","template_key":"welcome"}]`),
	)
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"status":     "active",
		"attributes": map[string]any{"country": "AU"},
	}
	if !Match(spec.Conditions, payload) {
		t.Fatal("expected match")
	}
	payload["attributes"].(map[string]any)["country"] = "NZ"
	if Match(spec.Conditions, payload) {
		t.Fatal("expected no match")
	}
}

func TestValidateRejectsBadTrigger(t *testing.T) {
	_, err := ValidateCreate("nope", json.RawMessage(`{"all":[]}`), json.RawMessage(`[{"type":"send_email","template_key":"welcome"}]`))
	if err == nil {
		t.Fatal("want error")
	}
}
