package service

import (
	"context"
	"errors"
	"strings"

	"github.com/gsarmaonline/kyc/core/billing"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type PlanView struct {
	Plan            sqlc.Plan
	EntitlementKeys []string
}

type CreatePlanInput struct {
	Key  string
	Name string
}

type CreateEntitlementInput struct {
	Key         string
	Description string
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

func (s *Service) GetPlan(ctx context.Context, id string) (PlanView, error) {
	plan, err := s.db.Q().GetPlan(ctx, id)
	if err != nil {
		return PlanView{}, mapNotFound(err, "plan not found")
	}
	keys, err := s.db.Q().ListEntitlementKeysByPlan(ctx, plan.ID)
	if err != nil {
		return PlanView{}, err
	}
	return PlanView{Plan: plan, EntitlementKeys: keys}, nil
}

func (s *Service) ListPlans(ctx context.Context) ([]PlanView, error) {
	plans, err := s.db.Q().ListPlans(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PlanView, 0, len(plans))
	for _, p := range plans {
		keys, err := s.db.Q().ListEntitlementKeysByPlan(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, PlanView{Plan: p, EntitlementKeys: keys})
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
	ent, err := s.db.Q().CreateEntitlement(ctx, sqlc.CreateEntitlementParams{
		ID:          ids.New(),
		Key:         key,
		Description: strings.TrimSpace(in.Description),
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
	sub, err := s.db.Q().UpsertSubscription(ctx, sqlc.UpsertSubscriptionParams{
		ID:               ids.New(),
		OrganisationID:   orgID,
		PlanID:           in.PlanID,
		Status:           status,
		CurrentPeriodEnd: pgtype.Timestamptz{},
	})
	return sub, err
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
	rows, err := s.db.Q().ListEntitlementIDsByKeys(ctx, uniq)
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
	return s.EffectiveEntitlements(ctx, orgID)
}

func (s *Service) EffectiveEntitlements(ctx context.Context, orgID string) ([]string, error) {
	org, err := s.GetOrganisation(ctx, orgID)
	if err != nil {
		return nil, err
	}
	_ = org

	var planKeys []string
	sub, err := s.db.Q().GetSubscriptionByOrganisation(ctx, orgID)
	if err == nil {
		planKeys, err = s.db.Q().ListEntitlementKeysByPlan(ctx, sub.PlanID)
		if err != nil {
			return nil, err
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	overrideRows, err := s.db.Q().ListOrganisationEntitlementOverrides(ctx, orgID)
	if err != nil {
		return nil, err
	}
	overrides := make(map[string]string, len(overrideRows))
	for _, row := range overrideRows {
		overrides[row.Key] = row.Effect
	}
	return billing.EffectiveKeys(planKeys, overrides), nil
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
	if _, err := s.db.Q().GetEntitlementByKey(ctx, entitlementKey); err != nil {
		return false, mapNotFound(err, "entitlement not found")
	}
	effective, err := s.EffectiveEntitlements(ctx, orgID)
	if err != nil {
		return false, err
	}
	return billing.Has(effective, entitlementKey), nil
}
