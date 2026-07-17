package httpserver

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

func (s *Server) handleAuthProviders(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"google":    s.google != nil && s.google.Enabled(),
		"dev_login": s.authDevLogin,
	})
}

func (s *Server) handleGoogleStart(w http.ResponseWriter, r *http.Request) {
	if s.google == nil || !s.google.Enabled() {
		writeError(w, apperr.Validation("google oauth is not configured"))
		return
	}
	state, err := service.SignOAuthState(s.oauthStateSecret, 10*time.Minute)
	if err != nil {
		writeError(w, err)
		return
	}
	http.Redirect(w, r, s.google.AuthCodeURL(state), http.StatusFound)
}

func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if s.google == nil || !s.google.Enabled() {
		writeError(w, apperr.Validation("google oauth is not configured"))
		return
	}
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		writeError(w, apperr.Unauthorized("google oauth error: "+errMsg))
		return
	}
	state := r.URL.Query().Get("state")
	if err := service.VerifyOAuthState(s.oauthStateSecret, state); err != nil {
		writeError(w, err)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, apperr.Unauthorized("missing oauth code"))
		return
	}
	tok, err := s.google.Exchange(r.Context(), code)
	if err != nil {
		writeError(w, err)
		return
	}
	identity, err := s.google.FetchIdentity(r.Context(), tok)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := s.svc.LoginWithGoogle(r.Context(), identity, s.platformAdminEmails)
	if err != nil {
		writeError(w, err)
		return
	}
	// Hand the session token to the SPA via fragment (not sent to server on later navigations).
	redirect := strings.TrimRight(s.appOrigin, "/") + "/#token=" + url.QueryEscape(result.Token)
	http.Redirect(w, r, redirect, http.StatusFound)
}

func (s *Server) handleDevLogin(w http.ResponseWriter, r *http.Request) {
	if !s.authDevLogin {
		writeError(w, apperr.Forbidden("dev login is disabled"))
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
	result, err := s.svc.DevLogin(r.Context(), body.Email, body.Name, s.platformAdminEmails)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, authResultJSON(result))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequireUser(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.Logout(r.Context(), p.SessionID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequireUser(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	me, err := s.svc.Me(r.Context(), p.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(me.Memberships))
	for _, row := range me.Memberships {
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
	writeJSON(w, http.StatusOK, map[string]any{
		"user":           userJSON(me.User),
		"memberships":    items,
		"platform_admin": p.PlatformAdmin || me.User.PlatformAdmin,
	})
}
