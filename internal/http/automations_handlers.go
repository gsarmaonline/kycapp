package httpserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

func (s *Server) handleCreateAutomation(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "automations:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name       string          `json:"name"`
		Trigger    string          `json:"trigger"`
		Enabled    *bool           `json:"enabled"`
		Conditions json.RawMessage `json:"conditions"`
		Actions    json.RawMessage `json:"actions"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.CreateAutomation(r.Context(), orgID, service.CreateAutomationInput{
		Name: body.Name, Trigger: body.Trigger, Enabled: body.Enabled,
		Conditions: body.Conditions, Actions: body.Actions,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, automationJSON(row))
}

func (s *Server) handleListAutomations(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "automations:read"); err != nil {
		writeError(w, err)
		return
	}
	rows, err := s.svc.ListAutomations(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, automationJSON(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetAutomation(w http.ResponseWriter, r *http.Request) {
	row, err := s.svc.GetAutomation(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), row.OrganisationID, "automations:read"); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, automationJSON(row))
}

func (s *Server) handlePatchAutomation(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetAutomation(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.OrganisationID, "automations:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name       *string         `json:"name"`
		Trigger    *string         `json:"trigger"`
		Enabled    *bool           `json:"enabled"`
		Conditions json.RawMessage `json:"conditions"`
		Actions    json.RawMessage `json:"actions"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.UpdateAutomation(r.Context(), r.PathValue("id"), service.UpdateAutomationInput{
		Name: body.Name, Trigger: body.Trigger, Enabled: body.Enabled,
		Conditions: body.Conditions, Actions: body.Actions,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, automationJSON(row))
}

func (s *Server) handleDeleteAutomation(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetAutomation(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.OrganisationID, "automations:manage"); err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteAutomation(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListAutomationRuns(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "automations:read"); err != nil {
		writeError(w, err)
		return
	}
	rows, err := s.svc.ListAutomationRuns(r.Context(), orgID, r.URL.Query().Get("automation_id"), 50)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, automationRunJSON(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func automationJSON(a sqlc.Automation) map[string]any {
	var conditions any
	var actions any
	_ = json.Unmarshal(a.Conditions, &conditions)
	_ = json.Unmarshal(a.Actions, &actions)
	if conditions == nil {
		conditions = map[string]any{"all": []any{}}
	}
	if actions == nil {
		actions = []any{}
	}
	return map[string]any{
		"id":              a.ID,
		"organisation_id": a.OrganisationID,
		"name":            a.Name,
		"trigger":         a.Trigger,
		"enabled":         a.Enabled,
		"conditions":      conditions,
		"actions":         actions,
		"created_at":      a.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":      a.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func automationRunJSON(r sqlc.AutomationRun) map[string]any {
	var payload any
	_ = json.Unmarshal(r.Payload, &payload)
	if payload == nil {
		payload = map[string]any{}
	}
	return map[string]any{
		"id":              r.ID,
		"organisation_id": r.OrganisationID,
		"automation_id":   r.AutomationID,
		"trigger":         r.Trigger,
		"status":          r.Status,
		"detail":          r.Detail,
		"payload":         payload,
		"created_at":      r.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}
