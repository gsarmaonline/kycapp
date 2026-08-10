package httpserver

import (
	"net/http"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

// Recovery credentials are platform-wide, so these routes are gated on a
// platform capability. Minting additionally requires that the caller already
// reaches every organisation, which the service enforces: a recovery credential
// is delegation, never a way around a boundary you are not already inside.

func recoveryJSON(c sqlc.RecoveryCredential) map[string]any {
	out := map[string]any{
		"id":           c.ID,
		"name":         c.Name,
		"token_prefix": c.TokenPrefix,
		"granted_by":   c.GrantedBy,
		"reason":       c.Reason,
		"expires_at":   c.ExpiresAt.UTC().Format(time.RFC3339Nano),
		"created_at":   c.CreatedAt.UTC().Format(time.RFC3339Nano),
		"active":       !c.RevokedAt.Valid && c.ExpiresAt.After(time.Now().UTC()),
	}
	if c.RevokedAt.Valid {
		out["revoked_at"] = c.RevokedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if c.LastUsedAt.Valid {
		out["last_used_at"] = c.LastUsedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	return out
}

func (s *Server) handleCreateRecoveryCredential(w http.ResponseWriter, r *http.Request) {
	if _, err := s.svc.RequirePlatformCapability(r.Context(), "api_keys:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name       string `json:"name"`
		Reason     string `json:"reason"`
		TTLMinutes int    `json:"ttl_minutes"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	created, err := s.svc.CreateRecoveryCredential(r.Context(), service.CreateRecoveryInput{
		Name:   body.Name,
		Reason: body.Reason,
		TTL:    time.Duration(body.TTLMinutes) * time.Minute,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	out := recoveryJSON(created.Credential)
	// Returned exactly once. There is no way to read it again.
	out["token"] = created.Raw
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleListRecoveryCredentials(w http.ResponseWriter, r *http.Request) {
	if _, err := s.svc.RequirePlatformCapability(r.Context(), "api_keys:read"); err != nil {
		writeError(w, err)
		return
	}
	rows, err := s.svc.ListRecoveryCredentials(r.Context(), 0)
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, c := range rows {
		items = append(items, recoveryJSON(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleRevokeRecoveryCredential(w http.ResponseWriter, r *http.Request) {
	if _, err := s.svc.RequirePlatformCapability(r.Context(), "api_keys:manage"); err != nil {
		writeError(w, err)
		return
	}
	row, err := s.svc.RevokeRecoveryCredential(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, recoveryJSON(row))
}
