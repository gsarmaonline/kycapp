package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/core/reach"
	"github.com/gsarmaonline/kyc/internal/accessmodel"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Merchant-hosted access control.
//
// A merchant declares their own scope kinds, capabilities and roles, and grants
// those roles to their app users. KYC stores the model and hands back an
// assembled grant set; the merchant's backend evaluates it locally, so KYC is
// not in the path of every request in their product.
//
// Two boundaries hold this together:
//
//   - Namespace. Merchant capabilities live in AppNamespace(orgID) and
//     can never name a KYC capability. A merchant administers their model
//     without being inside it.
//   - Opaque scope ids. KYC stores scope_id as a string it never resolves. A
//     grant naming a project that does not exist matches nothing, because no
//     resource carries that coordinate, so it fails closed and KYC avoids
//     duplicating the merchant's product structure.

var appKeyPattern = regexp.MustCompile(`^[a-z0-9_]+:[a-z0-9_]+$`)

var scopeKindPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// --- Scope kinds ---

func (s *Service) CreateAppScopeType(ctx context.Context, orgID, kind, label string) (sqlc.AppScopeType, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if !scopeKindPattern.MatchString(kind) {
		return sqlc.AppScopeType{}, apperr.Validation("kind must be lowercase letters, digits and underscores")
	}
	// No names are reserved here any more.
	//
	// global and organisation used to be refused, on the reasoning that a
	// merchant redefining them could collide with the tenancy boundary. They
	// cannot. A merchant's edges live in their own namespace and every edge
	// query filters on it, so a scope kind called global would be global:x
	// inside org:acme, reaching nothing outside it because no edge crosses.
	//
	// The blacklist was a defence in the wrong layer, and worse than redundant:
	// it implied the name carried power, which is how a graph system grows a
	// policy language bolted to its side. TestNamespacesCannotSeeEachOther is
	// what holds the boundary, and it asserts the structure rather than a rule.
	row, err := s.db.Q().CreateAppScopeType(ctx, sqlc.CreateAppScopeTypeParams{
		ID: ids.New(), OrganisationID: orgID, Kind: kind, Label: strings.TrimSpace(label),
	})
	if store.IsUniqueViolation(err) {
		return sqlc.AppScopeType{}, apperr.Conflict("scope kind already declared")
	}
	if err != nil {
		return sqlc.AppScopeType{}, err
	}
	return row, s.touchAppAccess(ctx, orgID)
}

func (s *Service) ListAppScopeTypes(ctx context.Context, orgID string) ([]sqlc.AppScopeType, error) {
	return s.db.Q().ListAppScopeTypes(ctx, orgID)
}

func (s *Service) DeleteAppScopeType(ctx context.Context, orgID, id string) error {
	if err := s.db.Q().DeleteAppScopeType(ctx, sqlc.DeleteAppScopeTypeParams{ID: id, OrganisationID: orgID}); err != nil {
		return err
	}
	return s.touchAppAccess(ctx, orgID)
}

// --- Capabilities ---

func (s *Service) CreateAppCapability(ctx context.Context, orgID, key, description string) (sqlc.AppCapability, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	if !appKeyPattern.MatchString(key) {
		return sqlc.AppCapability{}, apperr.Validation("capability must be resource:action, lowercase")
	}
	// The same shape rule KYC's own keys follow, so a merchant capability can
	// never be written in a form the vocabulary would reject, wildcards included.
	if err := ValidAppCapabilityKey(key); err != nil {
		return sqlc.AppCapability{}, apperr.Validation(err.Error())
	}
	// Split once, here, rather than in every reader. The pair is what the table
	// stores and the key derives from it, so the name and its halves cannot
	// disagree. appKeyPattern has already refused anything without both.
	resource, action, _ := strings.Cut(key, ":")
	row, err := s.db.Q().CreateAppCapability(ctx, sqlc.CreateAppCapabilityParams{
		ID: ids.New(), OrganisationID: orgID,
		Resource: resource, Action: action,
		Description: strings.TrimSpace(description),
	})
	if store.IsUniqueViolation(err) {
		return sqlc.AppCapability{}, apperr.Conflict("capability already declared")
	}
	if err != nil {
		return sqlc.AppCapability{}, err
	}
	// A new capability widens every wildcard grant that already exists, which is
	// the point of a wildcard and exactly why it has to move the version.
	return row, s.touchAppAccess(ctx, orgID)
}

