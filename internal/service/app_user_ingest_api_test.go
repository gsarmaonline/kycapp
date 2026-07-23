package service_test

import (
	"context"
	"net/http"
	"testing"
)

func TestAppUserIngestDiscoverAndMerge(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	first, token, _ := doBootstrapOrg(t, h, "ingest@acme.com", "Owner", "Ingest Co", "ingest-co")
	orgID := first["organisation"].(map[string]any)["id"].(string)
	auth := userAuth(token)

	orgRes := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID, nil, auth)
	if orgRes.Code != http.StatusOK {
		t.Fatalf("get org: %s", orgRes.Body.String())
	}
	var org map[string]any
	decodeBody(t, orgRes, &org)
	if org["app_user_authority"] != "kyc" {
		t.Fatalf("default authority=%v", org["app_user_authority"])
	}
	if org["app_user_ingest_upsert_key"] != "external_id" {
		t.Fatalf("default upsert key=%v", org["app_user_ingest_upsert_key"])
	}
	if org["app_user_attributes_mode"] != "discover" {
		t.Fatalf("default attributes mode=%v", org["app_user_attributes_mode"])
	}

	patch := doJSON(t, h, http.MethodPatch, "/v1/organisations/"+orgID, map[string]any{
		"app_user_authority":         "external",
		"app_user_ingest_upsert_key": "external_id",
		"app_user_attributes_mode":   "discover",
	}, auth)
	if patch.Code != http.StatusOK {
		t.Fatalf("patch org settings: %s", patch.Body.String())
	}

	created := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/app-users/ingest", map[string]any{
		"external_id":  "usr_1",
		"email":        "pat@example.com",
		"display_name": "Pat",
		"attributes": map[string]any{
			"plan_tier": "pro",
			"seats":     3,
		},
	}, auth)
	if created.Code != http.StatusCreated {
		t.Fatalf("ingest create: %d %s", created.Code, created.Body.String())
	}
	var createdBody map[string]any
	decodeBody(t, created, &createdBody)
	if createdBody["created"] != true {
		t.Fatalf("want created=true: %#v", createdBody)
	}
	attrs, _ := createdBody["attributes"].(map[string]any)
	if attrs["plan_tier"] != "pro" {
		t.Fatalf("attrs=%v", attrs)
	}

	defs := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/attribute-definitions", nil, auth)
	if defs.Code != http.StatusOK {
		t.Fatalf("list defs: %s", defs.Body.String())
	}
	var defsBody struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, defs, &defsBody)
	byKey := map[string]map[string]any{}
	for _, item := range defsBody.Items {
		byKey[item["key"].(string)] = item
	}
	if byKey["plan_tier"] == nil || byKey["plan_tier"]["section"] != "ingested" {
		t.Fatalf("plan_tier not discovered: %#v", byKey["plan_tier"])
	}
	if byKey["seats"] == nil || byKey["seats"]["value_type"] != "number" {
		t.Fatalf("seats not discovered as number: %#v", byKey["seats"])
	}

	updated := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/app-users/ingest", map[string]any{
		"external_id": "usr_1",
		"attributes": map[string]any{
			"plan_tier": "enterprise",
			"country":   "AU",
		},
	}, auth)
	if updated.Code != http.StatusOK {
		t.Fatalf("ingest update: %d %s", updated.Code, updated.Body.String())
	}
	var updatedBody map[string]any
	decodeBody(t, updated, &updatedBody)
	if updatedBody["created"] != false {
		t.Fatalf("want created=false: %#v", updatedBody)
	}
	merged, _ := updatedBody["attributes"].(map[string]any)
	if merged["plan_tier"] != "enterprise" {
		t.Fatalf("plan_tier not updated: %v", merged)
	}
	if merged["seats"] != float64(3) {
		t.Fatalf("seats should be preserved by merge: %v", merged)
	}
	if merged["country"] != "AU" {
		t.Fatalf("country missing: %v", merged)
	}
}

func TestAppUserIngestStrictRejectsUnknown(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	first, token, _ := doBootstrapOrg(t, h, "strict@acme.com", "Owner", "Strict Co", "strict-co")
	orgID := first["organisation"].(map[string]any)["id"].(string)
	auth := userAuth(token)

	patch := doJSON(t, h, http.MethodPatch, "/v1/organisations/"+orgID, map[string]any{
		"app_user_attributes_mode": "strict",
	}, auth)
	if patch.Code != http.StatusOK {
		t.Fatalf("patch: %s", patch.Body.String())
	}

	res := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/app-users/ingest", map[string]any{
		"external_id": "usr_x",
		"attributes":  map[string]any{"mystery_field": "yes"},
	}, auth)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("strict ingest want 400 got %d %s", res.Code, res.Body.String())
	}
}

func TestAppUserIngestByEmail(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	first, token, _ := doBootstrapOrg(t, h, "emailkey@acme.com", "Owner", "Email Key Co", "email-key-co")
	orgID := first["organisation"].(map[string]any)["id"].(string)
	auth := userAuth(token)

	patch := doJSON(t, h, http.MethodPatch, "/v1/organisations/"+orgID, map[string]any{
		"app_user_ingest_upsert_key": "email",
	}, auth)
	if patch.Code != http.StatusOK {
		t.Fatalf("patch: %s", patch.Body.String())
	}

	missing := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/app-users/ingest", map[string]any{
		"external_id": "usr_1",
		"attributes":  map[string]any{"country": "AU"},
	}, auth)
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("missing email want 400 got %d %s", missing.Code, missing.Body.String())
	}

	created := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/app-users/ingest", map[string]any{
		"email":        "ada@example.com",
		"external_id":  "usr_ada",
		"display_name": "Ada",
		"attributes":   map[string]any{"country": "AU"},
	}, auth)
	if created.Code != http.StatusCreated {
		t.Fatalf("ingest by email: %d %s", created.Code, created.Body.String())
	}

	again := doJSON(t, h, http.MethodPut, "/v1/organisations/"+orgID+"/app-users/ingest", map[string]any{
		"email":      "ada@example.com",
		"attributes": map[string]any{"country": "NZ"},
	}, auth)
	if again.Code != http.StatusOK {
		t.Fatalf("reingest by email: %d %s", again.Code, again.Body.String())
	}
	var body map[string]any
	decodeBody(t, again, &body)
	if body["external_id"] != "usr_ada" {
		t.Fatalf("external_id should remain: %#v", body)
	}
	attrs, _ := body["attributes"].(map[string]any)
	if attrs["country"] != "NZ" {
		t.Fatalf("attrs=%v", attrs)
	}
}
