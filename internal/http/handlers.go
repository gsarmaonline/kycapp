package httpserver

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/authn"
	"github.com/gsarmaonline/kyc/internal/service"
)

func (s *Server) handleAuthProviders(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"google":    s.google != nil && s.google.Enabled(),
		"dev_login": s.authDevLogin,
	})
}

func (s *Server) handleGoogleStart(w http.ResponseWriter, r *http.Request) {
	if s.google == nil || !s.google.Enabled() {
		writeError(w, apperr.Validation("google oauth is not configured"))
		return
	}
	state, err := service.SignOAuthState(s.oauthStateSecret, 10*time.Minute)
	if err != nil {
		writeError(w, err)
		return
	}
	http.Redirect(w, r, s.google.AuthCodeURL(state), http.StatusFound)
}

func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if s.google == nil || !s.google.Enabled() {
		writeError(w, apperr.Validation("google oauth is not configured"))
		return
	}
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		writeError(w, apperr.Unauthorized("google oauth error: "+errMsg))
		return
	}
	state := r.URL.Query().Get("state")
	if err := service.VerifyOAuthState(s.oauthStateSecret, state); err != nil {
		writeError(w, err)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, apperr.Unauthorized("missing oauth code"))
		return
	}
	tok, err := s.google.Exchange(r.Context(), code)
	if err != nil {
		writeError(w, err)
		return
	}
	identity, err := s.google.FetchIdentity(r.Context(), tok)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := s.svc.LoginWithGoogle(r.Context(), identity, s.platformAdminEmails)
	if err != nil {
		writeError(w, err)
		return
	}
	// Hand the session token to the SPA via fragment (not sent to server on later navigations).
	redirect := strings.TrimRight(s.appOrigin, "/") + "/#token=" + url.QueryEscape(result.Token)
	http.Redirect(w, r, redirect, http.StatusFound)
}

