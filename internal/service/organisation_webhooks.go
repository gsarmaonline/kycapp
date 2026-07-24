package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/gsarmaonline/kyc/core/automations"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

type OrganisationWebhookView struct {
	ID             string
	OrganisationID string
	Name           string
	URL            string
	SecretHint     string
	HasSecret      bool
	Status         string
	BodyTemplate   string
}

type CreateOrganisationWebhookInput struct {
	Name         string
	URL          string
	Secret       string
	BodyTemplate string
}

type UpdateOrganisationWebhookInput struct {
	Name         *string
	URL          *string
	Secret       *string // empty/nil keeps existing
	Status       *string
	BodyTemplate *string
}

func organisationWebhookView(row sqlc.OrganisationWebhook) OrganisationWebhookView {
	return OrganisationWebhookView{
		ID:             row.ID,
		OrganisationID: row.OrganisationID,
		Name:           row.Name,
		URL:            row.Url,
		SecretHint:     maskSecret(row.Secret),
		HasSecret:      strings.TrimSpace(row.Secret) != "",
		Status:         row.Status,
		BodyTemplate:   row.BodyTemplate,
	}
}

func (s *Service) ListOrganisationWebhooks(ctx context.Context, orgID string) ([]OrganisationWebhookView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return nil, err
	}
	rows, err := s.db.Q().ListOrganisationWebhooks(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]OrganisationWebhookView, 0, len(rows))
	for _, row := range rows {
		out = append(out, organisationWebhookView(row))
	}
	return out, nil
}

func (s *Service) GetOrganisationWebhook(ctx context.Context, orgID, id string) (OrganisationWebhookView, error) {
	row, err := s.db.Q().GetOrganisationWebhookForOrg(ctx, sqlc.GetOrganisationWebhookForOrgParams{
		ID: id, OrganisationID: orgID,
	})
	if err != nil {
		return OrganisationWebhookView{}, mapNotFound(err, "webhook not found")
	}
	return organisationWebhookView(row), nil
}

func validateWebhookURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return apperr.Validation("url is invalid")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && scheme != "http" {
		return apperr.Validation("url must be http or https")
	}
	return nil
}

func (s *Service) CreateOrganisationWebhook(ctx context.Context, orgID string, in CreateOrganisationWebhookInput) (OrganisationWebhookView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return OrganisationWebhookView{}, err
	}
	name := strings.TrimSpace(in.Name)
	rawURL := strings.TrimSpace(in.URL)
	if name == "" {
		return OrganisationWebhookView{}, apperr.Validation("name is required")
	}
	if err := validateWebhookURL(rawURL); err != nil {
		return OrganisationWebhookView{}, err
	}
	body := strings.TrimSpace(in.BodyTemplate)
	if err := automations.ValidateJSONTemplate(body); err != nil {
		return OrganisationWebhookView{}, apperr.Validation(err.Error())
	}
	row, err := s.db.Q().CreateOrganisationWebhook(ctx, sqlc.CreateOrganisationWebhookParams{
		ID:             ids.New(),
		OrganisationID: orgID,
		Name:           name,
		Url:            rawURL,
		Secret:         in.Secret,
		Status:         "connected",
		BodyTemplate:   body,
	})
	if err != nil {
		return OrganisationWebhookView{}, err
	}
	return organisationWebhookView(row), nil
}

func (s *Service) UpdateOrganisationWebhook(ctx context.Context, orgID, id string, in UpdateOrganisationWebhookInput) (OrganisationWebhookView, error) {
	existing, err := s.db.Q().GetOrganisationWebhookForOrg(ctx, sqlc.GetOrganisationWebhookForOrgParams{
		ID: id, OrganisationID: orgID,
	})
	if err != nil {
		return OrganisationWebhookView{}, mapNotFound(err, "webhook not found")
	}
	name := existing.Name
	rawURL := existing.Url
	status := existing.Status
	body := existing.BodyTemplate
	secret := ""
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
		if name == "" {
			return OrganisationWebhookView{}, apperr.Validation("name is required")
		}
	}
	if in.URL != nil {
		rawURL = strings.TrimSpace(*in.URL)
		if err := validateWebhookURL(rawURL); err != nil {
			return OrganisationWebhookView{}, err
		}
	}
	if in.Secret != nil {
		secret = *in.Secret
	}
	if in.Status != nil {
		status = strings.TrimSpace(*in.Status)
		if status != "connected" && status != "disconnected" {
			return OrganisationWebhookView{}, apperr.Validation("status must be connected or disconnected")
		}
	}
	if in.BodyTemplate != nil {
		body = strings.TrimSpace(*in.BodyTemplate)
		if err := automations.ValidateJSONTemplate(body); err != nil {
			return OrganisationWebhookView{}, apperr.Validation(err.Error())
		}
	}
	row, err := s.db.Q().UpdateOrganisationWebhook(ctx, sqlc.UpdateOrganisationWebhookParams{
		Name: name, Url: rawURL, Secret: secret, Status: status, BodyTemplate: body,
		ID: id, OrganisationID: orgID,
	})
	if err != nil {
		return OrganisationWebhookView{}, err
	}
	return organisationWebhookView(row), nil
}

func (s *Service) DeleteOrganisationWebhook(ctx context.Context, orgID, id string) error {
	if _, err := s.GetOrganisationWebhook(ctx, orgID, id); err != nil {
		return err
	}
	return s.db.Q().DeleteOrganisationWebhook(ctx, sqlc.DeleteOrganisationWebhookParams{
		ID: id, OrganisationID: orgID,
	})
}

func (s *Service) organisationWebhookRow(ctx context.Context, orgID, id string) (sqlc.OrganisationWebhook, error) {
	row, err := s.db.Q().GetOrganisationWebhookForOrg(ctx, sqlc.GetOrganisationWebhookForOrgParams{
		ID: id, OrganisationID: orgID,
	})
	if err != nil {
		return sqlc.OrganisationWebhook{}, mapNotFound(err, "webhook not found")
	}
	if row.Status != "connected" {
		return sqlc.OrganisationWebhook{}, fmt.Errorf("webhook is disconnected")
	}
	return row, nil
}
