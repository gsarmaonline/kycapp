package service

import (
	"context"
	"encoding/json"
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
	Key          string
	Name         string
	Description  string
	Subject      string
	BodyText     string
	BodyHTML     string
	BodySections []emailtemplates.BodySection
	FromName     string
	FromAddress  string
}

type UpdateEmailTemplateInput struct {
	Name         *string
	Description  *string
	Subject      *string
	BodyText     *string
	BodyHTML     *string
	BodySections *[]emailtemplates.BodySection
	FromName     *string
	FromAddress  *string
	Status       *string
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
		sections := []emailtemplates.BodySection{
			emailtemplates.SectionFromBodyHTML("sec_"+spec.Key, spec.BodyHTML),
		}
		rawSections, err := emailtemplates.MarshalBodySections(sections)
		if err != nil {
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
			BodySections:   rawSections,
			FromName:       "",
			FromAddress:    "",
			Status:         "active",
			IsSystem:       true,
		})
		if err != nil && !store.IsUniqueViolation(err) {
			return err
		}
	}
	return nil
}

func prepareBodySections(inSections []emailtemplates.BodySection, bodyHTML string) ([]emailtemplates.BodySection, string, json.RawMessage, error) {
	sections := inSections
	if len(sections) == 0 {
		html := strings.TrimSpace(bodyHTML)
		if html == "" {
			return nil, "", nil, apperr.Validation("body_sections or body_html is required")
		}
		sections = []emailtemplates.BodySection{
			emailtemplates.SectionFromBodyHTML("sec_"+ids.New(), bodyHTML),
		}
	}
	normalized, err := emailtemplates.NormalizeBodySections(sections)
	if err != nil {
		return nil, "", nil, apperr.Validation(err.Error())
	}
	raw, err := emailtemplates.MarshalBodySections(normalized)
	if err != nil {
		return nil, "", nil, err
	}
	syncedHTML := emailtemplates.SyncBodyHTMLFromSections(normalized)
	if syncedHTML == "" {
		syncedHTML = bodyHTML
	}
	return normalized, syncedHTML, raw, nil
}

func (s *Service) CreateEmailTemplate(ctx context.Context, orgID string, in CreateEmailTemplateInput) (sqlc.EmailTemplate, error) {
	key := emailtemplates.NormalizeKey(in.Key)
	name := strings.TrimSpace(in.Name)
	subject := strings.TrimSpace(in.Subject)
	description := strings.TrimSpace(in.Description)
	if !emailtemplates.ValidKey(key) {
		return sqlc.EmailTemplate{}, apperr.Validation("key must be lowercase snake_case (a-z, 0-9, _)")
	}
	if name == "" {
		return sqlc.EmailTemplate{}, apperr.Validation("name is required")
	}
	if subject == "" {
		return sqlc.EmailTemplate{}, apperr.Validation("subject is required")
	}
	_, syncedHTML, rawSections, err := prepareBodySections(in.BodySections, in.BodyHTML)
	if err != nil {
		return sqlc.EmailTemplate{}, err
	}
	bodyText := in.BodyText
	if strings.TrimSpace(bodyText) == "" && strings.TrimSpace(syncedHTML) == "" {
		return sqlc.EmailTemplate{}, apperr.Validation("body_text or body content is required")
	}
	fromName, fromAddr := emailtemplates.NormalizeFromFields(in.FromName, in.FromAddress)
	if err := s.EnsureDefaultEmailTemplates(ctx, orgID); err != nil {
		return sqlc.EmailTemplate{}, err
	}
	row, err := s.db.Q().CreateEmailTemplate(ctx, sqlc.CreateEmailTemplateParams{
		ID:             ids.New(),
		OrganisationID: orgID,
		Key:            key,
		Name:           name,
		Description:    description,
		Subject:        subject,
		BodyText:       bodyText,
		BodyHtml:       syncedHTML,
		BodySections:   rawSections,
		FromName:       fromName,
		FromAddress:    fromAddr,
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
	if in.BodySections != nil {
		_, syncedHTML, rawSections, err := prepareBodySections(*in.BodySections, "")
		if err != nil {
			return sqlc.EmailTemplate{}, err
		}
		params.BodySections = []byte(rawSections)
		params.BodyHtml = pgtype.Text{String: syncedHTML, Valid: true}
	} else if in.BodyHTML != nil {
		_, syncedHTML, rawSections, err := prepareBodySections(nil, *in.BodyHTML)
		if err != nil {
			return sqlc.EmailTemplate{}, err
		}
		params.BodyHtml = pgtype.Text{String: syncedHTML, Valid: true}
		params.BodySections = []byte(rawSections)
	}
	if in.FromName != nil {
		name, _ := emailtemplates.NormalizeFromFields(*in.FromName, "")
		params.FromName = pgtype.Text{String: name, Valid: true}
	}
	if in.FromAddress != nil {
		_, addr := emailtemplates.NormalizeFromFields("", *in.FromAddress)
		params.FromAddress = pgtype.Text{String: addr, Valid: true}
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
