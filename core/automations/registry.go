package automations

import (
	"github.com/gsarmaonline/kyc/core/resources"
)

// TriggerInfo is the editor-facing trigger descriptor (from core/resources).
type TriggerInfo = resources.TriggerInfo

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
	Requires    []string      `json:"requires"` // subject kinds (e.g. app_user)
}

// ConditionOpInfo describes a condition operator.
type ConditionOpInfo struct {
	Op          string   `json:"op"`
	Label       string   `json:"label"`
	NeedsValue  bool     `json:"needs_value"`
	NeedsList   bool     `json:"needs_list"`
	ValueTypes  []string `json:"value_types"` // which field value_types support this op; empty = all
}

// ConditionFieldInfo is a selectable condition path on the event payload.
type ConditionFieldInfo struct {
	Field      string   `json:"field"`
	Label      string   `json:"label"`
	ValueType  string   `json:"value_type"`
	Group      string   `json:"group"` // "user" | "attributes"
	EnumValues []string `json:"enum_values,omitempty"`
	AllowedOps []string `json:"allowed_ops,omitempty"`
}

var registeredOps = []ConditionOpInfo{
	{Op: OpEq, Label: "equals", NeedsValue: true, ValueTypes: []string{"string", "number", "boolean", "date", "dropdown"}},
	{Op: OpNeq, Label: "not equals", NeedsValue: true, ValueTypes: []string{"string", "number", "boolean", "date", "dropdown"}},
	{Op: OpContains, Label: "contains", NeedsValue: true, ValueTypes: []string{"string"}},
	{Op: OpIn, Label: "is one of", NeedsValue: true, NeedsList: true, ValueTypes: []string{"string", "number", "dropdown"}},
	{Op: OpNotIn, Label: "is not one of", NeedsValue: true, NeedsList: true, ValueTypes: []string{"string", "number", "dropdown"}},
	{Op: OpGt, Label: "greater than", NeedsValue: true, ValueTypes: []string{"number", "date"}},
	{Op: OpGte, Label: "greater or equal", NeedsValue: true, ValueTypes: []string{"number", "date"}},
	{Op: OpLt, Label: "less than", NeedsValue: true, ValueTypes: []string{"number", "date"}},
	{Op: OpLte, Label: "less or equal", NeedsValue: true, ValueTypes: []string{"number", "date"}},
	{Op: OpExists, Label: "exists", NeedsValue: false, ValueTypes: []string{"string", "number", "boolean", "date", "dropdown"}},
	{Op: OpNotExists, Label: "does not exist", NeedsValue: false, ValueTypes: []string{"string", "number", "boolean", "date", "dropdown"}},
}

func opInfo(op string) (ConditionOpInfo, bool) {
	for _, o := range registeredOps {
		if o.Op == op {
			return o, true
		}
	}
	return ConditionOpInfo{}, false
}

// OpsForValueType returns ops allowed for a field value type.
func OpsForValueType(valueType string) []ConditionOpInfo {
	if valueType == "" {
		valueType = "string"
	}
	var out []ConditionOpInfo
	for _, o := range registeredOps {
		if len(o.ValueTypes) == 0 {
			out = append(out, o)
			continue
		}
		for _, vt := range o.ValueTypes {
			if vt == valueType {
				out = append(out, o)
				break
			}
		}
	}
	return out
}

func allowedOpIDs(valueType string) []string {
	ops := OpsForValueType(valueType)
	out := make([]string, 0, len(ops))
	for _, o := range ops {
		out = append(out, o.Op)
	}
	return out
}

// BaseConditionFields are always available on app_user.* payloads.
func BaseConditionFields() []ConditionFieldInfo {
	fields := []ConditionFieldInfo{
		{Field: "id", Label: "User ID", ValueType: "string", Group: "user"},
		{Field: "email", Label: "Email", ValueType: "string", Group: "user"},
		{Field: "display_name", Label: "Display name", ValueType: "string", Group: "user"},
		{Field: "status", Label: "Status", ValueType: "string", Group: "user"},
		{Field: "external_id", Label: "External ID", ValueType: "string", Group: "user"},
	}
	for i := range fields {
		fields[i].AllowedOps = allowedOpIDs(fields[i].ValueType)
	}
	return fields
}

// ExpandTriggers builds lifecycle + attribute triggers via core/resources.
func ExpandTriggers(attrs []resources.AttributeKey) []TriggerInfo {
	return resources.ExpandTriggers(resources.Default(), map[string][]resources.AttributeKey{
		resources.AppUser: attrs,
	})
}

// AllowedTriggerIDs is the set of ExpandTriggers IDs for validation.
func AllowedTriggerIDs(attrs []resources.AttributeKey) map[string]bool {
	return resources.AllowedTriggerIDs(resources.Default(), map[string][]resources.AttributeKey{
		resources.AppUser: attrs,
	})
}

// ConditionOps returns the registered condition operators.
func ConditionOps() []ConditionOpInfo {
	out := make([]ConditionOpInfo, len(registeredOps))
	copy(out, registeredOps)
	return out
}

// KnownTrigger reports whether id matches a registered resource trigger shape.
func KnownTrigger(id string) bool {
	return resources.IsValidTrigger(id)
}

// KnownOp reports whether op is a registered condition operator.
func KnownOp(op string) bool {
	_, ok := opInfo(op)
	return ok
}

// AttributeConditionField builds a condition field path for an org attribute key.
func AttributeConditionField(key, label, valueType string, enumValues []string) ConditionFieldInfo {
	if label == "" {
		label = key
	}
	if valueType == "" {
		valueType = "string"
	}
	return ConditionFieldInfo{
		Field:      "attributes." + key,
		Label:      label,
		ValueType:  valueType,
		Group:      "attributes",
		EnumValues: enumValues,
		AllowedOps: allowedOpIDs(valueType),
	}
}
