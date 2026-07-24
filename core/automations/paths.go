package automations

import (
	"strings"

	"github.com/gsarmaonline/kyc/core/resources"
)

// Canonical field paths for app_user event data (conditions, webhook templates,
// db_insert mappings). Same vocabulary everywhere:
//
//	app_user.email              — core column
//	app_user.country            — org attribute key (not attributes.country)
//	organisation_id / trigger   — run metadata
//
// Triggers stay event IDs (app_user.created, app_user.attribute.country).
var appUserCoreFields = map[string]struct{}{
	"id": {}, "email": {}, "display_name": {}, "status": {}, "external_id": {}, "attributes": {},
}

// AppUserFieldPath returns app_user.<name> for a core or attribute key.
func AppUserFieldPath(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return resources.AppUser + "." + name
}

// NormalizeFieldPath rewrites legacy payload-relative paths to canonical refs.
// email → app_user.email, attributes.country → app_user.country.
func NormalizeFieldPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, resources.AppUser+".") {
		return path
	}
	if rest, ok := strings.CutPrefix(path, "attributes."); ok && rest != "" {
		return AppUserFieldPath(rest)
	}
	if _, ok := appUserCoreFields[path]; ok {
		return AppUserFieldPath(path)
	}
	return path
}

// NormalizeConditionsFields rewrites condition field paths to canonical form.
func NormalizeConditionsFields(cond Conditions) Conditions {
	n := cond.Normalize()
	for i := range n.All {
		n.All[i].Field = NormalizeFieldPath(n.All[i].Field)
	}
	for i := range n.Any {
		n.Any[i].Field = NormalizeFieldPath(n.Any[i].Field)
	}
	if len(n.Items) > 0 {
		for i := range n.Items {
			n.Items[i].Field = NormalizeFieldPath(n.Items[i].Field)
		}
	}
	return n
}

// Lookup resolves a field path against an automation payload.
// Supports canonical app_user.* paths, legacy email / attributes.*, and
// direct dotted paths (organisation_id, nested maps).
func Lookup(payload map[string]any, path string) (any, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false
	}
	if path == "payload" {
		return payload, true
	}

	if v, ok := lookupDirect(payload, path); ok {
		return v, true
	}

	canon := NormalizeFieldPath(path)
	if rest, ok := strings.CutPrefix(canon, resources.AppUser+"."); ok && rest != "" {
		return lookupAppUser(payload, rest)
	}

	if canon != path {
		return lookupDirect(payload, canon)
	}
	return nil, false
}

func lookupAppUser(payload map[string]any, rest string) (any, bool) {
	if user, ok := payload[resources.AppUser].(map[string]any); ok {
		if v, ok := lookupAppUserObject(user, rest); ok {
			return v, true
		}
	}
	return lookupAppUserObject(payload, rest)
}

func lookupAppUserObject(obj map[string]any, rest string) (any, bool) {
	if rest == "attributes" {
		v, ok := obj["attributes"]
		return v, ok
	}
	if _, isCore := appUserCoreFields[rest]; isCore {
		return lookupDirect(obj, rest)
	}
	// Org attribute key: prefer flattened value, else attributes bag.
	if v, ok := obj[rest]; ok {
		return v, true
	}
	attrs, ok := obj["attributes"].(map[string]any)
	if !ok {
		return nil, false
	}
	v, ok := attrs[rest]
	return v, ok
}

func lookupDirect(payload map[string]any, field string) (any, bool) {
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
