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
		"auth_mode":       v.AuthMode,
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
		Name     string `json:"name"`
		Secret   string `json:"secret"`
		Status   string `json:"status"`
		AuthMode string `json:"auth_mode"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	view, err := s.svc.CreateInboundWebhook(r.Context(), orgID, service.CreateInboundWebhookInput{
		Name: body.Name, Secret: body.Secret, Status: body.Status, AuthMode: body.AuthMode,
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
		Name     *string `json:"name"`
		Secret   *string `json:"secret"`
		Status   *string `json:"status"`
		AuthMode *string `json:"auth_mode"`
		Rotate   bool    `json:"rotate"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	view, err := s.svc.UpdateInboundWebhook(r.Context(), orgID, hookID, service.UpdateInboundWebhookInput{
		Name: body.Name, Secret: body.Secret, Status: body.Status, AuthMode: body.AuthMode, Rotate: body.Rotate,
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

func (s *Server) receiveInboundWebhook(w http.ResponseWriter, r *http.Request, hookID, pathSecret string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, apperr.Validation("could not read body"))
		return
	}
	auth := service.InboundAuthInput{
		HeaderSecret: r.Header.Get("X-KYC-Webhook-Secret"),
		QuerySecret:  firstNonEmpty(r.URL.Query().Get("secret"), r.URL.Query().Get("token")),
		PathSecret:   pathSecret,
	}
	if err := s.svc.HandleInboundWebhook(r.Context(), hookID, auth, body, r.Header.Get("Content-Type")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"received": true, "trigger": "webhook.received"})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// Public: POST /v1/hooks/inbound/{hookId} — header or query auth.
func (s *Server) handleInboundWebhook(w http.ResponseWriter, r *http.Request) {
	s.receiveInboundWebhook(w, r, r.PathValue("hookId"), "")
}

// Public: POST /v1/hooks/inbound/{hookId}/{token} — path-token auth.
func (s *Server) handleInboundWebhookWithPathToken(w http.ResponseWriter, r *http.Request) {
	s.receiveInboundWebhook(w, r, r.PathValue("hookId"), r.PathValue("token"))
}
