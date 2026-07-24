package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/core/automations"
	"github.com/gsarmaonline/kyc/core/resources"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/jobs"
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

// AutomationCatalog is the editor/runtime surface for triggers, actions, and condition fields.
type AutomationCatalog struct {
	Triggers        []automations.TriggerInfo        `json:"triggers"`
	Actions         []automations.ActionInfo         `json:"actions"`
	Ops             []automations.ConditionOpInfo    `json:"ops"`
	ConditionFields []automations.ConditionFieldInfo `json:"condition_fields"`
	Databases       []AutomationDatabaseOption       `json:"databases"`
	Webhooks        []AutomationWebhookOption        `json:"webhooks"`
}

type AutomationDatabaseOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AutomationWebhookOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Service) AutomationCatalog(ctx context.Context, orgID string) (AutomationCatalog, error) {
	attrs, err := s.appUserAttributeKeys(ctx, orgID)
	if err != nil {
		return AutomationCatalog{}, err
	}
	fields, err := s.automationConditionFields(ctx, orgID)
	if err != nil {
		return AutomationCatalog{}, err
	}
	dbs, err := s.ListOrganisationDatabases(ctx, orgID)
	if err != nil {
		return AutomationCatalog{}, err
	}
	dbOpts := make([]AutomationDatabaseOption, 0, len(dbs))
	for _, d := range dbs {
		if d.Status != "connected" {
			continue
		}
		dbOpts = append(dbOpts, AutomationDatabaseOption{ID: d.ID, Name: d.Name})
	}
	hooks, err := s.ListOrganisationWebhooks(ctx, orgID)
	if err != nil {
		return AutomationCatalog{}, err
	}
	whOpts := make([]AutomationWebhookOption, 0, len(hooks))
	for _, h := range hooks {
		if h.Status != "connected" {
			continue
		}
		whOpts = append(whOpts, AutomationWebhookOption{ID: h.ID, Name: h.Name})
	}
	return AutomationCatalog{
		Triggers:        automations.ExpandTriggers(attrs),
		Actions:         automations.Actions(),
		Ops:             automations.ConditionOps(),
		ConditionFields: fields,
		Databases:       dbOpts,
		Webhooks:        whOpts,
	}, nil
}

func (s *Service) appUserAttributeKeys(ctx context.Context, orgID string) ([]resources.AttributeKey, error) {
	defs, err := s.ListAttributeDefinitions(ctx, orgID, "active")
	if err != nil {
		return nil, err
	}
	out := make([]resources.AttributeKey, 0, len(defs))
	for _, d := range defs {
		out = append(out, resources.AttributeKey{Key: d.Key, Label: d.Label})
	}
	return out, nil
}

func (s *Service) automationConditionFields(ctx context.Context, orgID string) ([]automations.ConditionFieldInfo, error) {
	fields := automations.BaseConditionFields()
	defs, err := s.ListAttributeDefinitions(ctx, orgID, "active")
	if err != nil {
		return nil, err
	}
	for _, d := range defs {
		var enums []string
		if len(d.EnumValues) > 0 {
			_ = json.Unmarshal(d.EnumValues, &enums)
		}
		fields = append(fields, automations.AttributeConditionField(d.Key, d.Label, d.ValueType, enums))
	}
	return fields, nil
}

func (s *Service) validateAutomationSpec(ctx context.Context, orgID, trigger string, conditionsJSON, actionsJSON json.RawMessage) (automations.Spec, error) {
	spec, err := automations.ValidateCreate(trigger, conditionsJSON, actionsJSON)
	if err != nil {
		return automations.Spec{}, apperr.Validation(err.Error())
	}
	attrs, err := s.appUserAttributeKeys(ctx, orgID)
	if err != nil {
		return automations.Spec{}, err
	}
	if !automations.AllowedTriggerIDs(attrs)[spec.Trigger] {
		return automations.Spec{}, apperr.Validation(fmt.Sprintf("unknown trigger %q", spec.Trigger))
	}
	fields, err := s.automationConditionFields(ctx, orgID)
	if err != nil {
		return automations.Spec{}, err
	}
	if err := automations.ValidateConditionFields(spec.Conditions, fields); err != nil {
		return automations.Spec{}, apperr.Validation(err.Error())
	}
	// Subject compatibility is enforced inside ValidateCreate; re-check for clarity in errors.
	if err := automations.ValidateSubjectCompatibility(spec.Trigger, spec.Actions); err != nil {
		return automations.Spec{}, apperr.Validation(err.Error())
	}
	return spec, nil
}

// EnqueueResourceLifecycle fires {resource}.{lifecycle} for a payload.
func (s *Service) EnqueueResourceLifecycle(ctx context.Context, orgID, resource, lifecycle string, payload any) {
	s.EnqueueAutomationEvent(ctx, orgID, resources.LifecycleTrigger(resource, lifecycle), payload)
}

// EnqueueAttributeTriggers fires {resource}.attribute.{key} for each key.
func (s *Service) EnqueueAttributeTriggers(ctx context.Context, orgID, resource string, keys []string, payload any) {
	for _, key := range keys {
		if strings.TrimSpace(key) == "" {
			continue
		}
		s.EnqueueAutomationEvent(ctx, orgID, resources.AttributeTrigger(resource, key), payload)
	}
}

