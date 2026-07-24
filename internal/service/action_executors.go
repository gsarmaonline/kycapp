package service

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"

	"github.com/gsarmaonline/kyc/core/automations"
	"github.com/gsarmaonline/kyc/core/emailtemplates"
	"github.com/gsarmaonline/kyc/core/resources"
	"github.com/gsarmaonline/kyc/internal/mailer"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

// actionExecutor runs a validated action with resolved subjects.
type actionExecutor func(
	ctx context.Context,
	s *Service,
	orgID string,
	params map[string]any,
	payload map[string]any,
	subjects map[string]map[string]any,
) (string, error)

func (s *Service) executeAction(
	ctx context.Context,
	orgID string,
	a automations.Action,
	payload map[string]any,
	subjects map[string]map[string]any,
) (string, error) {
	a = a.Normalize()
	if err := automations.ValidateAction(a); err != nil {
		return "", err
	}
	exec, ok := actionExecutors[a.Type]
	if !ok {
		return "", fmt.Errorf("unsupported action %q", a.Type)
	}
	return exec(ctx, s, orgID, a.Params, payload, subjects)
}

var actionExecutors = map[string]actionExecutor{
	automations.ActionSendEmail: execSendEmail,
}

func execSendEmail(
	ctx context.Context,
	s *Service,
	orgID string,
	params map[string]any,
	_ map[string]any,
	subjects map[string]map[string]any,
) (string, error) {
	templateKey, err := automations.RequireStringParam(params, "template_key")
	if err != nil {
		return "", err
	}
	appUser, ok := subjects[resources.SubjectAppUser]
	if !ok || appUser == nil {
		return "", fmt.Errorf("send_email: requires an app_user subject from the trigger")
	}
	to := stringifyPayload(appUser["email"])
	if to == "" {
		return "", fmt.Errorf("send_email: app_user email is required")
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
	vars := map[string]string{
		"display_name": stringifyPayload(appUser["display_name"]),
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

// resolveSubjects builds subject maps for action execution from the trigger payload
// and declared resource relations.
func (s *Service) resolveSubjects(ctx context.Context, orgID, trigger string, payload map[string]any) (map[string]map[string]any, error) {
	tr, err := resources.ParseTrigger(trigger)
	if err != nil {
		return nil, err
	}
	res, ok := resources.ByKey(tr.Resource)
	if !ok {
		return nil, fmt.Errorf("unknown resource %q", tr.Resource)
	}
	subjects := map[string]map[string]any{
		tr.Resource: payload,
	}
	for _, kind := range res.Provides {
		if _, exists := subjects[kind]; !exists {
			subjects[kind] = payload
		}
	}
	for _, rel := range res.Relations {
		if _, exists := subjects[rel.Subject]; exists {
			continue
		}
		id := stringifyPayload(payload[rel.Via])
		if id == "" {
			return nil, fmt.Errorf("cannot resolve subject %q: payload.%s is empty", rel.Subject, rel.Via)
		}
		resolved, err := s.loadSubject(ctx, orgID, rel.Subject, id)
		if err != nil {
			return nil, err
		}
		subjects[rel.Subject] = resolved
	}
	return subjects, nil
}

func (s *Service) loadSubject(ctx context.Context, orgID, kind, id string) (map[string]any, error) {
	switch kind {
	case resources.SubjectUser:
		u, err := s.GetUser(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("resolve user %q: %w", id, err)
		}
		return map[string]any{
			"id":     u.ID,
			"email":  u.Email,
			"name":   u.Name,
			"status": u.Status,
		}, nil
	case resources.SubjectAppUser:
		u, err := s.GetAppUser(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("resolve app_user %q: %w", id, err)
		}
		if u.OrganisationID != orgID {
			return nil, fmt.Errorf("app_user %q does not belong to organisation", id)
		}
		return AppUserEventPayload(u)
	default:
		return nil, fmt.Errorf("no resolver for subject %q", kind)
	}
}
