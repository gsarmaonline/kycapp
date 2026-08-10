package httpserver

import (
	"net/http"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

func (s *Server) handleListRoles(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
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

func (s *Server) handleGetRole(w http.ResponseWriter, r *http.Request) {
	role, err := s.svc.GetRole(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), role.Role.OrganisationID, "roles:read"); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, roleJSON(role))
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

func (s *Server) handleDeleteRole(w http.ResponseWriter, r *http.Request) {
	role, err := s.svc.GetRole(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), role.Role.OrganisationID, "roles:manage"); err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteRole(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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
