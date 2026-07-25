package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpserver "github.com/gsarmaonline/kyc/internal/http"
	"github.com/gsarmaonline/kyc/internal/service"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const testSvcToken = "test-service-token"

func testServer(t *testing.T, db *store.Store) http.Handler {
	t.Helper()
	h, _ := testEnv(t, db)
	return h
}

// testEnv returns an HTTP handler and the shared Service (for enqueue / mailer wiring).
func testEnv(t *testing.T, db *store.Store) (http.Handler, *service.Service) {
	t.Helper()
	svc := service.New(db)
	h := httpserver.New(db, httpserver.Options{
		Service:             svc,
		APITokens:           []string{testSvcToken},
		AuthRateLimitPerMin: 0,
		AuthDevLogin:        true,
		AppOrigin:           "http://localhost:8080",
	}).Handler()
	return h, svc
}

func userAuth(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func svcAuth() map[string]string {
	return map[string]string{"Authorization": "Bearer " + testSvcToken}
}

func doDevLogin(t *testing.T, h http.Handler, email, name string) (map[string]any, string) {
	t.Helper()
	res := doJSON(t, h, http.MethodPost, "/v1/auth/dev-login", map[string]any{
		"email": email, "name": name,
	}, nil)
	if res.Code != http.StatusOK {
		t.Fatalf("dev-login status=%d body=%s", res.Code, res.Body.String())
	}
	var out map[string]any
	decodeBody(t, res, &out)
	token, _ := out["token"].(string)
	if token == "" {
		t.Fatal("dev-login missing session token")
	}
	return out, token
}

func doBootstrapOrg(t *testing.T, h http.Handler, email, name, orgName, slug string) (map[string]any, string, string) {
	t.Helper()
	login, token := doDevLogin(t, h, email, name)
	userID := login["user"].(map[string]any)["id"].(string)
	res := doJSON(t, h, http.MethodPost, "/v1/organisations", map[string]any{
		"name": orgName, "slug": slug,
	}, userAuth(token))
	if res.Code != http.StatusCreated {
		t.Fatalf("create org status=%d body=%s", res.Code, res.Body.String())
	}
	var org map[string]any
	decodeBody(t, res, &org)
	return map[string]any{
		"user":         login["user"],
		"organisation": org,
	}, token, userID
}

func openTestDB(t *testing.T, ctx context.Context) *store.Store {
	t.Helper()
	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("kyc"),
		postgres.WithUsername("kyc"),
		postgres.WithPassword("kyc"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("postgres: %v", err)
	}
	t.Cleanup(func() {
		cctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = container.Terminate(cctx)
	})
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	db, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(db.Close)
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(dst); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
}
