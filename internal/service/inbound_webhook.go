package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/core/resources"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

const (
	InboundAuthHeader = "header" // X-KYC-Webhook-Secret
	InboundAuthQuery  = "query"  // ?secret=
	InboundAuthPath   = "path"   // /v1/hooks/inbound/{id}/{secret}
)

// InboundWebhookView is an org inbound endpoint (fires webhook.received).
type InboundWebhookView struct {
	ID             string
	OrganisationID string
	Name           string
	URL            string
	AuthMode       string
	SecretHint     string
	HasSecret      bool
	Status         string
	// Secret is set on create/rotate, and also when auth embeds it in the URL
	// so admins can copy the vendor-facing endpoint.
	Secret string `json:"-"`
}

type CreateInboundWebhookInput struct {
	Name     string
	Secret   string // empty → auto-generate
	Status   string
	AuthMode string // header | query | path
}

type UpdateInboundWebhookInput struct {
	Name     *string
	Secret   *string
	Status   *string
	AuthMode *string
	Rotate   bool
}

// InboundAuthInput is how the caller presented credentials.
type InboundAuthInput struct {
	HeaderSecret string
	QuerySecret  string
	PathSecret   string
}

func normalizeInboundAuthMode(mode string) (string, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		return InboundAuthHeader, nil
	}
	switch mode {
	case InboundAuthHeader, InboundAuthQuery, InboundAuthPath:
		return mode, nil
	default:
		return "", apperr.Validation("auth_mode must be header, query, or path")
	}
}

func (s *Service) inboundWebhookURL(row sqlc.OrganisationInboundWebhook) string {
	base := s.publicBaseURL + "/v1/hooks/inbound/" + row.ID
	secret := strings.TrimSpace(row.Secret)
	switch row.AuthMode {
	case InboundAuthQuery:
		if secret == "" {
			return base + "?secret="
		}
		return base + "?secret=" + url.QueryEscape(secret)
	case InboundAuthPath:
		if secret == "" {
			return base + "/"
		}
		return base + "/" + url.PathEscape(secret)
	default:
		return base
	}
}

func inboundWebhookView(s *Service, row sqlc.OrganisationInboundWebhook, revealed string) InboundWebhookView {
	secretOut := revealed
	// For query/path modes the URL embeds the secret — expose it to admins for copy/paste.
	if secretOut == "" && (row.AuthMode == InboundAuthQuery || row.AuthMode == InboundAuthPath) {
		secretOut = row.Secret
	}
	return InboundWebhookView{
		ID:             row.ID,
		OrganisationID: row.OrganisationID,
		Name:           row.Name,
		URL:            s.inboundWebhookURL(row),
		AuthMode:       row.AuthMode,
		SecretHint:     maskSecret(row.Secret),
		HasSecret:      strings.TrimSpace(row.Secret) != "",
		Status:         row.Status,
		Secret:         secretOut,
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
	authMode, err := normalizeInboundAuthMode(in.AuthMode)
	if err != nil {
		return InboundWebhookView{}, err
	}
	secret := strings.TrimSpace(in.Secret)
	var revealed string
	if secret == "" {
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
		AuthMode:       authMode,
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
	authMode := existing.AuthMode
	var revealed string

	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
		if name == "" {
			return InboundWebhookView{}, apperr.Validation("name is required")
		}
	}
	if in.AuthMode != nil {
		authMode, err = normalizeInboundAuthMode(*in.AuthMode)
		if err != nil {
			return InboundWebhookView{}, err
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
		AuthMode:       authMode,
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

func secretsEqual(got, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" || want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// HandleInboundWebhook verifies credentials per auth_mode and enqueues webhook.received.
func (s *Service) HandleInboundWebhook(ctx context.Context, hookID string, auth InboundAuthInput, rawBody []byte, contentType string) error {
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

	switch row.AuthMode {
	case InboundAuthQuery:
		if !secretsEqual(auth.QuerySecret, row.Secret) {
			return apperr.Unauthorized("invalid webhook secret")
		}
	case InboundAuthPath:
		if !secretsEqual(auth.PathSecret, row.Secret) {
			return apperr.Unauthorized("invalid webhook secret")
		}
	default: // header
		if !secretsEqual(auth.HeaderSecret, row.Secret) {
			return apperr.Unauthorized("invalid webhook secret")
		}
	}

	body := parseInboundBody(rawBody, contentType)

	orgID := row.OrganisationID
	trigger := resources.LifecycleTrigger(resources.Webhook, resources.WebhookReceived)
	payload := map[string]any{
		"id":                   orgID,
		"organisation_id":      orgID,
		"trigger":              trigger,
		"inbound_webhook_id":   row.ID,
		"inbound_webhook_name": row.Name,
		"body":                 body,
		"content_type":         contentType,
		"received_at":          time.Now().UTC().Format(time.RFC3339),
	}
	s.EnqueueAutomationEvent(ctx, orgID, trigger, payload)
	return nil
}

func parseInboundBody(rawBody []byte, contentType string) any {
	if len(rawBody) == 0 {
		return map[string]any{}
	}
	ct := strings.ToLower(strings.TrimSpace(contentType))
	looksJSON := strings.Contains(ct, "json") || rawBody[0] == '{' || rawBody[0] == '['
	if looksJSON {
		var body any
		if err := json.Unmarshal(rawBody, &body); err == nil {
			return body
		}
	}
	// Source could not / did not send JSON — keep raw text.
	return string(rawBody)
}

func generateInboundSecret() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return "whsec_" + hex.EncodeToString(b), nil
}
