package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// HashAPIToken returns a stable SHA-256 hex digest of a bearer token.
func HashAPIToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// AuthenticateToken validates an env bootstrap token or a DB api key.
// Returns a short actor label for audit logs.
func (s *Service) AuthenticateToken(ctx context.Context, raw string, envTokens []string) (actor string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	for _, t := range envTokens {
		if subtle.ConstantTimeCompare([]byte(raw), []byte(t)) == 1 {
			return "env-token", true
		}
	}
	key, err := s.db.Q().GetAPIKeyByHash(ctx, HashAPIToken(raw))
	if err != nil {
		return "", false
	}
	return "api-key:" + key.Name, true
}

type CreateAPIKeyInput struct {
	Name string
}

type CreatedAPIKey struct {
	Key sqlc.ApiKey
	Raw string // shown once
}

func (s *Service) CreateAPIKey(ctx context.Context, in CreateAPIKeyInput) (CreatedAPIKey, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return CreatedAPIKey{}, apperr.Validation("name is required")
	}
	raw, err := newAPIToken()
	if err != nil {
		return CreatedAPIKey{}, err
	}
	prefix := raw
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	row, err := s.db.Q().CreateAPIKey(ctx, sqlc.CreateAPIKeyParams{
		ID:        ids.New(),
		Name:      name,
		KeyPrefix: prefix,
		KeyHash:   HashAPIToken(raw),
	})
	if err != nil {
		return CreatedAPIKey{}, err
	}
	return CreatedAPIKey{Key: row, Raw: raw}, nil
}

func (s *Service) ListAPIKeys(ctx context.Context) ([]sqlc.ApiKey, error) {
	return s.db.Q().ListAPIKeys(ctx)
}

func (s *Service) RevokeAPIKey(ctx context.Context, id string) (sqlc.ApiKey, error) {
	key, err := s.db.Q().RevokeAPIKey(ctx, id)
	return key, mapNotFound(err, "api key not found")
}

func (s *Service) RecordAudit(ctx context.Context, actor, method, path string, status int, orgID string) error {
	params := sqlc.InsertAuditEventParams{
		ID:         ids.New(),
		Actor:      actor,
		Method:     method,
		Path:       path,
		StatusCode: int32(status),
	}
	if orgID != "" {
		params.OrganisationID = pgtype.Text{String: orgID, Valid: true}
	}
	_, err := s.db.Q().InsertAuditEvent(ctx, params)
	return err
}

func (s *Service) ListAuditEvents(ctx context.Context, limit int32) ([]sqlc.AuditEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.db.Q().ListAuditEvents(ctx, limit)
}

func newAPIToken() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "kyc_" + hex.EncodeToString(b[:]), nil
}
