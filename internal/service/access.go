package service

import (
	"context"
	"fmt"
	"time"

	"github.com/gsarmaonline/kyc/core/reach"
	"github.com/gsarmaonline/kyc/internal/accessmodel"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/authn"
)

// Authorisation is a reachability question. Every gate below asks the same one:
// may this subject perform this action on this resource?
//
// The graph is read through a view over the tables that already hold the
// answer, so nothing about *state* changed when the engine did. A role edit, an
// invite, a suspension or a revoked key take effect exactly as before, because
// they are still the same rows. What changed is how those rows are interpreted:
// by a walk that returns the path it took, rather than by an assembled grant
// set.
//
// Two consequences worth knowing at the call sites, neither of them new:
//
//   - Out of reach is reported as 404, never 403. A caller with no reach into an
//     organisation must not be able to tell it from one that does not exist, or
//     tenants become enumerable by status code.
//   - Staff do not short-circuit. Their reach is edges on the star nodes, so a
//     read-only support role stays read-only inside a merchant's organisation.

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

// isBreakGlass reports whether the principal is an environment service token
// from API_TOKENS. It is resolved before any query, which is what makes it the
// root of trust on an empty or mis-seeded database.
//
// APIKeyID must be empty. A platform key stored in the database also has no
// organisation, so without that check it would be mistaken for break-glass and
// short-circuit past its owner.
//
// This is the one principal that does not go through the graph, and it is
// deliberate: reach that has to survive an empty store cannot be derived from
// the store. Everything else, recovery credentials included, is edges.
func isBreakGlass(p authn.Principal) bool {
	return p.Kind == authn.KindService && p.OrganisationID == "" && p.APIKeyID == ""
}

// principalNode is the node the walk starts from.
func principalNode(p authn.Principal) (reach.NodeRef, error) {
	switch {
	case p.RecoveryID != "":
		return reach.Node("recovery", p.RecoveryID), nil
	case p.APIKeyID != "":
		return reach.Node("key", p.APIKeyID), nil
	case p.Kind == authn.KindUser && p.UserID != "":
		return reach.Node("user", p.UserID), nil
	default:
		return reach.NodeRef{}, apperr.Forbidden("principal cannot be authorised")
	}
}

// evaluator returns the graph evaluator, building it once.
func (s *Service) evaluator() (*reach.Evaluator, error) {
	if s.reach != nil {
		return s.reach, nil
	}
	e, err := accessmodel.NewEvaluator(s.db.Q())
	if err != nil {
		return nil, fmt.Errorf("access: %w", err)
	}
	s.reach = e
	return e, nil
}

// decideNow exists so tests can pin time without threading a clock through
// every gate. Production always uses the wall clock.
var decideNow = func() time.Time { return time.Now().UTC() }

// check asks one reachability question.
func (s *Service) check(ctx context.Context, subject reach.NodeRef, permissionKey string, resource reach.NodeRef) (reach.Decision, error) {
	e, err := s.evaluator()
	if err != nil {
		return reach.Decision{}, err
	}
	p, ok := accessmodel.Permissions[permissionKey]
	if !ok {
		// Invariant: an unregistered key must deny. Treating it as absent would
		// turn a typo in a gate into an open door.
		return reach.Decision{}, apperr.Forbidden("unknown permission " + permissionKey)
	}
	return e.Check(ctx, reach.Request{
		Subject:  subject,
		Action:   p.Action,
		Resource: resource,
	}, decideNow())
}

// RequirePlatformCapability gates a platform-wide route on a named capability
// rather than on staff status, so a read-only support role can list without
// also being able to write.
//
// The resource is the type's star node: every organisation's slice of it,
// present and future. Only a principal holding an edge there passes, which is
// what makes platform reach visible in the data rather than implied by a flag.
func (s *Service) RequirePlatformCapability(ctx context.Context, permissionKey string) (authn.Principal, error) {
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return authn.Principal{}, err
	}
	if isBreakGlass(p) {
		return p, nil
	}
	perm, ok := accessmodel.Permissions[permissionKey]
	if !ok {
		return authn.Principal{}, apperr.Forbidden("unknown permission " + permissionKey)
	}
	subject, err := principalNode(p)
	if err != nil {
		return authn.Principal{}, apperr.Forbidden("platform access required")
	}
	d, err := s.check(ctx, subject, permissionKey, accessmodel.EveryArea(perm.Type))
	if err != nil {
		return authn.Principal{}, err
	}
	if d.Allowed {
		return p, nil
	}
	if d.Reason == reach.ReasonUnreachable {
		return authn.Principal{}, apperr.Forbidden("platform access required")
	}
	return authn.Principal{}, apperr.Forbidden("missing permission " + permissionKey)
}

