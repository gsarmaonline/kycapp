package httpserver

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/observability"
)

func activityJSON(a observability.Activity) map[string]any {
	payload := a.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	return map[string]any{
		"id":                a.ID,
		"organisation_id":   a.OrganisationID,
		"organisation_slug": a.OrganisationSlug,
		"organisation_name": a.OrganisationName,
		"actor_type":        a.ActorType,
		"actor_id":          a.ActorID,
		"actor_label":       a.ActorLabel,
		"action":            a.Action,
		"resource_type":     a.ResourceType,
		"resource_id":       a.ResourceID,
		"summary":           a.Summary,
		"payload":           payload,
		"created_at":        a.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func usageJSON(u observability.UsageRow) map[string]any {
	return map[string]any{
		"organisation_id": u.OrganisationID,
		"meter_key":       u.MeterKey,
		"period_start":    u.PeriodStart.UTC().Format(time.RFC3339),
		"dim1_key":        u.Dim1Key,
		"dim1_value":      u.Dim1Value,
		"dim2_key":        u.Dim2Key,
		"dim2_value":      u.Dim2Value,
		"count":           u.Count,
		"updated_at":      u.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (s *Server) handleListOrgActivity(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	limit := int32(50)
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			writeError(w, apperr.Validation("limit must be a positive integer"))
			return
		}
		if n > 200 {
			n = 200
		}
		limit = int32(n)
	}
	rows, err := s.svc.ListOrganisationActivity(r.Context(), orgID, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, activityJSON(a))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleListOrgUsage(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var from, to time.Time
	if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, apperr.Validation("from must be RFC3339"))
			return
		}
		from = t
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, apperr.Validation("to must be RFC3339"))
			return
		}
		to = t
	}
	rows, err := s.svc.ListOrganisationUsage(r.Context(), orgID, from, to)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, u := range rows {
		out = append(out, usageJSON(u))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}
