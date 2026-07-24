package automations

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	OpEq        = "eq"
	OpNeq       = "neq"
	OpExists    = "exists"
	OpNotExists = "not_exists"
	OpIn        = "in"
	OpNotIn     = "not_in"
	OpGt        = "gt"
	OpGte       = "gte"
	OpLt        = "lt"
	OpLte       = "lte"
	OpContains  = "contains"
)

const (
	ConditionModeAll = "all"
	ConditionModeAny = "any"
)

// Conditions is a rule group. Prefer Mode+Items for new rules.
// Legacy: only All populated (AND). Any alone means OR.
// If both All and Any are set (legacy combined form), Match requires
// (all of All) AND (at least one of Any).
type Conditions struct {
	Mode  string      `json:"mode,omitempty"` // all | any — when set with Items
	Items []Condition `json:"items,omitempty"`
	All   []Condition `json:"all,omitempty"`
	Any   []Condition `json:"any,omitempty"`
}

// Normalize flattens Mode/Items into All or Any for matching/persistence clarity.
func (c Conditions) Normalize() Conditions {
	if len(c.Items) > 0 {
		mode := strings.TrimSpace(c.Mode)
		if mode == "" {
			mode = ConditionModeAll
		}
		out := Conditions{Mode: mode}
		switch mode {
		case ConditionModeAny:
			out.Any = append([]Condition(nil), c.Items...)
		default:
			out.Mode = ConditionModeAll
			out.All = append([]Condition(nil), c.Items...)
		}
		return out
	}
	if c.All == nil {
		c.All = []Condition{}
	}
	if c.Any == nil {
		c.Any = []Condition{}
	}
	if len(c.Any) > 0 && len(c.All) == 0 {
		c.Mode = ConditionModeAny
	} else if c.Mode == "" {
		c.Mode = ConditionModeAll
	}
	return c
}

// Flat returns the active condition list for editors (All or Any, not both).
func (c Conditions) Flat() (mode string, items []Condition) {
	n := c.Normalize()
	if len(n.Any) > 0 && len(n.All) == 0 {
		return ConditionModeAny, n.Any
	}
	if len(n.All) > 0 && len(n.Any) == 0 {
		return ConditionModeAll, n.All
	}
	// Combined or empty — prefer all for editor flat view
	if len(n.All) > 0 {
		return ConditionModeAll, n.All
	}
	if len(n.Any) > 0 {
		return ConditionModeAny, n.Any
	}
	return ConditionModeAll, []Condition{}
}

type Condition struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value,omitempty"`
}

func validateConditionList(prefix string, list []Condition) ([]Condition, error) {
	out := make([]Condition, len(list))
	for i, c := range list {
		c.Field = strings.TrimSpace(c.Field)
		c.Op = strings.TrimSpace(c.Op)
		if c.Field == "" {
			return nil, fmt.Errorf("%s[%d].field is required", prefix, i)
		}
		info, ok := opInfo(c.Op)
		if !ok {
			return nil, fmt.Errorf("%s[%d].op %q is not supported", prefix, i, c.Op)
		}
		if info.NeedsValue && c.Value == nil {
			return nil, fmt.Errorf("%s[%d].value is required for %s", prefix, i, c.Op)
		}
		if info.NeedsList {
			listVals := valueAsStringList(c.Value)
			if len(listVals) == 0 {
				return nil, fmt.Errorf("%s[%d].value must be a non-empty list for %s", prefix, i, c.Op)
			}
		}
		out[i] = c
	}
	return out, nil
}

// Match reports whether conditions pass against payload.
func Match(cond Conditions, payload map[string]any) bool {
	n := cond.Normalize()
	// Mode+Items style already expanded into All/Any by Normalize.
	allOK := true
	if len(n.All) > 0 {
		for _, c := range n.All {
			if !matchOne(c, payload) {
				allOK = false
				break
			}
		}
	}
	anyOK := true
	if len(n.Any) > 0 {
		anyOK = false
		for _, c := range n.Any {
			if matchOne(c, payload) {
				anyOK = true
				break
			}
		}
	}
	return allOK && anyOK
}

