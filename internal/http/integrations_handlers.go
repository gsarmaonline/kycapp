package httpserver

import (
	"net/http"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
)

func integrationJSON(v service.IntegrationView) map[string]any {
	return map[string]any{
		"provider":        v.Provider,
		"status":          v.Status,
		"secret_hint":     v.SecretHint,
		"public_key_hint": v.PublicKeyHint,
		"has_secret":      v.HasSecret,
		"has_public_key":  v.HasPublicKey,
	}
}

func (s *Server) handleListOrgIntegrations(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	items, err := s.svc.ListOrganisationIntegrations(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, integrationJSON(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleUpsertStripeIntegration(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		SecretKey      string `json:"secret_key"`
		PublishableKey string `json:"publishable_key"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	view, err := s.svc.UpsertStripeIntegration(r.Context(), orgID, service.UpsertStripeIntegrationInput{
		SecretKey: body.SecretKey, PublishableKey: body.PublishableKey,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, integrationJSON(view))
}

func (s *Server) handleListStripeCatalog(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	items, err := s.svc.ListStripeCatalog(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"product_ref":  item.ProductRef,
			"product_name": item.ProductName,
			"price_ref":    item.PriceRef,
			"interval":     item.Interval,
			"currency":     item.Currency,
			"unit_amount":  item.UnitAmount,
			"active":       item.Active,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) handleImportStripeCatalog(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Items []struct {
			PriceRef string `json:"price_ref"`
			Key      string `json:"key"`
			Name     string `json:"name"`
		} `json:"items"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, apperr.Validation("invalid JSON body"))
		return
	}
	inputs := make([]service.ImportStripePriceInput, 0, len(body.Items))
	for _, item := range body.Items {
		inputs = append(inputs, service.ImportStripePriceInput{
			PriceRef: item.PriceRef, Key: item.Key, Name: item.Name,
		})
	}
	result, err := s.svc.ImportStripeCatalog(r.Context(), orgID, inputs)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stripeCatalogSyncJSON(result))
}

func (s *Server) handleSyncProductPlansToStripe(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	result, err := s.svc.SyncProductPlansToStripe(r.Context(), orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stripeCatalogSyncJSON(result))
}

func stripeCatalogSyncJSON(result service.StripeCatalogSyncResult) map[string]any {
	imported := make([]map[string]any, 0, len(result.Imported))
	for _, p := range result.Imported {
		imported = append(imported, productPlanJSON(p))
	}
	pushed := make([]map[string]any, 0, len(result.Pushed))
	for _, p := range result.Pushed {
		pushed = append(pushed, productPlanJSON(p))
	}
	return map[string]any{
		"imported": imported,
		"pushed":   pushed,
		"skipped":  result.Skipped,
	}
}

func (s *Server) handleDeleteOrgIntegration(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	if _, err := s.svc.RequireOrgPermission(r.Context(), orgID, "organisation:update"); err != nil {
		writeError(w, err)
		return
	}
	if err := s.svc.DeleteOrganisationIntegration(r.Context(), orgID, r.PathValue("provider")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
