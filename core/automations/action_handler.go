package automations

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

const (
	ActionSendEmail = "send_email"
)

// ActionHandler is the generic catalog + validation contract for an action type.
// Execution stays in the service layer (needs DB/mailer), keyed by Type().
type ActionHandler interface {
	Type() string
	Info() ActionInfo
	Validate(params map[string]any) error
}

var (
	actionMu       sync.RWMutex
	actionHandlers = map[string]ActionHandler{}
)

// RegisterActionHandler adds an action type to the catalog (safe for init()).
func RegisterActionHandler(h ActionHandler) {
	if h == nil || strings.TrimSpace(h.Type()) == "" {
		panic("automations: invalid action handler")
	}
	actionMu.Lock()
	defer actionMu.Unlock()
	actionHandlers[h.Type()] = h
}

// LookupActionHandler returns a registered handler by type.
func LookupActionHandler(typ string) (ActionHandler, bool) {
	actionMu.RLock()
	defer actionMu.RUnlock()
	h, ok := actionHandlers[strings.TrimSpace(typ)]
	return h, ok
}

// Actions returns the registered action catalog in a stable order.
func Actions() []ActionInfo {
	actionMu.RLock()
	defer actionMu.RUnlock()
	types := make([]string, 0, len(actionHandlers))
	for typ := range actionHandlers {
		types = append(types, typ)
	}
	sort.Strings(types)
	// Prefer send_email first when present.
	out := make([]ActionInfo, 0, len(types))
	if h, ok := actionHandlers[ActionSendEmail]; ok {
		out = append(out, h.Info())
	}
	for _, typ := range types {
		if typ == ActionSendEmail {
			continue
		}
		out = append(out, actionHandlers[typ].Info())
	}
	return out
}

// KnownAction reports whether typ is registered.
func KnownAction(typ string) bool {
	_, ok := LookupActionHandler(typ)
	return ok
}

// Action is a persisted automation step. Params are handler-defined.
// Legacy top-level keys (e.g. template_key) are lifted into Params on unmarshal.
type Action struct {
	Type   string         `json:"type"`
	Params map[string]any `json:"params,omitempty"`
}

type actionJSON struct {
	Type        string         `json:"type"`
	Params      map[string]any `json:"params,omitempty"`
	TemplateKey string         `json:"template_key,omitempty"` // legacy
}

// UnmarshalJSON accepts {type, params} and legacy {type, template_key}.
func (a *Action) UnmarshalJSON(data []byte) error {
	var raw actionJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	a.Type = strings.TrimSpace(raw.Type)
	a.Params = raw.Params
	if a.Params == nil {
		a.Params = map[string]any{}
	}
	if raw.TemplateKey != "" {
		if _, ok := a.Params["template_key"]; !ok {
			a.Params["template_key"] = strings.TrimSpace(raw.TemplateKey)
		}
	}
	return nil
}

// MarshalJSON always writes type + params (no legacy top-level keys).
func (a Action) MarshalJSON() ([]byte, error) {
	params := a.Params
	if params == nil {
		params = map[string]any{}
	}
	return json.Marshal(actionJSON{
		Type:   a.Type,
		Params: params,
	})
}

// Normalize trims type and ensures Params is non-nil.
func (a Action) Normalize() Action {
	a.Type = strings.TrimSpace(a.Type)
	if a.Params == nil {
		a.Params = map[string]any{}
	}
	return a
}

// ParamString reads a string param.
func (a Action) ParamString(key string) string {
	if a.Params == nil {
		return ""
	}
	v, ok := a.Params[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

// ValidateAction checks type + params via the registered handler.
func ValidateAction(a Action) error {
	a = a.Normalize()
	h, ok := LookupActionHandler(a.Type)
	if !ok {
		return fmt.Errorf("action type %q is not supported", a.Type)
	}
	if err := h.Validate(a.Params); err != nil {
		return err
	}
	return nil
}

// RequireStringParam is a shared helper for handlers.
func RequireStringParam(params map[string]any, key string) (string, error) {
	if params == nil {
		return "", fmt.Errorf("%s is required", key)
	}
	v, ok := params[key]
	if !ok || v == nil {
		return "", fmt.Errorf("%s is required", key)
	}
	s, ok := v.(string)
	if !ok {
		s = fmt.Sprint(v)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return s, nil
}
