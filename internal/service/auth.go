package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/authn"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const sessionTTL = 30 * 24 * time.Hour

// AuthenticateBearer resolves a Bearer token to a principal (session, env token, or API key).
func (s *Service) AuthenticateBearer(ctx context.Context, raw string, envTokens []string, platformAdminEmails []string) (authn.Principal, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return authn.Principal{}, false
	}

	if sess, err := s.db.Q().GetSessionByTokenHash(ctx, HashAPIToken(raw)); err == nil {
		if sess.UserStatus != "active" {
			return authn.Principal{}, false
		}
		admin := sess.PlatformAdmin || emailInList(sess.UserEmail, platformAdminEmails)
		return authn.Principal{
			Kind:          authn.KindUser,
			UserID:        sess.UserID,
			PlatformAdmin: admin,
			SessionID:     sess.ID,
			Actor:         "user:" + sess.UserID,
		}, true
	}

	for _, t := range envTokens {
		if len(raw) == len(t) && subtle.ConstantTimeCompare([]byte(raw), []byte(t)) == 1 {
			return authn.Principal{
				Kind:          authn.KindService,
				PlatformAdmin: true,
				Actor:         "env-token",
			}, true
		}
	}

	key, err := s.db.Q().GetAPIKeyByHash(ctx, HashAPIToken(raw))
	if err != nil {
		return authn.Principal{}, false
	}
	p := authn.Principal{
		Kind:  authn.KindService,
		Actor: "api-key:" + key.Name,
	}
	if key.OrganisationID.Valid {
		p.OrganisationID = key.OrganisationID.String
	} else {
		p.PlatformAdmin = true
	}
	return p, true
}

type AuthResult struct {
	User      sqlc.User
	Token     string
	ExpiresAt time.Time
	SessionID string
}

// GoogleIdentity is the verified profile from Google.
type GoogleIdentity struct {
	Sub           string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

// LoginWithGoogle upserts a user from a verified Google identity and issues a session.
func (s *Service) LoginWithGoogle(ctx context.Context, id GoogleIdentity, platformAdminEmails []string) (AuthResult, error) {
	sub := strings.TrimSpace(id.Sub)
	email := strings.ToLower(strings.TrimSpace(id.Email))
	name := strings.TrimSpace(id.Name)
	if sub == "" {
		return AuthResult{}, apperr.Unauthorized("google identity missing subject")
	}
	if email == "" || !strings.Contains(email, "@") {
		return AuthResult{}, apperr.Unauthorized("google identity missing email")
	}
	if !id.EmailVerified {
		return AuthResult{}, apperr.Unauthorized("google email is not verified")
	}
	if name == "" {
		name = email
	}
	admin := emailInList(email, platformAdminEmails)
	subArg := pgtype.Text{String: sub, Valid: true}
	avatar := strings.TrimSpace(id.Picture)
	avatarArg := pgtype.Text{String: avatar, Valid: avatar != ""}

	user, err := s.db.Q().GetUserByGoogleSub(ctx, subArg)
	if err == nil {
		return s.finishGoogleLogin(ctx, user, name, avatar, admin)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return AuthResult{}, err
	}

	existing, err := s.db.Q().GetUserByEmail(ctx, email)
	if err == nil {
		if existing.GoogleSub.Valid && existing.GoogleSub.String != sub {
			return AuthResult{}, apperr.Conflict("email already linked to another Google account")
		}
		user, err = s.db.Q().UpdateUser(ctx, sqlc.UpdateUserParams{
			ID:            existing.ID,
			Name:          pgtype.Text{String: name, Valid: true},
			GoogleSub:     subArg,
			AvatarUrl:     avatarArg,
			PlatformAdmin: pgtype.Bool{Bool: admin || existing.PlatformAdmin, Valid: true},
		})
		if err != nil {
			return AuthResult{}, err
		}
		return s.issueSession(ctx, user)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return AuthResult{}, err
	}

	user, err = s.db.Q().CreateUser(ctx, sqlc.CreateUserParams{
		ID:            ids.New(),
		Email:         email,
		Name:          name,
		Status:        "active",
		PlatformAdmin: admin,
		GoogleSub:     subArg,
		AvatarUrl:     avatar,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			return AuthResult{}, apperr.Conflict("email already exists")
		}
		return AuthResult{}, err
	}
	return s.issueSession(ctx, user)
}

// DevLogin issues a session for local/test use when AuthDevLogin is enabled.
func (s *Service) DevLogin(ctx context.Context, email, name string, platformAdminEmails []string) (AuthResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	name = strings.TrimSpace(name)
	if email == "" || !strings.Contains(email, "@") {
		return AuthResult{}, apperr.Validation("email is required")
	}
	if name == "" {
		name = email
	}
	admin := emailInList(email, platformAdminEmails)
	user, err := s.db.Q().GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		user, err = s.db.Q().CreateUser(ctx, sqlc.CreateUserParams{
			ID:            ids.New(),
			Email:         email,
			Name:          name,
			Status:        "active",
			PlatformAdmin: admin,
			AvatarUrl:     "",
		})
		if err != nil {
			if store.IsUniqueViolation(err) {
				return AuthResult{}, apperr.Conflict("email already exists")
			}
			return AuthResult{}, err
		}
	} else if err != nil {
		return AuthResult{}, err
	} else {
		if user.Status != "active" {
			return AuthResult{}, apperr.Unauthorized("account disabled")
		}
		if admin && !user.PlatformAdmin {
			user, err = s.db.Q().UpdateUser(ctx, sqlc.UpdateUserParams{
				ID:            user.ID,
				PlatformAdmin: pgtype.Bool{Bool: true, Valid: true},
			})
			if err != nil {
				return AuthResult{}, err
			}
		}
	}
	return s.issueSession(ctx, user)
}

