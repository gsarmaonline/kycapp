package service_test

import (
	"context"
	"net/http"
	"testing"
)

func TestOrgSettingsStripeAndAPIKeys(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	signed, token, _ := doBootstrapOrg(t, h, "settings@acme.com", "Settings", "Acme Settings", "acme-settings")
	orgID := signed["organisation"].(map[string]any)["id"].(string)
	auth := userAuth(token)

	stripe := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/integrations/stripe", map[string]any{
		"secret_key": "sk_test_1234567890abcdef", "publishable_key": "pk_test_abcdef1234567890",
	}, auth)
	if stripe.Code != http.StatusOK {
		t.Fatalf("stripe: %s", stripe.Body.String())
	}
	var integ map[string]any
	decodeBody(t, stripe, &integ)
	if integ["has_secret"] != true {
		t.Fatalf("expected has_secret: %#v", integ)
	}
	if integ["secret_hint"] == "sk_test_1234567890abcdef" {
		t.Fatal("secret must be masked")
	}

	createKey := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/api-keys", map[string]any{
		"name": "backend",
	}, auth)
	if createKey.Code != http.StatusCreated {
		t.Fatalf("create key: %s", createKey.Body.String())
	}
	var keyBody map[string]any
	decodeBody(t, createKey, &keyBody)
	raw := keyBody["token"].(string)

	check := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/entitlements", nil, map[string]string{
		"Authorization": "Bearer " + raw,
	})
	if check.Code != http.StatusOK {
		t.Fatalf("org api key should access org: %d %s", check.Code, check.Body.String())
	}

	plans := doJSON(t, h, http.MethodPost, "/v1/plans", map[string]any{
		"key": "x", "name": "X",
	}, map[string]string{"Authorization": "Bearer " + raw})
	if plans.Code != http.StatusForbidden {
		t.Fatalf("org api key must not be platform: %d %s", plans.Code, plans.Body.String())
	}

	arch := doJSON(t, h, http.MethodDelete, "/v1/organisations/"+orgID, nil, auth)
	if arch.Code != http.StatusOK {
		t.Fatalf("delete: %s", arch.Body.String())
	}

	listed := doJSON(t, h, http.MethodGet, "/v1/organisations", nil, auth)
	if listed.Code != http.StatusOK {
		t.Fatalf("list after delete: %s", listed.Body.String())
	}
	var listBody map[string]any
	decodeBody(t, listed, &listBody)
	items, _ := listBody["items"].([]any)
	for _, it := range items {
		row, _ := it.(map[string]any)
		if row["id"] == orgID {
			t.Fatalf("deleted org still listed: %#v", row)
		}
	}

	users := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/app-users", nil, auth)
	if users.Code != http.StatusNotFound {
		t.Fatalf("deleted org access want 404 got %d %s", users.Code, users.Body.String())
	}

	get := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID, nil, auth)
	if get.Code != http.StatusNotFound && get.Code != http.StatusForbidden {
		t.Fatalf("get deleted org want 404/403 got %d %s", get.Code, get.Body.String())
	}
}
