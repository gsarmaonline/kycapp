package service

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"

	"github.com/gsarmaonline/kyc/core/automations"
	"github.com/gsarmaonline/kyc/core/emailtemplates"
	"github.com/gsarmaonline/kyc/internal/mailer"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

// actionExecutor runs a validated action. Handlers live in core/automations;
// executors live here because they need Service deps (DB, mailer).
type actionExecutor func(ctx context.Context, s *Service, orgID string, params map[string]any, payload map[string]any) (string, error)

func (s *Service) executeAction(ctx context.Context, orgID string, a automations.Action, payload map[string]any) (string, error) {
	a = a.Normalize()
	if err := automations.ValidateAction(a); err != nil {
		return "", err
	}
	exec, ok := actionExecutors[a.Type]
	if !ok {
		return "", fmt.Errorf("unsupported action %q", a.Type)
	}
	return exec(ctx, s, orgID, a.Params, payload)
}

var actionExecutors = map[string]actionExecutor{
	automations.ActionSendEmail: execSendEmail,
}

func execSendEmail(ctx context.Context, s *Service, orgID string, params map[string]any, payload map[string]any) (string, error) {
	templateKey, err := automations.RequireStringParam(params, "template_key")
	if err != nil {
		return "", err
	}
	if err := s.EnsureDefaultEmailTemplates(ctx, orgID); err != nil {
		return "", err
	}
	tmpl, err := s.db.Q().GetEmailTemplateByOrgKey(ctx, sqlc.GetEmailTemplateByOrgKeyParams{
		OrganisationID: orgID,
		Key:            templateKey,
	})
	if err != nil {
		return "", fmt.Errorf("email template %q not found", templateKey)
	}
	org, err := s.GetOrganisation(ctx, orgID)
	if err != nil {
		return "", err
	}
	to := stringifyPayload(payload["email"])
	if to == "" {
		return "", fmt.Errorf("send_email: payload email is required")
	}
	vars := map[string]string{
		"display_name": stringifyPayload(payload["display_name"]),
		"org_name":     org.Name,
		"email":        to,
	}
	subject := emailtemplates.Render(tmpl.Subject, vars)
	textBody := emailtemplates.Render(tmpl.BodyText, vars)
	htmlInner := emailtemplates.Render(tmpl.BodyHtml, vars)
	if strings.TrimSpace(htmlInner) == "" {
		htmlInner = "<p>" + html.EscapeString(textBody) + "</p>"
	}
	htmlBody := emailtemplates.Wrap(htmlInner, BrandingFromOrg(org))
	ref, err := s.mailer.Send(ctx, mailer.Message{
		To:      []string{to},
		Subject: subject,
		HTML:    htmlBody,
		Text:    textBody,
		Tags: map[string]string{
			"org_id":       orgID,
			"template_key": templateKey,
			"source":       "automation",
		},
	})
	if err != nil {
		return "", fmt.Errorf("send_email: %w", err)
	}
	slog.Info("automation send_email",
		"org_id", orgID,
		"template", templateKey,
		"provider", s.mailer.Name(),
		"provider_ref", ref,
		"to", to,
	)
	return "send_email:" + templateKey + " → " + to + " (" + s.mailer.Name() + ":" + ref + ")", nil
}
