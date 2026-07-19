package service

import (
	"context"
	"errors"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/authn"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
)

// RequirePrincipal returns the authenticated principal or 401.
func RequirePrincipal(ctx context.Context) (authn.Principal, error) {
	p, ok := authn.FromContext(ctx)
	if !ok || p.ActorLabel() == "anonymous" {
		return authn.Principal{}, apperr.Unauthorized("authentication required")
	}
	return p, nil
}

// RequireUser requires a logged-in user (not a service API key).
func RequireUser(ctx context.Context) (authn.Principal, error) {
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return authn.Principal{}, err
	}
	if p.Kind != authn.KindUser || p.UserID == "" {
		return authn.Principal{}, apperr.Forbidden("user session required")
	}
	return p, nil
}

// RequirePlatform requires a platform admin user or service principal.
func RequirePlatform(ctx context.Context) (authn.Principal, error) {
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return authn.Principal{}, err
	}
	if !p.IsPlatform() {
		return authn.Principal{}, apperr.Forbidden("platform admin required")
	}
	return p, nil
}

// RequireOrgMember requires active membership in org, platform privilege, or an org-scoped API key.
func (s *Service) RequireOrgMember(ctx context.Context, orgID string) (authn.Principal, error) {
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return authn.Principal{}, err
	}
	if p.IsPlatform() {
		return p, nil
	}
	if p.Kind == authn.KindService && p.OrganisationID == orgID {
		return p, nil
	}
	if p.Kind != authn.KindUser || p.UserID == "" {
		return authn.Principal{}, apperr.Forbidden("not a member of this organisation")
	}
	_, err = s.db.Q().GetActiveMembershipByOrgAndUser(ctx, sqlc.GetActiveMembershipByOrgAndUserParams{
		OrganisationID: orgID,
		UserID:         p.UserID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return authn.Principal{}, apperr.Forbidden("not a member of this organisation")
	}
	if err != nil {
		return authn.Principal{}, err
	}
	return p, nil
}

// RequireOrgPermission requires org membership plus an RBAC permission (platform and org API keys bypass).
func (s *Service) RequireOrgPermission(ctx context.Context, orgID, permissionKey string) (authn.Principal, error) {
	p, err := s.RequireOrgMember(ctx, orgID)
	if err != nil {
		return authn.Principal{}, err
	}
	if p.IsPlatform() {
		return p, nil
	}
	if p.Kind == authn.KindService && p.OrganisationID == orgID {
		return p, nil
	}
	allowed, err := s.CheckAuthz(ctx, AuthzCheckInput{
		OrganisationID: orgID,
		UserID:         p.UserID,
		Permission:     permissionKey,
	})
	if err != nil {
		return authn.Principal{}, err
	}
	if !allowed {
		return authn.Principal{}, apperr.Forbidden("missing permission " + permissionKey)
	}
	return p, nil
}
