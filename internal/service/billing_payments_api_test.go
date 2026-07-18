package service_test

import (
	"context"
	"net/http"
	"testing"

	httpserver "github.com/gsarmaonline/kyc/internal/http"
	"github.com/gsarmaonline/kyc/internal/payments"
	"github.com/gsarmaonline/kyc/internal/service"
	"github.com/gsarmaonline/kyc/internal/store"
)

func testServerWithPayments(t *testing.T, db *store.Store) http.Handler {
	t.Helper()
	svc := service.New(db)
	svc.SetPayments(payments.NewNoop(), "", "", "http://localhost:8080")
	return httpserver.New(db, httpserver.Options{
		Service:             svc,
		APITokens:           []string{testSvcToken},
		AuthRateLimitPerMin: 0,
		AuthDevLogin:        true,
		AppOrigin:           "http://localhost:8080",
	}).Handler()
}

func TestBillingWebhookReconcileAndIdempotency(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServerWithPayments(t, db)

	boot, token, _ := doBootstrapOrg(t, h, "bill@example.com", "Bill", "Bill Co", "bill-co")
	orgID := boot["organisation"].(map[string]any)["id"].(string)

	res := doJSON(t, h, http.MethodPost, "/v1/plans", map[string]any{
		"key": "pro", "name": "Pro",
	}, svcAuth())
	if res.Code != http.StatusCreated {
		t.Fatalf("create plan status=%d body=%s", res.Code, res.Body.String())
	}
	var plan map[string]any
	decodeBody(t, res, &plan)
	planID := plan["id"].(string)

	priceRes := doJSON(t, h, http.MethodPut, "/v1/plans/"+planID+"/price", map[string]any{
		"interval":            "month",
		"currency":            "usd",
		"unit_amount":         2900,
		"processor_price_ref": "price_noop_pro_month",
	}, svcAuth())
	if priceRes.Code != http.StatusOK {
		t.Fatalf("upsert price status=%d body=%s", priceRes.Code, priceRes.Body.String())
	}

	payload := map[string]any{
		"id":                 "evt_test_1",
		"type":               "customer.subscription.updated",
		"organisation_id":    orgID,
		"customer_ref":       "cus_noop_" + orgID,
		"subscription_ref":   "sub_noop_1",
		"price_ref":          "price_noop_pro_month",
		"status":             "active",
		"current_period_end": "2026-08-01T00:00:00Z",
	}
	wh := doJSON(t, h, http.MethodPost, "/v1/billing/webhooks/noop", payload, nil)
	if wh.Code != http.StatusOK {
		t.Fatalf("webhook status=%d body=%s", wh.Code, wh.Body.String())
	}

	subRes := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/subscription", nil, userAuth(token))
	if subRes.Code != http.StatusOK {
		t.Fatalf("get subscription status=%d body=%s", subRes.Code, subRes.Body.String())
	}
	var sub map[string]any
	decodeBody(t, subRes, &sub)
	if sub["plan_id"] != planID {
		t.Fatalf("plan_id=%v want %s", sub["plan_id"], planID)
	}
	if sub["status"] != "active" {
		t.Fatalf("status=%v want active", sub["status"])
	}
	if sub["subscription_ref"] != "sub_noop_1" {
		t.Fatalf("subscription_ref=%v", sub["subscription_ref"])
	}

	wh2 := doJSON(t, h, http.MethodPost, "/v1/billing/webhooks/noop", payload, nil)
	if wh2.Code != http.StatusOK {
		t.Fatalf("replay webhook status=%d body=%s", wh2.Code, wh2.Body.String())
	}

	checkout := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/billing/checkout", map[string]any{
		"plan_id": planID,
	}, userAuth(token))
	if checkout.Code != http.StatusBadRequest {
		t.Fatalf("noop checkout status=%d want 400 body=%s", checkout.Code, checkout.Body.String())
	}
}

func TestBillingWebhookWrongProvider(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServerWithPayments(t, db)
	res := doJSON(t, h, http.MethodPost, "/v1/billing/webhooks/stripe", map[string]any{
		"id": "evt_x",
	}, nil)
	if res.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", res.Code, res.Body.String())
	}
}
