package service

import (
	"context"
	"errors"
	"strings"

	"github.com/gsarmaonline/kyc/core/billing"
	"github.com/gsarmaonline/kyc/core/resources"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/observability"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type PlanView struct {
	Plan                   sqlc.Plan
	EntitlementKeys        []string
	PlatformCapabilityKeys []string
	ProductFeatureKeys     []string
}

type EffectiveEntitlementsView struct {
	Entitlements         []string
	PlatformCapabilities []string
	ProductFeatures      []string
}

type CreatePlanInput struct {
	Key  string
	Name string
}

type CreateEntitlementInput struct {
	Key         string
	Description string
	Scope       string
}

type SetPlanEntitlementsInput struct {
	EntitlementKeys []string
}

type UpsertSubscriptionInput struct {
	PlanID string
	Status string
}

type EntitlementOverride struct {
	Key    string
	Effect string
}

type SetOrganisationEntitlementsInput struct {
	Overrides []EntitlementOverride
}

func (s *Service) CreatePlan(ctx context.Context, in CreatePlanInput) (sqlc.Plan, error) {
	key := strings.TrimSpace(in.Key)
	name := strings.TrimSpace(in.Name)
	if key == "" || name == "" {
		return sqlc.Plan{}, apperr.Validation("key and name are required")
	}
	plan, err := s.db.Q().CreatePlan(ctx, sqlc.CreatePlanParams{
		ID:     ids.New(),
		Key:    key,
		Name:   name,
		Status: "active",
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			return sqlc.Plan{}, apperr.Conflict("plan key already exists")
		}
		return sqlc.Plan{}, err
	}
	return plan, nil
}

func (s *Service) planView(ctx context.Context, plan sqlc.Plan) (PlanView, error) {
	rows, err := s.db.Q().ListEntitlementsByPlan(ctx, plan.ID)
	if err != nil {
		return PlanView{}, err
	}
	keys := make([]string, 0, len(rows))
	platform := make([]string, 0)
	product := make([]string, 0)
	for _, row := range rows {
		keys = append(keys, row.Key)
		switch row.Scope {
		case "product":
			product = append(product, row.Key)
		default:
			platform = append(platform, row.Key)
		}
	}
	return PlanView{
		Plan:                   plan,
		EntitlementKeys:        keys,
		PlatformCapabilityKeys: platform,
		ProductFeatureKeys:     product,
	}, nil
}

func (s *Service) GetPlan(ctx context.Context, id string) (PlanView, error) {
	plan, err := s.db.Q().GetPlan(ctx, id)
	if err != nil {
		return PlanView{}, mapNotFound(err, "plan not found")
	}
	return s.planView(ctx, plan)
}

