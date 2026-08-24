package accessmodel

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gsarmaonline/kyc/core/reach"
)

// A merchant's own model, as a schema.
//
// Their vocabulary already is one, and has been since it was built: scope kinds
// are types, capabilities are actions on a type, roles and groups are named sets
// reached through a relation. Nothing here invents a model. It reads the one
// they authored and states it in the language the engine speaks.
//
// The shape that falls out is the ordinary containment one. A grant is written
// at a scope, a capability names a resource, and a resource sits inside a scope:
//
//	project:apollo #can_read role:editor#holder     the grant
//	document:d1    #parent   project:apollo         the containment
//	rule read = can_read + parent->read             the reason it reaches
//
// The arrow is what makes one grant at project level cover every document in it,
// instead of one grant per document. It is also why the merchant has to write
// containment edges at all: without document:d1 knowing where it lives, no walk
// can arrive at it.

// MerchantModel is what a merchant declared, as the resolver needs it.
type MerchantModel struct {
	OrganisationID string
	// ScopeKinds are the containers a grant may be written at: project, region.
	ScopeKinds []string
	// CapabilityKeys are resource:action pairs. The resource becomes a type and
	// the action becomes something that type can answer.
	CapabilityKeys []string
}

// Reserved relation names in a merchant namespace. These carry structure rather
// than permission, so a capability naming one would change what the graph means
// rather than what it grants.
var merchantReservedResources = map[string]struct{}{
	"app_user": {}, "group": {}, "role": {},
}

