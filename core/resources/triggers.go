package resources

import (
	"fmt"
	"strings"
)

// TriggerKind classifies a parsed trigger ID.
type TriggerKind string

const (
	KindLifecycle TriggerKind = "lifecycle"
	KindAttribute TriggerKind = "attribute"
	KindSchedule  TriggerKind = "schedule"
	KindWebhook   TriggerKind = "webhook"
)

// Trigger is a parsed, validated trigger ID.
type Trigger struct {
	ID       string
	Resource string
	Kind     TriggerKind
	Event    string // lifecycle name, or attribute key when KindAttribute
	Label    string
}

// TriggerInfo is the editor-facing descriptor (JSON-friendly).
type TriggerInfo struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Resource    string   `json:"resource"`
	Kind        string   `json:"kind"` // lifecycle | attribute | schedule
	Provides    []string `json:"provides"`
}

// LifecycleTrigger builds {resource}.{event}.
func LifecycleTrigger(resource, event string) string {
	return strings.TrimSpace(resource) + "." + strings.TrimSpace(event)
}

// AttributeTrigger builds {resource}.attribute.{key}.
func AttributeTrigger(resource, key string) string {
	return strings.TrimSpace(resource) + "." + AttributeSegment + "." + strings.TrimSpace(key)
}

// ParseTrigger parses a trigger ID against the default resource catalog.
func ParseTrigger(id string) (Trigger, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Trigger{}, fmt.Errorf("trigger is required")
	}
	parts := strings.Split(id, ".")
	if len(parts) < 2 {
		return Trigger{}, fmt.Errorf("invalid trigger %q", id)
	}
	res, ok := ByKey(parts[0])
	if !ok {
		return Trigger{}, fmt.Errorf("unknown resource %q in trigger", parts[0])
	}

	// {resource}.attribute.{key}
	if len(parts) >= 3 && parts[1] == AttributeSegment {
		if !res.SupportsAttributes {
			return Trigger{}, fmt.Errorf("resource %q does not support attribute triggers", res.Key)
		}
		key := strings.Join(parts[2:], ".")
		if key == "" {
			return Trigger{}, fmt.Errorf("attribute key is required in trigger %q", id)
		}
		return Trigger{
			ID:       id,
			Resource: res.Key,
			Kind:     KindAttribute,
			Event:    key,
			Label:    res.Label + " attribute · " + key,
		}, nil
	}

	// {resource}.{lifecycle} — schedule.* uses KindSchedule
	if len(parts) != 2 {
		return Trigger{}, fmt.Errorf("invalid trigger %q", id)
	}
	event := parts[1]
	if !res.HasLifecycle(event) {
		return Trigger{}, fmt.Errorf("resource %q does not support lifecycle %q", res.Key, event)
	}
	kind := KindLifecycle
	label := res.Label + " " + event
	if res.Key == Schedule {
		kind = KindSchedule
		label = "Schedule · " + event
	}
	if res.Key == Webhook {
		kind = KindWebhook
		label = "Webhook · " + event
	}
	return Trigger{
		ID:       id,
		Resource: res.Key,
		Kind:     kind,
		Event:    event,
		Label:    label,
	}, nil
}

// IsValidTrigger reports whether id matches a known resource + pattern.
// Attribute keys are not checked against an org schema here — use ExpandTriggers
// / AllowedTriggerIDs for that.
func IsValidTrigger(id string) bool {
	_, err := ParseTrigger(id)
	return err == nil
}

// ExpandTriggers builds the full trigger catalog for resources, attaching
// attribute triggers from attrsByResource (keyed by resource key).
func ExpandTriggers(resources []Resource, attrsByResource map[string][]AttributeKey) []TriggerInfo {
	var out []TriggerInfo
	for _, r := range resources {
		provides := r.availableSubjectKinds()
		for _, event := range r.Lifecycles {
			id := LifecycleTrigger(r.Key, event)
			kind := string(KindLifecycle)
			label := r.Label + " " + event
			desc := fmt.Sprintf("Fires when a %s is %s.", strings.ToLower(r.Label), event)
			if r.Key == Schedule {
				kind = string(KindSchedule)
				label = "Schedule · " + event
				desc = fmt.Sprintf("Fires on an organisation schedule (%s, UTC). Subject is the org — not an app user.", event)
			}
			if r.Key == Webhook {
				kind = string(KindWebhook)
				label = "Webhook · " + event
				desc = "Fires when a POST hits a connected inbound webhook. Bind trigger_params.inbound_webhook_id to a specific endpoint. Subject is the org — not an app user."
			}
			out = append(out, TriggerInfo{
				ID:          id,
				Label:       label,
				Description: desc,
				Resource:    r.Key,
				Kind:        kind,
				Provides:    append([]string(nil), provides...),
			})
		}
		if !r.SupportsAttributes {
			continue
		}
		for _, attr := range attrsByResource[r.Key] {
			key := strings.TrimSpace(attr.Key)
			if key == "" {
				continue
			}
			label := strings.TrimSpace(attr.Label)
			if label == "" {
				label = key
			}
			id := AttributeTrigger(r.Key, key)
			out = append(out, TriggerInfo{
				ID:          id,
				Label:       r.Label + " attribute · " + label,
				Description: fmt.Sprintf("Fires when %s attribute %q is set or changed.", strings.ToLower(r.Label), key),
				Resource:    r.Key,
				Kind:        string(KindAttribute),
				Provides:    append([]string(nil), provides...),
			})
		}
	}
	return out
}

// AllowedTriggerIDs returns the set of expanded trigger IDs.
func AllowedTriggerIDs(resources []Resource, attrsByResource map[string][]AttributeKey) map[string]bool {
	out := map[string]bool{}
	for _, t := range ExpandTriggers(resources, attrsByResource) {
		out[t.ID] = true
	}
	return out
}

// ChangedAttributeKeys returns keys whose values differ between before and after.
// Keys only in after (newly set) are included; keys removed are included too.
func ChangedAttributeKeys(before, after map[string]any) []string {
	seen := map[string]bool{}
	var keys []string
	add := func(k string) {
		if k == "" || seen[k] {
			return
		}
		seen[k] = true
		keys = append(keys, k)
	}
	if before == nil {
		before = map[string]any{}
	}
	if after == nil {
		after = map[string]any{}
	}
	for k, v := range after {
		bv, ok := before[k]
		if !ok || !attrValuesEqual(bv, v) {
			add(k)
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			add(k)
		}
	}
	return keys
}

func attrValuesEqual(a, b any) bool {
	return fmt.Sprint(a) == fmt.Sprint(b)
}
