package service_test

import (
	"context"
	"net/http"
	"testing"
)

func TestProductFeaturesPlansAndCheck(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	signed, token, _ := doBootstrapOrg(t, h, "prod@acme.com", "Prod", "Acme Product", "acme-product")
	orgID := signed["organisation"].(map[string]any)["id"].(string)
	auth := userAuth(token)

	createFeat := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/product-features", map[string]any{
		"key": "premium_reports", "description": "Premium reports",
	}, auth)
	if createFeat.Code != http.StatusCreated {
		t.Fatalf("create feature: %s", createFeat.Body.String())
	}

	createPlan := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/product-plans", map[string]any{
		"key": "pro", "name": "Pro",
	}, auth)
	if createPlan.Code != http.StatusCreated {
		t.Fatalf("create plan: %s", createPlan.Body.String())
	}
	var plan map[string]any
	decodeBody(t, createPlan, &plan)
	planID := plan["id"].(string)

	setFeatures := doJSON(t, h, http.MethodPut, "/v1/product-plans/"+planID+"/features", map[string]any{
		"feature_keys": []string{"premium_reports"},
	}, auth)
	if setFeatures.Code != http.StatusOK {
		t.Fatalf("set features: %s", setFeatures.Body.String())
	}

	checkBefore := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "premium_reports",
	}, auth)
	var allowed map[string]any
	decodeBody(t, checkBefore, &allowed)
	if allowed["allowed"] != false {
		t.Fatalf("expected denied before activate: %#v", allowed)
	}

	activate := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/product-plan", map[string]any{
		"product_plan_id": planID,
	}, auth)
	if activate.Code != http.StatusOK {
		t.Fatalf("activate: %s", activate.Body.String())
	}

	checkAfter := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "premium_reports",
	}, auth)
	decodeBody(t, checkAfter, &allowed)
	if allowed["allowed"] != true {
		t.Fatalf("expected allowed after activate: %#v", allowed)
	}

	eff := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/entitlements", nil, auth)
	var body struct {
		ProductFeatures []string `json:"product_features"`
	}
	decodeBody(t, eff, &body)
	found := false
	for _, k := range body.ProductFeatures {
		if k == "premium_reports" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected premium_reports in product_features: %#v", body.ProductFeatures)
	}
}
