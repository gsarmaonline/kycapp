package httpserver

import (
	"net/http"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

func productFeatureJSON(v service.ProductFeatureView) map[string]any {
	e := v.Feature
	overrides := make([]map[string]any, 0, len(v.Overrides))
	for _, o := range v.Overrides {
		overrides = append(overrides, map[string]any{
			"subject_id": o.SubjectID,
			"effect":     o.Effect,
		})
	}
	out := map[string]any{
		"id":                 e.ID,
		"key":                e.Key,
		"description":        e.Description,
		"scope":              e.Scope,
		"enabled":            e.Enabled,
		"rollout_percentage": e.RolloutPercentage,
		"overrides":          overrides,
	}
	if e.OrganisationID.Valid {
		out["organisation_id"] = e.OrganisationID.String
	}
	return out
}

func productPlanPriceJSON(p sqlc.ProductPlanPrice) map[string]any {
	return map[string]any{
		"id":                    p.ID,
		"product_plan_id":       p.ProductPlanID,
		"interval":              p.Interval,
		"currency":              p.Currency,
		"unit_amount":           p.UnitAmount,
		"processor":             p.Processor,
		"processor_product_ref": p.ProcessorProductRef,
		"processor_price_ref":   p.ProcessorPriceRef,
		"status":                p.Status,
		"synced":                p.ProcessorPriceRef != "",
	}
}

func productPlanJSON(v service.ProductPlanView) map[string]any {
	prices := make([]map[string]any, 0, len(v.Prices))
	for _, p := range v.Prices {
		prices = append(prices, productPlanPriceJSON(p))
	}
	return map[string]any{
		"id":              v.Plan.ID,
		"organisation_id": v.Plan.OrganisationID,
		"key":             v.Plan.Key,
		"name":            v.Plan.Name,
		"status":          v.Plan.Status,
		"feature_keys":    v.FeatureKeys,
		"prices":          prices,
		"created_at":      v.Plan.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":      v.Plan.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func (s *Server) handleCreateProductFeature(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		Key               string `json:"key"`
		Description       string `json:"description"`
		Enabled           *bool  `json:"enabled"`
		RolloutPercentage *int32 `json:"rollout_percentage"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	view, err := s.svc.CreateProductFeature(r.Context(), orgID, service.CreateProductFeatureInput{
		Key:               body.Key,
		Description:       body.Description,
		Enabled:           body.Enabled,
		RolloutPercentage: body.RolloutPercentage,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, productFeatureJSON(view))
}

func (s *Server) handleListProductFeatures(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	items, err := s.svc.ListProductFeatures(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, v := range items {
		out = append(out, productFeatureJSON(v))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleGetProductFeature(w http.ResponseWriter, r *http.Request) {
	view, err := s.svc.GetProductFeature(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), view.Feature.OrganisationID.String, "product_features:read"); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, productFeatureJSON(view))
}

func (s *Server) handlePatchProductFeature(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetProductFeature(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.Feature.OrganisationID.String, "product_features:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Description       *string `json:"description"`
		Enabled           *bool   `json:"enabled"`
		RolloutPercentage *int32  `json:"rollout_percentage"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	view, err := s.svc.UpdateProductFeature(r.Context(), r.PathValue("id"), service.UpdateProductFeatureInput{
		Description:       body.Description,
		Enabled:           body.Enabled,
		RolloutPercentage: body.RolloutPercentage,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, productFeatureJSON(view))
}

func (s *Server) handleSetProductFeatureOverrides(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetProductFeature(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.Feature.OrganisationID.String, "product_features:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Overrides []struct {
			SubjectID string `json:"subject_id"`
			Effect    string `json:"effect"`
		} `json:"overrides"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	overrides := make([]service.ProductFeatureOverrideInput, 0, len(body.Overrides))
	for _, o := range body.Overrides {
		overrides = append(overrides, service.ProductFeatureOverrideInput{
			SubjectID: o.SubjectID,
			Effect:    o.Effect,
		})
	}
	view, err := s.svc.SetProductFeatureOverrides(r.Context(), r.PathValue("id"), service.SetProductFeatureOverridesInput{
		Overrides: overrides,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, productFeatureJSON(view))
}

func (s *Server) handleDeleteProductFeature(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetProductFeature(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.Feature.OrganisationID.String, "product_features:manage"); err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteProductFeature(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleCreateProductPlan(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		Key   string `json:"key"`
		Name  string `json:"name"`
		Price *struct {
			Interval   string `json:"interval"`
			Currency   string `json:"currency"`
			UnitAmount int64  `json:"unit_amount"`
			Status     string `json:"status"`
		} `json:"price"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	in := service.CreateProductPlanInput{Key: body.Key, Name: body.Name}
	if body.Price != nil {
		in.Price = &service.ProductPlanPriceInput{
			Interval:   body.Price.Interval,
			Currency:   body.Price.Currency,
			UnitAmount: body.Price.UnitAmount,
			Status:     body.Price.Status,
		}
	}
	plan, err := s.svc.CreateProductPlan(r.Context(), orgID, in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, productPlanJSON(plan))
}

func (s *Server) handleListProductPlans(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	items, err := s.svc.ListProductPlans(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, p := range items {
		out = append(out, productPlanJSON(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleGetProductPlan(w http.ResponseWriter, r *http.Request) {
	plan, err := s.svc.GetProductPlan(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), plan.Plan.OrganisationID, "product_features:read"); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, productPlanJSON(plan))
}

func (s *Server) handlePatchProductPlan(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetProductPlan(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.Plan.OrganisationID, "product_features:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name   *string `json:"name"`
		Status *string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	plan, err := s.svc.UpdateProductPlan(r.Context(), r.PathValue("id"), service.UpdateProductPlanInput{
		Name: body.Name, Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, productPlanJSON(plan))
}

func (s *Server) handleSetProductPlanFeatures(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetProductPlan(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.Plan.OrganisationID, "product_features:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		FeatureKeys []string `json:"feature_keys"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	plan, err := s.svc.SetProductPlanFeatures(r.Context(), r.PathValue("id"), service.SetProductPlanFeaturesInput{
		FeatureKeys: body.FeatureKeys,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, productPlanJSON(plan))
}

func (s *Server) handleUpsertProductPlanPrice(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetProductPlan(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.Plan.OrganisationID, "product_features:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Interval   string `json:"interval"`
		Currency   string `json:"currency"`
		UnitAmount int64  `json:"unit_amount"`
		Status     string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	price, err := s.svc.UpsertProductPlanPrice(r.Context(), r.PathValue("id"), service.ProductPlanPriceInput{
		Interval: body.Interval, Currency: body.Currency, UnitAmount: body.UnitAmount, Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, productPlanPriceJSON(price))
}

func (s *Server) handleListProductPlanPrices(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetProductPlan(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.Plan.OrganisationID, "product_features:read"); err != nil {
		writeError(w, err)
		return
	}
	prices, err := s.svc.ListProductPlanPrices(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(prices))
	for _, p := range prices {
		out = append(out, productPlanPriceJSON(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleDeleteProductPlan(w http.ResponseWriter, r *http.Request) {
	existing, err := s.svc.GetProductPlan(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.svc.RequireOrgPermission(r.Context(), existing.Plan.OrganisationID, "product_features:manage"); err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteProductPlan(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleSetActiveProductPlan(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	var body struct {
		ProductPlanID string `json:"product_plan_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	plan, err := s.svc.SetActiveProductPlan(r.Context(), orgID, body.ProductPlanID)
	if err != nil {
		writeError(w, err)
		return
	}
	if plan.Plan.ID == "" {
		writeJSON(w, http.StatusOK, map[string]any{"product_plan": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"product_plan": productPlanJSON(plan)})
}

func (s *Server) handleGetActiveProductPlan(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	plan, err := s.svc.GetActiveProductPlan(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, productPlanJSON(plan))
}
