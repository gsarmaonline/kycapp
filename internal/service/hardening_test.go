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
		Service:   svc,
		APITokens: []string{"secret-token"},
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

	// healthz stays public
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
		Service:   svc,
		APITokens: []string{"bootstrap"},
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

	// Use minted key
	list := doJSON(t, h, http.MethodGet, "/v1/organisations", nil, map[string]string{
		"Authorization": "Bearer " + raw,
	})
	if list.Code != http.StatusOK {
		t.Fatalf("minted key: %d %s", list.Code, list.Body.String())
	}

	// Mutation leaves audit trail
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
		CheckRateLimitPerMin: 2,
	}).Handler()

	signup := doJSON(t, h, http.MethodPost, "/v1/signup", map[string]any{
		"user":         map[string]string{"email": "rate@acme.com", "name": "Rate"},
		"organisation": map[string]string{"name": "Rate Org", "slug": "rate-org"},
	}, map[string]string{"Idempotency-Key": "rate-1"})
	var signed map[string]any
	decodeBody(t, signup, &signed)
	orgID := signed["organisation"].(map[string]any)["id"].(string)

	body := map[string]any{"organisation_id": orgID, "entitlement": "api_access"}
	if c := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", body, nil); c.Code != http.StatusOK {
		t.Fatalf("1st check: %d", c.Code)
	}
	if c := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", body, nil); c.Code != http.StatusOK {
		t.Fatalf("2nd check: %d", c.Code)
	}
	third := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", body, nil)
	if third.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd check want 429, got %d %s", third.Code, third.Body.String())
	}
}
