package httpserver

import (
	"net/http"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/authn"
	"github.com/gsarmaonline/kyc/internal/service"
)

func (s *Server) handleCreateOrganisation(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequirePrincipal(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	in := service.CreateOrganisationInput{
		Name:        body.Name,
		Slug:        body.Slug,
		AttachTrial: true,
	}
	if p.Kind == authn.KindUser {
		in.OwnerUserID = p.UserID
	} else if !p.IsPlatform() {
		writeError(w, apperr.Forbidden("cannot create organisation"))
		return
	}
	org, err := s.svc.CreateOrganisation(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, orgJSON(org))
}

func (s *Server) handleListOrganisations(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequirePrincipal(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	q := r.URL.Query()
	var list []map[string]any
	if p.IsPlatform() {
		rows, err := s.svc.ListOrganisations(r.Context(), q.Get("status"), q.Get("q"), queryLimit(r), q.Get("cursor"))
		if err != nil {
			writeError(w, err)
			return
		}
		list = make([]map[string]any, 0, len(rows))
		for _, o := range rows {
			list = append(list, orgJSON(o))
		}
	} else {
		rows, err := s.svc.ListOrganisationsForUser(r.Context(), p.UserID, q.Get("status"), q.Get("q"), queryLimit(r), q.Get("cursor"))
		if err != nil {
			writeError(w, err)
			return
		}
		list = make([]map[string]any, 0, len(rows))
		for _, o := range rows {
			list = append(list, orgJSON(o))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (s *Server) handleGetOrganisation(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgMember(r.Context(), orgID); err != nil {
		writeError(w, err)
		return
	}
	org, err := s.svc.GetOrganisation(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orgJSON(org))
}

func (s *Server) handlePatchOrganisation(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name   *string `json:"name"`
		Status *string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	org, err := s.svc.UpdateOrganisation(r.Context(), orgID, service.UpdateOrganisationInput{
		Name: body.Name, Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orgJSON(org))
}

func (s *Server) handleArchiveOrganisation(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	org, err := s.svc.ArchiveOrganisation(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orgJSON(org))
}
