package automations

import "testing"

func TestValidateTriggerParamsWebhook(t *testing.T) {
	err := ValidateTriggerParams(TriggerWebhookReceived, nil)
	if err == nil {
		t.Fatal("want inbound_webhook_id required")
	}
	err = ValidateTriggerParams(TriggerWebhookReceived, map[string]string{
		ParamInboundWebhookID: "hook1",
	})
	if err != nil {
		t.Fatal(err)
	}
	err = ValidateTriggerParams(TriggerWebhookReceived, map[string]string{
		ParamInboundWebhookID: "hook1",
		"extra":                "x",
	})
	if err == nil {
		t.Fatal("want unknown key error")
	}
}

func TestValidateTriggerParamsScoped(t *testing.T) {
	err := ValidateTriggerParams(TriggerAppUserCreated, map[string]string{
		ParamStatus: "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	err = ValidateTriggerParams(TriggerAppUserCreated, map[string]string{
		ParamInboundWebhookID: "hook1",
	})
	if err == nil {
		t.Fatal("app_user.created should reject inbound_webhook_id")
	}
	err = ValidateTriggerParams("app_user.attribute.country", map[string]string{
		"app_user.country": "AU",
	})
	if err != nil {
		t.Fatal(err)
	}
	err = ValidateTriggerParams("subscription.created", map[string]string{
		ParamPlanID: "plan1",
		ParamStatus: "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	err = ValidateTriggerParams("membership.updated", map[string]string{
		ParamRoleID: "role1",
	})
	if err != nil {
		t.Fatal(err)
	}
	err = ValidateTriggerParams("schedule.daily", map[string]string{
		ParamStatus: "active",
	})
	if err == nil {
		t.Fatal("schedule should reject params")
	}
}

func TestMatchTriggerParams(t *testing.T) {
	payload := map[string]any{"inbound_webhook_id": "hook1", "body": map[string]any{}}
	if !MatchTriggerParams(nil, payload) {
		t.Fatal("empty params should match")
	}
	if !MatchTriggerParams(map[string]string{ParamInboundWebhookID: "hook1"}, payload) {
		t.Fatal("expected match")
	}
	if MatchTriggerParams(map[string]string{ParamInboundWebhookID: "other"}, payload) {
		t.Fatal("expected no match")
	}

	attrPayload := map[string]any{
		"status":     "active",
		"attributes": map[string]any{"country": "AU"},
	}
	if !MatchTriggerParams(map[string]string{"app_user.country": "AU"}, attrPayload) {
		t.Fatal("expected attribute value match")
	}
	if MatchTriggerParams(map[string]string{"app_user.country": "NZ"}, attrPayload) {
		t.Fatal("expected attribute miss")
	}
	if !MatchTriggerParams(map[string]string{ParamStatus: "active", ParamPlanID: "p1"}, map[string]any{
		"status": "active", "plan_id": "p1",
	}) {
		t.Fatal("expected subscription scope match")
	}
}

func TestParamSchemaForAttributeUsesEnums(t *testing.T) {
	schema := ParamSchemaForTrigger("app_user.attribute.country", map[string][]string{
		"country": {"AU", "NZ"},
	})
	if len(schema) != 1 || schema[0].Input != "select" || len(schema[0].EnumValues) != 2 {
		t.Fatalf("schema=%#v", schema)
	}
}
