package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	httpserver "github.com/gsarmaonline/kyc/internal/http"
	"github.com/gsarmaonline/kyc/internal/service"
	"github.com/gsarmaonline/kyc/internal/store"
)

// serverWithPlatformAdmins builds a handler over an existing database with a
// specific PLATFORM_ADMIN_EMAILS list, so a test can change the list the way a
// redeploy would while keeping the same users and sessions.
func serverWithPlatformAdmins(t *testing.T, db *store.Store, emails ...string) http.Handler {
	t.Helper()
	return httpserver.New(db, httpserver.Options{
		Service:             service.New(db),
		APITokens:           []string{testSvcToken},
		PlatformAdminEmails: emails,
		AuthRateLimitPerMin: 0,
		AuthDevLogin:        true,
		AppOrigin:           "http://localhost:8080",
	}).Handler()
}

func meIsPlatformAdmin(t *testing.T, h http.Handler, token string) bool {
	t.Helper()
	res := doJSON(t, h, http.MethodGet, "/v1/me", nil, userAuth(token))
	if res.Code != http.StatusOK {
		t.Fatalf("/v1/me status=%d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		PlatformAdmin bool `json:"platform_admin"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode /v1/me: %v", err)
	}
	return body.PlatformAdmin
}

// Platform privilege must be derived per request from PLATFORM_ADMIN_EMAILS,
// never persisted. It used to be a one-way latch: matching the list wrote
// users.platform_admin = true and nothing ever cleared it, so removing an
// address from the list did not demote anyone and offboarding silently failed.
func TestPlatformAdminIsDerivedNotLatched(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)

	granting := serverWithPlatformAdmins(t, db, "boss@kyc.com")
	_, token := doDevLogin(t, granting, "boss@kyc.com", "Boss")

	if !meIsPlatformAdmin(t, granting, token) {
		t.Fatal("an address in PLATFORM_ADMIN_EMAILS must be platform admin")
	}

	// The same database and the same session, with the address removed from the
	// list. This is what an offboarding redeploy looks like.
	revoked := serverWithPlatformAdmins(t, db)
	if meIsPlatformAdmin(t, revoked, token) {
		t.Fatal("removing the address must demote: platform privilege is not persisted")
	}

	// Demotion must be real, not cosmetic: platform-only routes must refuse.
	denied := doJSON(t, revoked, http.MethodGet, "/v1/api-keys", nil, userAuth(token))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("demoted user on a platform route: want 403, got %d %s", denied.Code, denied.Body.String())
	}

	// And re-adding the address restores it, so the list is authoritative in
	// both directions.
	restored := serverWithPlatformAdmins(t, db, "boss@kyc.com")
	if !meIsPlatformAdmin(t, restored, token) {
		t.Fatal("re-adding the address must restore platform admin")
	}
}

// A user who never matched the list is not platform admin, and the break-glass
// service token still is. Break-glass is the root of trust and must keep
// working regardless of the email list.
func TestBreakGlassTokenIsAlwaysPlatform(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := serverWithPlatformAdmins(t, db)

	_, token := doDevLogin(t, h, "nobody@acme.com", "Nobody")
	if meIsPlatformAdmin(t, h, token) {
		t.Fatal("an ordinary user must not be platform admin")
	}
	if denied := doJSON(t, h, http.MethodGet, "/v1/api-keys", nil, userAuth(token)); denied.Code != http.StatusForbidden {
		t.Fatalf("ordinary user on a platform route: want 403, got %d", denied.Code)
	}

	if allowed := doJSON(t, h, http.MethodGet, "/v1/api-keys", nil, svcAuth()); allowed.Code != http.StatusOK {
		t.Fatalf("break-glass token must reach platform routes, got %d %s", allowed.Code, allowed.Body.String())
	}
}
