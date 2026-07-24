package automations

import (
	"encoding/json"
	"testing"
)

func TestValidateAndMatch(t *testing.T) {
	spec, err := ValidateCreate(
		"app_user.created",
		json.RawMessage(`{"all":[{"field":"app_user.country","op":"eq","value":"AU"}]}`),
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

func TestValidateCreateWebhookRequiresInbound(t *testing.T) {
	_, err := ValidateCreate(
		TriggerWebhookReceived,
		json.RawMessage(`{"all":[]}`),
		json.RawMessage(`[{"type":"call_webhook","params":{"webhook_id":"w1"}}]`),
	)
	if err == nil {
		t.Fatal("want inbound_webhook_id required")
	}
	spec, err := ValidateCreate(
		TriggerWebhookReceived,
		json.RawMessage(`{"all":[]}`),
		json.RawMessage(`[{"type":"call_webhook","params":{"webhook_id":"w1"}}]`),
		json.RawMessage(`{"inbound_webhook_id":"hook1"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if spec.TriggerParams[ParamInboundWebhookID] != "hook1" {
		t.Fatalf("params=%v", spec.TriggerParams)
	}
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

func TestMatchAnyAndTypedOps(t *testing.T) {
	anySpec, err := ValidateCreate(
		"app_user.created",
		json.RawMessage(`{"mode":"any","items":[{"field":"status","op":"eq","value":"active"},{"field":"status","op":"eq","value":"pending"}]}`),
		json.RawMessage(`[{"type":"send_email","params":{"template_key":"welcome"}}]`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !Match(anySpec.Conditions, map[string]any{"status": "pending"}) {
		t.Fatal("OR should match pending")
	}
	if Match(anySpec.Conditions, map[string]any{"status": "blocked"}) {
		t.Fatal("OR should not match blocked")
	}

	inSpec, err := ValidateCreate(
		"app_user.created",
		json.RawMessage(`{"all":[{"field":"attributes.country","op":"in","value":["AU","NZ"]}]}`),
		json.RawMessage(`[{"type":"send_email","params":{"template_key":"welcome"}}]`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if inSpec.Conditions.All[0].Field != "app_user.country" {
		t.Fatalf("legacy field not normalized: %q", inSpec.Conditions.All[0].Field)
	}
	if !Match(inSpec.Conditions, map[string]any{"attributes": map[string]any{"country": "NZ"}}) {
		t.Fatal("in should match NZ")
	}
	if Match(inSpec.Conditions, map[string]any{"attributes": map[string]any{"country": "US"}}) {
		t.Fatal("in should not match US")
	}

	numPayload := map[string]any{"attributes": map[string]any{"score": 80}}
	gt := Conditions{All: []Condition{{Field: "app_user.score", Op: OpGte, Value: 70}}}
	if !Match(gt, numPayload) {
		t.Fatal("gte should match")
	}
	lt := Conditions{All: []Condition{{Field: "app_user.score", Op: OpLt, Value: 50}}}
	if Match(lt, numPayload) {
		t.Fatal("lt should not match")
	}

	contains := Conditions{All: []Condition{{Field: "app_user.email", Op: OpContains, Value: "@example.com"}}}
	if !Match(contains, map[string]any{"email": "a@example.com"}) {
		t.Fatal("contains should match")
	}
}

func TestValidateConditionFieldsRejectsBadOp(t *testing.T) {
	fields := []ConditionFieldInfo{
		AttributeConditionField("country", "Country", "string", nil),
	}
	err := ValidateConditionFields(
		Conditions{All: []Condition{{Field: "attributes.country", Op: OpGt, Value: "AU"}}},
		fields,
	)
	if err == nil {
		t.Fatal("gt on string field should fail")
	}
	err = ValidateConditionFields(
		Conditions{All: []Condition{{Field: "attributes.country", Op: OpEq, Value: "AU"}}},
		fields,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateWebhookAndDBInsert(t *testing.T) {
	_, err := ValidateCreate(
		"subscription.created",
		json.RawMessage(`{"all":[]}`),
		json.RawMessage(`[{"type":"call_webhook","params":{"webhook_id":"wh1"}}]`),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ValidateCreate(
		"app_user.created",
		json.RawMessage(`{"all":[]}`),
		json.RawMessage(`[{"type":"call_webhook","params":{}}]`),
	)
	if err == nil {
		t.Fatal("want webhook_id required")
	}
	_, err = ValidateCreate(
		"app_user.created",
		json.RawMessage(`{"all":[]}`),
		json.RawMessage(`[{"type":"db_insert","params":{"database_id":"db1","table":"kyc_events"}}]`),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ValidateCreate(
		"app_user.created",
		json.RawMessage(`{"all":[]}`),
		json.RawMessage(`[{"type":"db_insert","params":{"database_id":"db1","table":"kyc;drop"}}]`),
	)
	if err == nil {
		t.Fatal("want invalid table")
	}
	_, err = ValidateCreate(
		"app_user.created",
		json.RawMessage(`{"all":[]}`),
		json.RawMessage(`[{"type":"db_insert","params":{"database_id":"db1","table":"events","mapping":{"email":"app_user.email"}}}]`),
	)
	if err != nil {
		t.Fatal(err)
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
