package service

import (
	"context"
	"sort"

	"github.com/gsarmaonline/kyc/core/reach"
	"github.com/gsarmaonline/kyc/internal/accessmodel"
	"github.com/gsarmaonline/kyc/internal/authn"
)

// Explaining a decision.
//
// The engine already records the route it took, so this adds no evaluation: it
// asks each permission in turn and reshapes the answer for a reader. What it
// does add is a filter, because a path is the most useful thing in the system
// and the most leaky.
//
// "Denied because role finance-approvers in organisation acme holds that grant"
// names a role, an organisation and a structure the asker may have no business
// knowing. So every hop is checked against what the *viewer* reaches, and a hop
// that leaves the organisation being viewed is reduced to the fact that it
// exists. The route stays honest about its length; it stops naming what the
// viewer cannot already see.

// RedactedNode is what a hop outside the viewer's reach renders as. It is a
// placeholder rather than a removal, because dropping the hop would make a
// four-hop route look like a three-hop one and misdescribe the model.
const RedactedNode = "(elsewhere)"

// PathHop is one edge the walk crossed, as text.
type PathHop struct {
	Object   string `json:"object"`
	Relation string `json:"relation"`
	Subject  string `json:"subject"`
	// Redacted reports that a name in this hop was withheld. Surfaced rather
	// than hidden: a reader who cannot see a hop should know one is there.
	Redacted bool `json:"redacted"`
}

// PermissionOutcome is one permission and how the graph answered it.
type PermissionOutcome struct {
	Key     string `json:"key"`
	Allowed bool   `json:"allowed"`
	// Reason is the engine's own vocabulary: allowed, unreachable, no_rule or
	// excluded. The distinction is the useful part, and a boolean throws it
	// away: "no path arrives" and "a path arrives but grants something else"
	// call for entirely different fixes.
	Reason string    `json:"reason"`
	Path   []PathHop `json:"path"`
}

// AccessExplanation is every permission answered for one subject.
type AccessExplanation struct {
	OrganisationID string              `json:"organisation_id"`
	Subject        string              `json:"subject"`
	Outcomes       []PermissionOutcome `json:"outcomes"`
}

// ExplainMembershipAccess answers every permission for one member and returns
// the route the engine took to each answer.
//
// The gate is members:read in that member's organisation, which is the same
// permission that shows the member at all. Nothing here is reachable to a
// caller who could not already list them.
func (s *Service) ExplainMembershipAccess(ctx context.Context, membershipID string) (AccessExplanation, error) {
	row, err := s.GetMembershipDetail(ctx, membershipID)
	if err != nil {
		return AccessExplanation{}, err
	}
	viewer, err := s.RequireOrgPermission(ctx, row.OrganisationID, "members:read")
	if err != nil {
		return AccessExplanation{}, err
	}
	return s.explainAccess(ctx, viewer, row.OrganisationID, reach.Node("user", row.UserID))
}

// explainAccess runs every permission for a subject inside one organisation.
func (s *Service) explainAccess(ctx context.Context, viewer authn.Principal, orgID string, subject reach.NodeRef) (AccessExplanation, error) {
	// A viewer who reaches every organisation is already entitled to the names
	// a redaction would withhold, so the filter would only make the answer
	// worse for them.
	unrestricted, err := s.ReachesEveryOrganisation(ctx, viewer)
	if err != nil {
		return AccessExplanation{}, err
	}
	visible, err := s.visibleNodes(ctx, orgID, subject, unrestricted)
	if err != nil {
		return AccessExplanation{}, err
	}

	keys := make([]string, 0, len(accessmodel.Permissions))
	for key := range accessmodel.Permissions {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := AccessExplanation{
		OrganisationID: orgID,
		Subject:        subject.String(),
		Outcomes:       make([]PermissionOutcome, 0, len(keys)),
	}
	for _, key := range keys {
		perm := accessmodel.Permissions[key]
		d, err := s.check(ctx, subject, key, accessmodel.Area(perm.Type, orgID))
		if err != nil {
			return AccessExplanation{}, err
		}
		out.Outcomes = append(out.Outcomes, PermissionOutcome{
			Key:     key,
			Allowed: d.Allowed,
			Reason:  string(d.Reason),
			Path:    filterPath(d.Path, visible, unrestricted),
		})
	}
	return out, nil
}

// nodeSet is the set of node names a viewer may be shown by name.
type nodeSet map[string]struct{}

func (s nodeSet) has(n reach.NodeRef) bool {
	_, ok := s[n.String()]
	return ok
}

// visibleNodes is every node this organisation's own structure is made of.
//
// An area node is keyed by the organisation id, so api_keys:acme is derivable
// rather than looked up. Roles are not: a role id says nothing about which
// tenant it belongs to, so the organisation's own roles are read and anything
// outside that set is treated as foreign. That is what keeps a platform role
// from being named to a tenant admin.
func (s *Service) visibleNodes(ctx context.Context, orgID string, subject reach.NodeRef, unrestricted bool) (nodeSet, error) {
	set := nodeSet{subject.String(): {}}
	if unrestricted {
		return set, nil
	}
	set[reach.Node("organisation", orgID).String()] = struct{}{}
	for _, perm := range accessmodel.Permissions {
		set[accessmodel.Area(perm.Type, orgID).String()] = struct{}{}
	}
	roles, err := s.db.Q().ListRolesByOrganisation(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for _, role := range roles {
		set[reach.Node("role", role.ID).String()] = struct{}{}
	}
	return set, nil
}

// filterPath reduces each hop to what the viewer may be told.
//
// A star node is always redacted for a restricted viewer, whatever its type.
// organisation:* is the platform's reach over every tenant, and naming it to a
// tenant admin discloses that the mechanism exists and that somebody holds it.
func filterPath(path []reach.Step, visible nodeSet, unrestricted bool) []PathHop {
	if len(path) == 0 {
		return []PathHop{}
	}
	hops := make([]PathHop, 0, len(path))
	for _, step := range path {
		hop := PathHop{Relation: step.Relation, Object: step.Object.String(), Subject: step.Subject.String()}
		if !unrestricted {
			if step.Object.IsWildcard() || !visible.has(step.Object) {
				hop.Object = RedactedNode
				hop.Redacted = true
			}
			if step.Subject.Node.IsWildcard() || !visible.has(step.Subject.Node) {
				hop.Subject = RedactedNode
				if step.Subject.IsUserset() {
					hop.Subject += "#" + step.Subject.Relation
				}
				hop.Redacted = true
			}
		}
		hops = append(hops, hop)
	}
	return hops
}
