package httpserver

import (
	"net/http"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

func (s *Server) handleCreateMembership(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
		RoleID string `json:"role_id"`
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	m, err := s.svc.CreateMembership(r.Context(), orgID, service.CreateMembershipInput{
		UserID: body.UserID,
		Email:  body.Email,
		RoleID: body.RoleID,
		Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, membershipJSON(m))
}

func (s *Server) handleListMemberships(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	rows, err := s.svc.ListOrganisationMemberships(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id":              row.ID,
			"organisation_id": row.OrganisationID,
			"user_id":         row.UserID,
			"role_id":         row.RoleID,
			"status":          row.Status,
			"created_at":      row.CreatedAt.UTC().Format(time.RFC3339Nano),
			"user_email":      row.UserEmail,
			"user_name":       row.UserName,
			"role_key":        row.RoleKey,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetMembership(w http.ResponseWriter, r *http.Request) {
	row, err := s.svc.GetMembershipDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), row.OrganisationID, "members:read"); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":              row.ID,
		"organisation_id": row.OrganisationID,
		"user_id":         row.UserID,
		"role_id":         row.RoleID,
		"status":          row.Status,
		"created_at":      row.CreatedAt.UTC().Format(time.RFC3339Nano),
		"user_email":      row.UserEmail,
		"user_name":       row.UserName,
		"role_key":        row.RoleKey,
	})
}

func (s *Server) handleAcceptMembership(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequireUser(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	m, err := s.svc.AcceptMembershipAsUser(r.Context(), r.PathValue("id"), p.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, membershipJSON(m))
}

func (s *Server) handlePatchMembership(w http.ResponseWriter, r *http.Request) {
	m, err := s.svc.GetMembership(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), m.OrganisationID, "members:invite"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		RoleID *string `json:"role_id"`
		Status *string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	out, err := s.svc.UpdateMembership(r.Context(), r.PathValue("id"), service.UpdateMembershipInput{
		RoleID: body.RoleID, Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, membershipJSON(out))
}

func (s *Server) handleRevokeMembership(w http.ResponseWriter, r *http.Request) {
	m, err := s.svc.GetMembership(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), m.OrganisationID, "members:remove"); err != nil {
		writeError(w, err)
		return
	}
	out, err := s.svc.RevokeMembership(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, membershipJSON(out))
}

// handleExplainMembershipAccess answers every permission for one member and
// returns the route the graph took to each answer.
//
// The gate lives in the service rather than here, because the organisation is
// only known after the membership is loaded, and because the same call decides
// how much of each route this caller may be shown.
// handleListOperatorRoles returns the organisation's roles and what each
// inherits. It is what makes owner, admin and member legible: the schema map
// draws the model, and these are instances of it.
func (s *Server) handleListOperatorRoles(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc.ListOperatorRoles(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleExplainMembershipAccess(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc.ExplainMembershipAccess(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
