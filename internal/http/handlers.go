package httpserver

import (
	"net/http"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		User struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"user"`
		Organisation struct {
			Name string `json:"name"`
			Slug string `json:"slug"`
		} `json:"organisation"`
		PlanKey string `json:"plan_key"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	result, status, err := s.svc.Signup(r.Context(), service.SignupInput{
		UserEmail:        body.User.Email,
		UserName:         body.User.Name,
		OrganisationName: body.Organisation.Name,
		OrganisationSlug: body.Organisation.Slug,
		PlanKey:          body.PlanKey,
	}, r.Header.Get("Idempotency-Key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, status, map[string]any{
		"user":         userJSON(result.User),
		"organisation": orgJSON(result.Organisation),
		"membership":   membershipJSON(result.Membership),
		"subscription": subscriptionJSON(result.Subscription),
	})
}

func (s *Server) handleCreateOrganisation(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	org, err := s.svc.CreateOrganisation(r.Context(), service.CreateOrganisationInput{
		Name: body.Name,
		Slug: body.Slug,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, orgJSON(org))
}

func (s *Server) handleListOrganisations(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	orgs, err := s.svc.ListOrganisations(r.Context(), q.Get("status"), q.Get("q"), queryLimit(r), q.Get("cursor"))
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(orgs))
	for _, o := range orgs {
		items = append(items, orgJSON(o))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetOrganisation(w http.ResponseWriter, r *http.Request) {
	org, err := s.svc.GetOrganisation(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orgJSON(org))
}

func (s *Server) handlePatchOrganisation(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   *string `json:"name"`
		Status *string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	org, err := s.svc.UpdateOrganisation(r.Context(), r.PathValue("id"), service.UpdateOrganisationInput{
		Name:   body.Name,
		Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orgJSON(org))
}

func (s *Server) handleArchiveOrganisation(w http.ResponseWriter, r *http.Request) {
	org, err := s.svc.ArchiveOrganisation(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orgJSON(org))
}

func (s *Server) handleListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := s.svc.ListRoles(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		items = append(items, map[string]any{
			"id":              role.ID,
			"organisation_id": role.OrganisationID,
			"key":             role.Key,
			"name":            role.Name,
			"description":     role.Description,
			"is_system":       role.IsSystem,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	user, err := s.svc.CreateUser(r.Context(), service.CreateUserInput{Email: body.Email, Name: body.Name})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, userJSON(user))
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
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
	user, err := s.svc.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, userJSON(user))
}

func (s *Server) handlePatchUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   *string `json:"name"`
		Status *string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	user, err := s.svc.UpdateUser(r.Context(), r.PathValue("id"), service.UpdateUserInput{
		Name: body.Name, Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, userJSON(user))
}

func (s *Server) handleListUserMemberships(w http.ResponseWriter, r *http.Request) {
	rows, err := s.svc.ListUserMemberships(r.Context(), r.PathValue("id"))
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

func (s *Server) handleCreateMembership(w http.ResponseWriter, r *http.Request) {
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
	m, err := s.svc.CreateMembership(r.Context(), r.PathValue("id"), service.CreateMembershipInput{
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
	rows, err := s.svc.ListOrganisationMemberships(r.Context(), r.PathValue("id"))
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

func (s *Server) handleAcceptMembership(w http.ResponseWriter, r *http.Request) {
	m, err := s.svc.AcceptMembership(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, membershipJSON(m))
}

func (s *Server) handlePatchMembership(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RoleID *string `json:"role_id"`
		Status *string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	m, err := s.svc.UpdateMembership(r.Context(), r.PathValue("id"), service.UpdateMembershipInput{
		RoleID: body.RoleID,
		Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, membershipJSON(m))
}

func (s *Server) handleRevokeMembership(w http.ResponseWriter, r *http.Request) {
	m, err := s.svc.RevokeMembership(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, membershipJSON(m))
}
