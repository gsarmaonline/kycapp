package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/payments"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
)

type ProductPlanPriceInput struct {
	Interval   string
	Currency   string
	UnitAmount int64
	Status     string
}

type ImportStripePriceInput struct {
	PriceRef string
	Key      string
	Name     string
}

type StripeCatalogSyncResult struct {
	Imported []ProductPlanView
	Pushed   []ProductPlanView
	Skipped  int
}

func (s *Service) orgStripeCatalog(ctx context.Context, orgID string) (payments.CatalogClient, error) {
	row, err := s.db.Q().GetOrganisationIntegration(ctx, sqlc.GetOrganisationIntegrationParams{
		OrganisationID: orgID,
		Provider:       "stripe",
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.Validation("connect Stripe in organisation settings first")
	}
	if err != nil {
		return nil, err
	}
	if row.Status != "connected" || strings.TrimSpace(row.SecretKey) == "" {
		return nil, apperr.Validation("connect Stripe in organisation settings first")
	}
	return payments.NewStripeCatalog(row.SecretKey)
}

func (s *Service) maybeOrgStripeCatalog(ctx context.Context, orgID string) (payments.CatalogClient, bool, error) {
	row, err := s.db.Q().GetOrganisationIntegration(ctx, sqlc.GetOrganisationIntegrationParams{
		OrganisationID: orgID,
		Provider:       "stripe",
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if row.Status != "connected" || strings.TrimSpace(row.SecretKey) == "" {
		return nil, false, nil
	}
	client, err := payments.NewStripeCatalog(row.SecretKey)
	if err != nil {
		return nil, false, err
	}
	return client, true, nil
}

func normalizeProductPlanPriceInput(in ProductPlanPriceInput) (interval, currency, status string, unitAmount int64, err error) {
	interval = strings.TrimSpace(in.Interval)
	if interval == "" {
		interval = "month"
	}
	switch interval {
	case "month", "year":
	default:
		return "", "", "", 0, apperr.Validation("interval must be month or year")
	}
	currency = strings.ToLower(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = "usd"
	}
	if in.UnitAmount < 0 {
		return "", "", "", 0, apperr.Validation("unit_amount must be >= 0")
	}
	status = strings.TrimSpace(in.Status)
	if status == "" {
		status = "active"
	}
	switch status {
	case "active", "archived":
	default:
		return "", "", "", 0, apperr.Validation("status must be active or archived")
	}
	return interval, currency, status, in.UnitAmount, nil
}

func (s *Service) UpsertProductPlanPrice(ctx context.Context, planID string, in ProductPlanPriceInput) (sqlc.ProductPlanPrice, error) {
	plan, err := s.db.Q().GetProductPlan(ctx, planID)
	if err != nil {
		return sqlc.ProductPlanPrice{}, mapNotFound(err, "product plan not found")
	}
	interval, currency, status, unitAmount, err := normalizeProductPlanPriceInput(in)
	if err != nil {
		return sqlc.ProductPlanPrice{}, err
	}

	existing, err := s.db.Q().GetProductPlanPriceByPlanInterval(ctx, sqlc.GetProductPlanPriceByPlanIntervalParams{
		ProductPlanID: planID,
		Interval:      interval,
		Processor:     "stripe",
	})
	var existingProductRef, existingPriceRef string
	priceID := ids.New()
	if err == nil {
		priceID = existing.ID
		existingProductRef = existing.ProcessorProductRef
		existingPriceRef = existing.ProcessorPriceRef
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.ProductPlanPrice{}, err
	}

	productRef, priceRef := existingProductRef, existingPriceRef
	if catalog, ok, cerr := s.maybeOrgStripeCatalog(ctx, plan.OrganisationID); cerr != nil {
		return sqlc.ProductPlanPrice{}, cerr
	} else if ok && status == "active" {
		refs, perr := catalog.EnsureProductPrice(ctx, payments.EnsureProductPriceInput{
			Name:               plan.Name,
			Interval:           interval,
			Currency:           currency,
			UnitAmount:         unitAmount,
			ExistingProductRef: existingProductRef,
			ExistingPriceRef:   existingPriceRef,
			Metadata: map[string]string{
				"kyc_org_id":           plan.OrganisationID,
				"kyc_product_plan_id":  plan.ID,
				"kyc_product_plan_key": plan.Key,
			},
		})
		if perr != nil {
			return sqlc.ProductPlanPrice{}, fmt.Errorf("sync price to Stripe: %w", perr)
		}
		productRef, priceRef = refs.ProductRef, refs.PriceRef
	}

	row, err := s.db.Q().UpsertProductPlanPrice(ctx, sqlc.UpsertProductPlanPriceParams{
		ID:                  priceID,
		ProductPlanID:       planID,
		Interval:            interval,
		Currency:            currency,
		UnitAmount:          unitAmount,
		Processor:           "stripe",
		ProcessorProductRef: productRef,
		ProcessorPriceRef:   priceRef,
		Status:              status,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			return sqlc.ProductPlanPrice{}, apperr.Conflict("price already linked")
		}
		return sqlc.ProductPlanPrice{}, err
	}
	return row, nil
}

func (s *Service) ListProductPlanPrices(ctx context.Context, planID string) ([]sqlc.ProductPlanPrice, error) {
	if _, err := s.db.Q().GetProductPlan(ctx, planID); err != nil {
		return nil, mapNotFound(err, "product plan not found")
	}
	return s.db.Q().ListProductPlanPricesByPlan(ctx, planID)
}

func (s *Service) ListStripeCatalog(ctx context.Context, orgID string) ([]payments.CatalogItem, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return nil, err
	}
	catalog, err := s.orgStripeCatalog(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return catalog.ListCatalog(ctx)
}

func (s *Service) ImportStripeCatalog(ctx context.Context, orgID string, items []ImportStripePriceInput) (StripeCatalogSyncResult, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return StripeCatalogSyncResult{}, err
	}
	catalog, err := s.orgStripeCatalog(ctx, orgID)
	if err != nil {
		return StripeCatalogSyncResult{}, err
	}
	remote, err := catalog.ListCatalog(ctx)
	if err != nil {
		return StripeCatalogSyncResult{}, err
	}
	byPrice := make(map[string]payments.CatalogItem, len(remote))
	for _, item := range remote {
		byPrice[item.PriceRef] = item
	}

	result := StripeCatalogSyncResult{Imported: []ProductPlanView{}, Pushed: []ProductPlanView{}}
	for _, in := range items {
		priceRef := strings.TrimSpace(in.PriceRef)
		if priceRef == "" {
			return StripeCatalogSyncResult{}, apperr.Validation("price_ref is required")
		}
		remoteItem, ok := byPrice[priceRef]
		if !ok {
			return StripeCatalogSyncResult{}, apperr.Validation("unknown Stripe price_ref: " + priceRef)
		}
		if _, err := s.db.Q().GetProductPlanPriceByProcessorRef(ctx, sqlc.GetProductPlanPriceByProcessorRefParams{
			Processor:         "stripe",
			ProcessorPriceRef: priceRef,
		}); err == nil {
			result.Skipped++
			continue
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return StripeCatalogSyncResult{}, err
		}

		name := strings.TrimSpace(in.Name)
		if name == "" {
			name = remoteItem.ProductName
		}
		if name == "" {
			name = remoteItem.PriceRef
		}
		key := strings.TrimSpace(in.Key)
		if key == "" {
			key = slugifyKey(name)
		}
		plan, err := s.CreateProductPlan(ctx, orgID, CreateProductPlanInput{
			Key:  key,
			Name: name,
		})
		if err != nil {
			if errors.Is(err, apperr.ErrConflict) {
				suffix := priceRef
				if len(suffix) > 6 {
					suffix = suffix[len(suffix)-6:]
				}
				key = slugifyKey(name + "_" + suffix)
				plan, err = s.CreateProductPlan(ctx, orgID, CreateProductPlanInput{Key: key, Name: name})
			}
			if err != nil {
				return StripeCatalogSyncResult{}, err
			}
		}
		_, err = s.db.Q().UpsertProductPlanPrice(ctx, sqlc.UpsertProductPlanPriceParams{
			ID:                  ids.New(),
			ProductPlanID:       plan.Plan.ID,
			Interval:            remoteItem.Interval,
			Currency:            strings.ToLower(remoteItem.Currency),
			UnitAmount:          remoteItem.UnitAmount,
			Processor:           "stripe",
			ProcessorProductRef: remoteItem.ProductRef,
			ProcessorPriceRef:   remoteItem.PriceRef,
			Status:              "active",
		})
		if err != nil {
			return StripeCatalogSyncResult{}, err
		}
		view, err := s.GetProductPlan(ctx, plan.Plan.ID)
		if err != nil {
			return StripeCatalogSyncResult{}, err
		}
		result.Imported = append(result.Imported, view)
	}
	return result, nil
}

func (s *Service) SyncProductPlansToStripe(ctx context.Context, orgID string) (StripeCatalogSyncResult, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return StripeCatalogSyncResult{}, err
	}
	catalog, err := s.orgStripeCatalog(ctx, orgID)
	if err != nil {
		return StripeCatalogSyncResult{}, err
	}
	rows, err := s.db.Q().ListUnsyncedProductPlanPricesByOrg(ctx, sqlc.ListUnsyncedProductPlanPricesByOrgParams{
		OrganisationID: orgID,
		Processor:      "stripe",
	})
	if err != nil {
		return StripeCatalogSyncResult{}, err
	}
	result := StripeCatalogSyncResult{Imported: []ProductPlanView{}, Pushed: []ProductPlanView{}}
	seenPlans := map[string]struct{}{}
	for _, row := range rows {
		plan, err := s.db.Q().GetProductPlan(ctx, row.ProductPlanID)
		if err != nil {
			return StripeCatalogSyncResult{}, err
		}
		refs, err := catalog.EnsureProductPrice(ctx, payments.EnsureProductPriceInput{
			Name:               plan.Name,
			Interval:           row.Interval,
			Currency:           row.Currency,
			UnitAmount:         row.UnitAmount,
			ExistingProductRef: row.ProcessorProductRef,
			ExistingPriceRef:   row.ProcessorPriceRef,
			Metadata: map[string]string{
				"kyc_org_id":           plan.OrganisationID,
				"kyc_product_plan_id":  plan.ID,
				"kyc_product_plan_key": plan.Key,
			},
		})
		if err != nil {
			return StripeCatalogSyncResult{}, fmt.Errorf("push plan %s to Stripe: %w", plan.Key, err)
		}
		if _, err := s.db.Q().UpsertProductPlanPrice(ctx, sqlc.UpsertProductPlanPriceParams{
			ID:                  row.ID,
			ProductPlanID:       row.ProductPlanID,
			Interval:            row.Interval,
			Currency:            row.Currency,
			UnitAmount:          row.UnitAmount,
			Processor:           row.Processor,
			ProcessorProductRef: refs.ProductRef,
			ProcessorPriceRef:   refs.PriceRef,
			Status:              row.Status,
		}); err != nil {
			return StripeCatalogSyncResult{}, err
		}
		if _, ok := seenPlans[plan.ID]; !ok {
			seenPlans[plan.ID] = struct{}{}
			view, err := s.GetProductPlan(ctx, plan.ID)
			if err != nil {
				return StripeCatalogSyncResult{}, err
			}
			result.Pushed = append(result.Pushed, view)
		}
	}
	return result, nil
}

func (s *Service) syncProductPlanNameToStripe(ctx context.Context, plan sqlc.ProductPlan) error {
	catalog, ok, err := s.maybeOrgStripeCatalog(ctx, plan.OrganisationID)
	if err != nil || !ok {
		return err
	}
	prices, err := s.db.Q().ListProductPlanPricesByPlan(ctx, plan.ID)
	if err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for _, p := range prices {
		ref := strings.TrimSpace(p.ProcessorProductRef)
		if ref == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		if err := catalog.UpdateProductName(ctx, ref, plan.Name); err != nil {
			return err
		}
	}
	return nil
}

var nonKeyChars = regexp.MustCompile(`[^a-z0-9_]+`)

func slugifyKey(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevUnderscore = false
		case r == ' ' || r == '-' || r == '_':
			if !prevUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	out = nonKeyChars.ReplaceAllString(out, "")
	if out == "" {
		return "plan"
	}
	if len(out) > 48 {
		out = out[:48]
	}
	return out
}
