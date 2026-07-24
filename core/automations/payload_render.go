package automations

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var placeholderRE = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

// ExampleWebhookBodyTemplate is a starter JSON shape shown in the UI.
// Placeholders use the shared app_user.* field vocabulary.
const ExampleWebhookBodyTemplate = `{
  "organisation_id": "{{organisation_id}}",
  "trigger": "{{trigger}}",
  "id": "{{app_user.id}}",
  "email": "{{app_user.email}}",
  "country": "{{app_user.country}}"
}`

// BuildWebhookBody renders a webhook JSON body.
// Empty template keeps the legacy dump: { organisation_id, payload }.
func BuildWebhookBody(template, orgID string, payload map[string]any) (json.RawMessage, error) {
	data := make(map[string]any, len(payload)+1)
	for k, v := range payload {
		data[k] = v
	}
	data["organisation_id"] = orgID

	template = strings.TrimSpace(template)
	if template == "" {
		return json.Marshal(map[string]any{
			"organisation_id": orgID,
			"payload":         payload,
		})
	}
	return RenderJSONTemplate(template, data)
}

// RenderJSONTemplate walks a JSON template and replaces {{path}} placeholders
// from data. A string that is exactly "{{path}}" is replaced with the typed
// value; otherwise placeholders inside strings are stringified.
// Path "payload" inserts the data map.
func RenderJSONTemplate(template string, data map[string]any) (json.RawMessage, error) {
	template = strings.TrimSpace(template)
	var root any
	if err := json.Unmarshal([]byte(template), &root); err != nil {
		return nil, fmt.Errorf("body_template must be valid JSON: %w", err)
	}
	out, err := renderValue(root, data)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func renderValue(v any, data map[string]any) (any, error) {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, child := range t {
			n, err := renderValue(child, data)
			if err != nil {
				return nil, err
			}
			out[k] = n
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, child := range t {
			n, err := renderValue(child, data)
			if err != nil {
				return nil, err
			}
			out[i] = n
		}
		return out, nil
	case string:
		return renderString(t, data), nil
	default:
		return t, nil
	}
}

func renderString(s string, data map[string]any) any {
	trimmed := strings.TrimSpace(s)
	if m := placeholderRE.FindStringSubmatch(trimmed); len(m) == 2 && m[0] == trimmed {
		return lookupPathOrNil(data, strings.TrimSpace(m[1]))
	}
	return placeholderRE.ReplaceAllStringFunc(s, func(match string) string {
		sub := placeholderRE.FindStringSubmatch(match)
		if len(sub) != 2 {
			return match
		}
		val := lookupPathOrNil(data, strings.TrimSpace(sub[1]))
		return stringifyTemplateValue(val)
	})
}

func lookupPathOrNil(data map[string]any, path string) any {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	val, ok := Lookup(data, path)
	if !ok {
		return nil
	}
	return val
}

func stringifyTemplateValue(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	case float64, float32, int, int32, int64, bool:
		return fmt.Sprint(t)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		return string(b)
	}
}

// ValidateJSONTemplate ensures template is empty or valid JSON.
func ValidateJSONTemplate(template string) error {
	template = strings.TrimSpace(template)
	if template == "" {
		return nil
	}
	var raw any
	if err := json.Unmarshal([]byte(template), &raw); err != nil {
		return fmt.Errorf("body_template must be valid JSON")
	}
	return nil
}
