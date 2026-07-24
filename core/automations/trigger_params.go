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
	ParamStatus           = "status"
	ParamPlanID           = "plan_id"
	ParamRoleID           = "role_id"
)

// Catalog option list keys for TriggerParamInfo.OptionsFrom.
const (
	OptionsInboundWebhooks = "inbound_webhooks"
	OptionsPlans           = "plans"
	OptionsRoles           = "roles"
)

// TriggerWebhookReceived is the shared inbound webhook event id.
var TriggerWebhookReceived = resources.LifecycleTrigger(resources.Webhook, resources.WebhookReceived)

var (
	appUserStatuses = []string{"active", "disabled", "archived"}
	membershipStatuses = []string{"invited", "active", "revoked"}
	subscriptionStatuses = []string{"trialing", "active", "past_due", "canceled"}
)

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

// ParamSchemaForTrigger returns the allowed trigger_params bindings for a trigger.
// attrEnums maps attribute key → enum values (for dropdown attributes).
func ParamSchemaForTrigger(trigger string, attrEnums map[string][]string) []resources.TriggerParamInfo {
	trigger = strings.TrimSpace(trigger)
	parsed, err := resources.ParseTrigger(trigger)
	if err != nil {
		return nil
	}
	switch parsed.Kind {
	case resources.KindWebhook:
		return []resources.TriggerParamInfo{{
			Key:         ParamInboundWebhookID,
			Label:       "Inbound webhook",
			Required:    true,
			Input:       "select",
			OptionsFrom: OptionsInboundWebhooks,
			Hint:        "Only fire when this endpoint receives a POST",
		}}
	case resources.KindAttribute:
		if parsed.Resource != resources.AppUser {
			return nil
		}
		path := AppUserFieldPath(parsed.Event)
		p := resources.TriggerParamInfo{
			Key:      path,
			Label:    "Value",
			Required: false,
			Input:    "text",
			Hint:     "Optional: only fire when the attribute equals this value",
		}
		if enums := attrEnums[parsed.Event]; len(enums) > 0 {
			p.Input = "select"
			p.EnumValues = append([]string(nil), enums...)
		}
		return []resources.TriggerParamInfo{p}
	case resources.KindSchedule:
		return nil
	case resources.KindLifecycle:
		switch parsed.Resource {
		case resources.AppUser:
			return []resources.TriggerParamInfo{{
				Key:        ParamStatus,
				Label:      "Status",
				Required:   false,
				Input:      "select",
				EnumValues: append([]string(nil), appUserStatuses...),
				Hint:       "Optional: only fire for users with this status",
			}}
		case resources.Membership:
			return []resources.TriggerParamInfo{
				{
					Key:         ParamRoleID,
					Label:       "Role",
					Required:    false,
					Input:       "select",
					OptionsFrom: OptionsRoles,
					Hint:        "Optional: only fire for this role",
				},
				{
					Key:        ParamStatus,
					Label:      "Status",
					Required:   false,
					Input:      "select",
					EnumValues: append([]string(nil), membershipStatuses...),
					Hint:       "Optional: only fire for this membership status",
				},
			}
		case resources.Subscription:
			return []resources.TriggerParamInfo{
				{
					Key:         ParamPlanID,
					Label:       "Plan",
					Required:    false,
					Input:       "select",
					OptionsFrom: OptionsPlans,
					Hint:        "Optional: only fire for this plan",
				},
				{
					Key:        ParamStatus,
					Label:      "Status",
					Required:   false,
					Input:      "select",
					EnumValues: append([]string(nil), subscriptionStatuses...),
					Hint:       "Optional: only fire for this subscription status",
				},
			}
		}
	}
	return nil
}

// EnrichTriggersWithParams attaches param schemas to catalog triggers.
func EnrichTriggersWithParams(triggers []resources.TriggerInfo, attrEnums map[string][]string) []resources.TriggerInfo {
	out := make([]resources.TriggerInfo, len(triggers))
	copy(out, triggers)
	for i := range out {
		out[i].Params = ParamSchemaForTrigger(out[i].ID, attrEnums)
	}
	return out
}

// ValidateTriggerParams checks known bindings for the trigger type.
func ValidateTriggerParams(trigger string, params map[string]string) error {
	return ValidateTriggerParamsWithEnums(trigger, params, nil)
}

// ValidateTriggerParamsWithEnums validates params, using attr enums only for schema discovery.
func ValidateTriggerParamsWithEnums(trigger string, params map[string]string, attrEnums map[string][]string) error {
	trigger = strings.TrimSpace(trigger)
	if params == nil {
		params = map[string]string{}
	}
	schema := ParamSchemaForTrigger(trigger, attrEnums)
	allowed := make(map[string]resources.TriggerParamInfo, len(schema))
	for _, p := range schema {
		allowed[p.Key] = p
		if p.Required && strings.TrimSpace(params[p.Key]) == "" {
			return fmt.Errorf("trigger_params.%s is required for %s", p.Key, trigger)
		}
	}
	for k, v := range params {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if _, ok := allowed[k]; !ok {
			return fmt.Errorf("unknown trigger_params key %q for %s", k, trigger)
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