func (s *Service) ListAppCapabilities(ctx context.Context, orgID string) ([]sqlc.AppCapability, error) {
	return s.db.Q().ListAppCapabilities(ctx, orgID)
}

func (s *Service) DeleteAppCapability(ctx context.Context, orgID, id string) error {
	if err := s.db.Q().DeleteAppCapability(ctx, sqlc.DeleteAppCapabilityParams{ID: id, OrganisationID: orgID}); err != nil {
		return err
	}
	return s.touchAppAccess(ctx, orgID)
}

// --- Roles ---

type AppRoleInput struct {
	Key         string
	Name        string
	Description string
	// Capabilities this role adds beyond its parents.
	Capabilities []string
	// Extends lists parent role ids. Multiple parents are allowed.
	Extends []string
}

func (s *Service) CreateAppRole(ctx context.Context, orgID string, in AppRoleInput) (sqlc.AppRole, error) {
	key := strings.ToLower(strings.TrimSpace(in.Key))
	if !scopeKindPattern.MatchString(key) {
		return sqlc.AppRole{}, apperr.Validation("role key must be lowercase letters, digits and underscores")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = key
	}
	if err := s.assertDeclaredCapabilities(ctx, orgID, in.Capabilities); err != nil {
		return sqlc.AppRole{}, err
	}

	// A role may legitimately carry nothing of its own and exist only to
	// combine parents. A nil slice would become SQL NULL against a NOT NULL
	// column, so it is normalised here rather than relying on the default.
	own := in.Capabilities
	if own == nil {
		own = []string{}
	}
	row, err := s.db.Q().CreateAppRole(ctx, sqlc.CreateAppRoleParams{
		ID: ids.New(), OrganisationID: orgID, Key: key, Name: name,
		Description: strings.TrimSpace(in.Description), OwnCapabilities: own,
	})
	if store.IsUniqueViolation(err) {
		return sqlc.AppRole{}, apperr.Conflict("role key already exists")
	}
	if err != nil {
		return sqlc.AppRole{}, err
	}
	if err := s.setAppRoleParents(ctx, orgID, row.ID, in.Extends); err != nil {
		return sqlc.AppRole{}, err
	}
	if err := s.recomputeAppRoles(ctx, orgID); err != nil {
		return sqlc.AppRole{}, err
	}
	return s.db.Q().GetAppRole(ctx, row.ID)
}

func (s *Service) UpdateAppRole(ctx context.Context, orgID, id string, in AppRoleInput) (sqlc.AppRole, error) {
	if in.Capabilities != nil {
		if err := s.assertDeclaredCapabilities(ctx, orgID, in.Capabilities); err != nil {
			return sqlc.AppRole{}, err
		}
	}
	params := sqlc.UpdateAppRoleParams{ID: id, OrganisationID: orgID}
	if in.Name != "" {
		params.Name = pgtype.Text{String: in.Name, Valid: true}
	}
	if in.Description != "" {
		params.Description = pgtype.Text{String: in.Description, Valid: true}
	}
	if in.Capabilities != nil {
		params.OwnCapabilities = in.Capabilities
	}
	row, err := s.db.Q().UpdateAppRole(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.AppRole{}, apperr.NotFound("role not found")
	}
	if err != nil {
		return sqlc.AppRole{}, err
	}
	if in.Extends != nil {
		if err := s.setAppRoleParents(ctx, orgID, id, in.Extends); err != nil {
			return sqlc.AppRole{}, err
		}
	}
	// Editing a role must reach everyone holding it, which is the whole reason
	// inheritance lives here rather than being copied at assignment.
	if err := s.recomputeAppRoles(ctx, orgID); err != nil {
		return sqlc.AppRole{}, err
	}
	return s.db.Q().GetAppRole(ctx, row.ID)
}

func (s *Service) ListAppRoles(ctx context.Context, orgID string) ([]sqlc.AppRole, error) {
	return s.db.Q().ListAppRoles(ctx, orgID)
}

func (s *Service) DeleteAppRole(ctx context.Context, orgID, id string) error {
	if err := s.db.Q().DeleteAppRole(ctx, sqlc.DeleteAppRoleParams{ID: id, OrganisationID: orgID}); err != nil {
		return err
	}
	return s.recomputeAppRoles(ctx, orgID)
}

