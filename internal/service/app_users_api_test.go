package service_test

import (
	"context"
	"net/http"
	"testing"
)

func TestAppUsersAndAttributeSchema(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)

	first, token, _ := doBootstrapOrg(t, h, "owner@acme.com", "Owner", "Acme", "acme-users")
	orgID := first["organisation"].(map[string]any)["id"].(string)

	resDef := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/attribute-definitions", map[string]any{
		"key": "country", "label": "Country", "value_type": "dropdown",
		"section": "identity", "required": true,
		"enum_values": []string{"AU", "NZ"},
	}, userAuth(token))
	if resDef.Code != http.StatusCreated {
		t.Fatalf("create attr status=%d body=%s", resDef.Code, resDef.Body.String())
	}

	resBad := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/app-users", map[string]any{
		"display_name": "Pat",
		"attributes":   map[string]any{},
	}, userAuth(token))
	if resBad.Code != http.StatusBadRequest {
		t.Fatalf("missing required attr want 400 got %d body=%s", resBad.Code, resBad.Body.String())
	}

	resUser := doJSON(t, h, http.MethodPost, "/v1/organisations/"+orgID+"/app-users", map[string]any{
		"email":        "pat@example.com",
		"display_name": "Pat",
		"attributes":   map[string]any{"country": "AU"},
	}, userAuth(token))
	if resUser.Code != http.StatusCreated {
		t.Fatalf("create app user status=%d body=%s", resUser.Code, resUser.Body.String())
	}

	resList := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/app-users", nil, userAuth(token))
	if resList.Code != http.StatusOK {
		t.Fatalf("list app users status=%d body=%s", resList.Code, resList.Body.String())
	}
	var listed struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, resList, &listed)
	if len(listed.Items) != 1 {
		t.Fatalf("want 1 app user, got %d", len(listed.Items))
	}
	attrs, _ := listed.Items[0]["attributes"].(map[string]any)
	if attrs["country"] != "AU" {
		t.Fatalf("country=%v", attrs["country"])
	}

	resDefs := doJSON(t, h, http.MethodGet, "/v1/organisations/"+orgID+"/attribute-definitions", nil, userAuth(token))
	if resDefs.Code != http.StatusOK {
		t.Fatalf("list defs status=%d body=%s", resDefs.Code, resDefs.Body.String())
	}
	var defs struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, resDefs, &defs)
	if len(defs.Items) != 1 || defs.Items[0]["section"] != "identity" {
		t.Fatalf("defs=%v", defs.Items)
	}
}
