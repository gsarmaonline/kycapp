package service_test

import (
	"context"
	"net/http"
	"testing"
)

func TestOrganisationOnboardingDeriveAndDismiss(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	signed, token, _ := doBootstrapOrg(t, h, "onboard@acme.com", "Owner", "Onboard Co", "onboard-co")
	orgID := signed["organisation"].(map[string]any)["id"].(string)
	auth := userAuth(token)

	initial := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/onboarding", nil, auth)
	if initial.Code != http.StatusOK {
		t.Fatalf("get onboarding: %s", initial.Body.String())
	}
	var view map[string]any
	decodeBody(t, initial, &view)
	if view["visible"] != true {
		t.Fatalf("expected visible for owner: %#v", view)
	}
	if int(view["completed_count"].(float64)) != 0 {
		t.Fatalf("new org should have 0 complete: %#v", view)
	}
	if int(view["total_count"].(float64)) != 6 {
		t.Fatalf("total_count: %#v", view["total_count"])
	}
	steps, _ := view["steps"].([]any)
	if len(steps) != 6 {
		t.Fatalf("steps: %#v", steps)
	}
	for _, raw := range steps {
		step := raw.(map[string]any)
		if step["done"] == true {
			t.Fatalf("expected all incomplete: %#v", step)
		}
	}

	brand := doJSON(t, h, http.MethodPatch, "/v1/organisations/"+orgID, map[string]any{
		"email_footer": "© Onboard Co",
	}, auth)
	if brand.Code != http.StatusOK {
		t.Fatalf("branding: %s", brand.Body.String())
	}
	feat := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/product-features", map[string]any{
		"key": "reports", "description": "Reports",
	}, auth)
	if feat.Code != http.StatusCreated {
		t.Fatalf("feature: %s", feat.Body.String())
	}
	plan := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/product-plans", map[string]any{
		"key": "pro", "name": "Pro",
	}, auth)
	if plan.Code != http.StatusCreated {
		t.Fatalf("plan: %s", plan.Body.String())
	}
	auto := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/automations", map[string]any{
		"name": "Welcome", "trigger": "app_user.created",
		"actions": []map[string]any{
			{"type": "send_email", "params": map[string]any{"template_key": "welcome"}},
		},
	}, auth)
	if auto.Code != http.StatusCreated {
		t.Fatalf("automation: %s", auto.Body.String())
	}
	key := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/api-keys", map[string]any{
		"name": "backend",
	}, auth)
	if key.Code != http.StatusCreated {
		t.Fatalf("api key: %s", key.Body.String())
	}
	user := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/app-users", map[string]any{
		"email": "pat@onboard.test", "display_name": "Pat",
	}, auth)
	if user.Code != http.StatusCreated {
		t.Fatalf("app user: %s", user.Body.String())
	}

	done := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/onboarding", nil, auth)
	decodeBody(t, done, &view)
	if int(view["completed_count"].(float64)) != 6 {
		t.Fatalf("expected all complete: %#v", view)
	}
	if view["visible"] != false {
		t.Fatalf("all done should hide panel: %#v", view)
	}

	// Fresh org for dismiss path
	signed2, token2, _ := doBootstrapOrg(t, h, "dismiss@acme.com", "Owner", "Dismiss Co", "dismiss-co")
	org2 := signed2["organisation"].(map[string]any)["id"].(string)
	auth2 := userAuth(token2)
	dismiss := doJSON(t, h, http.MethodPatch, "/v1/organisations/"+org2+"/onboarding", map[string]any{
		"dismissed": true,
	}, auth2)
	if dismiss.Code != http.StatusOK {
		t.Fatalf("dismiss: %s", dismiss.Body.String())
	}
	decodeBody(t, dismiss, &view)
	if view["dismissed"] != true || view["visible"] != false {
		t.Fatalf("dismissed view: %#v", view)
	}
}

func TestOrganisationOnboardingHiddenForMember(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	signed, ownerToken, _ := doBootstrapOrg(t, h, "owner-ob@acme.com", "Owner", "Member Hide", "member-hide")
	orgID := signed["organisation"].(map[string]any)["id"].(string)

	rolesRes := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/roles", nil, userAuth(ownerToken))
	var roles struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, rolesRes, &roles)
	var memberRoleID string
	for _, r := range roles.Items {
		if r["key"] == "member" {
			memberRoleID = r["id"].(string)
		}
	}
	if memberRoleID == "" {
		t.Fatal("member role missing")
	}

	inv := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/memberships", map[string]any{
		"email": "member-ob@acme.com", "role_id": memberRoleID,
	}, userAuth(ownerToken))
	if inv.Code != http.StatusCreated {
		t.Fatalf("invite: %s", inv.Body.String())
	}
	var membership map[string]any
	decodeBody(t, inv, &membership)

	_, mateToken := doDevLogin(t, h, "member-ob@acme.com", "Member")
	acc := doJSON(t, h, http.MethodPost, "/v1/memberships/"+membership["id"].(string)+"/accept", nil, userAuth(mateToken))
	if acc.Code != http.StatusOK {
		t.Fatalf("accept: %s", acc.Body.String())
	}

	got := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/onboarding", nil, userAuth(mateToken))
	if got.Code != http.StatusOK {
		t.Fatalf("member get: %s", got.Body.String())
	}
	var view map[string]any
	decodeBody(t, got, &view)
	if view["visible"] != false {
		t.Fatalf("member should not see checklist: %#v", view)
	}
	steps, _ := view["steps"].([]any)
	if len(steps) != 0 {
		t.Fatalf("member steps should be empty: %#v", steps)
	}

	denied := doJSON(t, h, http.MethodPatch, "/v1/organisations/"+orgID+"/onboarding", map[string]any{
		"dismissed": true,
	}, userAuth(mateToken))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("member dismiss want 403, got %d %s", denied.Code, denied.Body.String())
	}
}
