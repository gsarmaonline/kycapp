package automations

import "fmt"

// TriggerInfo describes a trigger the editor and validators know about.
type TriggerInfo struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// ActionParam describes a config field on an action.
type ActionParam struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
}

// ActionInfo describes an action type available to merchants.
type ActionInfo struct {
	Type        string        `json:"type"`
	Label       string        `json:"label"`
	Description string        `json:"description"`
	Params      []ActionParam `json:"params"`
}

// ConditionOpInfo describes a condition operator.
type ConditionOpInfo struct {
	Op         string `json:"op"`
	Label      string `json:"label"`
	NeedsValue bool   `json:"needs_value"`
}

// ConditionFieldInfo is a selectable condition path on the event payload.
type ConditionFieldInfo struct {
	Field     string `json:"field"`
	Label     string `json:"label"`
	ValueType string `json:"value_type"`
	Group     string `json:"group"` // "user" | "attributes"
}

var registeredTriggers = []TriggerInfo{
	{
		ID:          TriggerAppUserCreated,
		Label:       "App user created",
		Description: "Fires after an app user is created (including ingest insert).",
	},
	{
		ID:          TriggerAppUserUpdated,
		Label:       "App user updated",
		Description: "Fires after an app user is updated (including ingest upsert).",
	},
}

var registeredActions = []ActionInfo{
	{
		Type:        ActionSendEmail,
		Label:       "Send email",
		Description: "Render an org email template and deliver it to the app user's email.",
		Params: []ActionParam{
			{Key: "template_key", Label: "Template key", Required: true},
		},
	},
}

var registeredOps = []ConditionOpInfo{
	{Op: OpEq, Label: "equals", NeedsValue: true},
	{Op: OpNeq, Label: "not equals", NeedsValue: true},
	{Op: OpExists, Label: "exists", NeedsValue: false},
	{Op: OpNotExists, Label: "does not exist", NeedsValue: false},
}

// BaseConditionFields are always available on app_user.* payloads.
func BaseConditionFields() []ConditionFieldInfo {
	return []ConditionFieldInfo{
		{Field: "id", Label: "User ID", ValueType: "string", Group: "user"},
		{Field: "email", Label: "Email", ValueType: "string", Group: "user"},
		{Field: "display_name", Label: "Display name", ValueType: "string", Group: "user"},
		{Field: "status", Label: "Status", ValueType: "string", Group: "user"},
		{Field: "external_id", Label: "External ID", ValueType: "string", Group: "user"},
	}
}

// Triggers returns the registered trigger catalog.
func Triggers() []TriggerInfo {
	out := make([]TriggerInfo, len(registeredTriggers))
	copy(out, registeredTriggers)
	return out
}

// Actions returns the registered action catalog.
func Actions() []ActionInfo {
	out := make([]ActionInfo, len(registeredActions))
	copy(out, registeredActions)
	return out
}

// ConditionOps returns the registered condition operators.
func ConditionOps() []ConditionOpInfo {
	out := make([]ConditionOpInfo, len(registeredOps))
	copy(out, registeredOps)
	return out
}

// KnownTrigger reports whether id is a registered trigger.
func KnownTrigger(id string) bool {
	for _, t := range registeredTriggers {
		if t.ID == id {
			return true
		}
	}
	return false
}

// KnownAction reports whether typ is a registered action type.
func KnownAction(typ string) bool {
	for _, a := range registeredActions {
		if a.Type == typ {
			return true
		}
	}
	return false
}

// KnownOp reports whether op is a registered condition operator.
func KnownOp(op string) bool {
	for _, o := range registeredOps {
		if o.Op == op {
			return true
		}
	}
	return false
}

// ValidateAction checks action type and required params against the registry.
func ValidateAction(a Action) error {
	var info *ActionInfo
	for i := range registeredActions {
		if registeredActions[i].Type == a.Type {
			info = &registeredActions[i]
			break
		}
	}
	if info == nil {
		return fmt.Errorf("action type %q is not supported", a.Type)
	}
	for _, p := range info.Params {
		if !p.Required {
			continue
		}
		switch p.Key {
		case "template_key":
			if a.TemplateKey == "" {
				return fmt.Errorf("template_key is required for %s", a.Type)
			}
		}
	}
	return nil
}

// AttributeConditionField builds a condition field path for an org attribute key.
func AttributeConditionField(key, label, valueType string) ConditionFieldInfo {
	if label == "" {
		label = key
	}
	if valueType == "" {
		valueType = "string"
	}
	return ConditionFieldInfo{
		Field:     "attributes." + key,
		Label:     label,
		ValueType: valueType,
		Group:     "attributes",
	}
}
