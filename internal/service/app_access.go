package service

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/core/access"
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
//   - Namespace. Merchant capabilities live in access.OrgNamespace(orgID) and
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
	if kind == access.ScopeGlobal || kind == access.ScopeOrganisation {
		// These are KYC's own levels. Letting a merchant redefine them would
		// let a declared scope collide with the tenancy boundary.
		return sqlc.AppScopeType{}, apperr.Validation("kind is reserved: " + kind)
	}
	row, err := s.db.Q().CreateAppScopeType(ctx, sqlc.CreateAppScopeTypeParams{
		ID: ids.New(), OrganisationID: orgID, Kind: kind, Label: strings.TrimSpace(label),
	})
	if store.IsUniqueViolation(err) {
		return sqlc.AppScopeType{}, apperr.Conflict("scope kind already declared")
	}
	return row, err
}

func (s *Service) ListAppScopeTypes(ctx context.Context, orgID string) ([]sqlc.AppScopeType, error) {
	return s.db.Q().ListAppScopeTypes(ctx, orgID)
}

func (s *Service) DeleteAppScopeType(ctx context.Context, orgID, id string) error {
	return s.db.Q().DeleteAppScopeType(ctx, sqlc.DeleteAppScopeTypeParams{ID: id, OrganisationID: orgID})
}

// --- Capabilities ---

func (s *Service) CreateAppCapability(ctx context.Context, orgID, key, description string) (sqlc.AppCapability, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	if !appKeyPattern.MatchString(key) {
		return sqlc.AppCapability{}, apperr.Validation("capability must be resource:action, lowercase")
	}
	// Validate through the evaluator's own rules so a merchant capability can
	// never be shaped in a way KYC's registry would reject, including wildcards.
	if _, err := access.NewRegistry(access.OrgNamespace(orgID), key); err != nil {
		return sqlc.AppCapability{}, apperr.Validation(err.Error())
	}
	row, err := s.db.Q().CreateAppCapability(ctx, sqlc.CreateAppCapabilityParams{
		ID: ids.New(), OrganisationID: orgID, Key: key, Description: strings.TrimSpace(description),
	})
	if store.IsUniqueViolation(err) {
		return sqlc.AppCapability{}, apperr.Conflict("capability already declared")
	}
	return row, err
}

func (s *Service) ListAppCapabilities(ctx context.Context, orgID string) ([]sqlc.AppCapability, error) {
	return s.db.Q().ListAppCapabilities(ctx, orgID)
}

func (s *Service) DeleteAppCapability(ctx context.Context, orgID, id string) error {
	return s.db.Q().DeleteAppCapability(ctx, sqlc.DeleteAppCapabilityParams{ID: id, OrganisationID: orgID})
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

	ns := access.OrgNamespace(orgID)
	in := make([]access.Role, 0, len(roles))
	for _, r := range roles {
		own := make([]access.Capability, 0, len(r.OwnCapabilities))
		for _, k := range r.OwnCapabilities {
			own = append(own, access.Capability{Namespace: ns, Key: k})
		}
		in = append(in, access.Role{ID: r.ID, Own: own, Extends: parents[r.ID]})
	}

	expanded, err := access.ExpandRoles(in)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrRoleCycle):
			return apperr.Validation("role inheritance contains a cycle")
		case errors.Is(err, access.ErrRoleDepth):
			return apperr.Validation(err.Error())
		default:
			return apperr.Validation(err.Error())
		}
	}
	for id, caps := range expanded {
		keys := make([]string, 0, len(caps))
		for _, c := range caps {
			keys = append(keys, c.Key)
		}
		if err := s.db.Q().SetAppRoleEffectiveCapabilities(ctx, sqlc.SetAppRoleEffectiveCapabilitiesParams{
			ID: id, EffectiveCapabilities: keys,
		}); err != nil {
			return err
		}
	}
	return nil
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
	AllCapabilities    bool
	ExceptCapabilities []string
	ExceptScopes       []AppScopeRef
	ExceptAppUserIDs   []string
	Constraint         string // "" | self_subject
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