func (s *Service) setAppRoleParents(ctx context.Context, orgID, roleID string, parents []string) error {
	if err := s.db.Q().ReplaceAppRoleExtends(ctx, roleID); err != nil {
		return err
	}
	for _, parent := range parents {
		if parent == roleID {
			return apperr.Validation("a role cannot extend itself")
		}
		row, err := s.db.Q().GetAppRole(ctx, parent)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.OrganisationID != orgID) {
			return apperr.Validation("unknown parent role: " + parent)
		}
		if err != nil {
			return err
		}
		if err := s.db.Q().AddAppRoleExtends(ctx, sqlc.AddAppRoleExtendsParams{RoleID: roleID, ParentID: parent}); err != nil {
			return err
		}
	}
	return nil
}

// recomputeAppRoles materialises inheritance for every role in the
// organisation.
//
// Recomputing the whole set is deliberate. Role counts are in the tens, edits
// are rare, and doing it wholesale removes any question of which descendants
// needed refreshing. Cycles and excessive depth are rejected here, before
// anything is written, so a bad edit cannot leave the set half-updated.
func (s *Service) recomputeAppRoles(ctx context.Context, orgID string) error {
	roles, err := s.db.Q().ListAppRoles(ctx, orgID)
	if err != nil {
		return err
	}
	edges, err := s.db.Q().ListAppRoleExtends(ctx, orgID)
	if err != nil {
		return err
	}
	parents := map[string][]string{}
	for _, e := range edges {
		parents[e.RoleID] = append(parents[e.RoleID], e.ParentID)
	}

	in := make([]reach.Set, 0, len(roles))
	for _, r := range roles {
		in = append(in, reach.Set{ID: r.ID, Own: r.OwnCapabilities, Extends: parents[r.ID]})
	}

	expanded, err := reach.ExpandSets(in, MaxAppRoleDepth)
	if err != nil {
		if errors.Is(err, reach.ErrSetCycle) {
			return apperr.Validation("role inheritance contains a cycle")
		}
		return apperr.Validation(err.Error())
	}
	for id, keys := range expanded {
		if err := s.db.Q().SetAppRoleEffectiveCapabilities(ctx, sqlc.SetAppRoleEffectiveCapabilitiesParams{
			ID: id, EffectiveCapabilities: keys,
		}); err != nil {
			return err
		}
	}
	// Every role write ends here, create, update and delete alike, so this is
	// the one place the version has to move for a role change. Narrowing a role
	// revokes for everyone holding it without touching a single grant row.
	return s.touchAppAccess(ctx, orgID)
}

func (s *Service) assertDeclaredCapabilities(ctx context.Context, orgID string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	declared, err := s.db.Q().ListAppCapabilities(ctx, orgID)
	if err != nil {
		return err
	}
	index := make(map[string]struct{}, len(declared))
	for _, c := range declared {
		index[c.Key] = struct{}{}
	}
	for _, k := range keys {
		if _, ok := index[k]; !ok {
			// Invariant 3, per namespace: a role may only carry capabilities the
			// merchant has declared, so a typo cannot reach a grant.
			return apperr.Validation("capability not declared: " + k)
		}
	}
	return nil
}

// --- Grants ---

// AppGrantInput carries exactly one subject: one customer, one group, or
// everyone. A grant with none would apply to nobody; one with two would be
// ambiguous.
//
// The exception lists are the counterpart to the wildcards beside them. A
// wildcard claims a set nobody can enumerate; an exception names the members
// that do not belong. Each narrows this grant alone, never another, which is
// what keeps grants unordered and additive.
type AppGrantInput struct {
	SubjectKind string // app_user | group | everyone
	AppUserID   string
	GroupID     string
	RoleID      string
	ScopeKind   string
	ScopeID     string
	ExpiresAt   *time.Time
	GrantedBy   string

	// AllCapabilities carries every capability in the organisation's namespace,
	// including ones declared later. RoleID must be empty when it is set.
	AllCapabilities bool
	// AllScopes carries every scope in the organisation, of every kind,
	// including kinds declared later. It is the widest a grant can be, because
	// an organisation is where a merchant's world ends. ScopeKind and ScopeID
	// must be empty when it is set, the same way RoleID must be empty under
	// AllCapabilities: a grant cannot be both everywhere and somewhere.
	AllScopes  bool
	Constraint string // "" | self_subject
}

// AppScopeRef is one excluded scope. Kind and id stay paired, which is why the
// column is JSONB rather than two parallel arrays.
type AppScopeRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

const (
	subjectAppUser  = "app_user"
	subjectGroup    = "group"
	subjectEveryone = "everyone"
)

