package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gsarmaonline/kyc/core/automations"
	"github.com/gsarmaonline/kyc/internal/mailer"
)

// TestE2ELocalHappyPath populates core org models locally (testcontainers Postgres)
// with Stripe/Resend stubbed via noop payments + recording mailer, and a sync enqueuer
// instead of River. Requires Docker for Postgres.
func TestE2ELocalHappyPath(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h, svc := testEnv(t, db)

	recMail := mailer.NewRecording()
	svc.SetMailer(recMail)
	svc.SetEnqueuer(&syncEnqueuer{svc: svc})

	boot, token, _ := doBootstrapOrg(t, h, "e2e@kyc.test", "E2E Owner", "E2E Org", "e2e-org")
	orgID := boot["organisation"].(map[string]any)["id"].(string)
	auth := userAuth(token)

	// Branding
	brand := doJSON(t, h, http.MethodPatch, "/v1/organisations/"+orgID, map[string]any{
		"primary_color": "#1f4d3a",
		"accent_color":  "#16382a",
		"email_footer":  "© E2E Org",
		"email_typography": map[string]any{
			"header": map[string]any{"font": "georgia", "size": 22, "weight": 700, "style": "normal"},
			"body":   map[string]any{"font": "arial", "size": 16, "weight": 400, "style": "normal"},
			"footer": map[string]any{"font": "arial", "size": 12, "weight": 400, "style": "normal"},
		},
	}, auth)
	if brand.Code != http.StatusOK {
		t.Fatalf("branding: %d %s", brand.Code, brand.Body.String())
	}

	// Email templates (seeded + custom)
	emails := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/email-templates", nil, auth)
	if emails.Code != http.StatusOK {
		t.Fatalf("email templates: %s", emails.Body.String())
	}
	var emailList struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, emails, &emailList)
	if len(emailList.Items) < 3 {
		t.Fatalf("want seeded email templates, got %d", len(emailList.Items))
	}
	customEmail := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/email-templates", map[string]any{
		"key": "e2e_notice", "name": "E2E notice",
		"subject":   "Hello {{app_user.display_name}} from {{organisation.name}}",
		"body_text": "Hi {{app_user.display_name}}, email={{app_user.email}} country={{app_user.country}}",
		"body_html": "<p>Hi {{app_user.display_name}}</p>",
	}, auth)
	if customEmail.Code != http.StatusCreated {
		t.Fatalf("create email template: %s", customEmail.Body.String())
	}

	// Attributes (seeded) + custom
	attrs := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/attribute-definitions", nil, auth)
	if attrs.Code != http.StatusOK {
		t.Fatalf("attributes: %s", attrs.Body.String())
	}
	customAttr := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/attribute-definitions", map[string]any{
		"key": "plan_tier", "label": "Plan tier", "value_type": "dropdown",
		"enum_values": []string{"free", "pro"},
	}, auth)
	if customAttr.Code != http.StatusCreated {
		t.Fatalf("create attribute: %s", customAttr.Body.String())
	}

	// Product feature + plan
	feat := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/product-features", map[string]any{
		"key": "e2e_feature", "description": "E2E feature",
	}, auth)
	if feat.Code != http.StatusCreated {
		t.Fatalf("feature: %s", feat.Body.String())
	}
	plan := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/product-plans", map[string]any{
		"key": "e2e_pro", "name": "E2E Pro",
		"price": map[string]any{"interval": "month", "currency": "usd", "unit_amount": 990},
	}, auth)
	if plan.Code != http.StatusCreated {
		t.Fatalf("plan: %s", plan.Body.String())
	}
	var planBody map[string]any
	decodeBody(t, plan, &planBody)
	planID := planBody["id"].(string)
	setFeat := doJSON(t, h, http.MethodPut, "/v1/product-plans/"+planID+"/features", map[string]any{
		"feature_keys": []string{"e2e_feature"},
	}, auth)
	if setFeat.Code != http.StatusOK {
		t.Fatalf("set features: %s", setFeat.Body.String())
	}
	activate := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/product-plan", map[string]any{
		"product_plan_id": planID,
	}, auth)
	if activate.Code != http.StatusOK {
		t.Fatalf("activate plan: %s", activate.Body.String())
	}

	// Outbound webhook config (delivery to loopback is blocked by SSRF guard;
	// template rendering is checked below with the shared automations library).
	bodyTemplate := `{
  "organisation_id": "{{organisation_id}}",
  "email": "{{app_user.email}}",
  "country": "{{app_user.country}}"
}`
	wh := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/webhooks", map[string]any{
		"name":          "e2e-hook",
		"url":           "https://example.com/crm/hooks/kyc",
		"body_template": bodyTemplate,
	}, auth)
	if wh.Code != http.StatusCreated {
		t.Fatalf("webhook: %s", wh.Body.String())
	}
	var whBody map[string]any
	decodeBody(t, wh, &whBody)
	_ = whBody["id"]

	// Inbound webhook
	inb := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/inbound-webhooks", map[string]any{
		"name": "e2e-inbound", "auth_mode": "header",
	}, auth)
	if inb.Code != http.StatusCreated {
		t.Fatalf("inbound: %s", inb.Body.String())
	}
	var inbBody map[string]any
	decodeBody(t, inb, &inbBody)
	inboundID, _ := inbBody["id"].(string)
	inboundSecret, _ := inbBody["secret"].(string)
	if inboundID == "" || inboundSecret == "" {
		t.Fatalf("inbound missing id/secret: %#v", inbBody)
	}

	// Automation: welcome email (recording mailer; Stripe stays noop)
	auto := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/automations", map[string]any{
		"name":    "E2E welcome",
		"trigger": "app_user.created",
		"conditions": map[string]any{
			"mode": "all",
			"items": []map[string]any{
				{"field": "app_user.country", "op": "eq", "value": "AU"},
			},
		},
		"actions": []map[string]any{
			{
				"type":   "send_email",
				"params": map[string]any{"template_key": "welcome"},
			},
		},
	}, auth)
	if auto.Code != http.StatusCreated {
		t.Fatalf("automation: %s", auto.Body.String())
	}
	var autoBody map[string]any
	decodeBody(t, auto, &autoBody)
	autoID := autoBody["id"].(string)

	// Org API key
	keyRes := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/api-keys", map[string]any{
		"name": "e2e-backend",
	}, auth)
	if keyRes.Code != http.StatusCreated {
		t.Fatalf("api key: %s", keyRes.Body.String())
	}
	var keyBody map[string]any
	decodeBody(t, keyRes, &keyBody)
	apiToken, _ := keyBody["token"].(string)

	// Create app user → sync enqueuer runs automation (noop Stripe; recording mailer)
	userRes := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/app-users", map[string]any{
		"email":        "pat@e2e.test",
		"display_name": "Pat E2E",
		"external_id":  "ext-e2e-1",
		"attributes":   map[string]any{"country": "AU", "plan_tier": "pro"},
	}, map[string]string{"Authorization": "Bearer " + apiToken})
	if userRes.Code != http.StatusCreated {
		t.Fatalf("app user: %s", userRes.Body.String())
	}
	var appUser map[string]any
	decodeBody(t, userRes, &appUser)

	// Automation run succeeded
	runs := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/automation-runs?automation_id="+autoID, nil, auth)
	if runs.Code != http.StatusOK {
		t.Fatalf("runs: %s", runs.Body.String())
	}
	var runList struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, runs, &runList)
	if len(runList.Items) == 0 || runList.Items[0]["status"] != "success" {
		t.Fatalf("want successful automation run, got %#v", runList.Items)
	}

	// Email recorded with branding + placeholders
	msgs := recMail.Messages()
	if len(msgs) != 1 {
		t.Fatalf("want 1 email, got %d", len(msgs))
	}
	if msgs[0].To[0] != "pat@e2e.test" {
		t.Fatalf("to=%v", msgs[0].To)
	}
	if !strings.Contains(msgs[0].HTML, "E2E Org") && !strings.Contains(msgs[0].Subject, "Welcome") {
		// welcome template uses organisation.name / branding chrome
		if !strings.Contains(msgs[0].HTML, "Pat") && !strings.Contains(msgs[0].Text, "Pat") {
			t.Fatalf("email missing rendered name: subject=%q html=%q", msgs[0].Subject, msgs[0].HTML)
		}
	}
	if !strings.Contains(msgs[0].HTML, "© E2E Org") && !strings.Contains(msgs[0].HTML, "E2E Org") {
		t.Fatalf("branding missing from html: %s", msgs[0].HTML)
	}

	// Shared path vocabulary for webhook body_template
	rendered, err := automations.BuildWebhookBody(bodyTemplate, orgID, map[string]any{
		"email":      "pat@e2e.test",
		"attributes": map[string]any{"country": "AU"},
	})
	if err != nil {
		t.Fatalf("BuildWebhookBody: %v", err)
	}
	var hookJSON map[string]any
	if err := json.Unmarshal(rendered, &hookJSON); err != nil {
		t.Fatalf("webhook json: %v", err)
	}
	if hookJSON["email"] != "pat@e2e.test" || hookJSON["country"] != "AU" || hookJSON["organisation_id"] != orgID {
		t.Fatalf("webhook json=%v", hookJSON)
	}

	// Inbound webhook public endpoint
	inReq := httptest.NewRequest(http.MethodPost, "/v1/hooks/inbound/"+inboundID, strings.NewReader(`{"ping":true}`))
	inReq.Header.Set("Content-Type", "application/json")
	inReq.Header.Set("X-KYC-Webhook-Secret", inboundSecret)
	inRec := httptest.NewRecorder()
	h.ServeHTTP(inRec, inReq)
	if inRec.Code != http.StatusAccepted && inRec.Code != http.StatusOK {
		t.Fatalf("inbound post status=%d body=%s", inRec.Code, inRec.Body.String())
	}

	// KYC billing subscription present (free_plan from org create); noop webhook reconcile path
	sub := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/subscription", nil, auth)
	if sub.Code != http.StatusOK {
		t.Fatalf("subscription: %s", sub.Body.String())
	}

	// Entitlement check for product feature
	check := doJSON(t, h, http.MethodPost, "/v1/entitlements/check", map[string]any{
		"organisation_id": orgID, "entitlement": "e2e_feature",
	}, auth)
	var allowed map[string]any
	decodeBody(t, check, &allowed)
	if allowed["allowed"] != true {
		t.Fatalf("product feature check: %#v", allowed)
	}

	// Membership invite
	roles := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/roles", nil, auth)
	if roles.Code != http.StatusOK {
		t.Fatalf("roles: %s", roles.Body.String())
	}
	var roleList struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, roles, &roleList)
	var memberRoleID string
	for _, r := range roleList.Items {
		if r["key"] == "member" || r["key"] == "admin" {
			memberRoleID, _ = r["id"].(string)
			if r["key"] == "member" {
				break
			}
		}
	}
	if memberRoleID == "" && len(roleList.Items) > 0 {
		memberRoleID, _ = roleList.Items[0]["id"].(string)
	}
	if memberRoleID == "" {
		t.Fatal("no roles seeded")
	}
	invite := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/memberships", map[string]any{
		"email": "member@e2e.test", "role_id": memberRoleID,
	}, auth)
	if invite.Code != http.StatusCreated {
		t.Fatalf("invite: %s", invite.Body.String())
	}

	t.Logf("e2e local ok: org=%s user=%s automation=%s emails=%d",
		orgID, appUser["id"], autoID, len(msgs))
}
