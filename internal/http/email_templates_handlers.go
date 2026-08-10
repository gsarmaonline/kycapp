package httpserver

import (
	"net/http"

	"github.com/gsarmaonline/kyc/core/emailtemplates"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

func (s *Server) handleCreateEmailTemplate(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		Key          string                       `json:"key"`
		Name         string                       `json:"name"`
		Description  string                       `json:"description"`
		Subject      string                       `json:"subject"`
		BodyText     string                       `json:"body_text"`
		BodyHTML     string                       `json:"body_html"`
		BodySections []emailtemplates.BodySection `json:"body_sections"`
		FromName     string                       `json:"from_name"`
		FromAddress  string                       `json:"from_address"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.CreateEmailTemplate(r.Context(), orgID, service.CreateEmailTemplateInput{
		Key: body.Key, Name: body.Name, Description: body.Description,
		Subject: body.Subject, BodyText: body.BodyText, BodyHTML: body.BodyHTML,
		BodySections: body.BodySections, FromName: body.FromName, FromAddress: body.FromAddress,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, emailTemplateJSON(row))
}

func (s *Server) handleListEmailTemplates(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	rows, err := s.svc.ListEmailTemplates(r.Context(), orgID, r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, emailTemplateJSON(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetEmailTemplate(w http.ResponseWriter, r *http.Request) {
	row, err := s.svc.GetEmailTemplate(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), row.OrganisationID, "email_templates:read"); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, emailTemplateJSON(row))
}

func (s *Server) handlePatchEmailTemplate(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetEmailTemplate(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.OrganisationID, "email_templates:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name         *string                       `json:"name"`
		Description  *string                       `json:"description"`
		Subject      *string                       `json:"subject"`
		BodyText     *string                       `json:"body_text"`
		BodyHTML     *string                       `json:"body_html"`
		BodySections *[]emailtemplates.BodySection `json:"body_sections"`
		FromName     *string                       `json:"from_name"`
		FromAddress  *string                       `json:"from_address"`
		Status       *string                       `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.UpdateEmailTemplate(r.Context(), r.PathValue("id"), service.UpdateEmailTemplateInput{
		Name: body.Name, Description: body.Description, Subject: body.Subject,
		BodyText: body.BodyText, BodyHTML: body.BodyHTML, BodySections: body.BodySections,
		FromName: body.FromName, FromAddress: body.FromAddress, Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, emailTemplateJSON(row))
}

func (s *Server) handleDeleteEmailTemplate(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetEmailTemplate(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.OrganisationID, "email_templates:manage"); err != nil {
		writeError(w, err)
		return
	}
	row, err := s.svc.DeleteEmailTemplate(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, emailTemplateJSON(row))
}
