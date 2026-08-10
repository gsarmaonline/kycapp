package service

import (
	"context"
	"strings"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreateUserInput struct {
	Email string
	Name  string
}

func (s *Service) CreateUser(ctx context.Context, in CreateUserInput) (sqlc.User, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	name := strings.TrimSpace(in.Name)
	if email == "" || !strings.Contains(email, "@") {
		return sqlc.User{}, apperr.Validation("email is required")
	}
	if name == "" {
		return sqlc.User{}, apperr.Validation("name is required")
	}
	user, err := s.db.Q().CreateUser(ctx, sqlc.CreateUserParams{
		ID:     ids.New(),
		Email:  email,
		Name:   name,
		Status: "active",
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			return sqlc.User{}, apperr.Conflict("email already exists")
		}
		return sqlc.User{}, err
	}
	return user, nil
}

func (s *Service) GetUser(ctx context.Context, id string) (sqlc.User, error) {
	user, err := s.db.Q().GetUser(ctx, id)
	return user, mapNotFound(err, "user not found")
}

func (s *Service) ListUsers(ctx context.Context, q string, limit int32, cursor string) ([]sqlc.User, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.db.Q().ListUsers(ctx, sqlc.ListUsersParams{
		Q:      textArg(q),
		Cursor: textArg(cursor),
		Limit:  limit,
	})
}

type UpdateUserInput struct {
	Name   *string
	Status *string
}

func (s *Service) UpdateUser(ctx context.Context, id string, in UpdateUserInput) (sqlc.User, error) {
	params := sqlc.UpdateUserParams{ID: id}
	if in.Name != nil {
		params.Name = pgtype.Text{String: strings.TrimSpace(*in.Name), Valid: true}
	}
	if in.Status != nil {
		st := strings.TrimSpace(*in.Status)
		switch st {
		case "active", "disabled":
			params.Status = pgtype.Text{String: st, Valid: true}
		default:
			return sqlc.User{}, apperr.Validation("invalid status")
		}
	}
	user, err := s.db.Q().UpdateUser(ctx, params)
	return user, mapNotFound(err, "user not found")
}

func (s *Service) ListUserMemberships(ctx context.Context, userID string) ([]sqlc.ListMembershipsByUserRow, error) {
	if _, err := s.GetUser(ctx, userID); err != nil {
		return nil, err
	}
	return s.db.Q().ListMembershipsByUser(ctx, userID)
}
