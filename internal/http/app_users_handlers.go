package httpserver

import (
	"net/http"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

func (s *Server) handleCreateAttributeDefinition(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "attributes:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Key         string   `json:"key"`
		Label       string   `json:"label"`
		Description string   `json:"description"`
		ValueType   string   `json:"value_type"`
		Section     string   `json:"section"`
		SortOrder   int32    `json:"sort_order"`
		Required    bool     `json:"required"`
		EnumValues  []string `json:"enum_values"`
		IsPII       bool     `json:"is_pii"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.CreateAttributeDefinition(r.Context(), orgID, service.CreateAttributeDefinitionInput{
		Key: body.Key, Label: body.Label, Description: body.Description,
		ValueType: body.ValueType, Section: body.Section, SortOrder: body.SortOrder,
		Required: body.Required, EnumValues: body.EnumValues, IsPII: body.IsPII,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, attributeDefinitionJSON(row))
}

func (s *Server) handleListAttributeDefinitions(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "attributes:read"); err != nil {
		writeError(w, err)
		return
	}
	rows, err := s.svc.ListAttributeDefinitions(r.Context(), orgID, r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, attributeDefinitionJSON(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetAttributeDefinition(w http.ResponseWriter, r *http.Request) {
	row, err := s.svc.GetAttributeDefinition(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), row.OrganisationID, "attributes:read"); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, attributeDefinitionJSON(row))
}

func (s *Server) handlePatchAttributeDefinition(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetAttributeDefinition(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.OrganisationID, "attributes:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Label       *string   `json:"label"`
		Description *string   `json:"description"`
		ValueType   *string   `json:"value_type"`
		Section     *string   `json:"section"`
		SortOrder   *int32    `json:"sort_order"`
		Required    *bool     `json:"required"`
		EnumValues  *[]string `json:"enum_values"`
		IsPII       *bool     `json:"is_pii"`
		Status      *string   `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.UpdateAttributeDefinition(r.Context(), r.PathValue("id"), service.UpdateAttributeDefinitionInput{
		Label: body.Label, Description: body.Description, ValueType: body.ValueType,
		Section: body.Section, SortOrder: body.SortOrder, Required: body.Required,
		EnumValues: body.EnumValues, IsPII: body.IsPII, Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, attributeDefinitionJSON(row))
}

func (s *Server) handleDeleteAttributeDefinition(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetAttributeDefinition(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.OrganisationID, "attributes:manage"); err != nil {
		writeError(w, err)
		return
	}
	row, err := s.svc.DeleteAttributeDefinition(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, attributeDefinitionJSON(row))
}

func (s *Server) handleCreateAppUser(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "app_users:write"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		ExternalID  string         `json:"external_id"`
		Email       string         `json:"email"`
		DisplayName string         `json:"display_name"`
		Status      string         `json:"status"`
		Attributes  map[string]any `json:"attributes"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	row, err := s.svc.CreateAppUser(r.Context(), orgID, service.CreateAppUserInput{
		ExternalID: body.ExternalID, Email: body.Email, DisplayName: body.DisplayName,
		Status: body.Status, Attributes: body.Attributes,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, appUserJSON(row))
}

func (s *Server) handleListAppUsers(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "app_users:read"); err != nil {
		writeError(w, err)
		return
	}
	rows, err := s.svc.ListAppUsers(r.Context(), orgID, r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, appUserJSON(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetAppUser(w http.ResponseWriter, r *http.Request) {
	row, err := s.svc.GetAppUser(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), row.OrganisationID, "app_users:read"); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, appUserJSON(row))
}

func (s *Server) handlePatchAppUser(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetAppUser(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.OrganisationID, "app_users:write"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		ExternalID  *string         `json:"external_id"`
		Email       *string         `json:"email"`
		DisplayName *string         `json:"display_name"`
		Status      *string         `json:"status"`
		Attributes  *map[string]any `json:"attributes"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	in := service.UpdateAppUserInput{
		ExternalID: body.ExternalID, Email: body.Email,
		DisplayName: body.DisplayName, Status: body.Status,
	}
	if body.Attributes != nil {
		in.Attributes = *body.Attributes
	}
	row, err := s.svc.UpdateAppUser(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, appUserJSON(row))
}

func (s *Server) handleDeleteAppUser(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetAppUser(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.OrganisationID, "app_users:write"); err != nil {
		writeError(w, err)
		return
	}
	row, err := s.svc.DeleteAppUser(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, appUserJSON(row))
}
