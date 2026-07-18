package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gsarmaonline/kyc/core/automations"
	"github.com/gsarmaonline/kyc/core/emailtemplates"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreateAutomationInput struct {
	Name       string
	Trigger    string
	Enabled    *bool
	Conditions json.RawMessage
	Actions    json.RawMessage
}

type UpdateAutomationInput struct {
	Name       *string
	Trigger    *string
	Enabled    *bool
	Conditions json.RawMessage
	Actions    json.RawMessage
}

func (s *Service) CreateAutomation(ctx context.Context, orgID string, in CreateAutomationInput) (sqlc.Automation, error) {
	spec, err := automations.ValidateCreate(in.Trigger, in.Conditions, in.Actions)
	if err != nil {
		return sqlc.Automation{}, apperr.Validation(err.Error())
	}
	condRaw, err := automations.MarshalConditions(spec.Conditions)
	if err != nil {
		return sqlc.Automation{}, apperr.Validation("invalid conditions")
	}
	actRaw, err := automations.MarshalActions(spec.Actions)
	if err != nil {
		return sqlc.Automation{}, apperr.Validation("invalid actions")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	return s.db.Q().CreateAutomation(ctx, sqlc.CreateAutomationParams{
		ID:             ids.New(),
		OrganisationID: orgID,
		Name:           strings.TrimSpace(in.Name),
		Trigger:        spec.Trigger,
		Enabled:        enabled,
		Conditions:     condRaw,
		Actions:        actRaw,
	})
}

func (s *Service) GetAutomation(ctx context.Context, id string) (sqlc.Automation, error) {
	row, err := s.db.Q().GetAutomation(ctx, id)
	return row, mapNotFound(err, "automation not found")
}

func (s *Service) ListAutomations(ctx context.Context, orgID string) ([]sqlc.Automation, error) {
	return s.db.Q().ListAutomations(ctx, orgID)
}

func (s *Service) UpdateAutomation(ctx context.Context, id string, in UpdateAutomationInput) (sqlc.Automation, error) {
	existing, err := s.GetAutomation(ctx, id)
	if err != nil {
		return sqlc.Automation{}, err
	}

	trigger := existing.Trigger
	if in.Trigger != nil {
		trigger = *in.Trigger
	}
	condRaw := existing.Conditions
	if len(in.Conditions) > 0 {
		condRaw = in.Conditions
	}
	actRaw := existing.Actions
	if len(in.Actions) > 0 {
		actRaw = in.Actions
	}
	spec, err := automations.ValidateCreate(trigger, condRaw, actRaw)
	if err != nil {
		return sqlc.Automation{}, apperr.Validation(err.Error())
	}
	condRaw, err = automations.MarshalConditions(spec.Conditions)
	if err != nil {
		return sqlc.Automation{}, apperr.Validation("invalid conditions")
	}
	actRaw, err = automations.MarshalActions(spec.Actions)
	if err != nil {
		return sqlc.Automation{}, apperr.Validation("invalid actions")
	}

	params := sqlc.UpdateAutomationParams{
		ID:         id,
		Trigger:    pgtype.Text{String: spec.Trigger, Valid: true},
		Conditions: condRaw,
		Actions:    actRaw,
	}
	if in.Name != nil {
		params.Name = pgtype.Text{String: strings.TrimSpace(*in.Name), Valid: true}
	}
	if in.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *in.Enabled, Valid: true}
	}
	return s.db.Q().UpdateAutomation(ctx, params)
}

func (s *Service) DeleteAutomation(ctx context.Context, id string) error {
	if _, err := s.GetAutomation(ctx, id); err != nil {
		return err
	}
	return s.db.Q().DeleteAutomation(ctx, id)
}

func (s *Service) ListAutomationRuns(ctx context.Context, orgID, automationID string, limit int32) ([]sqlc.AutomationRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.db.Q().ListAutomationRuns(ctx, sqlc.ListAutomationRunsParams{
		OrganisationID: orgID,
		AutomationID:   textArg(automationID),
		LimitCount:     limit,
	})
}

// EnqueueAutomationEvent queues processing when an enqueuer is configured.
func (s *Service) EnqueueAutomationEvent(ctx context.Context, orgID, trigger string, payload any) {
	if s.enqueue == nil {
		return
	}
	if err := s.enqueue.EnqueueAutomationEvent(ctx, orgID, trigger, payload); err != nil {
		slog.Error("enqueue automation event", "org", orgID, "trigger", trigger, "err", err)
	}
}

