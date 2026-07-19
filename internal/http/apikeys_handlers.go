package httpserver

import (
	"net/http"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

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
	existing, err := s.svc.GetAPIKey(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if existing.OrganisationID.Valid {
		if _, err := s.svc.RequireOrgPermission(r.Context(), existing.OrganisationID.String, "organisation:update"); err != nil {
			writeError(w, err)
			return
		}
	} else if _, err := service.RequirePlatform(r.Context()); err != nil {
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

func (s *Server) handleCreateOrgAPIKey(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
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
	created, err := s.svc.CreateOrganisationAPIKey(r.Context(), orgID, service.CreateAPIKeyInput{Name: body.Name})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":              created.Key.ID,
		"name":            created.Key.Name,
		"key_prefix":      created.Key.KeyPrefix,
		"organisation_id": orgID,
		"token":           created.Raw,
		"created_at":      created.Key.CreatedAt.UTC().Format(time.RFC3339Nano),
	})
}

func (s *Server) handleListOrgAPIKeys(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	keys, err := s.svc.ListOrganisationAPIKeys(r.Context(), orgID)
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
