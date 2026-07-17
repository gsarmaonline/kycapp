package billing

import "sort"

// EffectiveKeys returns plan entitlements ∪ grants − denies.
func EffectiveKeys(planKeys []string, overrides map[string]string) []string {
	set := make(map[string]struct{}, len(planKeys))
	for _, k := range planKeys {
		if k == "" {
			continue
		}
		set[k] = struct{}{}
	}
	for key, effect := range overrides {
		switch effect {
		case "grant":
			set[key] = struct{}{}
		case "deny":
			delete(set, key)
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Has reports whether key is in the effective set.
func Has(effective []string, key string) bool {
	for _, k := range effective {
		if k == key {
			return true
		}
	}
	return false
}
