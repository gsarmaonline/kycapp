package service_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gsarmaonline/kyc/core/featureflags"
)

func TestFeatureFlagsRolloutAndOverrides(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	signed, token, _ := doBootstrapOrg(t, h, "flags@acme.com", "Flags", "Acme Flags", "acme-flags")
	orgID := signed["organisation"].(map[string]any)["id"].(string)
	auth := userAuth(token)

	create := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/feature-flags", map[string]any{
		"key": "new_checkout", "description": "New checkout", "enabled": true, "rollout_percentage": 0,
	}, auth)
	if create.Code != http.StatusCreated {
		t.Fatalf("create flag: %s", create.Body.String())
	}
	var flag map[string]any
	decodeBody(t, create, &flag)
	flagID := flag["id"].(string)

	checkOff := doJSON(t, h, http.MethodPost, "/v1/feature-flags/check", map[string]any{
		"organisation_id": orgID, "flag": "new_checkout", "subject_id": "user_1",
	}, auth)
	var result map[string]any
	decodeBody(t, checkOff, &result)
	if result["enabled"] != false || result["reason"] != featureflags.ReasonOff {
		t.Fatalf("expected off at 0%%: %#v", result)
	}

	patch := doJSON(t, h, http.MethodPatch, "/v1/feature-flags/"+flagID, map[string]any{
		"rollout_percentage": 100,
	}, auth)
	if patch.Code != http.StatusOK {
		t.Fatalf("patch: %s", patch.Body.String())
	}

	checkOn := doJSON(t, h, http.MethodPost, "/v1/feature-flags/check", map[string]any{
		"organisation_id": orgID, "flag": "new_checkout", "subject_id": "user_1",
	}, auth)
	decodeBody(t, checkOn, &result)
	if result["enabled"] != true || result["reason"] != featureflags.ReasonFull {
		t.Fatalf("expected full: %#v", result)
	}

	needSubject := doJSON(t, h, http.MethodPatch, "/v1/feature-flags/"+flagID, map[string]any{
		"rollout_percentage": 50,
	}, auth)
	if needSubject.Code != http.StatusOK {
		t.Fatalf("patch 50: %s", needSubject.Body.String())
	}
	missing := doJSON(t, h, http.MethodPost, "/v1/feature-flags/check", map[string]any{
		"organisation_id": orgID, "flag": "new_checkout",
	}, auth)
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without subject_id for partial rollout, got %d: %s", missing.Code, missing.Body.String())
	}

	overrides := doJSON(t, h, http.MethodPut, "/v1/feature-flags/"+flagID+"/overrides", map[string]any{
		"overrides": []map[string]any{
			{"subject_id": "vip_1", "effect": "include"},
			{"subject_id": "blocked_1", "effect": "exclude"},
		},
	}, auth)
	if overrides.Code != http.StatusOK {
		t.Fatalf("overrides: %s", overrides.Body.String())
	}

	vip := doJSON(t, h, http.MethodPost, "/v1/feature-flags/check", map[string]any{
		"organisation_id": orgID, "flag": "new_checkout", "subject_id": "vip_1",
	}, auth)
	decodeBody(t, vip, &result)
	if result["enabled"] != true || result["reason"] != featureflags.ReasonOverrideOn {
		t.Fatalf("vip include: %#v", result)
	}

	blocked := doJSON(t, h, http.MethodPost, "/v1/feature-flags/check", map[string]any{
		"organisation_id": orgID, "flag": "new_checkout", "subject_id": "blocked_1",
	}, auth)
	decodeBody(t, blocked, &result)
	if result["enabled"] != false || result["reason"] != featureflags.ReasonOverrideOff {
		t.Fatalf("blocked exclude: %#v", result)
	}

	kill := doJSON(t, h, http.MethodPatch, "/v1/feature-flags/"+flagID, map[string]any{
		"enabled": false,
	}, auth)
	if kill.Code != http.StatusOK {
		t.Fatalf("kill: %s", kill.Body.String())
	}
	killed := doJSON(t, h, http.MethodPost, "/v1/feature-flags/check", map[string]any{
		"organisation_id": orgID, "flag": "new_checkout", "subject_id": "vip_1",
	}, auth)
	decodeBody(t, killed, &result)
	if result["enabled"] != false || result["reason"] != featureflags.ReasonDisabled {
		t.Fatalf("kill switch should win over include: %#v", result)
	}
}
