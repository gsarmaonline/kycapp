package automations

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	TriggerAppUserCreated = "app_user.created"
	TriggerAppUserUpdated = "app_user.updated"

	OpEq        = "eq"
	OpNeq       = "neq"
	OpExists    = "exists"
	OpNotExists = "not_exists"

	ActionSendEmail = "send_email"
)

// Spec is a validated automation definition body.
type Spec struct {
	Trigger    string     `json:"trigger"`
	Conditions Conditions `json:"conditions"`
	Actions    []Action   `json:"actions"`
}

type Conditions struct {
	All []Condition `json:"all"`
}

type Condition struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value,omitempty"`
}

type Action struct {
	Type        string `json:"type"`
	TemplateKey string `json:"template_key,omitempty"`
}

var allowedTriggers = map[string]bool{
	TriggerAppUserCreated: true,
	TriggerAppUserUpdated: true,
}

var allowedOps = map[string]bool{
	OpEq: true, OpNeq: true, OpExists: true, OpNotExists: true,
}

// ValidateCreate checks trigger, conditions, and actions for create/update.
func ValidateCreate(trigger string, conditionsJSON, actionsJSON json.RawMessage) (Spec, error) {
	trigger = strings.TrimSpace(trigger)
	if !allowedTriggers[trigger] {
		return Spec{}, fmt.Errorf("trigger must be app_user.created or app_user.updated")
	}

	var cond Conditions
	if len(conditionsJSON) == 0 {
		cond = Conditions{All: []Condition{}}
	} else if err := json.Unmarshal(conditionsJSON, &cond); err != nil {
		return Spec{}, fmt.Errorf("invalid conditions JSON")
	}
	if cond.All == nil {
		cond.All = []Condition{}
	}
	for i, c := range cond.All {
		c.Field = strings.TrimSpace(c.Field)
		c.Op = strings.TrimSpace(c.Op)
		if c.Field == "" {
			return Spec{}, fmt.Errorf("conditions.all[%d].field is required", i)
		}
		if !allowedOps[c.Op] {
			return Spec{}, fmt.Errorf("conditions.all[%d].op must be eq, neq, exists, or not_exists", i)
		}
		if (c.Op == OpEq || c.Op == OpNeq) && c.Value == nil {
			return Spec{}, fmt.Errorf("conditions.all[%d].value is required for %s", i, c.Op)
		}
		cond.All[i] = c
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
		a.Type = strings.TrimSpace(a.Type)
		a.TemplateKey = strings.TrimSpace(a.TemplateKey)
		switch a.Type {
		case ActionSendEmail:
			if a.TemplateKey == "" {
				return Spec{}, fmt.Errorf("actions[%d].template_key is required for send_email", i)
			}
		default:
			return Spec{}, fmt.Errorf("actions[%d].type %q is not supported", i, a.Type)
		}
		actions[i] = a
	}

	return Spec{Trigger: trigger, Conditions: cond, Actions: actions}, nil
}

// Match reports whether all conditions pass against payload.
func Match(cond Conditions, payload map[string]any) bool {
	for _, c := range cond.All {
		if !matchOne(c, payload) {
			return false
		}
	}
	return true
}

func matchOne(c Condition, payload map[string]any) bool {
	val, ok := lookup(payload, c.Field)
	switch c.Op {
	case OpExists:
		return ok && val != nil && val != ""
	case OpNotExists:
		return !ok || val == nil || val == ""
	case OpEq:
		return ok && stringify(val) == stringify(c.Value)
	case OpNeq:
		return !ok || stringify(val) != stringify(c.Value)
	default:
		return false
	}
}

func lookup(payload map[string]any, field string) (any, bool) {
	parts := strings.Split(field, ".")
	var cur any = payload
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := m[p]
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

func stringify(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	case float64:
		// JSON numbers decode as float64
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%v", t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return strings.Trim(string(b), `"`)
	}
}

// MarshalConditions / MarshalActions for persistence.
func MarshalConditions(c Conditions) (json.RawMessage, error) {
	if c.All == nil {
		c.All = []Condition{}
	}
	return json.Marshal(c)
}

func MarshalActions(a []Action) (json.RawMessage, error) {
	if a == nil {
		a = []Action{}
	}
	return json.Marshal(a)
}