func (s *Service) CreateAppGrant(ctx context.Context, orgID string, in AppGrantInput) (sqlc.AppGrant, error) {
	kind := strings.ToLower(strings.TrimSpace(in.ScopeKind))
	scopeID := strings.TrimSpace(in.ScopeID)
	if kind == "" || scopeID == "" {
		return sqlc.AppGrant{}, apperr.Validation("scope_kind and scope_id are required")
	}
	declared, err := s.db.Q().ListAppScopeTypes(ctx, orgID)
	if err != nil {
		return sqlc.AppGrant{}, err
	}
	declaredKinds := make(map[string]struct{}, len(declared))
	for _, d := range declared {
		declaredKinds[d.Kind] = struct{}{}
	}
	if _, ok := declaredKinds[kind]; !ok {
		// The kind is checked; the id deliberately is not. An undeclared kind
		// would silently match nothing, which is the worst way to fail.
		return sqlc.AppGrant{}, apperr.Validation("scope kind not declared: " + kind)
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

	exceptCaps, err := s.validateExceptCapabilities(ctx, orgID, in)
	if err != nil {
		return sqlc.AppGrant{}, err
	}
	exceptScopes, err := validateExceptScopes(in.ExceptScopes, declaredKinds)
	if err != nil {
		return sqlc.AppGrant{}, err
	}
	constraint := strings.TrimSpace(in.Constraint)
	if constraint != "" && constraint != string(access.SelfSubject) {
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
		excluded, err := s.validateExcludedUsers(ctx, orgID, in.ExceptAppUserIDs)
		if err != nil {
			return sqlc.AppGrant{}, err
		}
		return s.db.Q().CreateAppEveryoneGrant(ctx, sqlc.CreateAppEveryoneGrantParams{
			ID: ids.New(), OrganisationID: orgID, RoleID: roleID,
			ScopeKind: kind, ScopeID: scopeID, ExpiresAt: expires, GrantedBy: in.GrantedBy,
			AllCapabilities: in.AllCapabilities, ExceptCapabilities: exceptCaps,
			ExceptScopes: exceptScopes, ExceptAppUserIds: excluded, ConstraintKind: constraint,
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
		excluded, err := s.validateExcludedUsers(ctx, orgID, in.ExceptAppUserIDs)
		if err != nil {
			return sqlc.AppGrant{}, err
		}
		return s.db.Q().CreateAppGroupGrant(ctx, sqlc.CreateAppGroupGrantParams{
			ID: ids.New(), OrganisationID: orgID, GroupID: textArg(in.GroupID), RoleID: roleID,
			ScopeKind: kind, ScopeID: scopeID, ExpiresAt: expires, GrantedBy: in.GrantedBy,
			AllCapabilities: in.AllCapabilities, ExceptCapabilities: exceptCaps,
			ExceptScopes: exceptScopes, ExceptAppUserIds: excluded, ConstraintKind: constraint,
		})

	case subjectAppUser:
		if in.AppUserID == "" || in.GroupID != "" {
			return sqlc.AppGrant{}, apperr.Validation("a customer grant needs exactly an app_user_id")
		}
		if len(in.ExceptAppUserIDs) > 0 {
			// Excluding people from a grant that names one person is either a
			// mistake or a way to write a grant that reaches nobody.
			return sqlc.AppGrant{}, apperr.Validation("except_app_user_ids applies only to group and everyone grants")
		}
		return s.db.Q().CreateAppUserGrant(ctx, sqlc.CreateAppUserGrantParams{
			ID: ids.New(), OrganisationID: orgID, AppUserID: textArg(in.AppUserID), RoleID: roleID,
			ScopeKind: kind, ScopeID: scopeID, ExpiresAt: expires, GrantedBy: in.GrantedBy,
			AllCapabilities: in.AllCapabilities, ExceptCapabilities: exceptCaps,
			ExceptScopes: exceptScopes, ConstraintKind: constraint,
		})
	}
	return sqlc.AppGrant{}, apperr.Validation("unknown subject_kind: " + subject)
}

// validateExceptCapabilities refuses carve-outs that would mislead: one without
// a wildcard does nothing, and one naming an undeclared capability protects
// against nothing while reading as though it does.
func (s *Service) validateExceptCapabilities(ctx context.Context, orgID string, in AppGrantInput) ([]string, error) {
	out := make([]string, 0, len(in.ExceptCapabilities))
	if len(in.ExceptCapabilities) == 0 {
		return out, nil
	}
	if !in.AllCapabilities {
		return nil, apperr.Validation("except_capabilities needs all_capabilities; a role already lists what it grants")
	}
	declared, err := s.db.Q().ListAppCapabilities(ctx, orgID)
	if err != nil {
		return nil, err
	}
	known := make(map[string]struct{}, len(declared))
	for _, d := range declared {
		known[d.Key] = struct{}{}
	}
	for _, raw := range in.ExceptCapabilities {
		key := strings.ToLower(strings.TrimSpace(raw))
		if key == "" {
			continue
		}
		if _, ok := known[key]; !ok {
			return nil, apperr.Validation("capability not declared: " + key)
		}
		out = append(out, key)
	}
	return out, nil
}

// validateExceptScopes keeps an excluded scope to a declared kind, for the same
// reason a granted one is checked: an undeclared kind excludes nothing.
func validateExceptScopes(refs []AppScopeRef, declaredKinds map[string]struct{}) (json.RawMessage, error) {
	out := make([]AppScopeRef, 0, len(refs))
	for _, r := range refs {
		kind := strings.ToLower(strings.TrimSpace(r.Kind))
		id := strings.TrimSpace(r.ID)
		if kind == "" || id == "" {
			return nil, apperr.Validation("an excluded scope needs a kind and an id")
		}
		if _, ok := declaredKinds[kind]; !ok {
			return nil, apperr.Validation("scope kind not declared: " + kind)
		}
		out = append(out, AppScopeRef{Kind: kind, ID: id})
	}
	return json.Marshal(out)
}

// validateExcludedUsers checks every excluded customer belongs to the
// organisation, so a typo cannot silently exclude nobody.
func (s *Service) validateExcludedUsers(ctx context.Context, orgID string, ids []string) ([]string, error) {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		u, err := s.db.Q().GetAppUser(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && u.OrganisationID != orgID) {
			return nil, apperr.Validation("unknown customer in except_app_user_ids")
		}
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
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
	return row, err
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
	return row, err
}

func (s *Service) DeleteAppUserGroup(ctx context.Context, orgID, id string) error {
	return s.db.Q().DeleteAppUserGroup(ctx, sqlc.DeleteAppUserGroupParams{ID: id, OrganisationID: orgID})
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
		return s.db.Q().AddAppUserToGroup(ctx, sqlc.AddAppUserToGroupParams{GroupID: groupID, AppUserID: appUserID})
	}
	return s.db.Q().RemoveAppUserFromGroup(ctx, sqlc.RemoveAppUserFromGroupParams{GroupID: groupID, AppUserID: appUserID})
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
	return s.db.Q().DeleteAppGrant(ctx, sqlc.DeleteAppGrantParams{ID: id, OrganisationID: orgID})
}

// AppAccessSet is what a merchant's backend caches and evaluates against.
type AppAccessSet struct {
	AppUserID string
	Namespace string
	Version   int64
	Grants    []access.Grant
}

// AppAccessFor assembles a customer's grant set.
//
// Capabilities come from the role's materialised set, so this never walks the
// inheritance graph however deep the merchant built it. The result is shaped as
// core/access grants because the merchant's SDK evaluates it with the same
// Decide the API uses.
func (s *Service) AppAccessFor(ctx context.Context, orgID, appUserID string) (AppAccessSet, error) {
	rows, err := s.db.Q().ListAppGrantsForUser(ctx, sqlc.ListAppGrantsForUserParams{
		AppUserID: textArg(appUserID), OrganisationID: orgID,
	})
	if err != nil {
		return AppAccessSet{}, err
	}
	version, err := s.db.Q().AppAccessVersion(ctx, sqlc.AppAccessVersionParams{
		AppUserID: appUserID, OrganisationID: orgID,
	})
	if err != nil {
		return AppAccessSet{}, err
	}

	ns := access.OrgNamespace(orgID)
	out := AppAccessSet{AppUserID: appUserID, Namespace: ns, Version: version}
	for _, r := range rows {
		caps := make([]access.Capability, 0, len(r.EffectiveCapabilities))
		for _, k := range r.EffectiveCapabilities {
			caps = append(caps, access.Capability{Namespace: ns, Key: k})
		}
		if len(caps) == 0 && !r.AllCapabilities {
			// An empty grant is invalid and would be rejected by the evaluator.
			// A role carrying nothing simply contributes nothing. A wildcard
			// carries plenty while listing none, so it is exempt.
			continue
		}
		g := access.Grant{
			ID:           r.ID,
			Scope:        access.Scope{Kind: r.ScopeKind, ID: r.ScopeID},
			Except:       decodeExceptScopes(r.ExceptScopes),
			Capabilities: caps,
			Constraint:   access.Constraint(r.ConstraintKind),
			Source:       grantSource(r.SubjectKind, r.GroupKey, r.RoleKey, r.AllCapabilities),
		}
		if r.AllCapabilities {
			g.AllCapabilitiesIn = ns
			for _, k := range r.ExceptCapabilities {
				g.ExceptCapabilities = append(g.ExceptCapabilities, access.Capability{Namespace: ns, Key: k})
			}
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

// decodeExceptScopes turns the stored JSON into scopes. A row that will not
// parse is treated as having no exclusions, which widens the grant, so it is
// stored as JSONB and validated on the way in rather than trusted here.
func decodeExceptScopes(raw json.RawMessage) []access.Scope {
	if len(raw) == 0 {
		return nil
	}
	var refs []AppScopeRef
	if err := json.Unmarshal(raw, &refs); err != nil {
		return nil
	}
	out := make([]access.Scope, 0, len(refs))
	for _, r := range refs {
		out = append(out, access.Scope{Kind: r.Kind, ID: r.ID})
	}
	return out
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
