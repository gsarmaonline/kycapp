package httpserver

import (
	"net/http"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

func featureFlagJSON(v service.FeatureFlagView) map[string]any {
	overrides := make([]map[string]any, 0, len(v.Overrides))
	for _, o := range v.Overrides {
		overrides = append(overrides, map[string]any{
			"subject_id": o.SubjectID,
			"effect":     o.Effect,
		})
	}
	return map[string]any{
		"id":                  v.Flag.ID,
		"organisation_id":     v.Flag.OrganisationID,
		"key":                 v.Flag.Key,
		"description":         v.Flag.Description,
		"enabled":             v.Flag.Enabled,
		"rollout_percentage":  v.Flag.RolloutPercentage,
		"overrides":           overrides,
		"created_at":          v.Flag.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":          v.Flag.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func (s *Server) handleCreateFeatureFlag(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "feature_flags:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Key               string `json:"key"`
		Description       string `json:"description"`
		Enabled           *bool  `json:"enabled"`
		RolloutPercentage *int32 `json:"rollout_percentage"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	view, err := s.svc.CreateFeatureFlag(r.Context(), orgID, service.CreateFeatureFlagInput{
		Key:               body.Key,
		Description:       body.Description,
		Enabled:           body.Enabled,
		RolloutPercentage: body.RolloutPercentage,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, featureFlagJSON(view))
}

func (s *Server) handleListFeatureFlags(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "feature_flags:read"); err != nil {
		writeError(w, err)
		return
	}
	items, err := s.svc.ListFeatureFlags(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, v := range items {
		out = append(out, featureFlagJSON(v))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleGetFeatureFlag(w http.ResponseWriter, r *http.Request) {
	view, err := s.svc.GetFeatureFlag(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), view.Flag.OrganisationID, "feature_flags:read"); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, featureFlagJSON(view))
}

func (s *Server) handlePatchFeatureFlag(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetFeatureFlag(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.Flag.OrganisationID, "feature_flags:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Description       *string `json:"description"`
		Enabled           *bool   `json:"enabled"`
		RolloutPercentage *int32  `json:"rollout_percentage"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	view, err := s.svc.UpdateFeatureFlag(r.Context(), r.PathValue("id"), service.UpdateFeatureFlagInput{
		Description:       body.Description,
		Enabled:           body.Enabled,
		RolloutPercentage: body.RolloutPercentage,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, featureFlagJSON(view))
}

func (s *Server) handleDeleteFeatureFlag(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetFeatureFlag(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.Flag.OrganisationID, "feature_flags:manage"); err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteFeatureFlag(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleSetFeatureFlagOverrides(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetFeatureFlag(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.Flag.OrganisationID, "feature_flags:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Overrides []struct {
			SubjectID string `json:"subject_id"`
			Effect    string `json:"effect"`
		} `json:"overrides"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	overrides := make([]service.FeatureFlagOverrideInput, 0, len(body.Overrides))
	for _, o := range body.Overrides {
		overrides = append(overrides, service.FeatureFlagOverrideInput{
			SubjectID: o.SubjectID,
			Effect:    o.Effect,
		})
	}
	view, err := s.svc.SetFeatureFlagOverrides(r.Context(), r.PathValue("id"), service.SetFeatureFlagOverridesInput{
		Overrides: overrides,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, featureFlagJSON(view))
}

func (s *Server) handleFeatureFlagsCheck(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequirePrincipal(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		OrganisationID string `json:"organisation_id"`
		Flag           string `json:"flag"`
		SubjectID      string `json:"subject_id"`
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
	result, err := s.svc.CheckFeatureFlag(r.Context(), body.OrganisationID, body.Flag, body.SubjectID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": result.Enabled,
		"reason":  result.Reason,
	})
}
