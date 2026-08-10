package service

import (
	"context"
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

// AppGrantInput carries exactly one subject: an app user or a group. A grant
// with neither would apply to nobody; one with both would be ambiguous.
type AppGrantInput struct {
	AppUserID string
	GroupID   string
	RoleID    string
	ScopeKind string
	ScopeID   string
	ExpiresAt *time.Time
	GrantedBy string
}

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
	known := false
	for _, d := range declared {
		if d.Kind == kind {
			known = true
		}
	}
	if !known {
		// The kind is checked; the id deliberately is not. An undeclared kind
		// would silently match nothing, which is the worst way to fail.
		return sqlc.AppGrant{}, apperr.Validation("scope kind not declared: " + kind)
	}
	role, err := s.db.Q().GetAppRole(ctx, in.RoleID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && role.OrganisationID != orgID) {
		return sqlc.AppGrant{}, apperr.Validation("unknown role")
	}
	if err != nil {
		return sqlc.AppGrant{}, err
	}

	var expires pgtype.Timestamptz
	if in.ExpiresAt != nil {
		expires = pgtype.Timestamptz{Time: in.ExpiresAt.UTC(), Valid: true}
	}
	if (in.AppUserID == "") == (in.GroupID == "") {
		return sqlc.AppGrant{}, apperr.Validation("provide exactly one of app_user_id or group_id")
	}
	if in.GroupID != "" {
		group, err := s.db.Q().GetAppUserGroup(ctx, in.GroupID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && group.OrganisationID != orgID) {
			return sqlc.AppGrant{}, apperr.Validation("unknown group")
		}
		if err != nil {
			return sqlc.AppGrant{}, err
		}
		return s.db.Q().CreateAppGroupGrant(ctx, sqlc.CreateAppGroupGrantParams{
			ID: ids.New(), OrganisationID: orgID, GroupID: textArg(in.GroupID), RoleID: in.RoleID,
			ScopeKind: kind, ScopeID: scopeID, ExpiresAt: expires, GrantedBy: in.GrantedBy,
		})
	}
	return s.db.Q().CreateAppUserGrant(ctx, sqlc.CreateAppUserGrantParams{
		ID: ids.New(), OrganisationID: orgID, AppUserID: textArg(in.AppUserID), RoleID: in.RoleID,
		ScopeKind: kind, ScopeID: scopeID, ExpiresAt: expires, GrantedBy: in.GrantedBy,
	})
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
	rows, err := s.db.Q().ListAppGrantsForUser(ctx, appUserID)
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
		if len(caps) == 0 {
			// An empty grant is invalid and would be rejected by the evaluator.
			// A role carrying nothing simply contributes nothing.
			continue
		}
		source := "app-role:" + r.RoleKey
		if r.GroupKey != "" {
			// Provenance: a capability held through a group should say so, or
			// "why does this customer have this?" is unanswerable.
			source = "group:" + r.GroupKey + " app-role:" + r.RoleKey
		}
		g := access.Grant{
			ID:           r.ID,
			Scope:        access.Scope{Kind: r.ScopeKind, ID: r.ScopeID},
			Capabilities: caps,
			Source:       source,
		}
		if r.ExpiresAt.Valid {
			t := r.ExpiresAt.Time
			g.ExpiresAt = &t
		}
		out.Grants = append(out.Grants, g)
	}
	return out, nil
}
