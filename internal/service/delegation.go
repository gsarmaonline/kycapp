package service

import (
	"context"
	"sort"

	"github.com/gsarmaonline/kyc/core/reach"
	"github.com/gsarmaonline/kyc/internal/accessmodel"
	"github.com/gsarmaonline/kyc/internal/apperr"
)

// No principal grants what it does not hold.
//
// This is invariant 2 in docs/authorisation.md, and until now it was written
// down rather than enforced. reach.CanWrite states the rule for one edge, but
// nothing called it, and KYC's own write paths do not write edges: they write
// roles, role_permissions and memberships, which the live view then presents as
// edges. So the rule had to be stated in the vocabulary those paths speak.
//
// What it stopped: a member holding roles:manage alone could put billing:manage
// on a role and hold it; a member holding members:invite alone could move their
// own membership to owner. Both were one call.
//
// The check is deliberately about the *resulting* set, not the delta. You may
// not author a role that grants what you do not hold, whether the permission is
// being added now or was already there. Narrowing is always allowed, because a
// smaller set cannot escalate.

// maxRoleInheritanceDepth bounds the walk over role_extends. The table has no
// cycle constraint beyond role_id <> parent_id, so a concurrent write can still
// close a loop. The visited set terminates it; this bounds the work.
const maxRoleInheritanceDepth = 16

// grantArea is the node a permission is held at for one organisation.
//
// A role in the platform organisation writes its edges on the type's star node,
// which is the whole of staff reach. The subset rule must therefore ask the
// granter about the same node, or a staff member would be measured against a
// tenant slice they hold nothing on and every platform grant would be refused.
func (s *Service) grantArea(ctx context.Context, typeName, orgID string) reach.NodeRef {
	if platform := s.PlatformOrganisationID(ctx); platform != "" && orgID == platform {
		return accessmodel.EveryArea(typeName)
	}
	return accessmodel.Area(typeName, orgID)
}

// requireCanGrant refuses to let the caller confer a permission it does not
// already hold in the same organisation.
//
// Break-glass satisfies the rule rather than bypassing it: it holds everything
// by definition, so there is nothing a subset check could refuse. Every other
// principal, staff and recovery credentials included, is measured by the same
// walk that answers an ordinary request. No second policy engine decides who
// may grant.
func (s *Service) requireCanGrant(ctx context.Context, orgID string, keys []string) error {
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return err
	}
	if isBreakGlass(p) {
		return nil
	}
	subject, err := principalNode(p)
	if err != nil {
		return apperr.Forbidden("principal cannot grant")
	}

	// Sorted so the refusal names the same permission every time, which makes
	// the error reproducible for whoever has to explain it to an operator.
	wanted := uniqueStrings(keys)
	sort.Strings(wanted)

	for _, key := range wanted {
		perm, ok := accessmodel.Permissions[key]
		if !ok {
			// Unknown here means unknown to the evaluator, so it could never be
			// checked. Refusing is the only safe answer.
			return apperr.Validation("unknown permission " + key)
		}
		d, err := s.check(ctx, subject, key, s.grantArea(ctx, perm.Type, orgID))
		if err != nil {
			return err
		}
		if !d.Allowed {
			return apperr.Forbidden("cannot grant " + key + ": the caller does not hold it")
		}
	}
	return nil
}

// effectiveRolePermissions returns everything holding a role confers: its own
// permissions plus everything its parents hold.
//
// Assigning a role confers its resolved set, not its direct rows, so the subset
// rule has to see through role_extends. Resolving here rather than in SQL keeps
// the walk and this check reading the same inheritance the evaluator does.
func (s *Service) effectiveRolePermissions(ctx context.Context, orgID, roleID string) ([]string, error) {
	rows, err := s.db.Q().ListOperatorRoleHierarchy(ctx, orgID)
	if err != nil {
		return nil, err
	}

	// role_extends names parents by id, but the hierarchy query reports them by
	// key, so both indexes are needed to start from an id and walk by key.
	byKey := make(map[string]int, len(rows))
	start := ""
	for i, r := range rows {
		byKey[r.Key] = i
		if r.ID == roleID {
			start = r.Key
		}
	}
	if start == "" {
		// The role is not in this organisation. The callers check that
		// separately and reject it; returning nothing here would silently make
		// the subset rule vacuous if one ever stopped.
		return nil, apperr.Validation("role does not belong to organisation")
	}

	granted := map[string]struct{}{}
	seen := map[string]struct{}{}
	frontier := []string{start}

	for depth := 0; depth < maxRoleInheritanceDepth && len(frontier) > 0; depth++ {
		var next []string
		for _, key := range frontier {
			if _, done := seen[key]; done {
				continue
			}
			seen[key] = struct{}{}
			i, ok := byKey[key]
			if !ok {
				continue
			}
			for _, perm := range rows[i].PermissionKeys {
				granted[perm] = struct{}{}
			}
			next = append(next, rows[i].ParentKeys...)
		}
		frontier = next
	}

	out := make([]string, 0, len(granted))
	for key := range granted {
		out = append(out, key)
	}
	sort.Strings(out)
	return out, nil
}

// requireCanAssignRole refuses to let the caller hand out a role carrying more
// than the caller holds.
func (s *Service) requireCanAssignRole(ctx context.Context, orgID, roleID string) error {
	keys, err := s.effectiveRolePermissions(ctx, orgID, roleID)
	if err != nil {
		return err
	}
	return s.requireCanGrant(ctx, orgID, keys)
}