// CreateAppGrant issues a grant and moves the version a merchant caches
// against. The write has three shapes -- one customer, one group, everyone --
// and each returns from its own branch, so the bump lives in a wrapper rather
// than being repeated three times and forgotten on the fourth.
func (s *Service) CreateAppGrant(ctx context.Context, orgID string, in AppGrantInput) (sqlc.AppGrant, error) {
	row, err := s.createAppGrant(ctx, orgID, in)
	if err != nil {
		return sqlc.AppGrant{}, err
	}
	if err := s.touchAppAccess(ctx, orgID); err != nil {
		return sqlc.AppGrant{}, err
	}
	return row, nil
}

func (s *Service) createAppGrant(ctx context.Context, orgID string, in AppGrantInput) (sqlc.AppGrant, error) {
	kind := strings.ToLower(strings.TrimSpace(in.ScopeKind))
	scopeID := strings.TrimSpace(in.ScopeID)

	declared, err := s.db.Q().ListAppScopeTypes(ctx, orgID)
	if err != nil {
		return sqlc.AppGrant{}, err
	}
	declaredKinds := make(map[string]struct{}, len(declared))
	for _, d := range declared {
		declaredKinds[d.Kind] = struct{}{}
	}

	switch {
	case in.AllScopes && (kind != "" || scopeID != ""):
		// A grant is everywhere or somewhere, never both. Accepting a kind here
		// would leave a row whose scope columns are ignored, and the next reader
		// would reasonably believe them.
		return sqlc.AppGrant{}, apperr.Validation("an organisation-wide grant carries no scope_kind or scope_id")
	case in.AllScopes:
		kind, scopeID = "", ""
	case kind == "" || scopeID == "":
		return sqlc.AppGrant{}, apperr.Validation("scope_kind and scope_id are required unless all_scopes is set")
	default:
		if _, ok := declaredKinds[kind]; !ok {
			// The kind is checked; the id deliberately is not, beyond the
			// wildcard. An undeclared kind would silently match nothing, which
			// is the worst way to fail.
			return sqlc.AppGrant{}, apperr.Validation("scope kind not declared: " + kind)
		}
	}

	// A grant carries a role or the wildcard, never both and never neither.
	roleID := pgtype.Text{}
	switch {
	case in.AllCapabilities && in.RoleID != "":
		return sqlc.AppGrant{}, apperr.Validation("a wildcard grant carries no role")
	case !in.AllCapabilities && in.RoleID == "":
		return sqlc.AppGrant{}, apperr.Validation("role_id is required unless all_capabilities is set")
	case !in.AllCapabilities:
		role, err := s.db.Q().GetAppRole(ctx, in.RoleID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && role.OrganisationID != orgID) {
			return sqlc.AppGrant{}, apperr.Validation("unknown role")
		}
		if err != nil {
			return sqlc.AppGrant{}, err
		}
		roleID = textArg(in.RoleID)
	}

	if err != nil {
		return sqlc.AppGrant{}, err
	}
	if err != nil {
		return sqlc.AppGrant{}, err
	}
	constraint := strings.TrimSpace(in.Constraint)
	if constraint != "" && constraint != string(AppSelfSubject) {
		return sqlc.AppGrant{}, apperr.Validation("unknown constraint: " + constraint)
	}

	var expires pgtype.Timestamptz
	if in.ExpiresAt != nil {
		expires = pgtype.Timestamptz{Time: in.ExpiresAt.UTC(), Valid: true}
	}

	subject := strings.TrimSpace(in.SubjectKind)
	if subject == "" {
		// Older callers sent no kind and inferred it from whichever id was set.
		subject = subjectAppUser
		if in.GroupID != "" {
			subject = subjectGroup
		}
	}

	switch subject {
	case subjectEveryone:
		if in.AppUserID != "" || in.GroupID != "" {
			return sqlc.AppGrant{}, apperr.Validation("an everyone grant names no subject")
		}
		return s.db.Q().CreateAppEveryoneGrant(ctx, sqlc.CreateAppEveryoneGrantParams{
			ID: ids.New(), OrganisationID: orgID, RoleID: roleID,
			ScopeKind: kind, ScopeID: scopeID, ExpiresAt: expires, GrantedBy: in.GrantedBy,
			AllCapabilities: in.AllCapabilities, AllScopes: in.AllScopes, ConstraintKind: constraint,
		})

	case subjectGroup:
		if in.GroupID == "" || in.AppUserID != "" {
			return sqlc.AppGrant{}, apperr.Validation("a group grant needs exactly a group_id")
		}
		group, err := s.db.Q().GetAppUserGroup(ctx, in.GroupID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && group.OrganisationID != orgID) {
			return sqlc.AppGrant{}, apperr.Validation("unknown group")
		}
		if err != nil {
			return sqlc.AppGrant{}, err
		}
		return s.db.Q().CreateAppGroupGrant(ctx, sqlc.CreateAppGroupGrantParams{
			ID: ids.New(), OrganisationID: orgID, GroupID: textArg(in.GroupID), RoleID: roleID,
			ScopeKind: kind, ScopeID: scopeID, ExpiresAt: expires, GrantedBy: in.GrantedBy,
			AllCapabilities: in.AllCapabilities, AllScopes: in.AllScopes, ConstraintKind: constraint,
		})

	case subjectAppUser:
		if in.AppUserID == "" || in.GroupID != "" {
			return sqlc.AppGrant{}, apperr.Validation("a customer grant needs exactly an app_user_id")
		}
		return s.db.Q().CreateAppUserGrant(ctx, sqlc.CreateAppUserGrantParams{
			ID: ids.New(), OrganisationID: orgID, AppUserID: textArg(in.AppUserID), RoleID: roleID,
			ScopeKind: kind, ScopeID: scopeID, ExpiresAt: expires, GrantedBy: in.GrantedBy,
			AllCapabilities: in.AllCapabilities, AllScopes: in.AllScopes, ConstraintKind: constraint,
		})
	}
	return sqlc.AppGrant{}, apperr.Validation("unknown subject_kind: " + subject)
}

