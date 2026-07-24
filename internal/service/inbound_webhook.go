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
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
)

// InboundHookView is the admin-facing inbound webhook config.
type InboundHookView struct {
	OrganisationID string
	URL            string
	SecretHint     string
	HasSecret      bool
	Status         string
	// Secret is only set when newly generated/rotated.
	Secret string `json:"-"`
}

type UpsertInboundHookInput struct {
	Secret *string // nil = keep; "" = clear; non-empty = set
	Status *string
	Rotate bool // generate a new secret
}

func (s *Service) inboundHookURL(orgID string) string {
	return s.publicBaseURL + "/v1/hooks/inbound/" + orgID
}

func (s *Service) GetInboundHook(ctx context.Context, orgID string) (InboundHookView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return InboundHookView{}, err
	}
	row, err := s.db.Q().GetOrganisationInboundHook(ctx, orgID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return InboundHookView{
				OrganisationID: orgID,
				URL:            s.inboundHookURL(orgID),
				Status:         "disconnected",
			}, nil
		}
		return InboundHookView{}, err
	}
	return InboundHookView{
		OrganisationID: row.OrganisationID,
		URL:            s.inboundHookURL(orgID),
		SecretHint:     maskSecret(row.Secret),
		HasSecret:      strings.TrimSpace(row.Secret) != "",
		Status:         row.Status,
	}, nil
}

func (s *Service) UpsertInboundHook(ctx context.Context, orgID string, in UpsertInboundHookInput) (InboundHookView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return InboundHookView{}, err
	}
	existing, err := s.db.Q().GetOrganisationInboundHook(ctx, orgID)
	secret := ""
	status := "disconnected"
	if err == nil {
		secret = existing.Secret
		status = existing.Status
	} else if err != pgx.ErrNoRows {
		return InboundHookView{}, err
	}

	var revealed string
	if in.Rotate {
		revealed, err = generateInboundSecret()
		if err != nil {
			return InboundHookView{}, err
		}
		secret = revealed
		status = "connected"
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
			return InboundHookView{}, apperr.Validation("status must be connected or disconnected")
		}
		status = st
	}
	if status == "connected" && strings.TrimSpace(secret) == "" {
		return InboundHookView{}, apperr.Validation("secret is required when status is connected")
	}

	row, err := s.db.Q().UpsertOrganisationInboundHook(ctx, sqlc.UpsertOrganisationInboundHookParams{
		OrganisationID: orgID,
		Secret:         secret,
		Status:         status,
	})
	if err != nil {
		return InboundHookView{}, err
	}
	view := InboundHookView{
		OrganisationID: row.OrganisationID,
		URL:            s.inboundHookURL(orgID),
		SecretHint:     maskSecret(row.Secret),
		HasSecret:      strings.TrimSpace(row.Secret) != "",
		Status:         row.Status,
		Secret:         revealed,
	}
	return view, nil
}

// HandleInboundWebhook verifies the org inbound secret and enqueues webhook.received.
func (s *Service) HandleInboundWebhook(ctx context.Context, orgID string, secretHeader string, rawBody []byte, contentType string) error {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return apperr.NotFound("organisation not found")
	}
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return mapNotFound(err, "organisation not found")
	}
	row, err := s.db.Q().GetOrganisationInboundHook(ctx, orgID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperr.Unauthorized("inbound webhook is not configured")
		}
		return err
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

	trigger := resources.LifecycleTrigger(resources.Webhook, resources.WebhookReceived)
	payload := map[string]any{
		"id":              orgID,
		"organisation_id": orgID,
		"trigger":         trigger,
		"body":            body,
		"content_type":    contentType,
		"received_at":     time.Now().UTC().Format(time.RFC3339),
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
