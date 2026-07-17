package service_test

import (
	"context"
	"net/http"
	"testing"

	httpserver "github.com/gsarmaonline/kyc/internal/http"
	"github.com/gsarmaonline/kyc/internal/service"
)

func TestAPIAuthRequired(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	svc := service.New(db)
	h := httpserver.New(db, httpserver.Options{
		Service:             svc,
		APITokens:           []string{"secret-token"},
		AuthRateLimitPerMin: 0,
		AuthDevLogin:        true,
	}).Handler()

	denied := doJSON(t, h, http.MethodGet, "/v1/organisations", nil, nil)
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d %s", denied.Code, denied.Body.String())
	}

	ok := doJSON(t, h, http.MethodGet, "/v1/organisations", nil, map[string]string{
		"Authorization": "Bearer secret-token",
	})
	if ok.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", ok.Code, ok.Body.String())
	}

	health := doJSON(t, h, http.MethodGet, "/healthz", nil, nil)
	if health.Code != http.StatusOK {
		t.Fatalf("healthz want 200, got %d", health.Code)
	}
}

func TestAPIKeyLifecycleAndAudit(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	svc := service.New(db)
	h := httpserver.New(db, httpserver.Options{
		Service:             svc,
		APITokens:           []string{"bootstrap"},
		AuthRateLimitPerMin: 0,
		AuthDevLogin:        true,
	}).Handler()

	auth := map[string]string{"Authorization": "Bearer bootstrap"}
	created := doJSON(t, h, http.MethodPost, "/v1/api-keys", map[string]any{
		"name": "product-a",
	}, auth)
	if created.Code != http.StatusCreated {
		t.Fatalf("create key: %s", created.Body.String())
	}
	var keyBody map[string]any
	decodeBody(t, created, &keyBody)
	raw := keyBody["token"].(string)
	keyID := keyBody["id"].(string)

	list := doJSON(t, h, http.MethodGet, "/v1/organisations", nil, map[string]string{
		"Authorization": "Bearer " + raw,
	})
	if list.Code != http.StatusOK {
		t.Fatalf("minted key: %d %s", list.Code, list.Body.String())
	}

	doJSON(t, h, http.MethodPost, "/v1/organisations", map[string]any{
		"name": "Audited Org",
		"slug": "audited-org",
	}, auth)

	events := doJSON(t, h, http.MethodGet, "/v1/audit-events", nil, auth)
	if events.Code != http.StatusOK {
		t.Fatalf("audit: %s", events.Body.String())
	}
	var audit struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, events, &audit)
	if len(audit.Items) == 0 {
		t.Fatal("expected audit events")
	}

	rev := doJSON(t, h, http.MethodDelete, "/v1/api-keys/"+keyID, nil, auth)
	if rev.Code != http.StatusOK {
		t.Fatalf("revoke: %s", rev.Body.String())
	}
	denied := doJSON(t, h, http.MethodGet, "/v1/organisations", nil, map[string]string{
		"Authorization": "Bearer " + raw,
	})
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("revoked key should 401, got %d", denied.Code)
	}
}

func TestCheckRateLimit(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	svc := service.New(db)
	h := httpserver.New(db, httpserver.Options{
		Service:              svc,
		APITokens:            []string{testSvcToken},
		CheckRateLimitPerMin: 2,
		AuthRateLimitPerMin:  0,
		AuthDevLogin:         true,
	}).Handler()

	signed, token, _ := doBootstrapOrg(t, h, "rate@acme.com", "Rate", "Rate Org", "rate-org")
	orgID := signed["organisation"].(map[string]any)["id"].(string)

	body := map[string]any{"organisation_id": orgID, "entitlement": "api_access"}
	auth := userAuth(token)
	if c := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", body, auth); c.Code != http.StatusOK {
		t.Fatalf("1st check: %d %s", c.Code, c.Body.String())
	}
	if c := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", body, auth); c.Code != http.StatusOK {
		t.Fatalf("2nd check: %d", c.Code)
	}
	third := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", body, auth)
	if third.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd check want 429, got %d %s", third.Code, third.Body.String())
	}
}

func TestMerchantCannotManagePlatformCatalog(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)
	_, token, _ := doBootstrapOrg(t, h, "merchant@acme.com", "Merchant", "Merchant Co", "merchant-co")

	denied := doJSON(t, h, http.MethodPost, "/v1/plans", map[string]any{
		"key": "evil", "name": "Evil",
	}, userAuth(token))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d %s", denied.Code, denied.Body.String())
	}

	keys := doJSON(t, h, http.MethodGet, "/v1/api-keys", nil, userAuth(token))
	if keys.Code != http.StatusForbidden {
		t.Fatalf("api-keys want 403, got %d", keys.Code)
	}
}
