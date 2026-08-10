package service_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gsarmaonline/kyc/internal/service"
)

func TestLoginLogoutMe(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	_, token := doDevLogin(t, h, "login@acme.com", "Login")

	me := doJSON(t, h, http.MethodGet, "/v1/me", nil, userAuth(token))
	if me.Code != http.StatusOK {
		t.Fatalf("me: %s", me.Body.String())
	}

	logout := doJSON(t, h, http.MethodPost, "/v1/auth/logout", nil, userAuth(token))
	if logout.Code != http.StatusOK {
		t.Fatalf("logout: %s", logout.Body.String())
	}
	after := doJSON(t, h, http.MethodGet, "/v1/me", nil, userAuth(token))
	if after.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session want 401, got %d", after.Code)
	}

	_, token2 := doDevLogin(t, h, "login@acme.com", "Login")
	me2 := doJSON(t, h, http.MethodGet, "/v1/me", nil, userAuth(token2))
	if me2.Code != http.StatusOK {
		t.Fatalf("re-login me: %s", me2.Body.String())
	}
}

func TestGoogleLoginLinksInviteUser(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	svc := service.New(db)

	// Simulate invite-created user (no google_sub).
	user, err := svc.CreateUser(ctx, service.CreateUserInput{Email: "invited@acme.com", Name: "invited@acme.com"})
	if err != nil {
		t.Fatal(err)
	}

	auth, err := svc.LoginWithGoogle(ctx, service.GoogleIdentity{
		Sub: "google-sub-1", Email: "invited@acme.com", EmailVerified: true, Name: "Invited Person",
	})
	if err != nil {
		t.Fatal(err)
	}
	if auth.User.ID != user.ID {
		t.Fatalf("should link existing invite user")
	}
	if !auth.User.GoogleSub.Valid || auth.User.GoogleSub.String != "google-sub-1" {
		t.Fatalf("google_sub not set: %#v", auth.User.GoogleSub)
	}
}
