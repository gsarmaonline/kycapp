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
	err = ValidateTriggerParams(TriggerAppUserCreated, map[string]string{
		ParamInboundWebhookID: "hook1",
	})
	if err == nil {
		t.Fatal("app_user.created should reject trigger_params")
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
}
