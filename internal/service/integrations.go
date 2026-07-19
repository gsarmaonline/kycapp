package service

import (
	"context"
	"strings"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

type IntegrationView struct {
	Provider       string
	Status         string
	SecretHint     string
	PublicKeyHint  string
	HasSecret      bool
	HasPublicKey   bool
}

type UpsertStripeIntegrationInput struct {
	SecretKey      string
	PublishableKey string
}

func maskSecret(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "••••"
	}
	return s[:4] + "…" + s[len(s)-4:]
}

func integrationView(row sqlc.OrganisationIntegration) IntegrationView {
	return IntegrationView{
		Provider:      row.Provider,
		Status:        row.Status,
		SecretHint:    maskSecret(row.SecretKey),
		PublicKeyHint: maskSecret(row.PublicKey),
		HasSecret:     strings.TrimSpace(row.SecretKey) != "",
		HasPublicKey:  strings.TrimSpace(row.PublicKey) != "",
	}
}

func (s *Service) ListOrganisationIntegrations(ctx context.Context, orgID string) ([]IntegrationView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return nil, err
	}
	rows, err := s.db.Q().ListOrganisationIntegrations(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]IntegrationView, 0, len(rows))
	for _, row := range rows {
		out = append(out, integrationView(row))
	}
	return out, nil
}

func (s *Service) UpsertStripeIntegration(ctx context.Context, orgID string, in UpsertStripeIntegrationInput) (IntegrationView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return IntegrationView{}, err
	}
	secret := strings.TrimSpace(in.SecretKey)
	pub := strings.TrimSpace(in.PublishableKey)
	if secret == "" && pub == "" {
		return IntegrationView{}, apperr.Validation("secret_key or publishable_key is required")
	}
	row, err := s.db.Q().UpsertOrganisationIntegration(ctx, sqlc.UpsertOrganisationIntegrationParams{
		OrganisationID: orgID,
		Provider:       "stripe",
		Status:         "connected",
		SecretKey:      secret,
		PublicKey:      pub,
	})
	if err != nil {
		return IntegrationView{}, err
	}
	return integrationView(row), nil
}

func (s *Service) DeleteOrganisationIntegration(ctx context.Context, orgID, provider string) error {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return err
	}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return apperr.Validation("provider is required")
	}
	return s.db.Q().DeleteOrganisationIntegration(ctx, sqlc.DeleteOrganisationIntegrationParams{
		OrganisationID: orgID,
		Provider:       provider,
	})
}
