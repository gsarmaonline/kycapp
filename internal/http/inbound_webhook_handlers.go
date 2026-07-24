package httpserver

import (
	"io"
	"net/http"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

func inboundWebhookJSON(v service.InboundWebhookView) map[string]any {
	out := map[string]any{
		"id":              v.ID,
		"organisation_id": v.OrganisationID,
		"name":            v.Name,
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

func (s *Server) handleListInboundWebhooks(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	items, err := s.svc.ListInboundWebhooks(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, inboundWebhookJSON(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleCreateInboundWebhook(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name   string `json:"name"`
		Secret string `json:"secret"`
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	view, err := s.svc.CreateInboundWebhook(r.Context(), orgID, service.CreateInboundWebhookInput{
		Name: body.Name, Secret: body.Secret, Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, inboundWebhookJSON(view))
}

func (s *Server) handleGetInboundWebhook(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	hookID := r.PathValue("hookId")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	view, err := s.svc.GetInboundWebhook(r.Context(), orgID, hookID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inboundWebhookJSON(view))
}

func (s *Server) handlePatchInboundWebhook(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	hookID := r.PathValue("hookId")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name   *string `json:"name"`
		Secret *string `json:"secret"`
		Status *string `json:"status"`
		Rotate bool    `json:"rotate"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	view, err := s.svc.UpdateInboundWebhook(r.Context(), orgID, hookID, service.UpdateInboundWebhookInput{
		Name: body.Name, Secret: body.Secret, Status: body.Status, Rotate: body.Rotate,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inboundWebhookJSON(view))
}

func (s *Server) handleDeleteInboundWebhook(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	hookID := r.PathValue("hookId")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteInboundWebhook(r.Context(), orgID, hookID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Public: POST /v1/hooks/inbound/{hookId} with X-KYC-Webhook-Secret.
func (s *Server) handleInboundWebhook(w http.ResponseWriter, r *http.Request) {
	hookID := r.PathValue("hookId")
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, apperr.Validation("could not read body"))
		return
	}
	secret := r.Header.Get("X-KYC-Webhook-Secret")
	if err := s.svc.HandleInboundWebhook(r.Context(), hookID, secret, body, r.Header.Get("Content-Type")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"received": true, "trigger": "webhook.received"})
}
