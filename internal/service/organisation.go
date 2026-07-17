package service

import (
	"context"
	"strings"

	"github.com/gsarmaonline/kyc/core/organisation"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreateOrganisationInput struct {
	Name string
	Slug string
}

func (s *Service) CreateOrganisation(ctx context.Context, in CreateOrganisationInput) (sqlc.Organisation, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return sqlc.Organisation{}, apperr.Validation("name is required")
	}
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		slug = organisation.Slugify(name)
	}

	var org sqlc.Organisation
	err := s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		var err error
		org, err = q.CreateOrganisation(ctx, sqlc.CreateOrganisationParams{
			ID:     ids.New(),
			Name:   name,
			Slug:   slug,
			Status: "active",
		})
		if err != nil {
			if store.IsUniqueViolation(err) {
				return apperr.Conflict("organisation slug already exists")
			}
			return err
		}
		_, _, _, err = seedSystemRoles(ctx, q, org.ID)
		return err
	})
	return org, err
}

func (s *Service) GetOrganisation(ctx context.Context, id string) (sqlc.Organisation, error) {
	org, err := s.db.Q().GetOrganisation(ctx, id)
	return org, mapNotFound(err, "organisation not found")
}

func (s *Service) ListOrganisations(ctx context.Context, status, q string, limit int32, cursor string) ([]sqlc.Organisation, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.db.Q().ListOrganisations(ctx, sqlc.ListOrganisationsParams{
		Status: textArg(status),
		Q:      textArg(q),
		Cursor: textArg(cursor),
		Limit:  limit,
	})
}

type UpdateOrganisationInput struct {
	Name   *string
	Status *string
}

func (s *Service) UpdateOrganisation(ctx context.Context, id string, in UpdateOrganisationInput) (sqlc.Organisation, error) {
	params := sqlc.UpdateOrganisationParams{ID: id}
	if in.Name != nil {
		params.Name = pgtype.Text{String: strings.TrimSpace(*in.Name), Valid: true}
	}
	if in.Status != nil {
		st := strings.TrimSpace(*in.Status)
		switch st {
		case "active", "suspended", "archived":
			params.Status = pgtype.Text{String: st, Valid: true}
		default:
			return sqlc.Organisation{}, apperr.Validation("invalid status")
		}
	}
	org, err := s.db.Q().UpdateOrganisation(ctx, params)
	return org, mapNotFound(err, "organisation not found")
}

func (s *Service) ArchiveOrganisation(ctx context.Context, id string) (sqlc.Organisation, error) {
	org, err := s.db.Q().ArchiveOrganisation(ctx, id)
	return org, mapNotFound(err, "organisation not found")
}

func (s *Service) ListRoles(ctx context.Context, orgID string) ([]sqlc.Role, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return nil, err
	}
	return s.db.Q().ListRolesByOrganisation(ctx, orgID)
}
