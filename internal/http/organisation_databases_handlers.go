package httpserver

import (
	"net/http"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

func organisationDatabaseJSON(v service.OrganisationDatabaseView) map[string]any {
	out := map[string]any{
		"id":              v.ID,
		"organisation_id": v.OrganisationID,
		"name":            v.Name,
		"driver":          v.Driver,
		"host":            v.Host,
		"port":            v.Port,
		"database_name":   v.DatabaseName,
		"username":        v.Username,
		"password_hint":   v.PasswordHint,
		"has_password":    v.HasPassword,
		"ssl_mode":        v.SSLMode,
		"status":          v.Status,
		"last_error":      v.LastError,
	}
	if v.LastCheckedAt != nil {
		out["last_checked_at"] = v.LastCheckedAt.UTC().Format(time.RFC3339Nano)
	} else {
		out["last_checked_at"] = nil
	}
	return out
}

func (s *Server) handleListOrgDatabases(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	items, err := s.svc.ListOrganisationDatabases(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, organisationDatabaseJSON(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleCreateOrgDatabase(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		Name         string `json:"name"`
		Host         string `json:"host"`
		Port         int32  `json:"port"`
		DatabaseName string `json:"database_name"`
		Username     string `json:"username"`
		Password     string `json:"password"`
		SSLMode      string `json:"ssl_mode"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	view, err := s.svc.CreateOrganisationDatabase(r.Context(), orgID, service.CreateOrganisationDatabaseInput{
		Name: body.Name, Host: body.Host, Port: body.Port, DatabaseName: body.DatabaseName,
		Username: body.Username, Password: body.Password, SSLMode: body.SSLMode,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, organisationDatabaseJSON(view))
}

func (s *Server) handleGetOrgDatabase(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	dbID := r.PathValue("dbId")
	view, err := s.svc.GetOrganisationDatabase(r.Context(), orgID, dbID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, organisationDatabaseJSON(view))
}

func (s *Server) handlePatchOrgDatabase(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	dbID := r.PathValue("dbId")
	var body struct {
		Name         *string `json:"name"`
		Host         *string `json:"host"`
		Port         *int32  `json:"port"`
		DatabaseName *string `json:"database_name"`
		Username     *string `json:"username"`
		Password     *string `json:"password"`
		SSLMode      *string `json:"ssl_mode"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	view, err := s.svc.UpdateOrganisationDatabase(r.Context(), orgID, dbID, service.UpdateOrganisationDatabaseInput{
		Name: body.Name, Host: body.Host, Port: body.Port, DatabaseName: body.DatabaseName,
		Username: body.Username, Password: body.Password, SSLMode: body.SSLMode,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, organisationDatabaseJSON(view))
}

func (s *Server) handleCheckOrgDatabase(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	dbID := r.PathValue("dbId")
	view, err := s.svc.CheckOrganisationDatabase(r.Context(), orgID, dbID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, organisationDatabaseJSON(view))
}

func (s *Server) handleDisconnectOrgDatabase(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	dbID := r.PathValue("dbId")
	view, err := s.svc.SetOrganisationDatabaseDisconnected(r.Context(), orgID, dbID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, organisationDatabaseJSON(view))
}

func (s *Server) handleDeleteOrgDatabase(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	dbID := r.PathValue("dbId")
	if err := s.svc.DeleteOrganisationDatabase(r.Context(), orgID, dbID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
