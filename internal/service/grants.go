package service

import (
	"context"
	"time"

	"github.com/gsarmaonline/kyc/core/access"
	"github.com/gsarmaonline/kyc/internal/authn"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
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

// allKYCCapabilities is what a platform principal, or an unscoped org API key,
// holds. Built once.
var allKYCCapabilities = buildAllCapabilities()

func buildAllCapabilities() []access.Capability {
	out := make([]access.Capability, 0, len(kycPermissionKeys)+1)
	out = append(out, capMember)
	for _, k := range kycPermissionKeys {
		out = append(out, KYCCapabilities.MustParse(k))
	}
	return out
}

// capabilityFor resolves a permission key, returning false for anything not in
// the registry. An unknown key must deny rather than be treated as absent.
func capabilityFor(key string) (access.Capability, bool) {
	c, err := KYCCapabilities.Parse(key)
	if err != nil {
		return access.Capability{}, false
	}
	return c, true
}

// grantsFor assembles what a principal holds in one organisation.
//
// Nothing is stored: every grant here is derived from a relationship that
// already exists — a platform flag, an API key row, an active membership.
// A grants table is only needed once something has no such relationship to
// derive from, such as time-boxed staff access.
//
// Assembly happens once per request; the decision itself is then a set lookup.
func (s *Service) grantsFor(ctx context.Context, p authn.Principal, orgID string) (access.GrantSet, error) {
	gs := access.GrantSet{PrincipalID: p.ActorLabel()}

	// Break-glass is the only principal that holds everything by definition.
	// Staff are deliberately not handled here: their reach and their
	// capabilities both come from the membership rows below, so a read-only
	// support role stays read-only. Short-circuiting on IsPlatform would hand
	// every staff member full power and make least privilege unexpressible.
	if isBreakGlass(p) {
		gs.Grants = append(gs.Grants, access.Grant{
			ID:           "break-glass",
			Scope:        access.GlobalScope(),
			Capabilities: allKYCCapabilities,
			Source:       "break-glass",
		})
		return gs, nil
	}

	if p.Kind == authn.KindService && p.OrganisationID != "" {
		if p.OrganisationID != orgID {
			return gs, nil // reaches nothing here
		}
		caps := allKYCCapabilities
		if len(p.Scopes) > 0 {
			caps = capsFromKeys(p.Scopes)
			// A key scoped only to keys we no longer recognise would produce an
			// empty, invalid grant. Membership still holds, so it can reach the
			// organisation but do nothing in it.
			caps = append(caps, capMember)
		}
		gs.Grants = append(gs.Grants, access.Grant{
			ID:           "api-key:" + p.APIKeyID,
			Scope:        access.OrgScope(orgID),
			Capabilities: caps,
			Source:       "api-key",
		})
		return gs, nil
	}

	if p.Kind != authn.KindUser || p.UserID == "" {
		return gs, nil
	}

	rows, err := s.db.Q().ListUserGrantSources(ctx, sqlc.ListUserGrantSourcesParams{
		UserID:         p.UserID,
		OrganisationID: orgID,
	})
	if err != nil {
		return access.GrantSet{}, err
	}
	gs.Grants = append(gs.Grants, grantsFromMemberships(rows, orgID)...)
	return gs, nil
}

// grantsFromMemberships folds membership rows into grants.
//
// Scope comes from the role's grants_global_reach column, never from a role
// name: this code cannot tell "root" from any other role, which is what keeps
// staff access expressible entirely as data.
func grantsFromMemberships(rows []sqlc.ListUserGrantSourcesRow, orgID string) []access.Grant {
	// One grant per membership, keyed by organisation.
	type acc struct {
		global bool
		caps   []access.Capability
		seen   map[string]struct{}
	}
	byOrg := map[string]*acc{}
	order := []string{}

	for _, r := range rows {
		a := byOrg[r.OrganisationID]
		if a == nil {
			a = &acc{seen: map[string]struct{}{}}
			byOrg[r.OrganisationID] = a
			order = append(order, r.OrganisationID)
		}
		a.global = a.global || r.GrantsGlobalReach
		// permission_key is NULL when the role carries no permissions. The
		// membership still confers reach, so the row is not skipped.
		if !r.PermissionKey.Valid {
			continue
		}
		if _, dup := a.seen[r.PermissionKey.String]; dup {
			continue
		}
		a.seen[r.PermissionKey.String] = struct{}{}
		if c, ok := capabilityFor(r.PermissionKey.String); ok {
			a.caps = append(a.caps, c)
		}
	}

	out := make([]access.Grant, 0, len(order))
	for _, id := range order {
		a := byOrg[id]
		scope := access.OrgScope(id)
		if a.global {
			scope = access.GlobalScope()
		} else if id != orgID {
			// A membership elsewhere without global reach says nothing here.
			continue
		}
		out = append(out, access.Grant{
			ID:           "membership:" + id,
			Scope:        scope,
			Capabilities: append(a.caps, capMember),
			Source:       "membership",
		})
	}
	return out
}

func capsFromKeys(keys []string) []access.Capability {
	out := make([]access.Capability, 0, len(keys))
	for _, k := range keys {
		if c, ok := capabilityFor(k); ok {
			out = append(out, c)
		}
	}
	return out
}

// orgRef is the scope coordinates of a resource inside one organisation.
func orgRef(orgID string) access.ScopeRef {
	return access.Ref(access.ScopeOrganisation, orgID)
}

// decideNow exists so tests can pin time without threading a clock through
// every gate. Production always uses the wall clock.
var decideNow = func() time.Time { return time.Now().UTC() }
