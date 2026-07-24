package automations

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gsarmaonline/kyc/core/resources"
)

// Trigger param keys (bound like action params).
const (
	ParamInboundWebhookID = "inbound_webhook_id"
)

// TriggerWebhookReceived is the shared inbound webhook event id.
var TriggerWebhookReceived = resources.LifecycleTrigger(resources.Webhook, resources.WebhookReceived)

// NormalizeTriggerParams parses JSON into a string map (empty → {}).
func NormalizeTriggerParams(raw json.RawMessage) (map[string]string, error) {
	if len(raw) == 0 {
		return map[string]string{}, nil
	}
	var anyMap map[string]any
	if err := json.Unmarshal(raw, &anyMap); err != nil {
		return nil, fmt.Errorf("invalid trigger_params JSON")
	}
	out := make(map[string]string, len(anyMap))
	for k, v := range anyMap {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		out[k] = strings.TrimSpace(fmt.Sprint(v))
	}
	return out, nil
}

// MarshalTriggerParams encodes params (nil → {}).
func MarshalTriggerParams(params map[string]string) (json.RawMessage, error) {
	if params == nil {
		params = map[string]string{}
	}
	clean := make(map[string]string, len(params))
	for k, v := range params {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		clean[k] = v
	}
	return json.Marshal(clean)
}

// ValidateTriggerParams checks known bindings for the trigger type.
func ValidateTriggerParams(trigger string, params map[string]string) error {
	trigger = strings.TrimSpace(trigger)
	if params == nil {
		params = map[string]string{}
	}
	switch trigger {
	case TriggerWebhookReceived:
		id := strings.TrimSpace(params[ParamInboundWebhookID])
		if id == "" {
			return fmt.Errorf("trigger_params.inbound_webhook_id is required for %s", trigger)
		}
		for k := range params {
			if k != ParamInboundWebhookID {
				return fmt.Errorf("unknown trigger_params key %q for %s", k, trigger)
			}
		}
	default:
		for k, v := range params {
			if strings.TrimSpace(v) != "" {
				return fmt.Errorf("trigger %q does not accept trigger_params (%s)", trigger, k)
			}
		}
	}
	return nil
}

// MatchTriggerParams reports whether the event payload satisfies trigger bindings.
// Empty / missing values are ignored (legacy automations with {} still match).
func MatchTriggerParams(params map[string]string, payload map[string]any) bool {
	if len(params) == 0 {
		return true
	}
	for k, want := range params {
		want = strings.TrimSpace(want)
		if want == "" {
			continue
		}
		got, ok := Lookup(payload, k)
		if !ok {
			return false
		}
		if strings.TrimSpace(fmt.Sprint(got)) != want {
			return false
		}
	}
	return true
}
