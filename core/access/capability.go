package access

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// NamespaceKYC is the namespace for capabilities defined by KYC itself.
const NamespaceKYC = "kyc"

// OrgNamespace is the namespace for capabilities a merchant declares for their
// own product. Keeping merchant capabilities in their own namespace is what lets
// KYC's set stay closed while theirs stays open: a typo in one cannot collide
// with, or widen, the other.
func OrgNamespace(orgID string) string { return "org:" + orgID }

// Capability is a namespaced verb, e.g. {kyc, members:invite}.
type Capability struct {
	Namespace string
	Key       string
}

func (c Capability) String() string { return c.Namespace + "/" + c.Key }

// IsZero reports whether the capability is unset.
func (c Capability) IsZero() bool { return c.Namespace == "" && c.Key == "" }

var (
	// ErrUnknownCapability means the key is not registered in its namespace.
	ErrUnknownCapability = errors.New("access: unknown capability")
	// ErrInvalidCapability means the key is malformed.
	ErrInvalidCapability = errors.New("access: invalid capability key")
)

// Registry is the closed set of capability keys for one namespace.
//
// KYC builds one from constants in code, so a typo cannot compile. A merchant
// namespace builds one from the keys they declared, so a typo is contained to
// their tenancy. Same type, different source of truth.
type Registry struct {
	namespace string
	keys      map[string]struct{}
}

// NewRegistry returns a registry for namespace holding exactly keys.
// Keys must be non-empty and unique; duplicates are an error rather than a
// silent merge, because a duplicate usually means two sources disagree.
func NewRegistry(namespace string, keys ...string) (*Registry, error) {
	if strings.TrimSpace(namespace) == "" {
		return nil, fmt.Errorf("%w: empty namespace", ErrInvalidCapability)
	}
	set := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		if err := validKey(k); err != nil {
			return nil, err
		}
		if _, dup := set[k]; dup {
			return nil, fmt.Errorf("%w: duplicate key %q", ErrInvalidCapability, k)
		}
		set[k] = struct{}{}
	}
	return &Registry{namespace: namespace, keys: set}, nil
}

// Namespace returns the registry's namespace.
func (r *Registry) Namespace() string { return r.namespace }

// Parse resolves a key into a Capability, rejecting anything unregistered.
// This is the only supported way to build a Capability from input.
func (r *Registry) Parse(key string) (Capability, error) {
	if err := validKey(key); err != nil {
		return Capability{}, err
	}
	if _, ok := r.keys[key]; !ok {
		return Capability{}, fmt.Errorf("%w: %s/%s", ErrUnknownCapability, r.namespace, key)
	}
	return Capability{Namespace: r.namespace, Key: key}, nil
}

// MustParse is Parse for capabilities defined as constants in code. It panics on
// an unknown key, which is what you want at init time in KYC's own namespace.
func (r *Registry) MustParse(key string) Capability {
	c, err := r.Parse(key)
	if err != nil {
		panic(err)
	}
	return c
}

// Has reports whether the key is registered.
func (r *Registry) Has(key string) bool {
	_, ok := r.keys[key]
	return ok
}

// Keys returns the registered keys, sorted. Useful for diffing a registry
// against capabilities already stored in grants.
func (r *Registry) Keys() []string {
	out := make([]string, 0, len(r.keys))
	for k := range r.keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Unknown returns the members of caps that this registry does not recognise, or
// that belong to a different namespace. Callers use it at startup to refuse to
// boot when stored grants reference a capability that no longer exists.
func (r *Registry) Unknown(caps []Capability) []Capability {
	var out []Capability
	for _, c := range caps {
		if c.Namespace != r.namespace || !r.Has(c.Key) {
			out = append(out, c)
		}
	}
	return out
}

// validKey enforces the resource:action shape. The format is not decorative:
// it is what lets an admin UI group capabilities by resource.
func validKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: empty key", ErrInvalidCapability)
	}
	if key != strings.TrimSpace(key) {
		return fmt.Errorf("%w: %q has surrounding whitespace", ErrInvalidCapability, key)
	}
	resource, action, ok := strings.Cut(key, ":")
	if !ok || resource == "" || action == "" {
		return fmt.Errorf("%w: %q must be resource:action", ErrInvalidCapability, key)
	}
	if strings.Contains(action, ":") {
		return fmt.Errorf("%w: %q has more than one colon", ErrInvalidCapability, key)
	}
	if strings.ContainsAny(key, "*?") {
		// Wildcards would silently widen an existing grant the day a new
		// capability ships. Expand them when authoring, store them concrete.
		return fmt.Errorf("%w: %q contains a wildcard", ErrInvalidCapability, key)
	}
	return nil
}