// MerchantSchema builds a namespace from what a merchant declared.
//
// It is derived, never stored. A merchant edits a capability and the schema
// changes with it, so the two cannot drift, and there is no migration to run
// when they add a scope kind.
func MerchantSchema(m MerchantModel) (*reach.Schema, error) {
	if strings.TrimSpace(m.OrganisationID) == "" {
		return nil, fmt.Errorf("accessmodel: merchant schema needs an organisation")
	}

	actions := map[string]struct{}{}
	// resource -> the actions its capabilities name.
	resources := map[string]map[string]struct{}{}
	for _, key := range m.CapabilityKeys {
		resource, action, ok := strings.Cut(key, ":")
		if !ok || resource == "" || action == "" {
			return nil, fmt.Errorf("accessmodel: capability %q is not resource:action", key)
		}
		if _, bad := merchantReservedResources[resource]; bad {
			// A capability on app_user would put grant edges on the principal
			// type, where the walk expects membership. It reads as a permission
			// and behaves as structure.
			return nil, fmt.Errorf("accessmodel: capability %q names a structural type", key)
		}
		actions[action] = struct{}{}
		if resources[resource] == nil {
			resources[resource] = map[string]struct{}{}
		}
		resources[resource][action] = struct{}{}
	}

	scopeKinds := make([]string, 0, len(m.ScopeKinds))
	for _, kind := range m.ScopeKinds {
		if _, bad := merchantReservedResources[kind]; bad {
			return nil, fmt.Errorf("accessmodel: scope kind %q names a structural type", kind)
		}
		scopeKinds = append(scopeKinds, kind)
	}
	sort.Strings(scopeKinds)

	// Resource types are the ones a capability names that are not already a
	// container. A merchant often names both the same thing, workspace:read
	// where workspace is also a scope kind, and then the container declaration
	// covers it.
	resourceTypes := make([]string, 0, len(resources))
	for _, resource := range sortedKeysOf(resources) {
		if _, isScope := indexOf(scopeKinds, resource); !isScope {
			resourceTypes = append(resourceTypes, resource)
		}
	}

	// parent is declared only when something will carry it: scopes nesting into
	// each other, or a resource sitting inside one. Declaring it otherwise is
	// inert, and the validator rejects a schema that says something the engine
	// ignores rather than letting it read as though it means something.
	nests := len(scopeKinds) > 1 || (len(scopeKinds) > 0 && len(resourceTypes) > 0)

	var b strings.Builder
	fmt.Fprintf(&b, "namespace %s\n\n", MerchantNamespace(m.OrganisationID))

	// A namespace with no capabilities is legal and reaches nothing. It is what
	// every merchant starts with, and refusing it would mean the schema could
	// not be rendered until the vocabulary was already finished.
	if len(actions) > 0 {
		fmt.Fprintf(&b, "action %s\n\n", strings.Join(sortedSet(actions), ", "))
	}

	b.WriteString("relation member_of : transitive\n")
	b.WriteString("relation holder    : direct\n")
	if nests {
		// direct, not transitive, and the validator is what taught this. A
		// container chain already recurses through the rules: document's
		// parent->read lands on project, whose own read is can_read +
		// parent->read, which lands on region. The arrow does the walking, so
		// the transitive flag would have no effect, and a flag that says
		// something the engine ignores is worse than one that is absent.
		b.WriteString("relation parent    : direct\n")
	}
	for _, action := range sortedSet(actions) {
		// Wildcard on both ends: the object star is scope_id '*', every instance
		// of a kind, and the subject star is the everyone grant.
		fmt.Fprintf(&b, "relation can_%s : direct, wildcard both\n", action)
	}

	b.WriteString("\ntype app_user\n")
	b.WriteString("\ntype group\n  relation member_of -> app_user | group#member_of\n")
	b.WriteString("\ntype role\n  relation holder -> app_user | group#member_of\n")

	const grantees = "app_user | group#member_of | role#holder"

	// A scope kind is a container. It answers every action in the namespace,
	// because a grant written there has to be able to carry any capability a
	// role holds.
	for _, kind := range scopeKinds {
		fmt.Fprintf(&b, "\ntype %s\n", kind)
		if len(scopeKinds) > 1 {
			// Scopes nest into each other, which is what lets a merchant model
			// a project inside a region without KYC knowing either.
			fmt.Fprintf(&b, "  relation parent -> %s\n", strings.Join(scopeKinds, " | "))
		}
		for _, action := range sortedSet(actions) {
			fmt.Fprintf(&b, "  relation can_%s -> %s\n", action, grantees)
		}
		for _, action := range sortedSet(actions) {
			if len(scopeKinds) > 1 {
				fmt.Fprintf(&b, "  rule %s = can_%s + parent->%s\n", action, action, action)
			} else {
				fmt.Fprintf(&b, "  rule %s = can_%s\n", action, action)
			}
		}
	}

	// A resource type answers only the actions its own capabilities name, and
	// reaches through its container. The arrow is the entire reason one grant at
	// project level covers every document inside it.
	for _, resource := range resourceTypes {
		fmt.Fprintf(&b, "\ntype %s\n", resource)
		if len(scopeKinds) > 0 {
			fmt.Fprintf(&b, "  relation parent -> %s\n", strings.Join(scopeKinds, " | "))
		}
		for _, action := range sortedSet(resources[resource]) {
			fmt.Fprintf(&b, "  relation can_%s -> %s\n", action, grantees)
		}
		for _, action := range sortedSet(resources[resource]) {
			if len(scopeKinds) > 0 {
				fmt.Fprintf(&b, "  rule %s = can_%s + parent->%s\n", action, action, action)
			} else {
				fmt.Fprintf(&b, "  rule %s = can_%s\n", action, action)
			}
		}
	}

	schema, err := reach.Parse(b.String())
	if err != nil {
		return nil, fmt.Errorf("accessmodel: merchant schema: %w", err)
	}
	// Warnings mean the schema declares something the engine ignores, which for
	// a generated schema is a bug here rather than a merchant's mistake.
	if w := schema.Warnings(); len(w) > 0 {
		return nil, fmt.Errorf("accessmodel: merchant schema has inert declarations: %v", w)
	}
	return schema, nil
}

func sortedSet(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedKeysOf(m map[string]map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func indexOf(haystack []string, needle string) (int, bool) {
	for i, s := range haystack {
		if s == needle {
			return i, true
		}
	}
	return 0, false
}
