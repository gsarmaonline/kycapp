package automations

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gsarmaonline/kyc/core/resources"
)

// Convenience aliases for common lifecycle triggers.
var (
	TriggerAppUserCreated      = resources.LifecycleTrigger(resources.AppUser, resources.LifecycleCreated)
	TriggerAppUserUpdated      = resources.LifecycleTrigger(resources.AppUser, resources.LifecycleUpdated)
	TriggerAppUserDeleted      = resources.LifecycleTrigger(resources.AppUser, resources.LifecycleDeleted)
	TriggerMembershipCreated   = resources.LifecycleTrigger(resources.Membership, resources.LifecycleCreated)
	TriggerMembershipUpdated   = resources.LifecycleTrigger(resources.Membership, resources.LifecycleUpdated)
	TriggerSubscriptionCreated = resources.LifecycleTrigger(resources.Subscription, resources.LifecycleCreated)
	TriggerSubscriptionUpdated = resources.LifecycleTrigger(resources.Subscription, resources.LifecycleUpdated)
)

// Spec is a validated automation definition body.
type Spec struct {
	Trigger       string            `json:"trigger"`
	TriggerParams map[string]string `json:"trigger_params,omitempty"`
	Conditions    Conditions        `json:"conditions"`
	Actions       []Action          `json:"actions"`
}

// ValidateCreate checks trigger, conditions, and actions for create/update
// against the registered catalogs in registry.go.
// triggerParamsJSON may be nil/empty; webhook.received requires inbound_webhook_id.
func ValidateCreate(trigger string, conditionsJSON, actionsJSON json.RawMessage, triggerParamsJSON ...json.RawMessage) (Spec, error) {
	trigger = strings.TrimSpace(trigger)

	var paramsRaw json.RawMessage
	if len(triggerParamsJSON) > 0 {
		paramsRaw = triggerParamsJSON[0]
	}
	params, err := NormalizeTriggerParams(paramsRaw)
	if err != nil {
		return Spec{}, err
	}
	trigger, params = NormalizeScheduleTrigger(trigger, params)

	if !KnownTrigger(trigger) {
		return Spec{}, fmt.Errorf("unknown trigger %q", trigger)
	}
	if err := ValidateTriggerParams(trigger, params); err != nil {
		return Spec{}, err
	}

	var cond Conditions
	if len(conditionsJSON) == 0 {
		cond = Conditions{All: []Condition{}}
	} else if err := json.Unmarshal(conditionsJSON, &cond); err != nil {
		return Spec{}, fmt.Errorf("invalid conditions JSON")
	}
	cond = NormalizeConditionsFields(cond.Normalize())
	if cond.All, err = validateConditionList("conditions.all", cond.All); err != nil {
		return Spec{}, err
	}
	if cond.Any, err = validateConditionList("conditions.any", cond.Any); err != nil {
		return Spec{}, err
	}

	var actions []Action
	if len(actionsJSON) == 0 {
		return Spec{}, fmt.Errorf("actions is required")
	}
	if err := json.Unmarshal(actionsJSON, &actions); err != nil {
		return Spec{}, fmt.Errorf("invalid actions JSON")
	}
	if len(actions) == 0 {
		return Spec{}, fmt.Errorf("at least one action is required")
	}
	for i, a := range actions {
		a = a.Normalize()
		if err := ValidateAction(a); err != nil {
			return Spec{}, fmt.Errorf("actions[%d]: %w", i, err)
		}
		actions[i] = a
	}
	actions = NormalizeActions(actions)
	if err := ValidateActionGraph(actions); err != nil {
		return Spec{}, err
	}
	if err := ValidateSubjectCompatibility(trigger, actions); err != nil {
		return Spec{}, err
	}

	return Spec{Trigger: trigger, TriggerParams: params, Conditions: cond, Actions: actions}, nil
}

// ValidateConditionFields ensures each condition field is in the allowed set
// and that the operator is valid for that field's value type.
func ValidateConditionFields(cond Conditions, fields []ConditionFieldInfo) error {
	byField := make(map[string]ConditionFieldInfo, len(fields))
	for _, f := range fields {
		byField[f.Field] = f
	}
	n := cond.Normalize()
	check := func(prefix string, list []Condition) error {
		for i, c := range list {
			field := NormalizeFieldPath(c.Field)
			info, ok := byField[field]
			if !ok {
				return fmt.Errorf("%s[%d].field %q is not an available condition field", prefix, i, c.Field)
			}
			if len(info.AllowedOps) == 0 {
				continue
			}
			allowed := false
			for _, op := range info.AllowedOps {
				if op == c.Op {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("%s[%d].op %q is not valid for field %q (%s)", prefix, i, c.Op, c.Field, info.ValueType)
			}
		}
		return nil
	}
	if err := check("conditions.all", n.All); err != nil {
		return err
	}
	return check("conditions.any", n.Any)
}