// ProcessAutomationEvent evaluates enabled automations for a trigger (worker entrypoint).
func (s *Service) ProcessAutomationEvent(ctx context.Context, orgID, trigger string, payloadJSON json.RawMessage) error {
	var payload map[string]any
	if len(payloadJSON) > 0 {
		if err := json.Unmarshal(payloadJSON, &payload); err != nil {
			return err
		}
	}
	if payload == nil {
		payload = map[string]any{}
	}

	rows, err := s.db.Q().ListEnabledAutomationsByTrigger(ctx, sqlc.ListEnabledAutomationsByTriggerParams{
		OrganisationID: orgID,
		Trigger:        trigger,
	})
	if err != nil {
		return err
	}

	for _, row := range rows {
		if err := s.runOneAutomation(ctx, row, trigger, payload, payloadJSON); err != nil {
			slog.Error("automation run failed", "automation_id", row.ID, "err", err)
		}
	}
	return nil
}

func (s *Service) runOneAutomation(ctx context.Context, row sqlc.Automation, trigger string, payload map[string]any, payloadJSON json.RawMessage) error {
	var cond automations.Conditions
	if err := json.Unmarshal(row.Conditions, &cond); err != nil {
		return s.recordRun(ctx, row, trigger, payloadJSON, "error", "invalid conditions: "+err.Error())
	}
	if !automations.Match(cond, payload) {
		return s.recordRun(ctx, row, trigger, payloadJSON, "skipped", "conditions not matched")
	}

	var actions []automations.Action
	if err := json.Unmarshal(row.Actions, &actions); err != nil {
		return s.recordRun(ctx, row, trigger, payloadJSON, "error", "invalid actions: "+err.Error())
	}

	var details []string
	for _, a := range actions {
		detail, err := s.executeAction(ctx, row.OrganisationID, a, payload)
		if err != nil {
			_ = s.recordRun(ctx, row, trigger, payloadJSON, "error", err.Error())
			return err
		}
		details = append(details, detail)
	}
	return s.recordRun(ctx, row, trigger, payloadJSON, "success", strings.Join(details, "; "))
}

func (s *Service) executeAction(ctx context.Context, orgID string, a automations.Action, payload map[string]any) (string, error) {
	switch a.Type {
	case automations.ActionSendEmail:
		if err := s.EnsureDefaultEmailTemplates(ctx, orgID); err != nil {
			return "", err
		}
		tmpl, err := s.db.Q().GetEmailTemplateByOrgKey(ctx, sqlc.GetEmailTemplateByOrgKeyParams{
			OrganisationID: orgID,
			Key:            a.TemplateKey,
		})
		if err != nil {
			return "", fmt.Errorf("email template %q not found", a.TemplateKey)
		}
		org, err := s.GetOrganisation(ctx, orgID)
		if err != nil {
			return "", err
		}
		vars := map[string]string{
			"display_name": stringifyPayload(payload["display_name"]),
			"org_name":     org.Name,
			"email":        stringifyPayload(payload["email"]),
		}
		subject := emailtemplates.Render(tmpl.Subject, vars)
		body := emailtemplates.Render(tmpl.BodyText, vars)
		slog.Info("automation send_email (stub delivery)",
			"org_id", orgID,
			"template", a.TemplateKey,
			"subject", subject,
			"to", vars["email"],
			"body_len", len(body),
		)
		return "send_email:" + a.TemplateKey + " → " + vars["email"], nil
	default:
		return "", fmt.Errorf("unsupported action %q", a.Type)
	}
}

func (s *Service) recordRun(ctx context.Context, row sqlc.Automation, trigger string, payloadJSON json.RawMessage, status, detail string) error {
	if len(payloadJSON) == 0 {
		payloadJSON = json.RawMessage(`{}`)
	}
	_, err := s.db.Q().CreateAutomationRun(ctx, sqlc.CreateAutomationRunParams{
		ID:             ids.New(),
		OrganisationID: row.OrganisationID,
		AutomationID:   row.ID,
		Trigger:        trigger,
		Status:         status,
		Detail:         detail,
		Payload:        payloadJSON,
	})
	return err
}

func stringifyPayload(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return strings.Trim(string(b), `"`)
}

// AppUserEventPayload builds the automation payload for an app user row.
func AppUserEventPayload(u sqlc.AppUser) (map[string]any, error) {
	attrs := map[string]any{}
	if len(u.Attributes) > 0 {
		if err := json.Unmarshal(u.Attributes, &attrs); err != nil {
			return nil, err
		}
	}
	out := map[string]any{
		"id":           u.ID,
		"display_name": u.DisplayName,
		"status":       u.Status,
		"attributes":   attrs,
	}
	if u.Email.Valid {
		out["email"] = u.Email.String
	} else {
		out["email"] = ""
	}
	if u.ExternalID.Valid {
		out["external_id"] = u.ExternalID.String
	}
	return out, nil
}
