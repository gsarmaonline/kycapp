package service_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gsarmaonline/kyc/core/featureflags"
)

func TestProductFeatureRolloutAndOverrides(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	signed, token, _ := doBootstrapOrg(t, h, "rollout@acme.com", "Rollout", "Acme Rollout", "acme-rollout")
	orgID := signed["organisation"].(map[string]any)["id"].(string)
	auth := userAuth(token)

	createFeat := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/product-features", map[string]any{
		"key": "new_checkout", "description": "New checkout", "enabled": true, "rollout_percentage": 0,
	}, auth)
	if createFeat.Code != http.StatusCreated {
		t.Fatalf("create feature: %s", createFeat.Body.String())
	}
	var feat map[string]any
	decodeBody(t, createFeat, &feat)
	featID := feat["id"].(string)

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
		"feature_keys": []string{"new_checkout"},
	}, auth)
	if setFeatures.Code != http.StatusOK {
		t.Fatalf("set features: %s", setFeatures.Body.String())
	}

	activate := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/product-plan", map[string]any{
		"product_plan_id": planID,
	}, auth)
	if activate.Code != http.StatusOK {
		t.Fatalf("activate: %s", activate.Body.String())
	}

	checkOff := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "new_checkout", "subject_id": "user_1",
	}, auth)
	var result map[string]any
	decodeBody(t, checkOff, &result)
	if result["allowed"] != false || result["reason"] != featureflags.ReasonOff {
		t.Fatalf("expected off at 0%%: %#v", result)
	}

	patch := doJSON(t, h, http.MethodPatch, "/v1/product-features/"+featID, map[string]any{
		"rollout_percentage": 100,
	}, auth)
	if patch.Code != http.StatusOK {
		t.Fatalf("patch: %s", patch.Body.String())
	}

	checkOn := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "new_checkout", "subject_id": "user_1",
	}, auth)
	decodeBody(t, checkOn, &result)
	if result["allowed"] != true || result["reason"] != featureflags.ReasonFull {
		t.Fatalf("expected full: %#v", result)
	}

	needSubject := doJSON(t, h, http.MethodPatch, "/v1/product-features/"+featID, map[string]any{
		"rollout_percentage": 50,
	}, auth)
	if needSubject.Code != http.StatusOK {
		t.Fatalf("patch 50: %s", needSubject.Body.String())
	}
	missing := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "new_checkout",
	}, auth)
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without subject_id for partial rollout, got %d: %s", missing.Code, missing.Body.String())
	}

	overrides := doJSON(t, h, http.MethodPut, "/v1/product-features/"+featID+"/overrides", map[string]any{
		"overrides": []map[string]any{
			{"subject_id": "vip_1", "effect": "include"},
			{"subject_id": "blocked_1", "effect": "exclude"},
		},
	}, auth)
	if overrides.Code != http.StatusOK {
		t.Fatalf("overrides: %s", overrides.Body.String())
	}

	vip := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "new_checkout", "subject_id": "vip_1",
	}, auth)
	decodeBody(t, vip, &result)
	if result["allowed"] != true || result["reason"] != featureflags.ReasonOverrideOn {
		t.Fatalf("vip include: %#v", result)
	}

	blocked := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "new_checkout", "subject_id": "blocked_1",
	}, auth)
	decodeBody(t, blocked, &result)
	if result["allowed"] != false || result["reason"] != featureflags.ReasonOverrideOff {
		t.Fatalf("blocked exclude: %#v", result)
	}

	kill := doJSON(t, h, http.MethodPatch, "/v1/product-features/"+featID, map[string]any{
		"enabled": false,
	}, auth)
	if kill.Code != http.StatusOK {
		t.Fatalf("kill: %s", kill.Body.String())
	}
	killed := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "new_checkout", "subject_id": "vip_1",
	}, auth)
	decodeBody(t, killed, &result)
	if result["allowed"] != false || result["reason"] != featureflags.ReasonDisabled {
		t.Fatalf("kill switch should win over include: %#v", result)
	}
}
