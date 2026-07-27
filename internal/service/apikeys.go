package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/observability"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// HashAPIToken returns a stable SHA-256 hex digest of a bearer token.
func HashAPIToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// AuthenticateToken validates a bearer token and returns an audit actor label.
func (s *Service) AuthenticateToken(ctx context.Context, raw string, envTokens []string) (actor string, ok bool) {
	p, ok := s.AuthenticateBearer(ctx, raw, envTokens, nil)
	if !ok {
		return "", false
	}
	return p.ActorLabel(), true
}

type CreateAPIKeyInput struct {
	Name   string
	Scopes []string
}

type CreatedAPIKey struct {
	Key sqlc.ApiKey
	Raw string // shown once
}

func (s *Service) CreateAPIKey(ctx context.Context, in CreateAPIKeyInput) (CreatedAPIKey, error) {
	return s.createAPIKey(ctx, "", in)
}

func (s *Service) CreateOrganisationAPIKey(ctx context.Context, orgID string, in CreateAPIKeyInput) (CreatedAPIKey, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return CreatedAPIKey{}, err
	}
	allowed, err := s.CheckEntitlement(ctx, orgID, "api_access")
	if err != nil {
		return CreatedAPIKey{}, err
	}
	if !allowed {
		return CreatedAPIKey{}, apperr.Forbidden("organisation lacks api_access entitlement")
	}
	return s.createAPIKey(ctx, orgID, in)
}

func (s *Service) createAPIKey(ctx context.Context, orgID string, in CreateAPIKeyInput) (CreatedAPIKey, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return CreatedAPIKey{}, apperr.Validation("name is required")
	}
	scopes, err := s.normalizeAPIKeyScopes(ctx, in.Scopes)
	if err != nil {
		return CreatedAPIKey{}, err
	}
	// Platform keys ignore scopes.
	if orgID == "" {
		scopes = []string{}
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
		ID:             ids.New(),
		Name:           name,
		KeyPrefix:      prefix,
		KeyHash:        HashAPIToken(raw),
		OrganisationID: textArg(orgID),
		Scopes:         scopes,
	})
	if err != nil {
		return CreatedAPIKey{}, err
	}
	if orgID != "" {
		s.recordActivity(ctx, observability.Activity{
			OrganisationID: orgID,
			Action:         observability.ActionAPIKeyCreated,
			ResourceType:   "api_key",
			ResourceID:     row.ID,
			Summary:        "API key created",
			Payload: map[string]any{
				"name":       row.Name,
				"key_prefix": row.KeyPrefix,
			},
		})
	}
	return CreatedAPIKey{Key: row, Raw: raw}, nil
}

func (s *Service) normalizeAPIKeyScopes(ctx context.Context, scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return []string{}, nil
	}
	seen := make(map[string]struct{}, len(scopes))
	clean := make([]string, 0, len(scopes))
	for _, raw := range scopes {
		key := strings.TrimSpace(raw)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		clean = append(clean, key)
	}
	if len(clean) == 0 {
		return []string{}, nil
	}
	rows, err := s.db.Q().ListPermissionIDsByKeys(ctx, clean)
	if err != nil {
		return nil, err
	}
	if len(rows) != len(clean) {
		known := make(map[string]struct{}, len(rows))
		for _, r := range rows {
			known[r.Key] = struct{}{}
		}
		for _, key := range clean {
			if _, ok := known[key]; !ok {
				return nil, apperr.Validation("unknown scope " + key)
			}
		}
		return nil, apperr.Validation("unknown scope")
	}
	sort.Strings(clean)
	return clean, nil
}

func (s *Service) ListAPIKeys(ctx context.Context) ([]sqlc.ApiKey, error) {
	return s.db.Q().ListAPIKeys(ctx)
}

func (s *Service) ListOrganisationAPIKeys(ctx context.Context, orgID string) ([]sqlc.ApiKey, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return nil, err
	}
	return s.db.Q().ListAPIKeysByOrg(ctx, textArg(orgID))
}

func (s *Service) RevokeAPIKey(ctx context.Context, id string) (sqlc.ApiKey, error) {
	key, err := s.db.Q().RevokeAPIKey(ctx, id)
	if err != nil {
		return key, mapNotFound(err, "api key not found")
	}
	if key.OrganisationID.Valid && key.OrganisationID.String != "" {
		s.recordActivity(ctx, observability.Activity{
			OrganisationID: key.OrganisationID.String,
			Action:         observability.ActionAPIKeyRevoked,
			ResourceType:   "api_key",
			ResourceID:     key.ID,
			Summary:        "API key revoked",
			Payload: map[string]any{
				"name":       key.Name,
				"key_prefix": key.KeyPrefix,
			},
		})
	}
	return key, nil
}

func (s *Service) GetAPIKey(ctx context.Context, id string) (sqlc.ApiKey, error) {
	key, err := s.db.Q().GetAPIKey(ctx, id)
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
