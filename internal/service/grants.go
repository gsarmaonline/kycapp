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

	// A recovery credential reaches everything, but as a grant rather than a
	// short-circuit: it goes through Decide exactly like a membership does.
	if p.RecoveryID != "" {
		gs.Grants = append(gs.Grants, access.Grant{
			ID:           "recovery:" + p.RecoveryID,
			Scope:        access.GlobalScope(),
			Capabilities: allKYCCapabilities,
			Source:       "recovery",
		})
		return gs, nil
	}

	if p.APIKeyID != "" {
		return s.keyGrants(ctx, p, orgID)
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

// keyGrants derives what an API key holds from the user who owns it.
//
// A key never has power of its own. It carries the intersection of its owner's
// grants and its own scopes, which means three things follow without any extra
// rule: a key can never exceed the person who holds it, demoting them demotes
// it on the next request, and revoking their membership stops it.
//
// That last one is the cost of this model, and it is why ownership is
// transferable: a key that has to outlive its owner's involvement must be moved
// to someone else before they are offboarded.
func (s *Service) keyGrants(ctx context.Context, p authn.Principal, orgID string) (access.GrantSet, error) {
	gs := access.GrantSet{PrincipalID: p.ActorLabel()}

	// An ownerless key confers nothing. Keys predating ownership fail closed
	// rather than keeping the unrestricted access they used to have.
	if p.OwnerUserID == "" {
		return gs, nil
	}
	// An organisation-scoped key acts only in its own organisation, whatever
	// else its owner can reach.
	if p.OrganisationID != "" && p.OrganisationID != orgID {
		return gs, nil
	}

	owner, err := s.grantsFor(ctx, authn.Principal{
		Kind:   authn.KindUser,
		UserID: p.OwnerUserID,
		Actor:  p.Actor,
	}, orgID)
	if err != nil {
		return access.GrantSet{}, err
	}

	var narrowed []access.Capability
	if len(p.Scopes) > 0 {
		narrowed = capsFromKeys(p.Scopes)
	}

	for _, g := range owner.Grants {
		// An org-scoped key must not inherit its owner's global reach: a staff
		// member's key bound to one merchant stays bound to it.
		if p.OrganisationID != "" && g.Scope.IsGlobal() {
			g.Scope = access.OrgScope(p.OrganisationID)
		}
		if narrowed != nil {
			// Empty scopes deliberately mean "everything my owner can do",
			// which is bounded, rather than the unrestricted access an unscoped
			// key used to grant.
			g.Capabilities = append(intersectCapabilities(g.Capabilities, narrowed), capMember)
		}
		g.ID = "api-key:" + p.APIKeyID
		g.Source = "api-key"
		gs.Grants = append(gs.Grants, g)
	}
	return gs, nil
}

func intersectCapabilities(have, want []access.Capability) []access.Capability {
	index := make(map[access.Capability]struct{}, len(have))
	for _, c := range have {
		index[c] = struct{}{}
	}
	out := make([]access.Capability, 0, len(want))
	for _, c := range want {
		if _, ok := index[c]; ok {
			out = append(out, c)
		}
	}
	return out
}

// grantsFromMemberships folds membership rows into grants.
//
// Scope comes from which organisation the membership is in: a membership of the
// platform organisation reaches everything. There is no role name here and no
// stored reach flag, so a role in a merchant organisation cannot produce global
// scope however it is configured.
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
		a.global = a.global || r.GlobalReach
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
