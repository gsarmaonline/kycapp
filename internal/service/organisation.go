package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/gsarmaonline/kyc/core/organisation"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreateOrganisationInput struct {
	Name         string
	Slug         string
	OwnerUserID  string // when set, creates owner membership for this user
	AttachTrial  bool
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
		ownerRole, _, _, err := seedSystemRoles(ctx, q, org.ID)
		if err != nil {
			return err
		}
		if err := ensureDefaultEmailTemplates(ctx, q, org.ID); err != nil {
			return err
		}
		if err := ensureDefaultAttributeDefinitions(ctx, q, org.ID); err != nil {
			return err
		}
		if in.OwnerUserID != "" {
			if _, err := q.GetUser(ctx, in.OwnerUserID); err != nil {
				return mapNotFound(err, "user not found")
			}
			if _, err := q.CreateMembership(ctx, sqlc.CreateMembershipParams{
				ID:             ids.New(),
				OrganisationID: org.ID,
				UserID:         in.OwnerUserID,
				RoleID:         ownerRole.ID,
				Status:         "active",
			}); err != nil {
				if store.IsUniqueViolation(err) {
					return apperr.Conflict("user already a member of organisation")
				}
				return err
			}
		}
		if in.AttachTrial {
			plan, err := q.GetPlanByKey(ctx, "trial")
			if err != nil {
				return err
			}
			_, err = q.CreateSubscription(ctx, sqlc.CreateSubscriptionParams{
				ID:               ids.New(),
				OrganisationID:   org.ID,
				PlanID:           plan.ID,
				Status:           "trialing",
				CurrentPeriodEnd: pgtype.Timestamptz{},
			})
			return err
		}
		return nil
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

func (s *Service) ListOrganisationsForUser(ctx context.Context, userID, status, q string, limit int32, cursor string) ([]sqlc.Organisation, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	// Merchants only see active orgs by default (archived stay in DB for ops).
	if status == "" {
		status = "active"
	}
	return s.db.Q().ListOrganisationsForUser(ctx, sqlc.ListOrganisationsForUserParams{
		UserID: userID,
		Status: textArg(status),
		Q:      textArg(q),
		Cursor: textArg(cursor),
		Limit:  limit,
	})
}

type UpdateOrganisationInput struct {
	Name                   *string
	Status                 *string
	PrimaryColor           *string
	AccentColor            *string
	EmailFooter            *string
	EmailFont              *string
	AppUserAuthority       *string
	AppUserIngestUpsertKey *string
	AppUserAttributesMode  *string
}

func (s *Service) UpdateOrganisation(ctx context.Context, id string, in UpdateOrganisationInput) (sqlc.Organisation, error) {
	return s.UpdateOrganisationBranding(ctx, id, in)
}

func (s *Service) ArchiveOrganisation(ctx context.Context, id string) (sqlc.Organisation, error) {
	org, err := s.db.Q().ArchiveOrganisation(ctx, id)
	return org, mapNotFound(err, "organisation not found")
}

// DeleteOrganisation permanently removes the organisation and cascaded tenant data.
func (s *Service) DeleteOrganisation(ctx context.Context, id string) error {
	_, err := s.db.Q().GetOrganisation(ctx, id)
	if err != nil {
		return mapNotFound(err, "organisation not found")
	}
	if s.uploadDir != "" {
		_ = os.RemoveAll(filepath.Join(s.uploadDir, id))
	}
	if err := s.db.Q().DeleteOrganisation(ctx, id); err != nil {
		return err
	}
	return nil
}
