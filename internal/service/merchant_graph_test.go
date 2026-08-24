package service_test

import (
	"net/http"
	"testing"
)

// KYC evaluating a merchant's authorisation.
//
// The tier used to hand back a description of authority for somebody else's
// backend to interpret, which left the merchant building the layer they came
// here to avoid. These tests are the other thing: they write what they own and
// ask a question.

func (f appAccessFixture) writeEdges(t *testing.T, edges ...map[string]any) {
	t.Helper()
	code, out := f.post(t, "/edges", map[string]any{"edges": edges})
	if code != http.StatusOK {
		t.Fatalf("write edges: %d %v", code, out)
	}
}

func (f appAccessFixture) check(t *testing.T, subject, action, resType, resID string) map[string]any {
	t.Helper()
	code, out := f.post(t, "/check", map[string]any{
		"subject_id": subject, "action": action,
		"resource_type": resType, "resource_id": resID,
	})
	if code != http.StatusOK {
		t.Fatalf("check: %d %v", code, out)
	}
	return out
}

// The whole claim, end to end: a grant at a container reaches a resource inside
// it, without a grant ever naming that resource. This is what the assembled
// grant set could never do, because KYC had never heard of the document.
func TestMerchantGrantReachesThroughContainment(t *testing.T) {
	f := newAppAccess(t, "mgraph")
	f.declareScope(t, "project")
	f.declareCapability(t, "document:read")
	f.createAppUser(t, "ana@customer.com")

	f.writeEdges(t,
		// Ana holds the editor role.
		map[string]any{
			"object_type": "role", "object_id": "editor", "relation": "holder",
			"subject_type": "app_user", "subject_id": "ana",
		},
		// The role is granted read on one project.
		map[string]any{
			"object_type": "project", "object_id": "apollo", "relation": "can_read",
			"subject_type": "role", "subject_id": "editor", "subject_relation": "holder",
		},
		// A document lives in that project. This is the fact KYC never had.
		map[string]any{
			"object_type": "document", "object_id": "d1", "relation": "parent",
			"subject_type": "project", "subject_id": "apollo",
		},
	)

	d := f.check(t, "ana", "read", "document", "d1")
	if d["allowed"] != true {
		t.Fatalf("a grant on the container must reach the document: %v", d)
	}
	// The route is the answer to "why", and a merchant asking about their own
	// namespace owns every node in it, so nothing is withheld.
	if hops := d["path"].([]any); len(hops) == 0 {
		t.Error("an allow must carry the route it took")
	}

	// A document in no project is unreachable, not merely denied: it has to be
	// indistinguishable from one that does not exist.
	if got := f.check(t, "ana", "read", "document", "orphan"); got["allowed"] != false ||
		got["reason"] != "unreachable" {
		t.Errorf("an uncontained document = %v, wanted unreachable", got)
	}
}

// Deleting a resource has to remove its edges, or access outlives the thing it
// was about.
func TestDeletingAnEdgeRemovesReach(t *testing.T) {
	f := newAppAccess(t, "mdelete")
	f.declareScope(t, "project")
	f.declareCapability(t, "document:read")

	containment := map[string]any{
		"object_type": "document", "object_id": "d1", "relation": "parent",
		"subject_type": "project", "subject_id": "apollo",
	}
	f.writeEdges(t,
		map[string]any{
			"object_type": "project", "object_id": "apollo", "relation": "can_read",
			"subject_type": "app_user", "subject_id": "ana",
		},
		containment,
	)
	if d := f.check(t, "ana", "read", "document", "d1"); d["allowed"] != true {
		t.Fatalf("setup: %v", d)
	}

	res := doJSON(t, f.h, http.MethodDelete,
		"/v1/organisations/"+f.orgID+"/edges", containment, userAuth(f.token))
	if res.Code != http.StatusNoContent {
		t.Fatalf("delete edge: %d %s", res.Code, res.Body.String())
	}
	if d := f.check(t, "ana", "read", "document", "d1"); d["allowed"] != false {
		t.Errorf("reach outlived the containment edge: %v", d)
	}
}

// Writes are idempotent. This runs in a merchant's own write path, so the first
// thing that happens when one fails is a retry.
func TestEdgeWritesAreIdempotent(t *testing.T) {
	f := newAppAccess(t, "midem")
	f.declareScope(t, "project")
	f.declareCapability(t, "project:read")

	edge := map[string]any{
		"object_type": "project", "object_id": "apollo", "relation": "can_read",
		"subject_type": "app_user", "subject_id": "ana",
	}
	f.writeEdges(t, edge)
	f.writeEdges(t, edge)

	if d := f.check(t, "ana", "read", "project", "apollo"); d["allowed"] != true {
		t.Errorf("re-writing the same edge broke it: %v", d)
	}
}

// The schema is derived from the vocabulary, so it cannot drift from it and
// there is no migration when a merchant adds a kind.
func TestMerchantSchemaFollowsTheVocabulary(t *testing.T) {
	f := newAppAccess(t, "mschema")
	f.declareScope(t, "project")
	f.declareCapability(t, "document:read")

	code, out := f.get(t, "/access-schema")
	if code != http.StatusOK {
		t.Fatalf("schema: %d %v", code, out)
	}
	if ns := out["namespace"].(string); ns == "kyc" {
		t.Fatalf("a merchant schema must not be in KYC's namespace, got %q", ns)
	}
	var sawDocument bool
	for _, raw := range out["nodes"].([]any) {
		if raw.(map[string]any)["label"] == "document" {
			sawDocument = true
		}
	}
	if !sawDocument {
		t.Errorf("declaring document:read must put document in the schema: %v", out["nodes"])
	}

	// Adding a capability changes the schema with no migration in between.
	f.declareCapability(t, "invoice:write")
	_, after := f.get(t, "/access-schema")
	var sawInvoice bool
	for _, raw := range after["nodes"].([]any) {
		if raw.(map[string]any)["label"] == "invoice" {
			sawInvoice = true
		}
	}
	if !sawInvoice {
		t.Error("the schema did not follow the vocabulary")
	}
}

// An edge naming something the merchant never declared is refused at write
// time. Not a security boundary, since it would reach nothing anyway: it is
// there so the failure is loud when it is written rather than silent when it is
// checked.
func TestUndeclaredTypesAreRefusedAtWriteTime(t *testing.T) {
	f := newAppAccess(t, "munknown")
	f.declareScope(t, "project")
	f.declareCapability(t, "project:read")

	if code, _ := f.post(t, "/edges", map[string]any{"edges": []map[string]any{{
		"object_type": "spaceship", "object_id": "x", "relation": "can_read",
		"subject_type": "app_user", "subject_id": "ana",
	}}}); code != http.StatusBadRequest {
		t.Errorf("an undeclared type must be refused, got %d", code)
	}

	if code, _ := f.post(t, "/check", map[string]any{
		"subject_id": "ana", "action": "teleport",
		"resource_type": "project", "resource_id": "apollo",
	}); code != http.StatusBadRequest {
		t.Errorf("an undeclared action must be refused, got %d", code)
	}
}
