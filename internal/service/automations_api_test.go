package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gsarmaonline/kyc/core/automations"
	"github.com/gsarmaonline/kyc/internal/service"
)

func TestAutomationsCRUDAndProcess(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)
	svc := service.New(db)

	first, token, _ := doBootstrapOrg(t, h, "owner@auto.com", "Owner", "AutoCo", "autoco")
	orgID := first["organisation"].(map[string]any)["id"].(string)

	created := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/automations", map[string]any{
		"name":    "AU welcome",
		"trigger": "app_user.created",
		"conditions": map[string]any{
			"all": []map[string]any{
				{"field": "attributes.country", "op": "eq", "value": "AU"},
			},
		},
		"actions": []map[string]any{
			{"type": "send_email", "template_key": "welcome"},
		},
	}, userAuth(token))
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var auto map[string]any
	decodeBody(t, created, &auto)
	autoID, _ := auto["id"].(string)

	listed := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/automations", nil, userAuth(token))
	if listed.Code != http.StatusOK {
		t.Fatalf("list status=%d", listed.Code)
	}

	payload := map[string]any{
		"id": "u1", "email": "pat@example.com", "display_name": "Pat", "status": "active",
		"attributes": map[string]any{"country": "AU"},
	}
	raw, _ := json.Marshal(payload)
	if err := svc.ProcessAutomationEvent(ctx, orgID, automations.TriggerAppUserCreated, raw); err != nil {
		t.Fatal(err)
	}

	runs := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/automation-runs?automation_id="+autoID, nil, userAuth(token))
	if runs.Code != http.StatusOK {
		t.Fatalf("runs status=%d body=%s", runs.Code, runs.Body.String())
	}
	var runList struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, runs, &runList)
	if len(runList.Items) != 1 || runList.Items[0]["status"] != "success" {
		t.Fatalf("runs=%v", runList.Items)
	}

	// NZ should skip
	payload["attributes"] = map[string]any{"country": "NZ"}
	raw, _ = json.Marshal(payload)
	if err := svc.ProcessAutomationEvent(ctx, orgID, automations.TriggerAppUserCreated, raw); err != nil {
		t.Fatal(err)
	}
	runs2 := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/automation-runs?automation_id="+autoID, nil, userAuth(token))
	decodeBody(t, runs2, &runList)
	if len(runList.Items) < 2 {
		t.Fatalf("want skip run, got %d", len(runList.Items))
	}
	foundSkip := false
	for _, item := range runList.Items {
		if item["status"] == "skipped" {
			foundSkip = true
		}
	}
	if !foundSkip {
		t.Fatalf("want skipped run in %#v", runList.Items)
	}
}
