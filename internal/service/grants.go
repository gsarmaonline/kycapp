package service

import (
	"github.com/gsarmaonline/kyc/core/access"
)

// CapOrganisationMember is inherent to reaching an organisation at all: every
// active membership, every org API key, and every platform principal holds it.
// It is not a row in the permissions catalog, because it is not something a role
// grants — it is what having any grant in an organisation means.
//
// Gates that only require tenancy, rather than a specific permission, ask for
// this capability.
const CapOrganisationMember = "organisation:member"

// kycPermissionKeys mirrors the permissions seeded by migrations.
//
// Invariant 3: capabilities are a closed set defined in code, so a typo cannot
// reach a grant. This list is authoritative; TestCapabilityRegistryMatchesSeed
// fails if the database catalog drifts away from it.
var kycPermissionKeys = []string{
	"activity:read",
	"api_keys:manage",
	"api_keys:read",
	// Administering a merchant's own access model is a KYC permission held by
	// their operators. It is not a capability in the merchant's namespace: they
	// administer the model without being inside it.
	"app_access:manage",
	"app_access:read",
	"app_users:read",
	"app_users:write",
	"attributes:manage",
	"attributes:read",
	"automations:manage",
	"automations:read",
	"billing:manage",
	"billing:read",
	"email_templates:manage",
	"email_templates:read",
	// feature_flags:* is deliberately absent: migration 000038 folded separate
	// feature flags into product features and deleted those permissions.
	"members:invite",
	"members:read",
	"members:remove",
	"organisation:read",
	"organisation:update",
	"product_features:manage",
	"product_features:read",
	"roles:manage",
	"roles:read",
	"usage:read",
}

// KYCCapabilities is the closed registry for KYC's own namespace: every seeded
// permission plus the inherent membership capability.
var KYCCapabilities = mustKYCRegistry()

func mustKYCRegistry() *access.Registry {
	keys := append([]string{CapOrganisationMember}, kycPermissionKeys...)
	r, err := access.NewRegistry(access.NamespaceKYC, keys...)
	if err != nil {
		panic(err)
	}
	return r
}

// capMember is resolved once; it is needed on every org-scoped request.
var capMember = KYCCapabilities.MustParse(CapOrganisationMember)

// capabilityFor resolves a permission key, returning false for anything not in
// the registry. An unknown key must deny rather than be treated as absent.
func capabilityFor(key string) (access.Capability, bool) {
	c, err := KYCCapabilities.Parse(key)
	if err != nil {
		return access.Capability{}, false
	}
	return c, true
}
