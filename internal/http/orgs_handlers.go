package httpserver

import (
	"io"
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
		Name:              body.Name,
		Slug:              body.Slug,
		AttachDefaultPlan: true,
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
		Name                   *string `json:"name"`
		Status                 *string `json:"status"`
		PrimaryColor           *string `json:"primary_color"`
		AccentColor            *string `json:"accent_color"`
		EmailFooter            *string `json:"email_footer"`
		EmailFont              *string `json:"email_font"`
		AppUserAuthority       *string `json:"app_user_authority"`
		AppUserIngestUpsertKey *string `json:"app_user_ingest_upsert_key"`
		AppUserAttributesMode  *string `json:"app_user_attributes_mode"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	org, err := s.svc.UpdateOrganisation(r.Context(), orgID, service.UpdateOrganisationInput{
		Name: body.Name, Status: body.Status,
		PrimaryColor: body.PrimaryColor, AccentColor: body.AccentColor,
		EmailFooter: body.EmailFooter, EmailFont: body.EmailFont,
		AppUserAuthority:       body.AppUserAuthority,
		AppUserIngestUpsertKey: body.AppUserIngestUpsertKey,
		AppUserAttributesMode:  body.AppUserAttributesMode,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orgJSON(org))
}

func (s *Server) handleUploadOrganisationLogo(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	if err := r.ParseMultipartForm(service.MaxLogoBytes + 1<<20); err != nil {
		writeError(w, apperr.Validation("invalid multipart form"))
		return
	}
	file, header, err := r.FormFile("logo")
	if err != nil {
		writeError(w, apperr.Validation("logo file is required"))
		return
	}
	defer file.Close()
	ct := header.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/octet-stream"
	}
	org, err := s.svc.SetOrganisationLogo(r.Context(), orgID, file, ct)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orgJSON(org))
}

func (s *Server) handleDeleteOrganisationLogo(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	org, err := s.svc.ClearOrganisationLogo(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orgJSON(org))
}

func (s *Server) handlePublicOrganisationLogo(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	rc, ct, err := s.svc.OpenOrganisationLogo(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}

func (s *Server) handleArchiveOrganisation(w http.ResponseWriter, r *http.Request) {
	// Legacy path: hard-delete (same as DELETE /v1/organisations/{id}).
	s.handleDeleteOrganisation(w, r)
}

func (s *Server) handleDeleteOrganisation(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermissionAnyStatus(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteOrganisation(r.Context(), orgID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": orgID})
}
