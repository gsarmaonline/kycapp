package httpserver

import (
	"net/http"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	user, err := s.svc.CreateUser(r.Context(), service.CreateUserInput{
		Email: body.Email, Name: body.Name,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, userJSON(user))
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	q := r.URL.Query()
	users, err := s.svc.ListUsers(r.Context(), q.Get("q"), queryLimit(r), q.Get("cursor"))
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(users))
	for _, u := range users {
		items = append(items, userJSON(u))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequirePrincipal(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	id := r.PathValue("id")
	if !p.IsPlatform() && p.UserID != id {
		writeError(w, apperr.Forbidden("cannot view other users"))
		return
	}
	user, err := s.svc.GetUser(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, userJSON(user))
}

func (s *Server) handlePatchUser(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequirePrincipal(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	id := r.PathValue("id")
	var body struct {
		Name   *string `json:"name"`
		Status *string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	if !p.IsPlatform() {
		if p.UserID != id {
			writeError(w, apperr.Forbidden("cannot update other users"))
			return
		}
		if body.Status != nil {
			writeError(w, apperr.Forbidden("cannot change own status"))
			return
		}
	}
	user, err := s.svc.UpdateUser(r.Context(), id, service.UpdateUserInput{
		Name: body.Name, Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, userJSON(user))
}

func (s *Server) handleListUserMemberships(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequirePrincipal(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	id := r.PathValue("id")
	if !p.IsPlatform() && p.UserID != id {
		writeError(w, apperr.Forbidden("cannot view other users' memberships"))
		return
	}
	rows, err := s.svc.ListUserMemberships(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id":                  row.ID,
			"organisation_id":     row.OrganisationID,
			"user_id":             row.UserID,
			"role_id":             row.RoleID,
			"status":              row.Status,
			"created_at":          row.CreatedAt.UTC().Format(time.RFC3339Nano),
			"organisation_name":   row.OrganisationName,
			"organisation_slug":   row.OrganisationSlug,
			"organisation_status": row.OrganisationStatus,
			"role_key":            row.RoleKey,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
