package service

import (
	"context"
	"strings"

	"github.com/gsarmaonline/kyc/core/emailtemplates"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreateEmailTemplateInput struct {
	Key         string
	Name        string
	Description string
	Subject     string
	BodyText    string
	BodyHTML    string
}

type UpdateEmailTemplateInput struct {
	Name        *string
	Description *string
	Subject     *string
	BodyText    *string
	BodyHTML    *string
	Status      *string
}

// EnsureDefaultEmailTemplates seeds system templates for an org (idempotent).
func (s *Service) EnsureDefaultEmailTemplates(ctx context.Context, orgID string) error {
	return ensureDefaultEmailTemplates(ctx, s.db.Q(), orgID)
}

func ensureDefaultEmailTemplates(ctx context.Context, q *sqlc.Queries, orgID string) error {
	for _, spec := range emailtemplates.Defaults() {
		_, err := q.GetEmailTemplateByOrgKey(ctx, sqlc.GetEmailTemplateByOrgKeyParams{
			OrganisationID: orgID,
			Key:            spec.Key,
		})
		if err == nil {
			continue
		}
		if err != pgx.ErrNoRows {
			return err
		}
		_, err = q.CreateEmailTemplate(ctx, sqlc.CreateEmailTemplateParams{
			ID:             ids.New(),
			OrganisationID: orgID,
			Key:            spec.Key,
			Name:           spec.Name,
			Description:    spec.Description,
			Subject:        spec.Subject,
			BodyText:       spec.BodyText,
			BodyHtml:       spec.BodyHTML,
			Status:         "active",
			IsSystem:       true,
		})
		if err != nil && !store.IsUniqueViolation(err) {
			return err
		}
	}
	return nil
}

func (s *Service) CreateEmailTemplate(ctx context.Context, orgID string, in CreateEmailTemplateInput) (sqlc.EmailTemplate, error) {
	fields, err := emailtemplates.ValidateCreate(emailtemplates.CreateFields{
		Key: in.Key, Name: in.Name, Description: in.Description,
		Subject: in.Subject, BodyText: in.BodyText, BodyHTML: in.BodyHTML,
	})
	if err != nil {
		return sqlc.EmailTemplate{}, apperr.Validation(err.Error())
	}
	if err := s.EnsureDefaultEmailTemplates(ctx, orgID); err != nil {
		return sqlc.EmailTemplate{}, err
	}
	row, err := s.db.Q().CreateEmailTemplate(ctx, sqlc.CreateEmailTemplateParams{
		ID:             ids.New(),
		OrganisationID: orgID,
		Key:            fields.Key,
		Name:           fields.Name,
		Description:    fields.Description,
		Subject:        fields.Subject,
		BodyText:       fields.BodyText,
		BodyHtml:       fields.BodyHTML,
		Status:         "active",
		IsSystem:       false,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			return sqlc.EmailTemplate{}, apperr.Conflict("email template key already exists in organisation")
		}
		return sqlc.EmailTemplate{}, err
	}
	return row, nil
}

func (s *Service) GetEmailTemplate(ctx context.Context, id string) (sqlc.EmailTemplate, error) {
	row, err := s.db.Q().GetEmailTemplate(ctx, id)
	return row, mapNotFound(err, "email template not found")
}

func (s *Service) ListEmailTemplates(ctx context.Context, orgID, status string) ([]sqlc.EmailTemplate, error) {
	if err := s.EnsureDefaultEmailTemplates(ctx, orgID); err != nil {
		return nil, err
	}
	return s.db.Q().ListEmailTemplates(ctx, sqlc.ListEmailTemplatesParams{
		OrganisationID: orgID,
		Status:         textArg(status),
	})
}

func (s *Service) UpdateEmailTemplate(ctx context.Context, id string, in UpdateEmailTemplateInput) (sqlc.EmailTemplate, error) {
	existing, err := s.GetEmailTemplate(ctx, id)
	if err != nil {
		return sqlc.EmailTemplate{}, err
	}
	params := sqlc.UpdateEmailTemplateParams{ID: existing.ID}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return sqlc.EmailTemplate{}, apperr.Validation("name cannot be empty")
		}
		params.Name = pgtype.Text{String: name, Valid: true}
	}
	if in.Description != nil {
		params.Description = pgtype.Text{String: strings.TrimSpace(*in.Description), Valid: true}
	}
	if in.Subject != nil {
		subject := strings.TrimSpace(*in.Subject)
		if subject == "" {
			return sqlc.EmailTemplate{}, apperr.Validation("subject cannot be empty")
		}
		params.Subject = pgtype.Text{String: subject, Valid: true}
	}
	if in.BodyText != nil {
		params.BodyText = pgtype.Text{String: *in.BodyText, Valid: true}
	}
	if in.BodyHTML != nil {
		params.BodyHtml = pgtype.Text{String: *in.BodyHTML, Valid: true}
	}
	if in.Status != nil {
		if err := emailtemplates.ValidateStatus(*in.Status); err != nil {
			return sqlc.EmailTemplate{}, apperr.Validation(err.Error())
		}
		params.Status = pgtype.Text{String: strings.TrimSpace(*in.Status), Valid: true}
	}
	row, err := s.db.Q().UpdateEmailTemplate(ctx, params)
	return row, err
}

func (s *Service) DeleteEmailTemplate(ctx context.Context, id string) (sqlc.EmailTemplate, error) {
	existing, err := s.GetEmailTemplate(ctx, id)
	if err != nil {
		return sqlc.EmailTemplate{}, err
	}
	if existing.IsSystem {
		return sqlc.EmailTemplate{}, apperr.Validation("system email templates cannot be deleted")
	}
	row, err := s.db.Q().ArchiveEmailTemplate(ctx, id)
	return row, mapNotFound(err, "email template not found")
}
