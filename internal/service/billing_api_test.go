package service_test

import (
	"context"
	"net/http"
	"testing"
)

func TestEntitlementsEffectiveAndCheck(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	signed, token, _ := doBootstrapOrg(t, h, "bill@acme.com", "Bill", "Acme Billing", "acme-billing")
	orgID := signed["organisation"].(map[string]any)["id"].(string)

	checkAPI := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "api_access",
	}, userAuth(token))
	var allowed map[string]any
	decodeBody(t, checkAPI, &allowed)
	if allowed["allowed"] != true {
		t.Fatalf("api_access should be allowed on trial: %#v", allowed)
	}
	checkSSO := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "sso",
	}, userAuth(token))
	decodeBody(t, checkSSO, &allowed)
	if allowed["allowed"] != false {
		t.Fatalf("sso should be denied on trial: %#v", allowed)
	}

	put := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/entitlements", map[string]any{
		"overrides": []map[string]string{{"key": "sso", "effect": "grant"}},
	}, svcAuth())
	if put.Code != http.StatusOK {
		t.Fatalf("overrides: %s", put.Body.String())
	}
	var eff struct {
		Entitlements         []string `json:"entitlements"`
		PlatformCapabilities []string `json:"platform_capabilities"`
		ProductFeatures      []string `json:"product_features"`
	}
	decodeBody(t, put, &eff)
	hasSSO, hasAPI := false, false
	for _, k := range eff.Entitlements {
		if k == "sso" {
			hasSSO = true
		}
		if k == "api_access" {
			hasAPI = true
		}
	}
	if !hasSSO || !hasAPI {
		t.Fatalf("effective=%v", eff.Entitlements)
	}
	if len(eff.PlatformCapabilities) == 0 {
		t.Fatalf("expected platform_capabilities, got %#v", eff)
	}
	for _, k := range eff.PlatformCapabilities {
		if k == "premium_reports" {
			t.Fatalf("product feature leaked into platform_capabilities: %#v", eff.PlatformCapabilities)
		}
	}

	createProduct := doJSON(t, h, http.MethodPost, "/v1/entitlements", map[string]any{
		"key": "custom_feature", "description": "Customer product feature", "scope": "product",
	}, svcAuth())
	if createProduct.Code != http.StatusCreated {
		t.Fatalf("create product entitlement: %s", createProduct.Body.String())
	}
	grantProduct := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/entitlements", map[string]any{
		"overrides": []map[string]string{
			{"key": "sso", "effect": "grant"},
			{"key": "custom_feature", "effect": "grant"},
		},
	}, svcAuth())
	if grantProduct.Code != http.StatusOK {
		t.Fatalf("grant product feature: %s", grantProduct.Body.String())
	}
	decodeBody(t, grantProduct, &eff)
	hasProduct := false
	for _, k := range eff.ProductFeatures {
		if k == "custom_feature" {
			hasProduct = true
		}
	}
	if !hasProduct {
		t.Fatalf("expected custom_feature in product_features: %#v", eff)
	}

	decodeBody(t, doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "sso",
	}, userAuth(token)), &allowed)
	if allowed["allowed"] != true {
		t.Fatalf("sso should be granted: %#v", allowed)
	}

	doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/entitlements", map[string]any{
		"overrides": []map[string]string{
			{"key": "sso", "effect": "grant"},
			{"key": "api_access", "effect": "deny"},
		},
	}, svcAuth())
	decodeBody(t, doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "api_access",
	}, userAuth(token)), &allowed)
	if allowed["allowed"] != false {
		t.Fatalf("api_access denied override failed: %#v", allowed)
	}

	pro := doJSON(t, h, http.MethodPost, "/v1/plans", map[string]any{
		"key": "pro", "name": "Pro",
	}, svcAuth())
	if pro.Code != http.StatusCreated {
		t.Fatalf("create plan: %s", pro.Body.String())
	}
	var plan map[string]any
	decodeBody(t, pro, &plan)
	planID := plan["id"].(string)
	setEnt := doJSON(t, h, http.MethodPut, "/v1/plans/"+planID+"/entitlements", map[string]any{
		"entitlement_keys": []string{"api_access", "sso"},
	}, svcAuth())
	if setEnt.Code != http.StatusOK {
		t.Fatalf("set plan ents: %s", setEnt.Body.String())
	}
	doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/entitlements", map[string]any{
		"overrides": []any{},
	}, svcAuth())
	sub := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/subscription", map[string]any{
		"plan_id": planID, "status": "active",
	}, svcAuth())
	if sub.Code != http.StatusOK {
		t.Fatalf("subscription: %s", sub.Body.String())
	}
	decodeBody(t, doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "sso",
	}, userAuth(token)), &allowed)
	if allowed["allowed"] != true {
		t.Fatalf("pro should include sso: %#v", allowed)
	}
}
