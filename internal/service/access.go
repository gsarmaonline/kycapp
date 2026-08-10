package service

import (
	"context"
	"errors"

	"github.com/gsarmaonline/kyc/core/access"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/authn"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
)

// RequirePrincipal returns the authenticated principal or 401.
func RequirePrincipal(ctx context.Context) (authn.Principal, error) {
	p, ok := authn.FromContext(ctx)
	if !ok || p.ActorLabel() == "anonymous" {
		return authn.Principal{}, apperr.Unauthorized("authentication required")
	}
	return p, nil
}

// RequireUser requires a logged-in user (not a service API key).
func RequireUser(ctx context.Context) (authn.Principal, error) {
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return authn.Principal{}, err
	}
	if p.Kind != authn.KindUser || p.UserID == "" {
		return authn.Principal{}, apperr.Forbidden("user session required")
	}
	return p, nil
}

// RequirePlatform requires a platform admin user or service principal.
func RequirePlatform(ctx context.Context) (authn.Principal, error) {
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return authn.Principal{}, err
	}
	if !p.IsPlatform() {
		return authn.Principal{}, apperr.Forbidden("platform admin required")
	}
	return p, nil
}

// RequireOrgMember requires active membership in an *active* org, platform privilege, or an org-scoped API key.
func (s *Service) RequireOrgMember(ctx context.Context, orgID string) (authn.Principal, error) {
	p, err := s.requireOrgMember(ctx, orgID, true)
	return p, err
}

// RequireOrgMemberAnyStatus is like RequireOrgMember but allows archived/suspended orgs (e.g. hard delete).
func (s *Service) RequireOrgMemberAnyStatus(ctx context.Context, orgID string) (authn.Principal, error) {
	return s.requireOrgMember(ctx, orgID, false)
}

func (s *Service) requireOrgMember(ctx context.Context, orgID string, requireActiveOrg bool) (authn.Principal, error) {
	return s.requireOrgAccess(ctx, orgID, capMember, requireActiveOrg)
}

// requireOrgAccess is the single org-scoped gate. It assembles the principal's
// grants once and asks the evaluator, so membership, API key scopes and platform
// reach all take the same path instead of four hand-written branches.
func (s *Service) requireOrgAccess(ctx context.Context, orgID string, cap access.Capability, requireActiveOrg bool) (authn.Principal, error) {
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return authn.Principal{}, err
	}
	// Platform short-circuits before the organisation is loaded, so platform
	// tooling can still act on an archived or partly-created tenant.
	if p.IsPlatform() {
		return p, nil
	}
	// Organisation lifecycle is a property of the resource, not of any grant:
	// a suspended organisation is invisible even to its own members.
	if err := s.assertOrgVisible(ctx, orgID, requireActiveOrg); err != nil {
		return authn.Principal{}, err
	}

	gs, err := s.grantsFor(ctx, p, orgID)
	if err != nil {
		return authn.Principal{}, err
	}
	d := access.Decide(gs, cap, access.Resource{Scope: orgRef(orgID)}, decideNow())
	if d.Allowed {
		return p, nil
	}
	return authn.Principal{}, orgDenial(d, cap)
}

// orgDenial maps a decision to an error. Out-of-scope is a 404 on purpose: a
// caller with no reach into an organisation must not be able to tell it apart
// from one that does not exist, or organisations become enumerable.
func orgDenial(d access.Decision, cap access.Capability) error {
	switch d.Reason {
	case access.ReasonOutOfScope:
		return apperr.NotFound("organisation not found")
	case access.ReasonMissingCapability:
		if cap == capMember {
			return apperr.Forbidden("not a member of this organisation")
		}
		return apperr.Forbidden("missing permission " + cap.Key)
	default:
		return apperr.Forbidden("access denied")
	}
}

func (s *Service) assertOrgVisible(ctx context.Context, orgID string, requireActive bool) error {
	org, err := s.db.Q().GetOrganisation(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("organisation not found")
	}
	if err != nil {
		return err
	}
	if requireActive && org.Status != "active" {
		return apperr.NotFound("organisation not found")
	}
	return nil
}

func (s *Service) hasActiveMembership(ctx context.Context, orgID, userID string) (bool, error) {
	_, err := s.db.Q().GetActiveMembershipByOrgAndUser(ctx, sqlc.GetActiveMembershipByOrgAndUserParams{
		OrganisationID: orgID,
		UserID:         userID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// RequireOrgPermission requires org membership plus an RBAC permission.
// Platform principals bypass. Org API keys succeed when scopes are empty (full access)
// or explicitly include the requested permission.
func (s *Service) RequireOrgPermission(ctx context.Context, orgID, permissionKey string) (authn.Principal, error) {
	return s.requireOrgPermission(ctx, orgID, permissionKey, true)
}

// RequireOrgPermissionAnyStatus allows permissions on archived orgs (hard delete).
func (s *Service) RequireOrgPermissionAnyStatus(ctx context.Context, orgID, permissionKey string) (authn.Principal, error) {
	return s.requireOrgPermission(ctx, orgID, permissionKey, false)
}

func (s *Service) requireOrgPermission(ctx context.Context, orgID, permissionKey string, requireActiveOrg bool) (authn.Principal, error) {
	cap, ok := capabilityFor(permissionKey)
	if !ok {
		// Invariant 3: an unregistered key must deny. Treating it as absent
		// would turn a typo in a gate into an open door.
		return authn.Principal{}, apperr.Forbidden("unknown permission " + permissionKey)
	}
	return s.requireOrgAccess(ctx, orgID, cap, requireActiveOrg)
}