// --- Groups ---
//
// A group answers "which principals"; a scope answers "which resources". Keeping
// them separate is what lets a grant gain a subject without changing shape.
//
// Membership is an explicit list. Rules over attributes would make every
// attribute write a permission change, which needs its own audit story and is
// deliberately not built here.

type AppUserGroupInput struct {
	Key         string
	Name        string
	Description string
	// Parents are groups this one extends. A member of this group is treated as
	// a member of every parent, which is the relation app_role_extends already
	// gave roles. Multiple parents are allowed because membership only ever
	// adds, so a diamond resolves the same way whatever order it is walked.
	Parents []string
}

func (s *Service) CreateAppUserGroup(ctx context.Context, orgID string, in AppUserGroupInput) (sqlc.AppUserGroup, error) {
	key := strings.ToLower(strings.TrimSpace(in.Key))
	if !scopeKindPattern.MatchString(key) {
		return sqlc.AppUserGroup{}, apperr.Validation("group key must be lowercase letters, digits and underscores")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = key
	}
	row, err := s.db.Q().CreateAppUserGroup(ctx, sqlc.CreateAppUserGroupParams{
		ID: ids.New(), OrganisationID: orgID, Key: key, Name: name,
		Description: strings.TrimSpace(in.Description),
	})
	if store.IsUniqueViolation(err) {
		return sqlc.AppUserGroup{}, apperr.Conflict("group key already exists")
	}
	if err != nil {
		return sqlc.AppUserGroup{}, err
	}
	if err := s.setAppUserGroupParents(ctx, orgID, row.ID, in.Parents); err != nil {
		return sqlc.AppUserGroup{}, err
	}
	if err := s.recomputeAppUserGroups(ctx, orgID); err != nil {
		return sqlc.AppUserGroup{}, err
	}
	return row, nil
}

// setAppUserGroupParents replaces a group's parents, mirroring
// setAppRoleParents exactly. Roles and groups are one mechanism, so a
// divergence between these two functions is a bug rather than a design.
func (s *Service) setAppUserGroupParents(ctx context.Context, orgID, groupID string, parents []string) error {
	if err := s.db.Q().ReplaceAppUserGroupExtends(ctx, groupID); err != nil {
		return err
	}
	for _, parent := range parents {
		if parent == groupID {
			return apperr.Validation("a group cannot extend itself")
		}
		row, err := s.db.Q().GetAppUserGroup(ctx, parent)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.OrganisationID != orgID) {
			return apperr.Validation("unknown parent group: " + parent)
		}
		if err != nil {
			return err
		}
		if err := s.db.Q().AddAppUserGroupExtends(ctx, sqlc.AddAppUserGroupExtendsParams{
			GroupID: groupID, ParentID: parent,
		}); err != nil {
			return err
		}
	}
	return nil
}

