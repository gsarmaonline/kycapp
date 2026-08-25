package service_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gsarmaonline/kyc/internal/service"
)

// The graph's version, and the one case it exists for.
//
// app_grants already had a version, so a merchant caches a grant set instead of
// polling. reach_edges could not answer the same question: it has created_at
// and no updated_at, an update moves neither, and a delete leaves nothing
// behind at all. The delete is what matters. A revocation that moves no
// version leaves every cache serving the *wider* permission, which is the one
// direction of staleness that costs something.

type versionFixture struct {
	h     http.Handler
	svc   *service.Service
	orgID string
	token string
}

func newVersionFixture(t *testing.T, slug string) versionFixture {
	t.Helper()
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h, svc := testEnv(t, db)
	org, token, _ := doBootstrapOrg(t, h, slug+"@merchant.com", "Owner", slug, slug)
	f := versionFixture{
		h: h, svc: svc,
		orgID: org["organisation"].(map[string]any)["id"].(string),
		token: token,
	}
	f.declare(t, "/app-scope-types", map[string]any{"kind": "project"})
	f.declare(t, "/app-capabilities", map[string]any{"key": "project:read"})
	return f
}

func (f versionFixture) declare(t *testing.T, path string, body map[string]any) {
	t.Helper()
	res := doJSON(t, f.h, http.MethodPost, "/v1/organisations/"+f.orgID+path, body, userAuth(f.token))
	if res.Code != http.StatusCreated {
		t.Fatalf("declare %s: %d %s", path, res.Code, res.Body.String())
	}
}

func (f versionFixture) version(t *testing.T) int64 {
	t.Helper()
	v, err := f.svc.MerchantGraphVersion(context.Background(), f.orgID)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	return v
}

var anaGrant = map[string]any{
	"object_type": "project", "object_id": "apollo", "relation": "can_read",
	"subject_type": "app_user", "subject_id": "ana",
}

func (f versionFixture) writeEdge(t *testing.T, edge map[string]any) {
	t.Helper()
	res := doJSON(t, f.h, http.MethodPost,
		"/v1/organisations/"+f.orgID+"/edges", map[string]any{"edges": []map[string]any{edge}}, userAuth(f.token))
	if res.Code != http.StatusOK {
		t.Fatalf("write edge: %d %s", res.Code, res.Body.String())
	}
}

func (f versionFixture) deleteEdge(t *testing.T, edge map[string]any) {
	t.Helper()
	res := doJSON(t, f.h, http.MethodDelete,
		"/v1/organisations/"+f.orgID+"/edges", edge, userAuth(f.token))
	if res.Code != http.StatusNoContent {
		t.Fatalf("delete edge: %d %s", res.Code, res.Body.String())
	}
}

// A namespace nothing has written reports 0. That is the honest answer rather
// than a missing row: there is nothing that could have gone stale.
//
// Measured before the vocabulary is declared, because declaring a scope kind or
// a capability is itself a change a cache has to see: a new capability widens
// every wildcard grant that already exists.
func TestAnUntouchedGraphHasVersionZero(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h, svc := testEnv(t, db)
	org, token, _ := doBootstrapOrg(t, h, "mvzero@merchant.com", "Owner", "mvzero", "mvzero")
	f := versionFixture{
		h: h, svc: svc,
		orgID: org["organisation"].(map[string]any)["id"].(string),
		token: token,
	}

	if v := f.version(t); v != 0 {
		t.Fatalf("an untouched graph must report 0, got %d", v)
	}

	// And declaring the vocabulary is a change, so it moves.
	f.declare(t, "/app-scope-types", map[string]any{"kind": "project"})
	if v := f.version(t); v == 0 {
		t.Fatal("declaring a scope kind must move the version")
	}
}

func TestWritingAnEdgeMovesTheVersion(t *testing.T) {
	f := newVersionFixture(t, "mvwrite")
	before := f.version(t)
	f.writeEdge(t, anaGrant)
	if after := f.version(t); after <= before {
		t.Fatalf("a write must move the version: %d -> %d", before, after)
	}
}

// The reason the whole thing exists. Revoking access moves no timestamp on any
// surviving row, so a version derived from the edges themselves would not
// change and every cache would keep serving the permission that was just taken
// away.
func TestRevokingAccessMovesTheVersion(t *testing.T) {
	f := newVersionFixture(t, "mvrevoke")
	f.writeEdge(t, anaGrant)
	granted := f.version(t)

	f.deleteEdge(t, anaGrant)
	revoked := f.version(t)

	if revoked <= granted {
		t.Fatalf("a revocation must move the version: %d -> %d", granted, revoked)
	}
}

// The version answers "has anything changed", so a read must not move it.
// Otherwise every cache invalidates itself by checking.
func TestReadingDoesNotMoveTheVersion(t *testing.T) {
	f := newVersionFixture(t, "mvread")
	f.writeEdge(t, anaGrant)
	before := f.version(t)

	res := doJSON(t, f.h, http.MethodPost, "/v1/organisations/"+f.orgID+"/check", map[string]any{
		"subject_id": "ana", "action": "read",
		"resource_type": "project", "resource_id": "apollo",
	}, userAuth(f.token))
	if res.Code != http.StatusOK {
		t.Fatalf("check: %d %s", res.Code, res.Body.String())
	}
	if after := f.version(t); after != before {
		t.Fatalf("a read must not move the version: %d -> %d", before, after)
	}
}

// Two merchants share one table and must not share a version, or every write
// by one invalidates every cache held by the other.
func TestVersionsAreNamespaced(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h, svc := testEnv(t, db)

	orgA, tokenA, _ := doBootstrapOrg(t, h, "a@merchant.com", "A", "Alpha", "mvalpha")
	orgB, _, _ := doBootstrapOrg(t, h, "b@merchant.com", "B", "Beta", "mvbeta")
	idA := orgA["organisation"].(map[string]any)["id"].(string)
	idB := orgB["organisation"].(map[string]any)["id"].(string)

	fa := versionFixture{h: h, svc: svc, orgID: idA, token: tokenA}
	fa.declare(t, "/app-scope-types", map[string]any{"kind": "project"})
	fa.declare(t, "/app-capabilities", map[string]any{"key": "project:read"})

	beforeB, err := svc.MerchantGraphVersion(ctx, idB)
	if err != nil {
		t.Fatal(err)
	}
	fa.writeEdge(t, anaGrant)
	afterB, err := svc.MerchantGraphVersion(ctx, idB)
	if err != nil {
		t.Fatal(err)
	}
	if afterB != beforeB {
		t.Fatalf("one merchant's write moved another's version: %d -> %d", beforeB, afterB)
	}
	if fa.version(t) == 0 {
		t.Fatal("the writing merchant's version must have moved")
	}
}
