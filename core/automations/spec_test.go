package automations

import (
	"encoding/json"
	"testing"
)

func TestValidateAndMatch(t *testing.T) {
	spec, err := ValidateCreate(
		"app_user.created",
		json.RawMessage(`{"all":[{"field":"attributes.country","op":"eq","value":"AU"}]}`),
		json.RawMessage(`[{"type":"send_email","params":{"template_key":"welcome"}}]`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Actions[0].ParamString("template_key") != "welcome" {
		t.Fatalf("params=%v", spec.Actions[0].Params)
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

func TestActionLegacyTemplateKey(t *testing.T) {
	spec, err := ValidateCreate(
		"app_user.created",
		json.RawMessage(`{"all":[]}`),
		json.RawMessage(`[{"type":"send_email","template_key":"welcome"}]`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Actions[0].ParamString("template_key") != "welcome" {
		t.Fatalf("legacy lift failed: %#v", spec.Actions[0])
	}
	raw, err := MarshalActions(spec.Actions)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(raw) || !containsParams(raw) {
		t.Fatalf("marshal should use params: %s", raw)
	}
}

func containsParams(raw json.RawMessage) bool {
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return false
	}
	if len(items) == 0 {
		return false
	}
	_, ok := items[0]["params"]
	_, legacy := items[0]["template_key"]
	return ok && !legacy
}

func TestValidateRejectsBadTrigger(t *testing.T) {
	_, err := ValidateCreate("nope", json.RawMessage(`{"all":[]}`), json.RawMessage(`[{"type":"send_email","params":{"template_key":"welcome"}}]`))
	if err == nil {
		t.Fatal("want error")
	}
}

func TestValidateRejectsUnknownAction(t *testing.T) {
	_, err := ValidateCreate(
		"app_user.created",
		json.RawMessage(`{"all":[]}`),
		json.RawMessage(`[{"type":"explode","params":{}}]`),
	)
	if err == nil {
		t.Fatal("want error")
	}
}

func TestValidateSubjectCompatibility(t *testing.T) {
	_, err := ValidateCreate(
		"subscription.created",
		json.RawMessage(`{"all":[]}`),
		json.RawMessage(`[{"type":"send_email","params":{"template_key":"welcome"}}]`),
	)
	if err == nil {
		t.Fatal("send_email on subscription should fail — no app_user subject")
	}
	_, err = ValidateCreate(
		"membership.created",
		json.RawMessage(`{"all":[]}`),
		json.RawMessage(`[{"type":"send_email","params":{"template_key":"welcome"}}]`),
	)
	if err == nil {
		t.Fatal("send_email on membership should fail — provides user, not app_user")
	}
	_, err = ValidateCreate(
		"app_user.created",
		json.RawMessage(`{"all":[]}`),
		json.RawMessage(`[{"type":"send_email","params":{"template_key":"welcome"}}]`),
	)
	if err != nil {
		t.Fatal(err)
	}
}
