package service_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gsarmaonline/kyc/internal/observability"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func openTestObsStore(t *testing.T, ctx context.Context) observability.Store {
	t.Helper()
	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("kyc_obs"),
		postgres.WithUsername("kyc"),
		postgres.WithPassword("kyc"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("obs postgres: %v", err)
	}
	t.Cleanup(func() {
		cctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = container.Terminate(cctx)
	})
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("obs dsn: %v", err)
	}
	obs, err := observability.OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatalf("obs open: %v", err)
	}
	t.Cleanup(obs.Close)
	return obs
}

func TestOrganisationActivityAndUsage(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	obs := openTestObsStore(t, ctx)

	h, svc := testEnv(t, db)
	svc.SetObservability(obs)

	boot, token, _ := doBootstrapOrg(t, h, "obs@example.com", "Obs User", "Obs Org", "obs-org")
	auth := userAuth(token)
	orgID := boot["organisation"].(map[string]any)["id"].(string)

	activity := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/activity", nil, auth)
	if activity.Code != http.StatusOK {
		t.Fatalf("activity status=%d body=%s", activity.Code, activity.Body.String())
	}
	var actOut struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, activity, &actOut)
	foundOrgCreated := false
	for _, item := range actOut.Items {
		if item["action"] == observability.ActionOrgCreated {
			foundOrgCreated = true
			break
		}
	}
	if !foundOrgCreated {
		t.Fatalf("expected org.created in activity: %#v", actOut.Items)
	}

	check := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID,
		"entitlement":     "api_access",
	}, auth)
	if check.Code != http.StatusOK {
		t.Fatalf("entitlements check status=%d body=%s", check.Code, check.Body.String())
	}

	usage := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/usage", nil, auth)
	if usage.Code != http.StatusOK {
		t.Fatalf("usage status=%d body=%s", usage.Code, usage.Body.String())
	}
	var usageOut struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, usage, &usageOut)
	if len(usageOut.Items) == 0 {
		t.Fatal("expected usage counters after entitlement check")
	}
	foundMeter := false
	for _, item := range usageOut.Items {
		if item["meter_key"] == observability.MeterEntitlementCheck {
			foundMeter = true
			break
		}
	}
	if !foundMeter {
		t.Fatalf("expected entitlement.check meter: %#v", usageOut.Items)
	}

	var n int
	if err := db.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'activity_events'
	`).Scan(&n); err != nil {
		t.Fatalf("primary schema probe: %v", err)
	}
	if n != 0 {
		t.Fatalf("activity_events should not exist on primary DB")
	}
}
