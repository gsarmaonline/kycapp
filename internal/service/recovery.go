package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
)

// Recovery credentials are the data-backed replacement for a shared environment
// token. They resolve to an ordinary global-scope grant, so the access path has
// no bypass in it: the evaluator decides, exactly as it does for a membership.
//
// What they gain over an environment token: a named person minted it, a stated
// reason, a required expiry, revocation without a deploy, and an audit trail
// that says who rather than "env-token".
//
// What they cannot do is recover a database that is itself broken, which is why
// API_TOKENS still exists as a last resort. See docs/authentication.md.

const (
	// MaxRecoveryTTL bounds how long a recovery credential may live. A
	// long-lived one is a permanent bypass under another name.
	MaxRecoveryTTL = 7 * 24 * time.Hour
	// DefaultRecoveryTTL is used when no expiry is requested.
	DefaultRecoveryTTL = 24 * time.Hour
)

type CreateRecoveryInput struct {
	Name   string
	Reason string
	TTL    time.Duration
}

type CreatedRecovery struct {
	Credential sqlc.RecoveryCredential
	// Raw is returned exactly once, at creation.
	Raw string
}

// CreateRecoveryCredential mints a credential that reaches every organisation.
//
// The caller must already hold global reach, so this is delegation rather than
// escalation: you cannot mint a way around a boundary you are not already
// inside. Invariant 2 holds without a special case.
func (s *Service) CreateRecoveryCredential(ctx context.Context, in CreateRecoveryInput) (CreatedRecovery, error) {
	p, err := RequireUser(ctx)
	if err != nil {
		return CreatedRecovery{}, apperr.Forbidden("a user session is required to mint a recovery credential")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return CreatedRecovery{}, apperr.Validation("name is required")
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return CreatedRecovery{}, apperr.Validation("reason is required: a recovery credential with no stated reason is a back door")
	}

	global, err := s.ReachesEveryOrganisation(ctx, p)
	if err != nil {
		return CreatedRecovery{}, err
	}
	if !global {
		return CreatedRecovery{}, apperr.Forbidden("only a principal that already reaches every organisation may mint a recovery credential")
	}

	ttl := in.TTL
	if ttl <= 0 {
		ttl = DefaultRecoveryTTL
	}
	if ttl > MaxRecoveryTTL {
		return CreatedRecovery{}, apperr.Validation("expiry is longer than the maximum recovery lifetime")
	}

	raw, err := newRecoveryToken()
	if err != nil {
		return CreatedRecovery{}, err
	}
	prefix := raw
	if len(prefix) > 16 {
		prefix = prefix[:16]
	}
	row, err := s.db.Q().CreateRecoveryCredential(ctx, sqlc.CreateRecoveryCredentialParams{
		ID:          ids.New(),
		Name:        name,
		TokenPrefix: prefix,
		TokenHash:   HashAPIToken(raw),
		GrantedBy:   p.UserID,
		Reason:      reason,
		ExpiresAt:   time.Now().UTC().Add(ttl),
	})
	if err != nil {
		return CreatedRecovery{}, err
	}
	return CreatedRecovery{Credential: row, Raw: raw}, nil
}

func (s *Service) ListRecoveryCredentials(ctx context.Context, limit int32) ([]sqlc.RecoveryCredential, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.db.Q().ListRecoveryCredentials(ctx, limit)
}

func (s *Service) RevokeRecoveryCredential(ctx context.Context, id string) (sqlc.RecoveryCredential, error) {
	row, err := s.db.Q().RevokeRecoveryCredential(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.RecoveryCredential{}, apperr.NotFound("recovery credential not found")
	}
	return row, err
}

// newRecoveryToken uses a distinct prefix so a leaked credential is obvious in
// a log or a paste, and greppable in an incident.
func newRecoveryToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "kyc_recovery_" + hex.EncodeToString(b[:]), nil
}
