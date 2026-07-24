package httpserver

import (
	"io"
	"net/http"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

func inboundHookJSON(v service.InboundHookView) map[string]any {
	out := map[string]any{
		"organisation_id": v.OrganisationID,
		"url":             v.URL,
		"secret_hint":     v.SecretHint,
		"has_secret":      v.HasSecret,
		"status":          v.Status,
	}
	if v.Secret != "" {
		out["secret"] = v.Secret
	}
	return out
}

func (s *Server) handleGetInboundHook(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	view, err := s.svc.GetInboundHook(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inboundHookJSON(view))
}

func (s *Server) handlePutInboundHook(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Secret *string `json:"secret"`
		Status *string `json:"status"`
		Rotate bool    `json:"rotate"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	view, err := s.svc.UpsertInboundHook(r.Context(), orgID, service.UpsertInboundHookInput{
		Secret: body.Secret,
		Status: body.Status,
		Rotate: body.Rotate,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inboundHookJSON(view))
}

// Public: POST /v1/hooks/inbound/{orgId} with X-KYC-Webhook-Secret.
func (s *Server) handleInboundWebhook(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgId")
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, apperr.Validation("could not read body"))
		return
	}
	secret := r.Header.Get("X-KYC-Webhook-Secret")
	if err := s.svc.HandleInboundWebhook(r.Context(), orgID, secret, body, r.Header.Get("Content-Type")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"received": true, "trigger": "webhook.received"})
}
