package httpserver

import (
	"io"
	"net/http"
	"strings"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

func (s *Server) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	plan, err := s.svc.CreatePlan(r.Context(), service.CreatePlanInput{Key: body.Key, Name: body.Name})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, planJSON(service.PlanView{
		Plan:                   plan,
		EntitlementKeys:        []string{},
		PlatformCapabilityKeys: []string{},
		ProductFeatureKeys:     []string{},
	}))
}

func (s *Server) handleListPlans(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePrincipal(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	plans, err := s.svc.ListPlans(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(plans))
	for _, p := range plans {
		items = append(items, planJSON(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePrincipal(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	plan, err := s.svc.GetPlan(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, planJSON(plan))
}

func (s *Server) handleSetPlanEntitlements(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		EntitlementKeys []string `json:"entitlement_keys"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	plan, err := s.svc.SetPlanEntitlements(r.Context(), r.PathValue("id"), service.SetPlanEntitlementsInput{
		EntitlementKeys: body.EntitlementKeys,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, planJSON(plan))
}

func (s *Server) handleCreateEntitlement(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Key         string `json:"key"`
		Description string `json:"description"`
		Scope       string `json:"scope"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	ent, err := s.svc.CreateEntitlement(r.Context(), service.CreateEntitlementInput{
		Key: body.Key, Description: body.Description, Scope: body.Scope,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, entitlementJSON(ent))
}

func (s *Server) handleListEntitlementsCatalog(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePrincipal(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	ents, err := s.svc.ListEntitlements(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(ents))
	for _, e := range ents {
		items = append(items, entitlementJSON(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleUpsertSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	// Assigning plans is platform-only until Stripe self-serve exists.
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	_ = orgID
	var body struct {
		PlanID string `json:"plan_id"`
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	sub, err := s.svc.UpsertSubscription(r.Context(), r.PathValue("id"), service.UpsertSubscriptionInput{
		PlanID: body.PlanID, Status: body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, subscriptionJSON(sub))
}

func (s *Server) handleGetSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "billing:read"); err != nil {
		writeError(w, err)
		return
	}
	sub, err := s.svc.GetSubscription(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, subscriptionJSON(sub))
}

func (s *Server) handleSetOrgEntitlements(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Overrides []struct {
			Key    string `json:"key"`
			Effect string `json:"effect"`
		} `json:"overrides"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	overrides := make([]service.EntitlementOverride, 0, len(body.Overrides))
	for _, o := range body.Overrides {
		overrides = append(overrides, service.EntitlementOverride{Key: o.Key, Effect: o.Effect})
	}
	if _, err := s.svc.SetOrganisationEntitlements(r.Context(), r.PathValue("id"), service.SetOrganisationEntitlementsInput{
		Overrides: overrides,
	}); err != nil {
		writeError(w, err)
		return
	}
	view, err := s.svc.EffectiveEntitlementsView(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, effectiveEntitlementsJSON(view))
}

func (s *Server) handleGetOrgEntitlements(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "billing:read"); err != nil {
		writeError(w, err)
		return
	}
	view, err := s.svc.EffectiveEntitlementsView(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, effectiveEntitlementsJSON(view))
}

func (s *Server) handleEntitlementsCheck(w http.ResponseWriter, r *http.Request) {
	p, err := service.RequirePrincipal(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		OrganisationID string `json:"organisation_id"`
		Entitlement    string `json:"entitlement"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	if !p.IsPlatform() {
		if _, err := s.svc.RequireOrgMember(r.Context(), body.OrganisationID); err != nil {
			writeError(w, err)
			return
		}
	}
	allowed, err := s.svc.CheckEntitlement(r.Context(), body.OrganisationID, body.Entitlement)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"allowed": allowed})
}

func (s *Server) handleUpsertPlanPrice(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePlatform(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Interval          string `json:"interval"`
		Currency          string `json:"currency"`
		UnitAmount        int64  `json:"unit_amount"`
		ProcessorPriceRef string `json:"processor_price_ref"`
		Status            string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	price, err := s.svc.UpsertPlanPrice(r.Context(), r.PathValue("id"), service.UpsertPlanPriceInput{
		Interval:          body.Interval,
		Currency:          body.Currency,
		UnitAmount:        body.UnitAmount,
		ProcessorPriceRef: body.ProcessorPriceRef,
		Status:            body.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, planPriceJSON(price))
}

func (s *Server) handleListPlanPrices(w http.ResponseWriter, r *http.Request) {
	if _, err := service.RequirePrincipal(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	prices, err := s.svc.ListPlanPrices(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(prices))
	for _, p := range prices {
		items = append(items, planPriceJSON(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleBillingCheckout(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "billing:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		PlanID     string `json:"plan_id"`
		Interval   string `json:"interval"`
		SuccessURL string `json:"success_url"`
		CancelURL  string `json:"cancel_url"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	out, err := s.svc.CreateBillingCheckout(r.Context(), orgID, service.CreateCheckoutInput{
		PlanID:     body.PlanID,
		Interval:   body.Interval,
		SuccessURL: body.SuccessURL,
		CancelURL:  body.CancelURL,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": out.URL})
}

func (s *Server) handleBillingPortal(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "billing:manage"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		ReturnURL string `json:"return_url"`
	}
	_ = decodeJSON(r, &body) // body optional
	out, err := s.svc.CreateBillingPortal(r.Context(), orgID, service.CreatePortalInput{
		ReturnURL: body.ReturnURL,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": out.URL})
}

func (s *Server) handleBillingWebhook(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(strings.TrimSpace(r.PathValue("provider")))
	if provider == "" || provider != s.svc.PaymentsProvider() {
		writeError(w, apperr.NotFound("webhook provider not configured"))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, apperr.Validation("could not read body"))
		return
	}
	if err := s.svc.HandlePaymentWebhook(r.Context(), r.Header, body); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"received": true})
}
