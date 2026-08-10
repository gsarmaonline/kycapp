package service_test

import (
	"context"
	"net/http"
	"testing"
)

func TestEmailTemplatesDefaultsAndCustom(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	first, token, _ := doBootstrapOrg(t, h, "owner@mail.com", "Owner", "MailCo", "mailco")
	orgID := first["organisation"].(map[string]any)["id"].(string)

	list := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/email-templates", nil, userAuth(token))
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	var listed struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, list, &listed)
	if len(listed.Items) < 3 {
		t.Fatalf("want seeded defaults, got %d", len(listed.Items))
	}
	keys := map[string]bool{}
	var welcomeID string
	for _, item := range listed.Items {
		k, _ := item["key"].(string)
		keys[k] = true
		if k == "welcome" {
			welcomeID, _ = item["id"].(string)
		}
	}
	for _, want := range []string{"welcome", "payment_thank_you", "profile_incomplete"} {
		if !keys[want] {
			t.Fatalf("missing default %s in %#v", want, keys)
		}
	}

	created := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/email-templates", map[string]any{
		"key": "seasonal_offer", "name": "Seasonal offer",
		"subject": "A special offer", "body_text": "Hello {{display_name}}",
		// Required since migration 000041 added per-template body blocks:
		// a template must carry body_sections or body_html.
		"body_html": "<p>Hello {{display_name}}</p>",
	}, userAuth(token))
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}

	if welcomeID == "" {
		t.Fatal("welcome id missing")
	}
	patched := doJSON(t, h, http.MethodPatch, "/v1/email-templates/"+welcomeID, map[string]any{
		"subject": "Welcome aboard, {{display_name}}",
	}, userAuth(token))
	if patched.Code != http.StatusOK {
		t.Fatalf("patch status=%d body=%s", patched.Code, patched.Body.String())
	}
	var welcome map[string]any
	decodeBody(t, patched, &welcome)
	if welcome["subject"] != "Welcome aboard, {{display_name}}" {
		t.Fatalf("subject=%v", welcome["subject"])
	}
	if welcome["is_system"] != true {
		t.Fatalf("welcome should remain system")
	}
}
