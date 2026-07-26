package httpserver

import (
	"net/http"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

func (s *Server) handleGetOrgOnboarding(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	view, err := s.svc.GetOrganisationOnboarding(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, onboardingJSON(view))
}

func (s *Server) handlePatchOrgOnboarding(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		Dismissed *bool `json:"dismissed"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	if body.Dismissed == nil {
		writeError(w, apperr.Validation("dismissed is required"))
		return
	}
	view, err := s.svc.DismissOrganisationOnboarding(r.Context(), orgID, service.DismissOnboardingInput{
		Dismissed: *body.Dismissed,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, onboardingJSON(view))
}

func onboardingJSON(view service.OnboardingView) map[string]any {
	steps := make([]map[string]any, 0, len(view.Steps))
	for _, step := range view.Steps {
		steps = append(steps, map[string]any{
			"key":   step.Key,
			"label": step.Label,
			"done":  step.Done,
			"href":  step.Href,
		})
	}
	return map[string]any{
		"visible":         view.Visible,
		"dismissed":       view.Dismissed,
		"completed_count": view.CompletedCount,
		"total_count":     view.TotalCount,
		"steps":           steps,
	}
}
