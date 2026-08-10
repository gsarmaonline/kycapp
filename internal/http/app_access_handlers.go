package httpserver

import (
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
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "app_access:read"); err != nil {
		writeError(w, err)
		return
	}
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
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "app_access:manage"); err != nil {
		writeError(w, err)
		return
	}
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
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "app_access:manage"); err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteAppScopeType(r.Context(), orgID, r.PathValue("typeId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListAppCapabilities(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "app_access:read"); err != nil {
		writeError(w, err)
		return
	}
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
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "app_access:manage"); err != nil {
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
	row, err := s.svc.CreateAppCapability(r.Context(), orgID, body.Key, body.Description)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": row.ID, "key": row.Key, "description": row.Description})
}

func (s *Server) handleDeleteAppCapability(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "app_access:manage"); err != nil {
		writeError(w, err)
		return
	}
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
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "app_access:read"); err != nil {
		writeError(w, err)
		return
	}
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
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "app_access:manage"); err != nil {
		writeError(w, err)
		return
	}
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
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "app_access:manage"); err != nil {
		writeError(w, err)
		return
	}
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
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "app_access:manage"); err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteAppRole(r.Context(), orgID, r.PathValue("roleId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateAppGrant(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	p, err := s.svc.RequireOrgPermission(r.Context(), orgID, "app_access:manage")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		AppUserID string `json:"app_user_id"`
		RoleID    string `json:"role_id"`
		ScopeKind string `json:"scope_kind"`
		ScopeID   string `json:"scope_id"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	in := service.AppGrantInput{
		AppUserID: body.AppUserID, RoleID: body.RoleID,
		ScopeKind: body.ScopeKind, ScopeID: body.ScopeID,
		GrantedBy: p.ActorLabel(),
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
		"id": grant.ID, "app_user_id": grant.AppUserID, "role_id": grant.RoleID,
		"scope_kind": grant.ScopeKind, "scope_id": grant.ScopeID,
	})
}

func (s *Server) handleDeleteAppGrant(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "app_access:manage"); err != nil {
		writeError(w, err)
		return
	}
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
		caps := make([]string, 0, len(g.Capabilities))
		for _, c := range g.Capabilities {
			caps = append(caps, c.Key)
		}
		item := map[string]any{
			"id": g.ID, "scope_kind": g.Scope.Kind, "scope_id": g.Scope.ID,
			"capabilities": caps, "source": g.Source,
		}
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