func (s *Server) handleDevLogin(w http.ResponseWriter, r *http.Request) {
	if !s.authDevLogin {
		writeError(w, apperr.Forbidden("dev login is disabled"))
		return
	}
	var body struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	result, err := s.svc.DevLogin(r.Context(), body.Email, body.Name, s.platformAdminEmails)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, authResultJSON(result))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequireUser(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.Logout(r.Context(), p.SessionID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequireUser(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	me, err := s.svc.Me(r.Context(), p.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(me.Memberships))
	for _, row := range me.Memberships {
		items = append(items, map[string]any{
			"id":                  row.ID,
			"organisation_id":     row.OrganisationID,
			"user_id":             row.UserID,
			"role_id":             row.RoleID,
			"status":              row.Status,
			"created_at":          row.CreatedAt.UTC().Format(time.RFC3339Nano),
			"organisation_name":   row.OrganisationName,
			"organisation_slug":   row.OrganisationSlug,
			"organisation_status": row.OrganisationStatus,
			"role_key":            row.RoleKey,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user":         userJSON(me.User),
		"memberships":  items,
		"platform_admin": p.PlatformAdmin || me.User.PlatformAdmin,
	})
}

func (s *Server) handleCreateOrganisation(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequirePrincipal(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	in := service.CreateOrganisationInput{
		Name:        body.Name,
		Slug:        body.Slug,
		AttachTrial: true,
	}
	if p.Kind == authn.KindUser {
		in.OwnerUserID = p.UserID
	} else if !p.IsPlatform() {
		writeError(w, apperr.Forbidden("cannot create organisation"))
		return
	}
	org, err := s.svc.CreateOrganisation(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, orgJSON(org))
}

func (s *Server) handleListOrganisations(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequirePrincipal(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	q := r.URL.Query()
	var list []map[string]any
	if p.IsPlatform() {
		rows, err := s.svc.ListOrganisations(r.Context(), q.Get("status"), q.Get("q"), queryLimit(r), q.Get("cursor"))
		if err != nil {
			writeError(w, err)
			return
		}
		list = make([]map[string]any, 0, len(rows))
		for _, o := range rows {
			list = append(list, orgJSON(o))
		}
	} else {
		rows, err := s.svc.ListOrganisationsForUser(r.Context(), p.UserID, q.Get("status"), q.Get("q"), queryLimit(r), q.Get("cursor"))
		if err != nil {
			writeError(w, err)
			return
		}
		list = make([]map[string]any, 0, len(rows))
		for _, o := range rows {
			list = append(list, orgJSON(o))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (s *Server) handleGetOrganisation(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgMember(r.Context(), orgID); err != nil {
		writeError(w, err)
		return
	}
	org, err := s.svc.GetOrganisation(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orgJSON(org))
}

func (s *Server) handlePatchOrganisation(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name   *string `json:"name"`
		Status *string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	org, err := s.svc.UpdateOrganisation(r.Context(), orgID, service.UpdateOrganisationInput{
		Name: body.Name, Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orgJSON(org))
}

func (s *Server) handleArchiveOrganisation(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	org, err := s.svc.ArchiveOrganisation(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orgJSON(org))
}

func (s *Server) handleListRoles(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "roles:read"); err != nil {
		writeError(w, err)
		return
	}
	roles, err := s.svc.ListRoles(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		items = append(items, roleJSON(role))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	user, err := s.svc.CreateUser(r.Context(), service.CreateUserInput{
		Email: body.Email, Name: body.Name,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, userJSON(user))
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	q := r.URL.Query()
	users, err := s.svc.ListUsers(r.Context(), q.Get("q"), queryLimit(r), q.Get("cursor"))
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(users))
	for _, u := range users {
		items = append(items, userJSON(u))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequirePrincipal(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	id := r.PathValue("id")
	if !p.IsPlatform() && p.UserID != id {
		writeError(w, apperr.Forbidden("cannot view other users"))
		return
	}
	user, err := s.svc.GetUser(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, userJSON(user))
}

func (s *Server) handlePatchUser(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequirePrincipal(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	id := r.PathValue("id")
	var body struct {
		Name   *string `json:"name"`
		Status *string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	if !p.IsPlatform() {
		if p.UserID != id {
			writeError(w, apperr.Forbidden("cannot update other users"))
			return
		}
		if body.Status != nil {
			writeError(w, apperr.Forbidden("cannot change own status"))
			return
		}
	}
	user, err := s.svc.UpdateUser(r.Context(), id, service.UpdateUserInput{
		Name: body.Name, Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, userJSON(user))
}

func (s *Server) handleListUserMemberships(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequirePrincipal(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	id := r.PathValue("id")
	if !p.IsPlatform() && p.UserID != id {
		writeError(w, apperr.Forbidden("cannot view other users' memberships"))
		return
	}
	rows, err := s.svc.ListUserMemberships(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id":                  row.ID,
			"organisation_id":     row.OrganisationID,
			"user_id":             row.UserID,
			"role_id":             row.RoleID,
			"status":              row.Status,
			"created_at":          row.CreatedAt.UTC().Format(time.RFC3339Nano),
			"organisation_name":   row.OrganisationName,
			"organisation_slug":   row.OrganisationSlug,
			"organisation_status": row.OrganisationStatus,
			"role_key":            row.RoleKey,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateMembership(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "members:invite"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
		RoleID string `json:"role_id"`
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	m, err := s.svc.CreateMembership(r.Context(), orgID, service.CreateMembershipInput{
		UserID: body.UserID,
		Email:  body.Email,
		RoleID: body.RoleID,
		Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, membershipJSON(m))
}

func (s *Server) handleListMemberships(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "members:read"); err != nil {
		writeError(w, err)
		return
	}
	rows, err := s.svc.ListOrganisationMemberships(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id":              row.ID,
			"organisation_id": row.OrganisationID,
			"user_id":         row.UserID,
			"role_id":         row.RoleID,
			"status":          row.Status,
			"created_at":      row.CreatedAt.UTC().Format(time.RFC3339Nano),
			"user_email":      row.UserEmail,
			"user_name":       row.UserName,
			"role_key":        row.RoleKey,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleAcceptMembership(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequireUser(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	m, err := s.svc.AcceptMembershipAsUser(r.Context(), r.PathValue("id"), p.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, membershipJSON(m))
}

func (s *Server) handlePatchMembership(w http.ResponseWriter, r *http.Request) {
	m, err := s.svc.GetMembership(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), m.OrganisationID, "members:invite"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		RoleID *string `json:"role_id"`
		Status *string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	out, err := s.svc.UpdateMembership(r.Context(), r.PathValue("id"), service.UpdateMembershipInput{
		RoleID: body.RoleID, Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, membershipJSON(out))
}

func (s *Server) handleRevokeMembership(w http.ResponseWriter, r *http.Request) {
	m, err := s.svc.GetMembership(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), m.OrganisationID, "members:remove"); err != nil {
		writeError(w, err)
		return
	}
	out, err := s.svc.RevokeMembership(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, membershipJSON(out))
}

func (s *Server) handleListPermissions(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePrincipal(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	q := r.URL.Query()
	perms, err := s.svc.ListPermissions(r.Context(), q.Get("category"), q.Get("resource"))
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(perms))
	for _, p := range perms {
		items = append(items, permissionJSON(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetPermission(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePrincipal(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	p, err := s.svc.GetPermission(r.Context(), r.PathValue("key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, permissionJSON(p))
}

func (s *Server) handleCreateRole(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "roles:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Key            string   `json:"key"`
		Name           string   `json:"name"`
		Description    string   `json:"description"`
		PermissionKeys []string `json:"permission_keys"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	role, err := s.svc.CreateRole(r.Context(), orgID, service.CreateRoleInput{
		Key: body.Key, Name: body.Name, Description: body.Description, PermissionKeys: body.PermissionKeys,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, roleJSON(role))
}

func (s *Server) handlePatchRole(w http.ResponseWriter, r *http.Request) {
	role, err := s.svc.GetRole(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), role.Role.OrganisationID, "roles:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name           *string   `json:"name"`
		Description    *string   `json:"description"`
		PermissionKeys *[]string `json:"permission_keys"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	updated, err := s.svc.UpdateRole(r.Context(), r.PathValue("id"), service.UpdateRoleInput{
		Name: body.Name, Description: body.Description, PermissionKeys: body.PermissionKeys,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, roleJSON(updated))
}

func (s *Server) handleAuthzCheck(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequirePrincipal(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		OrganisationID string `json:"organisation_id"`
		UserID         string `json:"user_id"`
		Permission     string `json:"permission"`
		Resource       string `json:"resource"`
		Action         string `json:"action"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	if !p.IsPlatform() {
		if _, err := s.svc.RequireOrgMember(r.Context(), body.OrganisationID); err != nil {
			writeError(w, err)
			return
		}
	}
	allowed, err := s.svc.CheckAuthz(r.Context(), service.AuthzCheckInput{
		OrganisationID: body.OrganisationID,
		UserID:         body.UserID,
		Permission:     body.Permission,
		Resource:       body.Resource,
		Action:         body.Action,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"allowed": allowed})
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	created, err := s.svc.CreateAPIKey(r.Context(), service.CreateAPIKeyInput{Name: body.Name})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         created.Key.ID,
		"name":       created.Key.Name,
		"key_prefix": created.Key.KeyPrefix,
		"token":      created.Raw,
		"created_at": created.Key.CreatedAt.UTC().Format(time.RFC3339Nano),
	})
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	keys, err := s.svc.ListAPIKeys(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		item := map[string]any{
			"id":         k.ID,
			"name":       k.Name,
			"key_prefix": k.KeyPrefix,
			"created_at": k.CreatedAt.UTC().Format(time.RFC3339Nano),
			"revoked":    k.RevokedAt.Valid,
		}
		if k.RevokedAt.Valid {
			item["revoked_at"] = k.RevokedAt.Time.UTC().Format(time.RFC3339Nano)
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	key, err := s.svc.RevokeAPIKey(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": key.ID, "name": key.Name, "key_prefix": key.KeyPrefix, "revoked": true,
	})
}

func (s *Server) handleListAuditEvents(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	events, err := s.svc.ListAuditEvents(r.Context(), queryLimit(r))
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(events))
	for _, e := range events {
		item := map[string]any{
			"id": e.ID, "actor": e.Actor, "method": e.Method, "path": e.Path,
			"status_code": e.StatusCode, "created_at": e.CreatedAt.UTC().Format(time.RFC3339Nano),
		}
		if e.OrganisationID.Valid {
			item["organisation_id"] = e.OrganisationID.String
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	plan, err := s.svc.CreatePlan(r.Context(), service.CreatePlanInput{Key: body.Key, Name: body.Name})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, planJSON(service.PlanView{Plan: plan, EntitlementKeys: []string{}}))
}

func (s *Server) handleListPlans(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePrincipal(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	plans, err := s.svc.ListPlans(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(plans))
	for _, p := range plans {
		items = append(items, planJSON(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePrincipal(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	plan, err := s.svc.GetPlan(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, planJSON(plan))
}

func (s *Server) handleSetPlanEntitlements(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		EntitlementKeys []string `json:"entitlement_keys"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	plan, err := s.svc.SetPlanEntitlements(r.Context(), r.PathValue("id"), service.SetPlanEntitlementsInput{
		EntitlementKeys: body.EntitlementKeys,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, planJSON(plan))
}

func (s *Server) handleCreateEntitlement(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Key         string `json:"key"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	ent, err := s.svc.CreateEntitlement(r.Context(), service.CreateEntitlementInput{
		Key: body.Key, Description: body.Description,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, entitlementJSON(ent))
}

func (s *Server) handleListEntitlementsCatalog(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePrincipal(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	ents, err := s.svc.ListEntitlements(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(ents))
	for _, e := range ents {
		items = append(items, entitlementJSON(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleUpsertSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	// Assigning plans is platform-only until Stripe self-serve exists.
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	_ = orgID
	var body struct {
		PlanID string `json:"plan_id"`
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	sub, err := s.svc.UpsertSubscription(r.Context(), r.PathValue("id"), service.UpsertSubscriptionInput{
		PlanID: body.PlanID, Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, subscriptionJSON(sub))
}

func (s *Server) handleGetSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "billing:read"); err != nil {
		writeError(w, err)
		return
	}
	sub, err := s.svc.GetSubscription(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, subscriptionJSON(sub))
}

func (s *Server) handleSetOrgEntitlements(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Overrides []struct {
			Key    string `json:"key"`
			Effect string `json:"effect"`
		} `json:"overrides"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	overrides := make([]service.EntitlementOverride, 0, len(body.Overrides))
	for _, o := range body.Overrides {
		overrides = append(overrides, service.EntitlementOverride{Key: o.Key, Effect: o.Effect})
	}
	keys, err := s.svc.SetOrganisationEntitlements(r.Context(), r.PathValue("id"), service.SetOrganisationEntitlementsInput{
		Overrides: overrides,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entitlements": keys})
}

func (s *Server) handleGetOrgEntitlements(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "billing:read"); err != nil {
		writeError(w, err)
		return
	}
	keys, err := s.svc.EffectiveEntitlements(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entitlements": keys})
}

func (s *Server) handleEntitlementsCheck(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequirePrincipal(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		OrganisationID string `json:"organisation_id"`
		Entitlement    string `json:"entitlement"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	if !p.IsPlatform() {
		if _, err := s.svc.RequireOrgMember(r.Context(), body.OrganisationID); err != nil {
			writeError(w, err)
			return
		}
	}
	allowed, err := s.svc.CheckEntitlement(r.Context(), body.OrganisationID, body.Entitlement)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"allowed": allowed})
}

func (s *Server) handleCreateAttributeDefinition(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "attributes:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Key         string   `json:"key"`
		Label       string   `json:"label"`
		Description string   `json:"description"`
		ValueType   string   `json:"value_type"`
		Section     string   `json:"section"`
		SortOrder   int32    `json:"sort_order"`
		Required    bool     `json:"required"`
		EnumValues  []string `json:"enum_values"`
		IsPII       bool     `json:"is_pii"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.CreateAttributeDefinition(r.Context(), orgID, service.CreateAttributeDefinitionInput{
		Key: body.Key, Label: body.Label, Description: body.Description,
		ValueType: body.ValueType, Section: body.Section, SortOrder: body.SortOrder,
		Required: body.Required, EnumValues: body.EnumValues, IsPII: body.IsPII,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, attributeDefinitionJSON(row))
}

func (s *Server) handleListAttributeDefinitions(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "attributes:read"); err != nil {
		writeError(w, err)
		return
	}
	rows, err := s.svc.ListAttributeDefinitions(r.Context(), orgID, r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, attributeDefinitionJSON(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handlePatchAttributeDefinition(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetAttributeDefinition(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.OrganisationID, "attributes:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Label       *string   `json:"label"`
		Description *string   `json:"description"`
		ValueType   *string   `json:"value_type"`
		Section     *string   `json:"section"`
		SortOrder   *int32    `json:"sort_order"`
		Required    *bool     `json:"required"`
		EnumValues  *[]string `json:"enum_values"`
		IsPII       *bool     `json:"is_pii"`
		Status      *string   `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.UpdateAttributeDefinition(r.Context(), r.PathValue("id"), service.UpdateAttributeDefinitionInput{
		Label: body.Label, Description: body.Description, ValueType: body.ValueType,
		Section: body.Section, SortOrder: body.SortOrder, Required: body.Required,
		EnumValues: body.EnumValues, IsPII: body.IsPII, Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, attributeDefinitionJSON(row))
}

func (s *Server) handleCreateAppUser(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "app_users:write"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		ExternalID  string         `json:"external_id"`
		Email       string         `json:"email"`
		DisplayName string         `json:"display_name"`
		Status      string         `json:"status"`
		Attributes  map[string]any `json:"attributes"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.CreateAppUser(r.Context(), orgID, service.CreateAppUserInput{
		ExternalID: body.ExternalID, Email: body.Email, DisplayName: body.DisplayName,
		Status: body.Status, Attributes: body.Attributes,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, appUserJSON(row))
}

func (s *Server) handleListAppUsers(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "app_users:read"); err != nil {
		writeError(w, err)
		return
	}
	rows, err := s.svc.ListAppUsers(r.Context(), orgID, r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, appUserJSON(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetAppUser(w http.ResponseWriter, r *http.Request) {
	row, err := s.svc.GetAppUser(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), row.OrganisationID, "app_users:read"); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, appUserJSON(row))
}

func (s *Server) handlePatchAppUser(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetAppUser(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.OrganisationID, "app_users:write"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		ExternalID  *string         `json:"external_id"`
		Email       *string         `json:"email"`
		DisplayName *string         `json:"display_name"`
		Status      *string         `json:"status"`
		Attributes  *map[string]any `json:"attributes"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	in := service.UpdateAppUserInput{
		ExternalID: body.ExternalID, Email: body.Email,
		DisplayName: body.DisplayName, Status: body.Status,
	}
	if body.Attributes != nil {
		in.Attributes = *body.Attributes
	}
	row, err := s.svc.UpdateAppUser(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, appUserJSON(row))
}

func (s *Server) handleCreateEmailTemplate(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "email_templates:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Key         string `json:"key"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Subject     string `json:"subject"`
		BodyText    string `json:"body_text"`
		BodyHTML    string `json:"body_html"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.CreateEmailTemplate(r.Context(), orgID, service.CreateEmailTemplateInput{
		Key: body.Key, Name: body.Name, Description: body.Description,
		Subject: body.Subject, BodyText: body.BodyText, BodyHTML: body.BodyHTML,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, emailTemplateJSON(row))
}

func (s *Server) handleListEmailTemplates(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "email_templates:read"); err != nil {
		writeError(w, err)
		return
	}
	rows, err := s.svc.ListEmailTemplates(r.Context(), orgID, r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, emailTemplateJSON(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetEmailTemplate(w http.ResponseWriter, r *http.Request) {
	row, err := s.svc.GetEmailTemplate(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), row.OrganisationID, "email_templates:read"); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, emailTemplateJSON(row))
}

func (s *Server) handlePatchEmailTemplate(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetEmailTemplate(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.OrganisationID, "email_templates:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Subject     *string `json:"subject"`
		BodyText    *string `json:"body_text"`
		BodyHTML    *string `json:"body_html"`
		Status      *string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.UpdateEmailTemplate(r.Context(), r.PathValue("id"), service.UpdateEmailTemplateInput{
		Name: body.Name, Description: body.Description, Subject: body.Subject,
		BodyText: body.BodyText, BodyHTML: body.BodyHTML, Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, emailTemplateJSON(row))
}
