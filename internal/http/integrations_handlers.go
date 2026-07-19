package httpserver

import (
	"net/http"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

func integrationJSON(v service.IntegrationView) map[string]any {
	return map[string]any{
		"provider":        v.Provider,
		"status":          v.Status,
		"secret_hint":     v.SecretHint,
		"public_key_hint": v.PublicKeyHint,
		"has_secret":      v.HasSecret,
		"has_public_key":  v.HasPublicKey,
	}
}

func (s *Server) handleListOrgIntegrations(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	items, err := s.svc.ListOrganisationIntegrations(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, integrationJSON(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleUpsertStripeIntegration(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		SecretKey      string `json:"secret_key"`
		PublishableKey string `json:"publishable_key"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	view, err := s.svc.UpsertStripeIntegration(r.Context(), orgID, service.UpsertStripeIntegrationInput{
		SecretKey: body.SecretKey, PublishableKey: body.PublishableKey,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, integrationJSON(view))
}

func (s *Server) handleDeleteOrgIntegration(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteOrganisationIntegration(r.Context(), orgID, r.PathValue("provider")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