func (s *Service) finishGoogleLogin(ctx context.Context, user sqlc.User, name, avatar string, admin bool) (AuthResult, error) {
	if user.Status != "active" {
		return AuthResult{}, apperr.Unauthorized("account disabled")
	}
	params := sqlc.UpdateUserParams{ID: user.ID}
	changed := false
	if name != "" && name != user.Name {
		params.Name = pgtype.Text{String: name, Valid: true}
		changed = true
	}
	if avatar != "" && avatar != user.AvatarUrl {
		params.AvatarUrl = pgtype.Text{String: avatar, Valid: true}
		changed = true
	}
	if admin && !user.PlatformAdmin {
		params.PlatformAdmin = pgtype.Bool{Bool: true, Valid: true}
		changed = true
	}
	if changed {
		updated, err := s.db.Q().UpdateUser(ctx, params)
		if err != nil {
			return AuthResult{}, err
		}
		user = updated
	}
	return s.issueSession(ctx, user)
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	_, err := s.db.Q().RevokeSession(ctx, sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

type MeResult struct {
	User        sqlc.User
	Memberships []sqlc.ListMembershipsByUserRow
}

func (s *Service) Me(ctx context.Context, userID string) (MeResult, error) {
	user, err := s.GetUser(ctx, userID)
	if err != nil {
		return MeResult{}, err
	}
	mems, err := s.db.Q().ListMembershipsByUser(ctx, userID)
	if err != nil {
		return MeResult{}, err
	}
	return MeResult{User: user, Memberships: mems}, nil
}

func (s *Service) issueSession(ctx context.Context, user sqlc.User) (AuthResult, error) {
	raw, err := newSessionToken()
	if err != nil {
		return AuthResult{}, err
	}
	expires := time.Now().UTC().Add(sessionTTL)
	sess, err := s.db.Q().CreateSession(ctx, sqlc.CreateSessionParams{
		ID:        ids.New(),
		UserID:    user.ID,
		TokenHash: HashAPIToken(raw),
		ExpiresAt: expires,
	})
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{
		User:      user,
		Token:     raw,
		ExpiresAt: expires,
		SessionID: sess.ID,
	}, nil
}

func newSessionToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "kyc_sess_" + hex.EncodeToString(b[:]), nil
}

func emailInList(email string, list []string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, e := range list {
		if strings.ToLower(strings.TrimSpace(e)) == email {
			return true
		}
	}
	return false
}

// --- OAuth state (CSRF) ---

type oauthStatePayload struct {
	Nonce string `json:"n"`
	Exp   int64  `json:"e"`
}

func SignOAuthState(secret string, ttl time.Duration) (string, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	payload := oauthStatePayload{
		Nonce: hex.EncodeToString(nonce[:]),
		Exp:   time.Now().Add(ttl).Unix(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(raw)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func VerifyOAuthState(secret, state string) error {
	parts := strings.Split(state, ".")
	if len(parts) != 2 {
		return apperr.Unauthorized("invalid oauth state")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return apperr.Unauthorized("invalid oauth state")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return apperr.Unauthorized("invalid oauth state")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(raw)
	if subtle.ConstantTimeCompare(sig, mac.Sum(nil)) != 1 {
		return apperr.Unauthorized("invalid oauth state")
	}
	var payload oauthStatePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return apperr.Unauthorized("invalid oauth state")
	}
	if time.Now().Unix() > payload.Exp {
		return apperr.Unauthorized("oauth state expired")
	}
	return nil
}