func (s *Service) CreateAutomation(ctx context.Context, orgID string, in CreateAutomationInput) (sqlc.Automation, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return sqlc.Automation{}, apperr.Validation("name is required")
	}
	spec, err := s.validateAutomationSpec(ctx, orgID, in.Trigger, in.Conditions, in.Actions)
	if err != nil {
		return sqlc.Automation{}, err
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
		Name:           name,
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
	spec, err := s.validateAutomationSpec(ctx, existing.OrganisationID, trigger, condRaw, actRaw)
	if err != nil {
		return sqlc.Automation{}, err
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
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return sqlc.Automation{}, apperr.Validation("name is required")
		}
		params.Name = pgtype.Text{String: name, Valid: true}
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
	actions = automations.NormalizeActions(actions)

	subjects, err := s.resolveSubjects(ctx, row.OrganisationID, trigger, payload)
	if err != nil {
		_ = s.recordRun(ctx, row, trigger, payloadJSON, "error", "resolve subjects: "+err.Error())
		return err
	}

	actionPayload := make(map[string]any, len(payload)+1)
	for k, v := range payload {
		actionPayload[k] = v
	}
	actionPayload["trigger"] = trigger

	meta := actionRunMeta{AutomationID: row.ID, Trigger: trigger}
	details, err := automations.RunActionGraph(actions, func(a automations.Action) (string, error) {
		return s.executeAction(ctx, row.OrganisationID, a, actionPayload, subjects, meta)
	})
	if err != nil {
		if errors.Is(err, automations.ErrActionPaused) {
			return s.recordRun(ctx, row, trigger, payloadJSON, "paused", strings.Join(details, "; "))
		}
		msg := err.Error()
		if len(details) > 0 {
			msg = strings.Join(details, "; ") + "; " + msg
		}
		_ = s.recordRun(ctx, row, trigger, payloadJSON, "error", msg)
		return err
	}
	return s.recordRun(ctx, row, trigger, payloadJSON, "success", strings.Join(details, "; "))
}

// ResumeAutomation continues a workflow after a delay action.
func (s *Service) ResumeAutomation(ctx context.Context, orgID, automationID, trigger string, payloadJSON json.RawMessage, nextActionID string) error {
	row, err := s.db.Q().GetAutomation(ctx, automationID)
	if err != nil {
		return err
	}
	if row.OrganisationID != orgID {
		return apperr.NotFound("automation not found")
	}
	if !row.Enabled {
		return s.recordRun(ctx, row, trigger, payloadJSON, "skipped", "automation disabled before resume")
	}

	var payload map[string]any
	if len(payloadJSON) > 0 {
		if err := json.Unmarshal(payloadJSON, &payload); err != nil {
			return s.recordRun(ctx, row, trigger, payloadJSON, "error", "invalid resume payload")
		}
	}
	if payload == nil {
		payload = map[string]any{}
	}

	var actions []automations.Action
	if err := json.Unmarshal(row.Actions, &actions); err != nil {
		return s.recordRun(ctx, row, trigger, payloadJSON, "error", "invalid actions: "+err.Error())
	}
	actions = automations.NormalizeActions(actions)

	subjects, err := s.resolveSubjects(ctx, orgID, trigger, payload)
	if err != nil {
		_ = s.recordRun(ctx, row, trigger, payloadJSON, "error", "resolve subjects: "+err.Error())
		return err
	}

	actionPayload := make(map[string]any, len(payload)+1)
	for k, v := range payload {
		actionPayload[k] = v
	}
	actionPayload["trigger"] = trigger

	meta := actionRunMeta{AutomationID: row.ID, Trigger: trigger}
	details, err := automations.RunActionGraphFrom(actions, nextActionID, func(a automations.Action) (string, error) {
		return s.executeAction(ctx, orgID, a, actionPayload, subjects, meta)
	})
	if err != nil {
		if errors.Is(err, automations.ErrActionPaused) {
			return s.recordRun(ctx, row, trigger, payloadJSON, "paused", strings.Join(details, "; "))
		}
		msg := err.Error()
		if len(details) > 0 {
			msg = strings.Join(details, "; ") + "; " + msg
		}
		_ = s.recordRun(ctx, row, trigger, payloadJSON, "error", msg)
		return err
	}
	return s.recordRun(ctx, row, trigger, payloadJSON, "success", strings.Join(details, "; "))
}

// ProcessScheduleTick fires due schedule.* triggers for all orgs (UTC).
func (s *Service) ProcessScheduleTick(ctx context.Context, at time.Time) error {
	due := jobs.DueScheduleTriggers(at)
	if len(due) == 0 {
		return nil
	}
	for _, trigger := range due {
		orgIDs, err := s.db.Q().ListOrgIDsWithEnabledTrigger(ctx, trigger)
		if err != nil {
			return err
		}
		for _, orgID := range orgIDs {
			payload := map[string]any{
				"id":              orgID,
				"organisation_id": orgID,
				"trigger":         trigger,
				"scheduled_at":    at.UTC().Format(time.RFC3339),
			}
			s.EnqueueAutomationEvent(ctx, orgID, trigger, payload)
		}
	}
	return nil
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