func (s *Service) ListPlans(ctx context.Context) ([]PlanView, error) {
	plans, err := s.db.Q().ListPlans(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PlanView, 0, len(plans))
	for _, p := range plans {
		view, err := s.planView(ctx, p)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, nil
}

func (s *Service) SetPlanEntitlements(ctx context.Context, planID string, in SetPlanEntitlementsInput) (PlanView, error) {
	if _, err := s.db.Q().GetPlan(ctx, planID); err != nil {
		return PlanView{}, mapNotFound(err, "plan not found")
	}
	keys := uniqueStrings(in.EntitlementKeys)
	var rows []sqlc.ListEntitlementIDsByKeysRow
	if len(keys) > 0 {
		var err error
		rows, err = s.db.Q().ListEntitlementIDsByKeys(ctx, keys)
		if err != nil {
			return PlanView{}, err
		}
		if len(rows) != len(keys) {
			return PlanView{}, apperr.Validation("one or more entitlement_keys are unknown")
		}
	}
	err := s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		if err := q.DeletePlanEntitlements(ctx, planID); err != nil {
			return err
		}
		for _, row := range rows {
			if err := q.AddPlanEntitlement(ctx, sqlc.AddPlanEntitlementParams{
				PlanID:        planID,
				EntitlementID: row.ID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return PlanView{}, err
	}
	return s.GetPlan(ctx, planID)
}

func (s *Service) CreateEntitlement(ctx context.Context, in CreateEntitlementInput) (sqlc.Entitlement, error) {
	key := strings.TrimSpace(in.Key)
	if key == "" {
		return sqlc.Entitlement{}, apperr.Validation("key is required")
	}
	scope := strings.TrimSpace(in.Scope)
	if scope == "" {
		scope = "platform"
	}
	switch scope {
	case "platform", "product":
	default:
		return sqlc.Entitlement{}, apperr.Validation("scope must be platform or product")
	}
	ent, err := s.db.Q().CreateEntitlement(ctx, sqlc.CreateEntitlementParams{
		ID:          ids.New(),
		Key:         key,
		Description: strings.TrimSpace(in.Description),
		Scope:       scope,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			return sqlc.Entitlement{}, apperr.Conflict("entitlement key already exists")
		}
		return sqlc.Entitlement{}, err
	}
	return ent, nil
}

func (s *Service) ListEntitlements(ctx context.Context) ([]sqlc.Entitlement, error) {
	return s.db.Q().ListEntitlements(ctx)
}

func (s *Service) UpsertSubscription(ctx context.Context, orgID string, in UpsertSubscriptionInput) (sqlc.Subscription, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return sqlc.Subscription{}, err
	}
	if strings.TrimSpace(in.PlanID) == "" {
		return sqlc.Subscription{}, apperr.Validation("plan_id is required")
	}
	if _, err := s.db.Q().GetPlan(ctx, in.PlanID); err != nil {
		return sqlc.Subscription{}, mapNotFound(err, "plan not found")
	}
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "active"
	}
	switch status {
	case "trialing", "active", "past_due", "canceled":
	default:
		return sqlc.Subscription{}, apperr.Validation("invalid subscription status")
	}
	existing, existingErr := s.db.Q().GetSubscriptionByOrganisation(ctx, orgID)
	created := errors.Is(existingErr, pgx.ErrNoRows)
	var prevStatus string
	if existingErr == nil {
		prevStatus = existing.Status
	} else if !created {
		return sqlc.Subscription{}, existingErr
	}
	sub, err := s.db.Q().UpsertSubscription(ctx, sqlc.UpsertSubscriptionParams{
		ID:               ids.New(),
		OrganisationID:   orgID,
		PlanID:           in.PlanID,
		Status:           status,
		CurrentPeriodEnd: pgtype.Timestamptz{},
	})
	if err != nil {
		return sqlc.Subscription{}, err
	}
	lifecycle := resources.LifecycleUpdated
	action := observability.ActionSubscriptionUpdated
	summary := "Subscription updated"
	if created {
		lifecycle = resources.LifecycleCreated
		action = observability.ActionSubscriptionCreated
		summary = "Subscription created"
	}
	s.EnqueueResourceLifecycle(ctx, orgID, resources.Subscription, lifecycle, subscriptionEventPayload(sub))
	org, _ := s.db.Q().GetOrganisation(ctx, orgID)
	payload := map[string]any{}
	if prevStatus != "" && prevStatus != sub.Status {
		action = observability.ActionSubscriptionStatusChanged
		summary = "Subscription status changed"
		payload["from_status"] = prevStatus
		payload["to_status"] = sub.Status
	}
	s.recordActivity(ctx, activityForSubscription(org, sub, action, summary, payload))
	return sub, nil
}

func subscriptionEventPayload(sub sqlc.Subscription) map[string]any {
	out := map[string]any{
		"id":              sub.ID,
		"organisation_id": sub.OrganisationID,
		"plan_id":         sub.PlanID,
		"status":          sub.Status,
	}
	if sub.CurrentPeriodEnd.Valid {
		out["current_period_end"] = sub.CurrentPeriodEnd.Time.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return out
}

func (s *Service) GetSubscription(ctx context.Context, orgID string) (sqlc.Subscription, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return sqlc.Subscription{}, err
	}
	sub, err := s.db.Q().GetSubscriptionByOrganisation(ctx, orgID)
	return sub, mapNotFound(err, "subscription not found")
}

func (s *Service) SetOrganisationEntitlements(ctx context.Context, orgID string, in SetOrganisationEntitlementsInput) ([]string, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(in.Overrides))
	for _, o := range in.Overrides {
		switch strings.TrimSpace(o.Effect) {
		case "grant", "deny":
		default:
			return nil, apperr.Validation("override effect must be grant or deny")
		}
		k := strings.TrimSpace(o.Key)
		if k == "" {
			return nil, apperr.Validation("override key is required")
		}
		keys = append(keys, k)
	}
	uniq := uniqueStrings(keys)
	rows, err := s.db.Q().ListEntitlementIDsByKeysForOrg(ctx, sqlc.ListEntitlementIDsByKeysForOrgParams{
		Keys:           uniq,
		OrganisationID: textArg(orgID),
	})
	if err != nil {
		return nil, err
	}
	if len(rows) != len(uniq) {
		return nil, apperr.Validation("one or more entitlement keys are unknown")
	}
	byKey := make(map[string]string, len(rows))
	for _, r := range rows {
		byKey[r.Key] = r.ID
	}

	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		if err := q.DeleteOrganisationEntitlements(ctx, orgID); err != nil {
			return err
		}
		for _, o := range in.Overrides {
			if err := q.UpsertOrganisationEntitlement(ctx, sqlc.UpsertOrganisationEntitlementParams{
				OrganisationID: orgID,
				EntitlementID:  byKey[strings.TrimSpace(o.Key)],
				Effect:         strings.TrimSpace(o.Effect),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	view, err := s.EffectiveEntitlementsView(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return view.Entitlements, nil
}

func (s *Service) EffectiveEntitlements(ctx context.Context, orgID string) ([]string, error) {
	view, err := s.EffectiveEntitlementsView(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return view.Entitlements, nil
}

func (s *Service) EffectiveEntitlementsView(ctx context.Context, orgID string) (EffectiveEntitlementsView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return EffectiveEntitlementsView{}, err
	}

	var baseKeys []string
	sub, err := s.db.Q().GetSubscriptionByOrganisation(ctx, orgID)
	if err == nil {
		baseKeys, err = s.db.Q().ListEntitlementKeysByPlan(ctx, sub.PlanID)
		if err != nil {
			return EffectiveEntitlementsView{}, err
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return EffectiveEntitlementsView{}, err
	}
	productPlanKeys, err := s.activeProductPlanFeatureKeys(ctx, orgID)
	if err != nil {
		return EffectiveEntitlementsView{}, err
	}
	baseKeys = append(baseKeys, productPlanKeys...)

	overrideRows, err := s.db.Q().ListOrganisationEntitlementOverrides(ctx, orgID)
	if err != nil {
		return EffectiveEntitlementsView{}, err
	}
	overrides := make(map[string]string, len(overrideRows))
	for _, row := range overrideRows {
		overrides[row.Key] = row.Effect
	}
	keys := billing.EffectiveKeys(baseKeys, overrides)
	platform, product, err := s.splitEntitlementKeysByScope(ctx, orgID, keys)
	if err != nil {
		return EffectiveEntitlementsView{}, err
	}
	return EffectiveEntitlementsView{
		Entitlements:         keys,
		PlatformCapabilities: platform,
		ProductFeatures:      product,
	}, nil
}

func (s *Service) splitEntitlementKeysByScope(ctx context.Context, orgID string, keys []string) (platform, product []string, err error) {
	platform = []string{}
	product = []string{}
	if len(keys) == 0 {
		return platform, product, nil
	}
	rows, err := s.db.Q().ListEntitlementScopesByKeysForOrg(ctx, sqlc.ListEntitlementScopesByKeysForOrgParams{
		Keys:           keys,
		OrganisationID: textArg(orgID),
	})
	if err != nil {
		return nil, nil, err
	}
	byKey := make(map[string]string, len(rows))
	for _, row := range rows {
		byKey[row.Key] = row.Scope
	}
	for _, key := range keys {
		switch byKey[key] {
		case "product":
			product = append(product, key)
		default:
			platform = append(platform, key)
		}
	}
	return platform, product, nil
}

func (s *Service) CheckEntitlement(ctx context.Context, orgID, entitlementKey string) (bool, error) {
	entitlementKey = strings.TrimSpace(entitlementKey)
	if orgID == "" || entitlementKey == "" {
		return false, apperr.Validation("organisation_id and entitlement are required")
	}
	org, err := s.GetOrganisation(ctx, orgID)
	if err != nil {
		return false, err
	}
	if org.Status != "active" {
		return false, nil
	}
	if _, err := s.db.Q().GetEntitlementForOrgCheck(ctx, sqlc.GetEntitlementForOrgCheckParams{
		Key:            entitlementKey,
		OrganisationID: textArg(orgID),
	}); err != nil {
		return false, mapNotFound(err, "entitlement not found")
	}
	effective, err := s.EffectiveEntitlements(ctx, orgID)
	if err != nil {
		return false, err
	}
	allowed := billing.Has(effective, entitlementKey)
	s.incrUsage(ctx, observability.UsageDelta{
		OrganisationID: orgID,
		MeterKey:       observability.MeterEntitlementCheck,
		Dim1Key:        "entitlement",
		Dim1Value:      entitlementKey,
		Dim2Key:        "result",
		Dim2Value:      entitlementResultLabel(allowed),
		Delta:          1,
	})
	return allowed, nil
}
