package service

import (
	"context"
	"errors"
	"strings"

	"github.com/gsarmaonline/kyc/core/resources"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreateMembershipInput struct {
	UserID string
	Email  string
	RoleID string
	Status string // optional; default invited
}

func (s *Service) GetMembership(ctx context.Context, id string) (sqlc.Membership, error) {
	m, err := s.db.Q().GetMembership(ctx, id)
	return m, mapNotFound(err, "membership not found")
}

func (s *Service) GetMembershipDetail(ctx context.Context, id string) (sqlc.GetMembershipDetailRow, error) {
	m, err := s.db.Q().GetMembershipDetail(ctx, id)
	return m, mapNotFound(err, "membership not found")
}

func (s *Service) CreateMembership(ctx context.Context, orgID string, in CreateMembershipInput) (sqlc.Membership, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return sqlc.Membership{}, err
	}
	if strings.TrimSpace(in.RoleID) == "" {
		return sqlc.Membership{}, apperr.Validation("role_id is required")
	}
	role, err := s.db.Q().GetRole(ctx, in.RoleID)
	if err != nil {
		return sqlc.Membership{}, mapNotFound(err, "role not found")
	}
	if role.OrganisationID != orgID {
		return sqlc.Membership{}, apperr.Validation("role does not belong to organisation")
	}

	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "invited"
	}
	switch status {
	case "invited", "active":
	default:
		return sqlc.Membership{}, apperr.Validation("invalid membership status")
	}

	var userID string
	switch {
	case strings.TrimSpace(in.UserID) != "":
		user, err := s.GetUser(ctx, in.UserID)
		if err != nil {
			return sqlc.Membership{}, err
		}
		userID = user.ID
	case strings.TrimSpace(in.Email) != "":
		email := strings.ToLower(strings.TrimSpace(in.Email))
		user, err := s.db.Q().GetUserByEmail(ctx, email)
		if errors.Is(err, pgx.ErrNoRows) {
			user, err = s.db.Q().CreateUser(ctx, sqlc.CreateUserParams{
				ID:            ids.New(),
				Email:         email,
				Name:          email,
				Status:        "active",
				PlatformAdmin: false,
			})
			if err != nil {
				if store.IsUniqueViolation(err) {
					return sqlc.Membership{}, apperr.Conflict("email already exists")
				}
				return sqlc.Membership{}, err
			}
		} else if err != nil {
			return sqlc.Membership{}, err
		}
		userID = user.ID
	default:
		return sqlc.Membership{}, apperr.Validation("user_id or email is required")
	}

	m, err := s.db.Q().CreateMembership(ctx, sqlc.CreateMembershipParams{
		ID:             ids.New(),
		OrganisationID: orgID,
		UserID:         userID,
		RoleID:         in.RoleID,
		Status:         status,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			return sqlc.Membership{}, apperr.Conflict("user already a member of organisation")
		}
		return sqlc.Membership{}, err
	}
	s.EnqueueResourceLifecycle(ctx, orgID, resources.Membership, resources.LifecycleCreated, membershipEventPayload(m))
	return m, nil
}

func (s *Service) ListOrganisationMemberships(ctx context.Context, orgID string) ([]sqlc.ListMembershipsByOrganisationRow, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return nil, err
	}
	return s.db.Q().ListMembershipsByOrganisation(ctx, orgID)
}

type UpdateMembershipInput struct {
	RoleID *string
	Status *string
}

func (s *Service) UpdateMembership(ctx context.Context, id string, in UpdateMembershipInput) (sqlc.Membership, error) {
	m, err := s.db.Q().GetMembership(ctx, id)
	if err != nil {
		return sqlc.Membership{}, mapNotFound(err, "membership not found")
	}
	params := sqlc.UpdateMembershipParams{ID: id}
	if in.RoleID != nil {
		role, err := s.db.Q().GetRole(ctx, *in.RoleID)
		if err != nil {
			return sqlc.Membership{}, mapNotFound(err, "role not found")
		}
		if role.OrganisationID != m.OrganisationID {
			return sqlc.Membership{}, apperr.Validation("role does not belong to organisation")
		}
		params.RoleID = pgtype.Text{String: *in.RoleID, Valid: true}
	}
	if in.Status != nil {
		st := strings.TrimSpace(*in.Status)
		switch st {
		case "invited", "active", "revoked":
			params.Status = pgtype.Text{String: st, Valid: true}
		default:
			return sqlc.Membership{}, apperr.Validation("invalid status")
		}
	}
	out, err := s.db.Q().UpdateMembership(ctx, params)
	if err != nil {
		return sqlc.Membership{}, mapNotFound(err, "membership not found")
	}
	s.EnqueueResourceLifecycle(ctx, out.OrganisationID, resources.Membership, resources.LifecycleUpdated, membershipEventPayload(out))
	return out, nil
}

func membershipEventPayload(m sqlc.Membership) map[string]any {
	return map[string]any{
		"id":              m.ID,
		"organisation_id": m.OrganisationID,
		"user_id":         m.UserID,
		"role_id":         m.RoleID,
		"status":          m.Status,
	}
}

func (s *Service) AcceptMembership(ctx context.Context, id string) (sqlc.Membership, error) {
	m, err := s.db.Q().AcceptMembership(ctx, id)
	return m, mapNotFound(err, "membership not found or not invited")
}

// AcceptMembershipAsUser accepts an invite only if it belongs to the given user.
func (s *Service) AcceptMembershipAsUser(ctx context.Context, id, userID string) (sqlc.Membership, error) {
	m, err := s.db.Q().GetMembership(ctx, id)
	if err != nil {
		return sqlc.Membership{}, mapNotFound(err, "membership not found")
	}
	if m.UserID != userID {
		return sqlc.Membership{}, apperr.Forbidden("invite does not belong to this user")
	}
	if m.Status != "invited" {
		return sqlc.Membership{}, apperr.Validation("membership is not invited")
	}
	return s.AcceptMembership(ctx, id)
}

func (s *Service) RevokeMembership(ctx context.Context, id string) (sqlc.Membership, error) {
	m, err := s.db.Q().RevokeMembership(ctx, id)
	return m, mapNotFound(err, "membership not found")
}
