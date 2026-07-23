package service

import (
	"context"
	"errors"
	"strings"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type ProductPlanView struct {
	Plan        sqlc.ProductPlan
	FeatureKeys []string
	Prices      []sqlc.ProductPlanPrice
}

type CreateProductFeatureInput struct {
	Key         string
	Description string
}

type UpdateProductFeatureInput struct {
	Description string
}

type CreateProductPlanInput struct {
	Key   string
	Name  string
	Price *ProductPlanPriceInput
}

type UpdateProductPlanInput struct {
	Name   *string
	Status *string
}

type SetProductPlanFeaturesInput struct {
	FeatureKeys []string
}

func (s *Service) CreateProductFeature(ctx context.Context, orgID string, in CreateProductFeatureInput) (sqlc.Entitlement, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return sqlc.Entitlement{}, err
	}
	key := strings.TrimSpace(in.Key)
	if key == "" {
		return sqlc.Entitlement{}, apperr.Validation("key is required")
	}
	ent, err := s.db.Q().CreateProductFeature(ctx, sqlc.CreateProductFeatureParams{
		ID:             ids.New(),
		Key:            key,
		Description:    strings.TrimSpace(in.Description),
		OrganisationID: textArg(orgID),
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			return sqlc.Entitlement{}, apperr.Conflict("product feature key already exists")
		}
		return sqlc.Entitlement{}, err
	}
	return ent, nil
}

func (s *Service) ListProductFeatures(ctx context.Context, orgID string) ([]sqlc.Entitlement, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return nil, err
	}
	return s.db.Q().ListProductFeaturesByOrg(ctx, textArg(orgID))
}

func (s *Service) GetProductFeature(ctx context.Context, id string) (sqlc.Entitlement, error) {
	ent, err := s.db.Q().GetEntitlement(ctx, id)
	if err != nil {
		return sqlc.Entitlement{}, mapNotFound(err, "product feature not found")
	}
	if ent.Scope != "product" || !ent.OrganisationID.Valid {
		return sqlc.Entitlement{}, apperr.NotFound("product feature not found")
	}
	return ent, nil
}

func (s *Service) UpdateProductFeature(ctx context.Context, id string, in UpdateProductFeatureInput) (sqlc.Entitlement, error) {
	existing, err := s.GetProductFeature(ctx, id)
	if err != nil {
		return sqlc.Entitlement{}, err
	}
	ent, err := s.db.Q().UpdateProductFeature(ctx, sqlc.UpdateProductFeatureParams{
		ID:             id,
		Description:    strings.TrimSpace(in.Description),
		OrganisationID: existing.OrganisationID,
	})
	if err != nil {
		return sqlc.Entitlement{}, mapNotFound(err, "product feature not found")
	}
	return ent, nil
}

func (s *Service) DeleteProductFeature(ctx context.Context, id string) error {
	existing, err := s.GetProductFeature(ctx, id)
	if err != nil {
		return err
	}
	return s.db.Q().DeleteProductFeature(ctx, sqlc.DeleteProductFeatureParams{
		ID:             id,
		OrganisationID: existing.OrganisationID,
	})
}

func (s *Service) CreateProductPlan(ctx context.Context, orgID string, in CreateProductPlanInput) (ProductPlanView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return ProductPlanView{}, err
	}
	key := strings.TrimSpace(in.Key)
	name := strings.TrimSpace(in.Name)
	if key == "" || name == "" {
		return ProductPlanView{}, apperr.Validation("key and name are required")
	}
	plan, err := s.db.Q().CreateProductPlan(ctx, sqlc.CreateProductPlanParams{
		ID:             ids.New(),
		OrganisationID: orgID,
		Key:            key,
		Name:           name,
		Status:         "active",
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			return ProductPlanView{}, apperr.Conflict("product plan key already exists")
		}
		return ProductPlanView{}, err
	}
	if in.Price != nil {
		if _, err := s.UpsertProductPlanPrice(ctx, plan.ID, *in.Price); err != nil {
			_ = s.db.Q().DeleteProductPlan(ctx, sqlc.DeleteProductPlanParams{
				ID:             plan.ID,
				OrganisationID: orgID,
			})
			return ProductPlanView{}, err
		}
	}
	return s.productPlanView(ctx, plan)
}

func (s *Service) productPlanView(ctx context.Context, plan sqlc.ProductPlan) (ProductPlanView, error) {
	keys, err := s.db.Q().ListProductPlanFeatureKeys(ctx, plan.ID)
	if err != nil {
		return ProductPlanView{}, err
	}
	prices, err := s.db.Q().ListProductPlanPricesByPlan(ctx, plan.ID)
	if err != nil {
		return ProductPlanView{}, err
	}
	return ProductPlanView{Plan: plan, FeatureKeys: keys, Prices: prices}, nil
}

func (s *Service) GetProductPlan(ctx context.Context, id string) (ProductPlanView, error) {
	plan, err := s.db.Q().GetProductPlan(ctx, id)
	if err != nil {
		return ProductPlanView{}, mapNotFound(err, "product plan not found")
	}
	return s.productPlanView(ctx, plan)
}

func (s *Service) ListProductPlans(ctx context.Context, orgID string) ([]ProductPlanView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return nil, err
	}
	plans, err := s.db.Q().ListProductPlansByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]ProductPlanView, 0, len(plans))
	for _, p := range plans {
		view, err := s.productPlanView(ctx, p)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, nil
}

