package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/core/resources"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

// InboundWebhookView is an org inbound endpoint (fires webhook.received).
type InboundWebhookView struct {
	ID             string
	OrganisationID string
	Name           string
	URL            string
	SecretHint     string
	HasSecret      bool
	Status         string
	// Secret is only set when newly generated/rotated.
	Secret string `json:"-"`
}

type CreateInboundWebhookInput struct {
	Name   string
	Secret string // empty → auto-generate
	Status string // default connected when secret present
}

type UpdateInboundWebhookInput struct {
	Name   *string
	Secret *string // nil keep; "" clear; non-empty set
	Status *string
	Rotate bool
}

func (s *Service) inboundWebhookURL(hookID string) string {
	return s.publicBaseURL + "/v1/hooks/inbound/" + hookID
}

func inboundWebhookView(s *Service, row sqlc.OrganisationInboundWebhook, revealed string) InboundWebhookView {
	return InboundWebhookView{
		ID:             row.ID,
		OrganisationID: row.OrganisationID,
		Name:           row.Name,
		URL:            s.inboundWebhookURL(row.ID),
		SecretHint:     maskSecret(row.Secret),
		HasSecret:      strings.TrimSpace(row.Secret) != "",
		Status:         row.Status,
		Secret:         revealed,
	}
}

func (s *Service) ListInboundWebhooks(ctx context.Context, orgID string) ([]InboundWebhookView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return nil, err
	}
	rows, err := s.db.Q().ListOrganisationInboundWebhooks(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]InboundWebhookView, 0, len(rows))
	for _, row := range rows {
		out = append(out, inboundWebhookView(s, row, ""))
	}
	return out, nil
}

func (s *Service) GetInboundWebhook(ctx context.Context, orgID, id string) (InboundWebhookView, error) {
	row, err := s.db.Q().GetOrganisationInboundWebhookForOrg(ctx, sqlc.GetOrganisationInboundWebhookForOrgParams{
		ID: id, OrganisationID: orgID,
	})
	if err != nil {
		return InboundWebhookView{}, mapNotFound(err, "inbound webhook not found")
	}
	return inboundWebhookView(s, row, ""), nil
}

func (s *Service) CreateInboundWebhook(ctx context.Context, orgID string, in CreateInboundWebhookInput) (InboundWebhookView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return InboundWebhookView{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return InboundWebhookView{}, apperr.Validation("name is required")
	}
	secret := strings.TrimSpace(in.Secret)
	var revealed string
	if secret == "" {
		var err error
		revealed, err = generateInboundSecret()
		if err != nil {
			return InboundWebhookView{}, err
		}
		secret = revealed
	} else {
		revealed = secret
	}
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "connected"
	}
	if status != "connected" && status != "disconnected" {
		return InboundWebhookView{}, apperr.Validation("status must be connected or disconnected")
	}
	row, err := s.db.Q().CreateOrganisationInboundWebhook(ctx, sqlc.CreateOrganisationInboundWebhookParams{
		ID:             ids.New(),
		OrganisationID: orgID,
		Name:           name,
		Secret:         secret,
		Status:         status,
	})
	if err != nil {
		return InboundWebhookView{}, err
	}
	return inboundWebhookView(s, row, revealed), nil
}

func (s *Service) UpdateInboundWebhook(ctx context.Context, orgID, id string, in UpdateInboundWebhookInput) (InboundWebhookView, error) {
	existing, err := s.db.Q().GetOrganisationInboundWebhookForOrg(ctx, sqlc.GetOrganisationInboundWebhookForOrgParams{
		ID: id, OrganisationID: orgID,
	})
	if err != nil {
		return InboundWebhookView{}, mapNotFound(err, "inbound webhook not found")
	}
	name := existing.Name
	secret := existing.Secret
	status := existing.Status
	var revealed string

	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
		if name == "" {
			return InboundWebhookView{}, apperr.Validation("name is required")
		}
	}
	if in.Rotate {
		revealed, err = generateInboundSecret()
		if err != nil {
			return InboundWebhookView{}, err
		}
		secret = revealed
		if status == "disconnected" {
			status = "connected"
		}
	} else if in.Secret != nil {
		secret = strings.TrimSpace(*in.Secret)
		if secret == "" {
			status = "disconnected"
		} else if status != "connected" {
			status = "connected"
		}
	}
	if in.Status != nil {
		st := strings.TrimSpace(*in.Status)
		if st != "connected" && st != "disconnected" {
			return InboundWebhookView{}, apperr.Validation("status must be connected or disconnected")
		}
		status = st
	}
	if status == "connected" && strings.TrimSpace(secret) == "" {
		return InboundWebhookView{}, apperr.Validation("secret is required when status is connected")
	}

	row, err := s.db.Q().UpdateOrganisationInboundWebhook(ctx, sqlc.UpdateOrganisationInboundWebhookParams{
		ID:             id,
		OrganisationID: orgID,
		Name:           name,
		Secret:         secret,
		Status:         status,
	})
	if err != nil {
		return InboundWebhookView{}, err
	}
	return inboundWebhookView(s, row, revealed), nil
}

func (s *Service) DeleteInboundWebhook(ctx context.Context, orgID, id string) error {
	if _, err := s.GetInboundWebhook(ctx, orgID, id); err != nil {
		return err
	}
	return s.db.Q().DeleteOrganisationInboundWebhook(ctx, sqlc.DeleteOrganisationInboundWebhookParams{
		ID: id, OrganisationID: orgID,
	})
}

// HandleInboundWebhook verifies secret for a specific inbound endpoint and enqueues webhook.received.
func (s *Service) HandleInboundWebhook(ctx context.Context, hookID string, secretHeader string, rawBody []byte, contentType string) error {
	hookID = strings.TrimSpace(hookID)
	if hookID == "" {
		return apperr.NotFound("inbound webhook not found")
	}
	row, err := s.db.Q().GetOrganisationInboundWebhook(ctx, hookID)
	if err != nil {
		return mapNotFound(err, "inbound webhook not found")
	}
	if row.Status != "connected" || strings.TrimSpace(row.Secret) == "" {
		return apperr.Unauthorized("inbound webhook is disconnected")
	}
	got := strings.TrimSpace(secretHeader)
	if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(row.Secret)) != 1 {
		return apperr.Unauthorized("invalid webhook secret")
	}

	var body any
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if len(rawBody) == 0 {
		body = map[string]any{}
	} else if strings.Contains(ct, "json") || (len(rawBody) > 0 && (rawBody[0] == '{' || rawBody[0] == '[')) {
		if err := json.Unmarshal(rawBody, &body); err != nil {
			return apperr.Validation("body must be valid JSON")
		}
	} else {
		body = string(rawBody)
	}

	orgID := row.OrganisationID
	trigger := resources.LifecycleTrigger(resources.Webhook, resources.WebhookReceived)
	payload := map[string]any{
		"id":                  orgID,
		"organisation_id":     orgID,
		"trigger":             trigger,
		"inbound_webhook_id":  row.ID,
		"inbound_webhook_name": row.Name,
		"body":                body,
		"content_type":        contentType,
		"received_at":         time.Now().UTC().Format(time.RFC3339),
	}
	s.EnqueueAutomationEvent(ctx, orgID, trigger, payload)
	return nil
}

func generateInboundSecret() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return "whsec_" + hex.EncodeToString(b), nil
}
