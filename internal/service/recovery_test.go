package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func mintRecovery(t *testing.T, h http.Handler, token, name, reason string, ttlMinutes int) (string, string) {
	t.Helper()
	body := map[string]any{"name": name, "reason": reason}
	if ttlMinutes != 0 {
		body["ttl_minutes"] = ttlMinutes
	}
	res := doJSON(t, h, http.MethodPost, "/v1/recovery-credentials", body, userAuth(token))
	if res.Code != http.StatusCreated {
		t.Fatalf("mint recovery credential: %d %s", res.Code, res.Body.String())
	}
	var out struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Token == "" || out.ID == "" {
		t.Fatalf("missing token or id: %s", res.Body.String())
	}
	return out.ID, out.Token
}

// A recovery credential reaches every organisation, and does so as an ordinary
// grant: it goes through the evaluator rather than bypassing it.
func TestRecoveryCredentialReachesEveryOrganisation(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := serverWithPlatformAdmins(t, db, "ops@kyc.com")

	merchant, _, _ := doBootstrapOrg(t, h, "owner@nu.com", "Owner", "Nu", "nu-recovery")
	orgID := merchant["organisation"].(map[string]any)["id"].(string)

	_, opsToken := doDevLogin(t, h, "ops@kyc.com", "Ops")
	id, raw := mintRecovery(t, h, opsToken, "incident-1", "database restore verification", 0)

	read := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/app-users", nil, keyAuth(raw))
	if read.Code != http.StatusOK {
		t.Fatalf("recovery credential must reach a merchant: %d %s", read.Code, read.Body.String())
	}

	// Revocation takes effect immediately and without a deploy, which is the
	// point of moving off a shared environment token.
	rev := doJSON(t, h, http.MethodDelete, "/v1/recovery-credentials/"+id, nil, userAuth(opsToken))
	if rev.Code != http.StatusOK {
		t.Fatalf("revoke: %d %s", rev.Code, rev.Body.String())
	}
	after := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/app-users", nil, keyAuth(raw))
	if after.Code != http.StatusUnauthorized {
		t.Fatalf("revoked credential must not authenticate: want 401, got %d %s", after.Code, after.Body.String())
	}
}

// Minting one requires already reaching every organisation. It is delegation,
// never a way around a boundary the caller is not already inside.
func TestRecoveryCredentialCannotBeMintedByAMerchant(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := serverWithPlatformAdmins(t, db)

	_, ownerToken, _ := doBootstrapOrg(t, h, "owner@mu.com", "Owner", "Mu", "mu-recovery")

	res := doJSON(t, h, http.MethodPost, "/v1/recovery-credentials", map[string]any{
		"name": "sneaky", "reason": "because",
	}, userAuth(ownerToken))
	if res.Code != http.StatusForbidden {
		t.Fatalf("a merchant owner must not mint a recovery credential: want 403, got %d %s", res.Code, res.Body.String())
	}
}

// A recovery credential with no stated reason is indistinguishable from a back
// door, and one that never expires is a permanent bypass under another name.
func TestRecoveryCredentialRequiresReasonAndBoundedExpiry(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := serverWithPlatformAdmins(t, db, "ops2@kyc.com")
	_, opsToken := doDevLogin(t, h, "ops2@kyc.com", "Ops")

	noReason := doJSON(t, h, http.MethodPost, "/v1/recovery-credentials", map[string]any{
		"name": "no-reason",
	}, userAuth(opsToken))
	if noReason.Code != http.StatusBadRequest {
		t.Fatalf("reason must be required: got %d %s", noReason.Code, noReason.Body.String())
	}

	tooLong := doJSON(t, h, http.MethodPost, "/v1/recovery-credentials", map[string]any{
		"name": "forever", "reason": "incident", "ttl_minutes": 60 * 24 * 30,
	}, userAuth(opsToken))
	if tooLong.Code != http.StatusBadRequest {
		t.Fatalf("expiry must be bounded: got %d %s", tooLong.Code, tooLong.Body.String())
	}
}

// Every use is attributable. An environment token can only ever say
// "env-token"; a recovery credential names itself and records who minted it.
func TestRecoveryCredentialIsAttributable(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := serverWithPlatformAdmins(t, db, "ops3@kyc.com")

	login, opsToken := doDevLogin(t, h, "ops3@kyc.com", "Ops")
	opsID := userIDFrom(t, login)
	_, _ = mintRecovery(t, h, opsToken, "incident-42", "restoring a backup", 30)

	list := doJSON(t, h, http.MethodGet, "/v1/recovery-credentials", nil, userAuth(opsToken))
	if list.Code != http.StatusOK {
		t.Fatalf("list: %d %s", list.Code, list.Body.String())
	}
	var body struct {
		Items []struct {
			Name      string `json:"name"`
			Reason    string `json:"reason"`
			GrantedBy string `json:"granted_by"`
			Active    bool   `json:"active"`
		} `json:"items"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("want one credential, got %d", len(body.Items))
	}
	got := body.Items[0]
	if got.Name != "incident-42" || got.Reason != "restoring a backup" {
		t.Errorf("name/reason not recorded: %+v", got)
	}
	if got.GrantedBy != opsID {
		t.Errorf("granted_by = %q, want the minting user %q", got.GrantedBy, opsID)
	}
	if !got.Active {
		t.Error("a freshly minted credential must be active")
	}
}
