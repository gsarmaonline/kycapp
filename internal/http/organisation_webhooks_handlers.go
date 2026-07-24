package httpserver

import (
	"net/http"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

func organisationWebhookJSON(v service.OrganisationWebhookView) map[string]any {
	return map[string]any{
		"id":              v.ID,
		"organisation_id": v.OrganisationID,
		"name":            v.Name,
		"url":             v.URL,
		"secret_hint":     v.SecretHint,
		"has_secret":      v.HasSecret,
		"status":          v.Status,
	}
}

func (s *Server) handleListOrgWebhooks(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	items, err := s.svc.ListOrganisationWebhooks(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, organisationWebhookJSON(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleCreateOrgWebhook(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name   string `json:"name"`
		URL    string `json:"url"`
		Secret string `json:"secret"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	view, err := s.svc.CreateOrganisationWebhook(r.Context(), orgID, service.CreateOrganisationWebhookInput{
		Name: body.Name, URL: body.URL, Secret: body.Secret,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, organisationWebhookJSON(view))
}

func (s *Server) handleGetOrgWebhook(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	whID := r.PathValue("webhookId")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	view, err := s.svc.GetOrganisationWebhook(r.Context(), orgID, whID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, organisationWebhookJSON(view))
}

func (s *Server) handlePatchOrgWebhook(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	whID := r.PathValue("webhookId")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name   *string `json:"name"`
		URL    *string `json:"url"`
		Secret *string `json:"secret"`
		Status *string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	view, err := s.svc.UpdateOrganisationWebhook(r.Context(), orgID, whID, service.UpdateOrganisationWebhookInput{
		Name: body.Name, URL: body.URL, Secret: body.Secret, Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, organisationWebhookJSON(view))
}

func (s *Server) handleDeleteOrgWebhook(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	whID := r.PathValue("webhookId")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteOrganisationWebhook(r.Context(), orgID, whID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