func (s *Service) UpdateProductPlan(ctx context.Context, id string, in UpdateProductPlanInput) (ProductPlanView, error) {
	if _, err := s.GetProductPlan(ctx, id); err != nil {
		return ProductPlanView{}, err
	}
	params := sqlc.UpdateProductPlanParams{ID: id}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return ProductPlanView{}, apperr.Validation("name is required")
		}
		params.Name = pgtype.Text{String: name, Valid: true}
	}
	if in.Status != nil {
		status := strings.TrimSpace(*in.Status)
		switch status {
		case "active", "archived":
		default:
			return ProductPlanView{}, apperr.Validation("status must be active or archived")
		}
		params.Status = pgtype.Text{String: status, Valid: true}
	}
	plan, err := s.db.Q().UpdateProductPlan(ctx, params)
	if err != nil {
		return ProductPlanView{}, err
	}
	if in.Name != nil {
		if err := s.syncProductPlanNameToStripe(ctx, plan); err != nil {
			return ProductPlanView{}, err
		}
	}
	return s.productPlanView(ctx, plan)
}

func (s *Service) SetProductPlanFeatures(ctx context.Context, planID string, in SetProductPlanFeaturesInput) (ProductPlanView, error) {
	plan, err := s.db.Q().GetProductPlan(ctx, planID)
	if err != nil {
		return ProductPlanView{}, mapNotFound(err, "product plan not found")
	}
	keys := uniqueStrings(in.FeatureKeys)
	var rows []sqlc.ListEntitlementIDsByKeysForOrgRow
	if len(keys) > 0 {
		rows, err = s.db.Q().ListEntitlementIDsByKeysForOrg(ctx, sqlc.ListEntitlementIDsByKeysForOrgParams{
			Keys:           keys,
			OrganisationID: textArg(plan.OrganisationID),
		})
		if err != nil {
			return ProductPlanView{}, err
		}
		if len(rows) != len(keys) {
			return ProductPlanView{}, apperr.Validation("one or more feature_keys are unknown")
		}
		for _, row := range rows {
			ent, err := s.db.Q().GetEntitlement(ctx, row.ID)
			if err != nil {
				return ProductPlanView{}, err
			}
			if ent.Scope != "product" || !ent.OrganisationID.Valid || ent.OrganisationID.String != plan.OrganisationID {
				return ProductPlanView{}, apperr.Validation("product plans may only include this organisation's product features")
			}
		}
	}
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		if err := q.DeleteProductPlanFeatures(ctx, planID); err != nil {
			return err
		}
		for _, row := range rows {
			if err := q.AddProductPlanFeature(ctx, sqlc.AddProductPlanFeatureParams{
				ProductPlanID: planID,
				EntitlementID: row.ID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return ProductPlanView{}, err
	}
	return s.GetProductPlan(ctx, planID)
}

func (s *Service) DeleteProductPlan(ctx context.Context, id string) error {
	plan, err := s.db.Q().GetProductPlan(ctx, id)
	if err != nil {
		return mapNotFound(err, "product plan not found")
	}
	active, err := s.db.Q().GetOrganisationProductPlan(ctx, plan.OrganisationID)
	if err == nil && active.ProductPlanID == id {
		if err := s.db.Q().ClearOrganisationProductPlan(ctx, plan.OrganisationID); err != nil {
			return err
		}
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return s.db.Q().DeleteProductPlan(ctx, sqlc.DeleteProductPlanParams{
		ID:             id,
		OrganisationID: plan.OrganisationID,
	})
}

func (s *Service) SetActiveProductPlan(ctx context.Context, orgID, planID string) (ProductPlanView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return ProductPlanView{}, err
	}
	if strings.TrimSpace(planID) == "" {
		if err := s.db.Q().ClearOrganisationProductPlan(ctx, orgID); err != nil {
			return ProductPlanView{}, err
		}
		return ProductPlanView{}, nil
	}
	plan, err := s.db.Q().GetProductPlan(ctx, planID)
	if err != nil {
		return ProductPlanView{}, mapNotFound(err, "product plan not found")
	}
	if plan.OrganisationID != orgID {
		return ProductPlanView{}, apperr.NotFound("product plan not found")
	}
	if plan.Status != "active" {
		return ProductPlanView{}, apperr.Validation("product plan must be active")
	}
	if _, err := s.db.Q().UpsertOrganisationProductPlan(ctx, sqlc.UpsertOrganisationProductPlanParams{
		OrganisationID: orgID,
		ProductPlanID:  planID,
	}); err != nil {
		return ProductPlanView{}, err
	}
	return s.productPlanView(ctx, plan)
}

func (s *Service) GetActiveProductPlan(ctx context.Context, orgID string) (ProductPlanView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return ProductPlanView{}, err
	}
	row, err := s.db.Q().GetOrganisationProductPlan(ctx, orgID)
	if err != nil {
		return ProductPlanView{}, mapNotFound(err, "active product plan not found")
	}
	return s.GetProductPlan(ctx, row.ProductPlanID)
}

func (s *Service) activeProductPlanFeatureKeys(ctx context.Context, orgID string) ([]string, error) {
	row, err := s.db.Q().GetOrganisationProductPlan(ctx, orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.db.Q().ListProductPlanFeatureKeys(ctx, row.ProductPlanID)
}