// recomputeAppUserGroups materialises nesting for every group in the
// organisation.
//
// It stores, per group, the set of groups a member of it effectively belongs
// to: itself plus everything it extends, transitively. Grant assembly joins
// straight through that array and walks nothing on the read path, which is the
// same trade a role's effective_capabilities makes, and for the same reason: a
// merchant's backend reads the assembled answer without this graph.
//
// Recomputing the whole set is deliberate, exactly as recomputeAppRoles does.
// Group counts are small, edits are rare, and doing it wholesale removes any
// question of which descendants needed refreshing.
func (s *Service) recomputeAppUserGroups(ctx context.Context, orgID string) error {
	groups, err := s.db.Q().ListAppUserGroups(ctx, orgID)
	if err != nil {
		return err
	}
	edges, err := s.db.Q().ListAppUserGroupExtends(ctx, orgID)
	if err != nil {
		return err
	}
	parents := map[string][]string{}
	for _, e := range edges {
		parents[e.GroupID] = append(parents[e.GroupID], e.ParentID)
	}

	in := make([]reach.Set, 0, len(groups))
	for _, g := range groups {
		// Own is the group's own id, so the expansion answers "which groups does
		// a member of this one belong to" and always contains the group itself.
		in = append(in, reach.Set{ID: g.ID, Own: []string{g.ID}, Extends: parents[g.ID]})
	}

	expanded, err := reach.ExpandSets(in, MaxAppRoleDepth)
	if err != nil {
		if errors.Is(err, reach.ErrSetCycle) {
			return apperr.Validation("group nesting contains a cycle")
		}
		return apperr.Validation(err.Error())
	}
	for id, reached := range expanded {
		if err := s.db.Q().SetAppUserGroupEffectiveParents(ctx, sqlc.SetAppUserGroupEffectiveParentsParams{
			ID: id, EffectiveParentIds: reached,
		}); err != nil {
			return err
		}
	}
	// Every group write ends here, so this is the one place the version has to
	// move for a nesting change. Re-parenting a group revokes and grants for
	// everyone inside it without touching a membership row.
	return s.touchAppAccess(ctx, orgID)
}

func (s *Service) ListAppUserGroups(ctx context.Context, orgID string) ([]sqlc.ListAppUserGroupsRow, error) {
	return s.db.Q().ListAppUserGroups(ctx, orgID)
}

func (s *Service) UpdateAppUserGroup(ctx context.Context, orgID, id string, in AppUserGroupInput) (sqlc.AppUserGroup, error) {
	params := sqlc.UpdateAppUserGroupParams{ID: id, OrganisationID: orgID}
	if in.Name != "" {
		params.Name = pgtype.Text{String: in.Name, Valid: true}
	}
	if in.Description != "" {
		params.Description = pgtype.Text{String: in.Description, Valid: true}
	}
	row, err := s.db.Q().UpdateAppUserGroup(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.AppUserGroup{}, apperr.NotFound("group not found")
	}
	if err != nil {
		return sqlc.AppUserGroup{}, err
	}
	// Parents are replaced wholesale, like a role's. A nil slice therefore
	// clears them, which is what an edit form submitting no parents means.
	if err := s.setAppUserGroupParents(ctx, orgID, row.ID, in.Parents); err != nil {
		return sqlc.AppUserGroup{}, err
	}
	if err := s.recomputeAppUserGroups(ctx, orgID); err != nil {
		return sqlc.AppUserGroup{}, err
	}
	return row, nil
}

func (s *Service) DeleteAppUserGroup(ctx context.Context, orgID, id string) error {
	if err := s.db.Q().DeleteAppUserGroup(ctx, sqlc.DeleteAppUserGroupParams{ID: id, OrganisationID: orgID}); err != nil {
		return err
	}
	// The cascade removes the extends rows, but every descendant still carries
	// the deleted id in its stored expansion. Leaving those behind would keep
	// joining grants through a group that no longer exists.
	return s.recomputeAppUserGroups(ctx, orgID)
}

// SetAppUserGroupMember adds or removes one member, after checking the group and
// the app user belong to the same organisation. Without that check a merchant
// could add another tenant's customer to their group.
func (s *Service) SetAppUserGroupMember(ctx context.Context, orgID, groupID, appUserID string, member bool) error {
	group, err := s.db.Q().GetAppUserGroup(ctx, groupID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && group.OrganisationID != orgID) {
		return apperr.NotFound("group not found")
	}
	if err != nil {
		return err
	}
	user, err := s.GetAppUser(ctx, appUserID)
	if err != nil {
		return err
	}
	if user.OrganisationID != orgID {
		return apperr.NotFound("app user not found")
	}
	if member {
		if err := s.db.Q().AddAppUserToGroup(ctx, sqlc.AddAppUserToGroupParams{GroupID: groupID, AppUserID: appUserID}); err != nil {
			return err
		}
		return s.touchAppAccess(ctx, orgID)
	}
	// Removing somebody revokes everything the group carried for them, and a
	// removal deletes a row rather than writing one, so nothing derived from
	// the surviving rows could ever have seen it.
	if err := s.db.Q().RemoveAppUserFromGroup(ctx, sqlc.RemoveAppUserFromGroupParams{GroupID: groupID, AppUserID: appUserID}); err != nil {
		return err
	}
	return s.touchAppAccess(ctx, orgID)
}