func matchOne(c Condition, payload map[string]any) bool {
	val, ok := lookup(payload, c.Field)
	switch c.Op {
	case OpExists:
		return ok && !isEmpty(val)
	case OpNotExists:
		return !ok || isEmpty(val)
	case OpEq:
		return ok && valuesEqual(val, c.Value)
	case OpNeq:
		return !ok || !valuesEqual(val, c.Value)
	case OpContains:
		if !ok {
			return false
		}
		return strings.Contains(strings.ToLower(stringify(val)), strings.ToLower(stringify(c.Value)))
	case OpIn:
		if !ok {
			return false
		}
		return valueInList(val, c.Value)
	case OpNotIn:
		if !ok {
			return true
		}
		return !valueInList(val, c.Value)
	case OpGt, OpGte, OpLt, OpLte:
		if !ok {
			return false
		}
		return compareOrdered(val, c.Value, c.Op)
	default:
		return false
	}
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return s == ""
	}
	return false
}

func valuesEqual(a, b any) bool {
	if an, ok := asNumber(a); ok {
		if bn, ok := asNumber(b); ok {
			return an == bn
		}
	}
	if at, ok := asTime(a); ok {
		if bt, ok := asTime(b); ok {
			return at.Equal(bt)
		}
	}
	if ab, ok := a.(bool); ok {
		if bb, ok := b.(bool); ok {
			return ab == bb
		}
		bs := strings.ToLower(stringify(b))
		return (ab && (bs == "true" || bs == "1")) || (!ab && (bs == "false" || bs == "0"))
	}
	return stringify(a) == stringify(b)
}

func valueInList(val, list any) bool {
	for _, item := range valueAsStringList(list) {
		if valuesEqual(val, item) {
			return true
		}
	}
	return false
}

func valueAsStringList(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			s := strings.TrimSpace(stringify(x))
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(t))
		for _, x := range t {
			s := strings.TrimSpace(x)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		s := strings.TrimSpace(stringify(v))
		if s == "" {
			return nil
		}
		parts := strings.Split(s, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	}
}

func compareOrdered(left, right any, op string) bool {
	if ln, ok := asNumber(left); ok {
		if rn, ok := asNumber(right); ok {
			switch op {
			case OpGt:
				return ln > rn
			case OpGte:
				return ln >= rn
			case OpLt:
				return ln < rn
			case OpLte:
				return ln <= rn
			}
		}
	}
	if lt, ok := asTime(left); ok {
		if rt, ok := asTime(right); ok {
			switch op {
			case OpGt:
				return lt.After(rt)
			case OpGte:
				return lt.After(rt) || lt.Equal(rt)
			case OpLt:
				return lt.Before(rt)
			case OpLte:
				return lt.Before(rt) || lt.Equal(rt)
			}
		}
	}
	// Fallback lexicographic compare on strings
	ls, rs := stringify(left), stringify(right)
	switch op {
	case OpGt:
		return ls > rs
	case OpGte:
		return ls >= rs
	case OpLt:
		return ls < rs
	case OpLte:
		return ls <= rs
	}
	return false
}

func asNumber(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func asTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case string:
		s := strings.TrimSpace(t)
		for _, layout := range []string{time.RFC3339, "2006-01-02", time.RFC3339Nano} {
			if parsed, err := time.Parse(layout, s); err == nil {
				return parsed, true
			}
		}
		return time.Time{}, false
	default:
		return time.Time{}, false
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

// MarshalConditions persists a normalized conditions object (all and/or any).
func MarshalConditions(c Conditions) (json.RawMessage, error) {
	n := c.Normalize()
	// Persist in compact form: mode+items when single group, else all/any.
	if len(n.All) > 0 && len(n.Any) == 0 {
		return json.Marshal(Conditions{Mode: ConditionModeAll, Items: n.All, All: n.All})
	}
	if len(n.Any) > 0 && len(n.All) == 0 {
		return json.Marshal(Conditions{Mode: ConditionModeAny, Items: n.Any, Any: n.Any})
	}
	if n.All == nil {
		n.All = []Condition{}
	}
	if n.Any == nil {
		n.Any = []Condition{}
	}
	return json.Marshal(n)
}

// MarshalActions for persistence.
func MarshalActions(a []Action) (json.RawMessage, error) {
	if a == nil {
		a = []Action{}
	}
	return json.Marshal(a)
}
