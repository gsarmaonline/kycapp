package access

import (
	"errors"
	"reflect"
	"testing"
)

// Invariant 3: capabilities are a closed set per namespace. Parse is the only
// way in, so an unregistered key can never reach a grant.
func TestRegistryRejectsUnknownKeys(t *testing.T) {
	r := mustRegistry(NamespaceKYC, "app_users:read")

	if _, err := r.Parse("app_users:read"); err != nil {
		t.Fatalf("registered key must parse: %v", err)
	}
	if _, err := r.Parse("app_users:delete"); !errors.Is(err, ErrUnknownCapability) {
		t.Errorf("want ErrUnknownCapability, got %v", err)
	}
}

func TestRegistryRejectsMalformedKeys(t *testing.T) {
	for _, key := range []string{
		"",                // empty
		"app_users",       // no action
		"app_users:",      // empty action
		":read",           // empty resource
		"a:b:c",           // two colons
		" app_users:read", // whitespace
		"app_users:*",     // wildcard
	} {
		t.Run(key, func(t *testing.T) {
			if _, err := NewRegistry(NamespaceKYC, key); !errors.Is(err, ErrInvalidCapability) {
				t.Errorf("want ErrInvalidCapability, got %v", err)
			}
		})
	}
}

// A wildcard would silently widen every grant holding it the day a new
// capability ships, so it must be impossible to register one.
func TestRegistryRejectsWildcards(t *testing.T) {
	if _, err := NewRegistry(NamespaceKYC, "app_users:*"); err == nil {
		t.Error("wildcards must be rejected at registration")
	}
}

func TestRegistryRejectsDuplicatesAndEmptyNamespace(t *testing.T) {
	if _, err := NewRegistry(NamespaceKYC, "a:b", "a:b"); !errors.Is(err, ErrInvalidCapability) {
		t.Errorf("duplicate key: want ErrInvalidCapability, got %v", err)
	}
	if _, err := NewRegistry("  ", "a:b"); !errors.Is(err, ErrInvalidCapability) {
		t.Errorf("empty namespace: want ErrInvalidCapability, got %v", err)
	}
}

// Namespaces isolate merchants from KYC and from each other: the same key in
// two namespaces is two different capabilities.
func TestNamespacesIsolateIdenticalKeys(t *testing.T) {
	kyc := mustRegistry(NamespaceKYC, "app_users:read")
	acme := mustRegistry(OrgNamespace("acme"), "app_users:read")

	a, _ := kyc.Parse("app_users:read")
	b, _ := acme.Parse("app_users:read")
	if a == b {
		t.Fatal("same key in different namespaces must not be equal")
	}

	gs := GrantSet{Grants: []Grant{grant("g1", OrgScope("acme"), b)}}
	if d := Decide(gs, a, acmeRes(), now); d.Allowed {
		t.Error("a merchant capability must not satisfy a KYC capability")
	}
}

// Unknown lets a caller refuse to boot when stored grants reference a
// capability that has since been removed from code.
func TestRegistryUnknownFindsStaleCapabilities(t *testing.T) {
	r := mustRegistry(NamespaceKYC, "app_users:read")
	stale := Capability{Namespace: NamespaceKYC, Key: "app_users:purge"}
	foreign := Capability{Namespace: OrgNamespace("acme"), Key: "app_users:read"}
	live, _ := r.Parse("app_users:read")

	got := r.Unknown([]Capability{live, stale, foreign})
	if !reflect.DeepEqual(got, []Capability{stale, foreign}) {
		t.Errorf("Unknown = %v, want the stale and foreign entries", got)
	}
}

func TestRegistryKeysAreSorted(t *testing.T) {
	r := mustRegistry(NamespaceKYC, "b:read", "a:write", "a:read")
	want := []string{"a:read", "a:write", "b:read"}
	if got := r.Keys(); !reflect.DeepEqual(got, want) {
		t.Errorf("Keys() = %v, want %v", got, want)
	}
}
