package httpserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

// Merchant-hosted access control.
//
// These routes are gated by KYC permissions (app_access:read / app_access:manage),
// not by the merchant's own capabilities. That is the two-tier boundary: KYC's
// RBAC decides who may administer the model; the model decides what the
// merchant's customers may do. An operator administers it without being in it.

func (s *Server) handleListAppScopeTypes(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	rows, err := s.svc.ListAppScopeTypes(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, t := range rows {
		items = append(items, map[string]any{"id": t.ID, "kind": t.Kind, "label": t.Label})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateAppScopeType(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		Kind  string `json:"kind"`
		Label string `json:"label"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.CreateAppScopeType(r.Context(), orgID, body.Kind, body.Label)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": row.ID, "kind": row.Kind, "label": row.Label})
}

func (s *Server) handleDeleteAppScopeType(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if err := s.svc.DeleteAppScopeType(r.Context(), orgID, r.PathValue("typeId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListAppCapabilities(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	rows, err := s.svc.ListAppCapabilities(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, c := range rows {
		items = append(items, map[string]any{"id": c.ID, "key": c.Key, "description": c.Description})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateAppCapability(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		Key         string `json:"key"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.CreateAppCapability(r.Context(), orgID, body.Key, body.Description)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": row.ID, "key": row.Key, "description": row.Description})
}

func (s *Server) handleDeleteAppCapability(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if err := s.svc.DeleteAppCapability(r.Context(), orgID, r.PathValue("capId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// appRoleJSON exposes both what the role declares and what it resolves to.
// Showing only the chain is how inheritance surprises people, so the effective
// set is always returned alongside it.
func appRoleJSON(r sqlc.AppRole) map[string]any {
	return map[string]any{
		"id":                     r.ID,
		"key":                    r.Key,
		"name":                   r.Name,
		"description":            r.Description,
		"own_capabilities":       r.OwnCapabilities,
		"effective_capabilities": r.EffectiveCapabilities,
	}
}

func (s *Server) handleListAppRoles(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	rows, err := s.svc.ListAppRoles(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, role := range rows {
		items = append(items, appRoleJSON(role))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateAppRole(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		Key          string   `json:"key"`
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		Capabilities []string `json:"capabilities"`
		Extends      []string `json:"extends"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	role, err := s.svc.CreateAppRole(r.Context(), orgID, service.AppRoleInput{
		Key: body.Key, Name: body.Name, Description: body.Description,
		Capabilities: body.Capabilities, Extends: body.Extends,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, appRoleJSON(role))
}

func (s *Server) handlePatchAppRole(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		Name         string    `json:"name"`
		Description  string    `json:"description"`
		Capabilities *[]string `json:"capabilities"`
		Extends      *[]string `json:"extends"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	in := service.AppRoleInput{Name: body.Name, Description: body.Description}
	if body.Capabilities != nil {
		in.Capabilities = *body.Capabilities
	}
	if body.Extends != nil {
		in.Extends = *body.Extends
	}
	role, err := s.svc.UpdateAppRole(r.Context(), orgID, r.PathValue("roleId"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, appRoleJSON(role))
}

func (s *Server) handleDeleteAppRole(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if err := s.svc.DeleteAppRole(r.Context(), orgID, r.PathValue("roleId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateAppGrant(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	p, err := service.RequirePrincipal(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		SubjectKind string `json:"subject_kind"`
		AppUserID   string `json:"app_user_id"`
		GroupID     string `json:"group_id"`
		RoleID      string `json:"role_id"`
		ScopeKind   string `json:"scope_kind"`
		ScopeID     string `json:"scope_id"`
		ExpiresAt   string `json:"expires_at"`

		AllCapabilities    bool                  `json:"all_capabilities"`
		ExceptCapabilities []string              `json:"except_capabilities"`
		ExceptScopes       []service.AppScopeRef `json:"except_scopes"`
		ExceptAppUserIDs   []string              `json:"except_app_user_ids"`
		Constraint         string                `json:"constraint"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	in := service.AppGrantInput{
		SubjectKind: body.SubjectKind,
		AppUserID:   body.AppUserID, GroupID: body.GroupID, RoleID: body.RoleID,
		ScopeKind: body.ScopeKind, ScopeID: body.ScopeID,
		GrantedBy:          p.ActorLabel(),
		AllCapabilities:    body.AllCapabilities,
		ExceptCapabilities: body.ExceptCapabilities,
		ExceptScopes:       body.ExceptScopes,
		ExceptAppUserIDs:   body.ExceptAppUserIDs,
		Constraint:         body.Constraint,
	}
	if body.ExpiresAt != "" {
		at, parseErr := time.Parse(time.RFC3339, body.ExpiresAt)
		if parseErr != nil {
			writeError(w, apperr.Validation("expires_at must be RFC3339"))
			return
		}
		in.ExpiresAt = &at
	}
	grant, err := s.svc.CreateAppGrant(r.Context(), orgID, in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": grant.ID, "app_user_id": grant.AppUserID.String, "group_id": grant.GroupID.String,
		"subject_kind": grant.SubjectKind, "role_id": grant.RoleID.String,
		"scope_kind": grant.ScopeKind, "scope_id": grant.ScopeID,
		"all_capabilities": grant.AllCapabilities, "constraint": grant.ConstraintKind,
	})
}

func (s *Server) handleDeleteAppGrant(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if err := s.svc.DeleteAppGrant(r.Context(), orgID, r.PathValue("grantId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleAppUserAccess returns the assembled grant set for one customer.
//
// This is the endpoint a merchant's backend caches and evaluates locally, which
// is why it returns the whole set plus a version rather than answering a single
// question. A per-request check would put a network hop inside every request of
// their product and make KYC own their latency.
func (s *Server) handleAppUserAccess(w http.ResponseWriter, r *http.Request) {
	appUser, err := s.svc.GetAppUser(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), appUser.OrganisationID, "app_access:read"); err != nil {
		writeError(w, err)
		return
	}
	set, err := s.svc.AppAccessFor(r.Context(), appUser.OrganisationID, appUser.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	grants := make([]map[string]any, 0, len(set.Grants))
	for _, g := range set.Grants {
		caps := g.Capabilities
		item := map[string]any{
			"id": g.ID, "scope_kind": g.Scope.Kind, "scope_id": g.Scope.ID,
			"capabilities": caps, "source": g.Source,
		}
		// Everything the evaluator needs must be on the wire. A backend that
		// reads only capabilities would treat a narrowed grant as a plain one
		// and allow more than the merchant granted, so these are always
		// present rather than omitted when empty.
		item["all_capabilities"] = g.AllCapabilities
		item["except_capabilities"] = nonNil(g.ExceptCapabilities)
		item["except_scopes"] = scopeRefs(g.Except)
		item["constraint"] = string(g.Constraint)
		if g.ExpiresAt != nil {
			item["expires_at"] = g.ExpiresAt.UTC().Format(time.RFC3339Nano)
		}
		grants = append(grants, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"app_user_id": set.AppUserID,
		"namespace":   set.Namespace,
		"version":     set.Version,
		"grants":      grants,
	})
}

// --- Groups ---
//
// A group is a set of app users a grant can target. It answers "which
// principals", where a scope answers "which resources", so a grant simply gains
// a subject rather than changing shape.

func (s *Server) handleListAppUserGroups(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	rows, err := s.svc.ListAppUserGroups(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, g := range rows {
		items = append(items, map[string]any{
			"id": g.ID, "key": g.Key, "name": g.Name,
			"description": g.Description, "member_count": g.MemberCount,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateAppUserGroup(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		Key         string   `json:"key"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Parents     []string `json:"parents"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.CreateAppUserGroup(r.Context(), orgID, service.AppUserGroupInput{
		Key: body.Key, Name: body.Name, Description: body.Description, Parents: body.Parents,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": row.ID, "key": row.Key, "name": row.Name, "description": row.Description, "member_count": 0,
	})
}

func (s *Server) handleDeleteAppUserGroup(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if err := s.svc.DeleteAppUserGroup(r.Context(), orgID, r.PathValue("groupId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListAppUserGroupMembers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.svc.ListAppUserGroupMembers(r.Context(), r.PathValue("groupId"))
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, m := range rows {
		items = append(items, map[string]any{
			"id": m.ID, "email": m.Email, "display_name": m.DisplayName, "status": m.Status,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleAddAppUserGroupMember(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		AppUserID string `json:"app_user_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	if err := s.svc.SetAppUserGroupMember(r.Context(), orgID, r.PathValue("groupId"), body.AppUserID, true); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (s *Server) handleRemoveAppUserGroupMember(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if err := s.svc.SetAppUserGroupMember(r.Context(), orgID, r.PathValue("groupId"), r.PathValue("appUserId"), false); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListAppGrants(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	rows, err := s.svc.ListAppGrantsForOrg(r.Context(), orgID, 0)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, g := range rows {
		item := map[string]any{
			"id": g.ID, "role_key": g.RoleKey,
			"scope_kind": g.ScopeKind, "scope_id": g.ScopeID,
			"subject_kind": g.SubjectKind, "subject_label": g.AppUserEmail,
			"all_capabilities":    g.AllCapabilities,
			"except_capabilities": stringsOrEmpty(g.ExceptCapabilities),
			"except_scopes":       exceptScopesJSON(g.ExceptScopes),
			"except_app_users":    stringsOrEmpty(g.ExceptAppUserIds),
			"constraint":          g.ConstraintKind,
		}
		switch g.SubjectKind {
		case "group":
			item["subject_label"] = g.GroupKey
		case "everyone":
			item["subject_label"] = "every customer"
		}
		if g.ExpiresAt.Valid {
			item["expires_at"] = g.ExpiresAt.Time.UTC().Format(time.RFC3339Nano)
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// nonNil keeps an absent list serialising as [] rather than null. A backend
// that treats null as "no exceptions" would be right; one that dereferences it
// would not, and this costs nothing.
func nonNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func scopeRefs(scopes []service.AppScope) []service.AppScopeRef {
	out := make([]service.AppScopeRef, 0, len(scopes))
	for _, sc := range scopes {
		out = append(out, service.AppScopeRef{Kind: sc.Kind, ID: sc.ID})
	}
	return out
}

// stringsOrEmpty keeps a null out of the JSON: the client renders a list, and a
// missing array and an empty one should not read differently.
func stringsOrEmpty(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func exceptScopesJSON(raw json.RawMessage) []service.AppScopeRef {
	out := []service.AppScopeRef{}
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func (s *Server) handlePatchAppUserGroup(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Parents     []string `json:"parents"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.UpdateAppUserGroup(r.Context(), orgID, r.PathValue("groupId"), service.AppUserGroupInput{
		Name: body.Name, Description: body.Description, Parents: body.Parents,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": row.ID, "key": row.Key, "name": row.Name, "description": row.Description,
	})
}

// handleAppUserGroups answers "which groups is this customer in?", which is
// half of "why does this customer have this access?". The other half is the
// grant set, with its source naming the group a capability came through.
func (s *Server) handleAppUserGroups(w http.ResponseWriter, r *http.Request) {
	appUser, err := s.svc.GetAppUser(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), appUser.OrganisationID, "app_access:read"); err != nil {
		writeError(w, err)
		return
	}
	rows, err := s.svc.ListGroupsForAppUser(r.Context(), appUser.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, g := range rows {
		items = append(items, map[string]any{"id": g.ID, "key": g.Key, "name": g.Name})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// Single-object reads and edits, so each object has the index / new / show /
// edit shape the rest of the admin UI uses.

func (s *Server) handleGetAppScopeType(w http.ResponseWriter, r *http.Request) {
	row, err := s.svc.GetAppScopeType(r.Context(), r.PathValue("id"), r.PathValue("typeId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": row.ID, "kind": row.Kind, "label": row.Label})
}

func (s *Server) handlePatchAppScopeType(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Label string `json:"label"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.UpdateAppScopeType(r.Context(), r.PathValue("id"), r.PathValue("typeId"), body.Label)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": row.ID, "kind": row.Kind, "label": row.Label})
}

func (s *Server) handleGetAppCapability(w http.ResponseWriter, r *http.Request) {
	row, err := s.svc.GetAppCapability(r.Context(), r.PathValue("id"), r.PathValue("capId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": row.ID, "key": row.Key, "description": row.Description})
}

func (s *Server) handlePatchAppCapability(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.UpdateAppCapability(r.Context(), r.PathValue("id"), r.PathValue("capId"), body.Description)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": row.ID, "key": row.Key, "description": row.Description})
}

func (s *Server) handleGetAppRole(w http.ResponseWriter, r *http.Request) {
	view, err := s.svc.GetAppRoleView(r.Context(), r.PathValue("id"), r.PathValue("roleId"))
	if err != nil {
		writeError(w, err)
		return
	}
	out := appRoleJSON(view.Role)
	out["extends"] = view.Extends
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetAppUserGroup(w http.ResponseWriter, r *http.Request) {
	row, err := s.svc.GetAppUserGroupByID(r.Context(), r.PathValue("id"), r.PathValue("groupId"))
	if err != nil {
		writeError(w, err)
		return
	}
	// Parents belong to the group now, so an edit form round-tripping the object
	// without them would silently clear the nesting on every save.
	parents, err := s.svc.ListAppUserGroupParents(r.Context(), r.PathValue("id"), row.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": row.ID, "key": row.Key, "name": row.Name, "description": row.Description,
		"parents": parents,
	})
}