// ListAppUserGroupParents returns the ids a group extends, after checking the
// group belongs to the organisation. Without that check one merchant could read
// another's nesting by guessing an id.
func (s *Service) ListAppUserGroupParents(ctx context.Context, orgID, groupID string) ([]string, error) {
	if _, err := s.GetAppUserGroupByID(ctx, orgID, groupID); err != nil {
		return nil, err
	}
	ids, err := s.db.Q().ListAppUserGroupParents(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}

func (s *Service) ListAppUserGroupMembers(ctx context.Context, groupID string) ([]sqlc.ListAppUserGroupMembersRow, error) {
	return s.db.Q().ListAppUserGroupMembers(ctx, groupID)
}

func (s *Service) ListGroupsForAppUser(ctx context.Context, appUserID string) ([]sqlc.ListGroupsForAppUserRow, error) {
	return s.db.Q().ListGroupsForAppUser(ctx, appUserID)
}

func (s *Service) ListAppGrantsForOrg(ctx context.Context, orgID string, limit int32) ([]sqlc.ListAppGrantsForOrgRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	return s.db.Q().ListAppGrantsForOrg(ctx, sqlc.ListAppGrantsForOrgParams{OrganisationID: orgID, Limit: limit})
}

func (s *Service) DeleteAppGrant(ctx context.Context, orgID, id string) error {
	if err := s.db.Q().DeleteAppGrant(ctx, sqlc.DeleteAppGrantParams{ID: id, OrganisationID: orgID}); err != nil {
		return err
	}
	return s.touchAppAccess(ctx, orgID)
}

// touchAppAccess moves the version a merchant caches against.
//
// It has to be called by hand after every write, which is the honest cost of
// deriving the version from a counter rather than from the rows. The
// alternative was the bug this replaced: AppAccessVersion took
// MAX(created_at) over app_grants and over group members, so a delete moved
// nothing at all, and deleting the *newest* grant moved the number backwards.
// A cache holding the higher value read that as "still current", and the
// permission it kept serving was the one that had just been revoked.
//
// Over-invalidating is the safe direction, so this is organisation-wide rather
// than per customer. A grant made to a group changes what its members hold
// without touching their rows, and no per-user counter can see that.
//
// It shares reach_namespace_versions with the edge graph deliberately. The two
// stores answer the same question today and are meant to become one, so a
// single counter means the version does not have to be re-derived when they
// merge.
func (s *Service) touchAppAccess(ctx context.Context, orgID string) error {
	_, err := s.db.Q().BumpReachNamespaceVersion(ctx, accessmodel.MerchantNamespace(orgID))
	return err
}

// AppAccessSet is what a merchant's backend caches and evaluates against.
type AppAccessSet struct {
	AppUserID string
	Namespace string
	Version   int64
	Grants    []AppGrant
}

// AppAccessFor assembles a customer's grant set.
//
// Capabilities come from the role's materialised set, so this never walks the
// inheritance graph however deep the merchant built it. The result is shaped as
// AppGrants, which is the wire shape the merchant's backend evaluates against
// in its own process.
func (s *Service) AppAccessFor(ctx context.Context, orgID, appUserID string) (AppAccessSet, error) {
	rows, err := s.db.Q().ListAppGrantsForUser(ctx, sqlc.ListAppGrantsForUserParams{
		AppUserID: textArg(appUserID), OrganisationID: orgID,
	})
	if err != nil {
		return AppAccessSet{}, err
	}
	// The version is a counter, not a MAX() over the rows. A revocation removes
	// rows, so anything derived from the surviving ones cannot see it, and that
	// is precisely the change a cache must not miss.
	version, err := s.db.Q().GetReachNamespaceVersion(ctx, accessmodel.MerchantNamespace(orgID))
	if err != nil {
		return AppAccessSet{}, err
	}

	out := AppAccessSet{AppUserID: appUserID, Namespace: AppNamespace(orgID), Version: version}
	for _, r := range rows {
		caps := append([]string(nil), r.EffectiveCapabilities...)
		if len(caps) == 0 && !r.AllCapabilities {
			// An empty grant is invalid and would be rejected by the evaluator.
			// A role carrying nothing simply contributes nothing. A wildcard
			// carries plenty while listing none, so it is exempt.
			continue
		}
		g := AppGrant{
			ID:           r.ID,
			Scope:        AppScope{Kind: r.ScopeKind, ID: r.ScopeID},
			Capabilities: caps,
			Constraint:   AppConstraint(r.ConstraintKind),
			Source:       grantSource(r.SubjectKind, r.GroupKey, r.RoleKey, r.AllCapabilities),
		}
		g.AllScopes = r.AllScopes
		if r.AllCapabilities {
			g.AllCapabilities = true
		}
		if r.ExpiresAt.Valid {
			t := r.ExpiresAt.Time
			g.ExpiresAt = &t
		}
		out.Grants = append(out.Grants, g)
	}
	return out, nil
}

// grantSource records how a customer came to hold a grant. Provenance is the
// whole answer to "why does this person have this?", and a grant that arrives
// through a group or through the everyone rule is exactly the case where nobody
// can work it out from the grants list alone.
func grantSource(subjectKind, groupKey, roleKey string, wildcard bool) string {
	what := "app-role:" + roleKey
	if wildcard {
		what = "all-capabilities"
	}
	switch subjectKind {
	case subjectEveryone:
		return "everyone " + what
	case subjectGroup:
		return "group:" + groupKey + " " + what
	default:
		return what
	}
}

// --- Single-object reads and edits ---
//
// The admin UI follows an index / new / show / edit shape for every object, so
// each of these needs to be fetchable and editable on its own rather than only
// as a row in a list.

func (s *Service) GetAppScopeType(ctx context.Context, orgID, id string) (sqlc.AppScopeType, error) {
	row, err := s.db.Q().GetAppScopeType(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.OrganisationID != orgID) {
		return sqlc.AppScopeType{}, apperr.NotFound("scope kind not found")
	}
	return row, err
}

func (s *Service) UpdateAppScopeType(ctx context.Context, orgID, id, label string) (sqlc.AppScopeType, error) {
	row, err := s.db.Q().UpdateAppScopeType(ctx, sqlc.UpdateAppScopeTypeParams{
		ID: id, OrganisationID: orgID,
		Label: pgtype.Text{String: strings.TrimSpace(label), Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.AppScopeType{}, apperr.NotFound("scope kind not found")
	}
	return row, err
}

func (s *Service) GetAppCapability(ctx context.Context, orgID, id string) (sqlc.AppCapability, error) {
	row, err := s.db.Q().GetAppCapability(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.OrganisationID != orgID) {
		return sqlc.AppCapability{}, apperr.NotFound("capability not found")
	}
	return row, err
}

func (s *Service) UpdateAppCapability(ctx context.Context, orgID, id, description string) (sqlc.AppCapability, error) {
	row, err := s.db.Q().UpdateAppCapability(ctx, sqlc.UpdateAppCapabilityParams{
		ID: id, OrganisationID: orgID,
		Description: pgtype.Text{String: strings.TrimSpace(description), Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.AppCapability{}, apperr.NotFound("capability not found")
	}
	return row, err
}

// AppRoleView is a role with the ids of the roles it builds on, which an edit
// form needs and the list does not.
type AppRoleView struct {
	Role    sqlc.AppRole
	Extends []string
}

func (s *Service) GetAppRoleView(ctx context.Context, orgID, id string) (AppRoleView, error) {
	role, err := s.db.Q().GetAppRole(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && role.OrganisationID != orgID) {
		return AppRoleView{}, apperr.NotFound("role not found")
	}
	if err != nil {
		return AppRoleView{}, err
	}
	parents, err := s.db.Q().ListAppRoleParents(ctx, id)
	if err != nil {
		return AppRoleView{}, err
	}
	return AppRoleView{Role: role, Extends: parents}, nil
}

func (s *Service) GetAppUserGroupByID(ctx context.Context, orgID, id string) (sqlc.AppUserGroup, error) {
	row, err := s.db.Q().GetAppUserGroup(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.OrganisationID != orgID) {
		return sqlc.AppUserGroup{}, apperr.NotFound("group not found")
	}
	return row, err
}