// ReachesEveryOrganisation reports whether the principal reaches every tenant,
// present and future. It is the subset rule asked at the star node: only a
// principal that already holds reach there may issue it.
func (s *Service) ReachesEveryOrganisation(ctx context.Context, p authn.Principal) (bool, error) {
	if isBreakGlass(p) {
		return true, nil
	}
	subject, err := principalNode(p)
	if err != nil {
		return false, nil
	}
	e, err := s.evaluator()
	if err != nil {
		return false, err
	}
	d, err := e.Check(ctx, reach.Request{
		Subject:  subject,
		Action:   "reach",
		Resource: reach.Star("organisation"),
	}, decideNow())
	if err != nil {
		return false, err
	}
	return d.Allowed, nil
}

// RequireOrgMember requires reach into an *active* organisation.
func (s *Service) RequireOrgMember(ctx context.Context, orgID string) (authn.Principal, error) {
	return s.requireOrgAccess(ctx, orgID, accessmodel.CapOrganisationMember, true)
}

// RequireOrgMemberAnyStatus is RequireOrgMember for a lifecycle route: a
// suspended organisation stays visible to its own members, so the state can be
// seen and acted on rather than the tenant simply vanishing.
func (s *Service) RequireOrgMemberAnyStatus(ctx context.Context, orgID string) (authn.Principal, error) {
	return s.requireOrgAccess(ctx, orgID, accessmodel.CapOrganisationMember, false)
}

// RequireOrgPermission requires reach into the organisation plus a named
// permission there.
func (s *Service) RequireOrgPermission(ctx context.Context, orgID, permissionKey string) (authn.Principal, error) {
	return s.requireOrgAccess(ctx, orgID, permissionKey, true)
}

// RequireOrgPermissionAnyStatus allows permissions on suspended or archived
// organisations, which lifecycle routes need: status is settable through PATCH,
// so without it suspending a tenant would make the route that restores it
// return 404 as well.
func (s *Service) RequireOrgPermissionAnyStatus(ctx context.Context, orgID, permissionKey string) (authn.Principal, error) {
	return s.requireOrgAccess(ctx, orgID, permissionKey, false)
}

// requireOrgAccess is the single org-scoped gate. Membership, API key reach,
// recovery and platform reach all take the same path, because by the time the
// walk runs they are all just edges.
func (s *Service) requireOrgAccess(ctx context.Context, orgID, permissionKey string, requireActiveOrg bool) (authn.Principal, error) {
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return authn.Principal{}, err
	}
	if isBreakGlass(p) {
		return p, nil
	}
	subject, err := principalNode(p)
	if err != nil {
		return authn.Principal{}, apperr.NotFound("organisation not found")
	}

	e, err := s.evaluator()
	if err != nil {
		return authn.Principal{}, err
	}

	// Can this principal see the organisation at all? Lifecycle lives in this
	// question: `member = belongs - suspended + oversees` hides a suspended
	// tenant from its own members while leaving it visible to platform staff.
	// `reach` is the same without the subtraction, for the routes that must work
	// on a suspended tenant.
	visibility := "member"
	if !requireActiveOrg {
		visibility = "reach"
	}
	seen, err := e.Check(ctx, reach.Request{
		Subject:  subject,
		Action:   visibility,
		Resource: reach.Node("organisation", orgID),
	}, decideNow())
	if err != nil {
		return authn.Principal{}, err
	}
	if !seen.Allowed {
		// Out of reach and non-existent are the same answer on purpose.
		return authn.Principal{}, apperr.NotFound("organisation not found")
	}
	if permissionKey == accessmodel.CapOrganisationMember {
		return p, nil
	}

	perm, ok := accessmodel.Permissions[permissionKey]
	if !ok {
		return authn.Principal{}, apperr.Forbidden("unknown permission " + permissionKey)
	}
	d, err := s.check(ctx, subject, permissionKey, accessmodel.Area(perm.Type, orgID))
	if err != nil {
		return authn.Principal{}, err
	}
	if d.Allowed {
		return p, nil
	}
	return authn.Principal{}, apperr.Forbidden("missing permission " + permissionKey)
}
