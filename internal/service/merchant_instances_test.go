package service_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gsarmaonline/kyc/internal/service"
)

// The instance layer, end to end.
//
// The map drew types and nothing else, so a merchant who had declared a model
// and filled it looked at a picture that could not show them a single thing
// they had made. These cover the three properties that makes it useful: it
// finds instances from both ends of an edge, it names them the way their author
// named them, and it says so when it is showing a sample.

// instancesByType reads the response into a lookup, so a test can assert about
// one type without depending on how many others happen to exist.
func instancesByType(t *testing.T, out map[string]any) map[string]map[string]any {
	t.Helper()
	byType := map[string]map[string]any{}
	raw, ok := out["types"].([]any)
	if !ok {
		t.Fatalf("no types in %v", out)
	}
	for _, r := range raw {
		m := r.(map[string]any)
		byType[m["type"].(string)] = m
	}
	return byType
}

func labelsOf(m map[string]any) []string {
	var out []string
	for _, r := range m["instances"].([]any) {
		out = append(out, r.(map[string]any)["label"].(string))
	}
	return out
}

// A resource is only ever an object and a customer is only ever a subject, so
// reading one end of the edge table would miss a whole type. This is the test
// that fails if the UNION is dropped for a plain scan of object_type.
func TestInstancesComeFromBothEndsOfAnEdge(t *testing.T) {
	f := newAppAccess(t, "minst")
	f.declareScope(t, "project")
	f.declareCapability(t, "document:read")

	// document:d1 is an object here and app_user:ana a subject. Neither appears
	// at the other end anywhere in this namespace.
	f.writeEdges(t, map[string]any{
		"object_type": "document", "object_id": "d1", "relation": "can_read",
		"subject_type": "app_user", "subject_id": "ana",
	})

	code, out := f.get(t, "/access-instances")
	if code != http.StatusOK {
		t.Fatalf("instances: %d %v", code, out)
	}
	byType := instancesByType(t, out)

	if got := labelsOf(byType["document"]); len(got) != 1 || got[0] != "d1" {
		t.Errorf("the object end was not read: %v", got)
	}
	if _, ok := byType["app_user"]; !ok {
		t.Errorf("the subject end was not read: %v", byType)
	}
}

// The star is not an instance. It is the wildcard standing for every instance
// of a kind, and counting it reports one more project than the merchant has.
func TestInstancesExcludeTheWildcardNode(t *testing.T) {
	f := newAppAccess(t, "mstar")
	f.declareScope(t, "project")
	f.declareCapability(t, "project:read")

	f.writeEdges(t, map[string]any{
		"object_type": "project", "object_id": "*", "relation": "can_read",
		"subject_type": "app_user", "subject_id": "ana",
	})

	_, out := f.get(t, "/access-instances")
	byType := instancesByType(t, out)
	if got, ok := byType["project"]; ok {
		for _, label := range labelsOf(got) {
			if label == "*" {
				t.Errorf("the wildcard was drawn as a project: %v", labelsOf(got))
			}
		}
	}
}

// A role is stored under an opaque id and the merchant named it "maintainer".
// Showing them the id would be showing them nothing.
func TestInstancesUseTheNameTheirAuthorChose(t *testing.T) {
	f := newAppAccess(t, "mlabel")
	f.declareScope(t, "project")
	f.declareCapability(t, "deploy:read")

	roleID := f.createRole(t, "base", []string{"deploy:read"}, nil)
	f.createRole(t, "maintainer", []string{"deploy:read"}, []string{roleID})

	_, out := f.get(t, "/access-instances")
	byType := instancesByType(t, out)

	roles, ok := byType["role"]
	if !ok {
		t.Fatalf("role extension put no roles in the graph: %v", byType)
	}
	for _, label := range labelsOf(roles) {
		if label == roleID {
			t.Errorf("a role was labelled with its stored id rather than its key: %v", labelsOf(roles))
		}
	}
	var sawBase bool
	for _, label := range labelsOf(roles) {
		if label == "base" {
			sawBase = true
		}
	}
	if !sawBase {
		t.Errorf("the role key is what the merchant recognises, got %v", labelsOf(roles))
	}
}

// A merchant who has only ever used the admin pages has written no edges, and
// their model is entirely in the grant store.
//
// ProjectMerchant is what moves that store into the edge table and nothing in
// the running system calls it, so reading reach_edges alone drew a map with no
// roles and no scopes on it. That is the complaint the instance layer exists to
// answer, and this is the test that fails if the extra sources are dropped.
func TestInstancesSeeAModelBuiltOnlyFromTheAdminPages(t *testing.T) {
	f := newAppAccess(t, "mstore")
	f.declareScope(t, "project")
	f.declareCapability(t, "deploy:read")

	roleID := f.createRole(t, "maintainer", []string{"deploy:read"}, nil)
	userID := f.createAppUser(t, "ana@customer.com")
	if code, out := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "role_id": roleID,
		"scope_kind": "project", "scope_id": "apollo",
	}); code != http.StatusCreated {
		t.Fatalf("grant: %d %v", code, out)
	}

	byType := instancesByType(t, mustGet(t, f, "/access-instances"))

	for _, want := range []struct{ typ, label string }{
		{"role", "maintainer"},
		{"project", "apollo"},
	} {
		found := false
		for _, got := range labelsOf(byType[want.typ]) {
			if got == want.label {
				found = true
			}
		}
		if !found {
			t.Errorf("no %s named %q on the map: %v", want.typ, want.label, byType[want.typ])
		}
	}
}

// The same scope named by a grant and by an edge is one scope. UNION ALL would
// count it twice and the cap notice would quote a number the merchant cannot
// reconcile with their own product.
func TestInstancesCountASharedNodeOnce(t *testing.T) {
	f := newAppAccess(t, "mdedup")
	f.declareScope(t, "project")
	f.declareCapability(t, "project:read")

	roleID := f.createRole(t, "reader", []string{"project:read"}, nil)
	userID := f.createAppUser(t, "bo@customer.com")
	if code, out := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "role_id": roleID,
		"scope_kind": "project", "scope_id": "apollo",
	}); code != http.StatusCreated {
		t.Fatalf("grant: %d %v", code, out)
	}
	// The same project, now also named by an edge.
	f.writeEdges(t, map[string]any{
		"object_type": "project", "object_id": "apollo", "relation": "can_read",
		"subject_type": "app_user", "subject_id": userID,
	})

	projects := instancesByType(t, mustGet(t, f, "/access-instances"))["project"]
	if total, _ := projects["total"].(float64); int(total) != 1 {
		t.Errorf("one project counted %v times", projects["total"])
	}
	if got := len(labelsOf(projects)); got != 1 {
		t.Errorf("one project drawn %d times: %v", got, labelsOf(projects))
	}
}

func mustGet(t *testing.T, f appAccessFixture, path string) map[string]any {
	t.Helper()
	code, out := f.get(t, path)
	if code != http.StatusOK {
		t.Fatalf("get %s: %d %v", path, code, out)
	}
	return out
}

// The cap, and the count that keeps it honest.
//
// A capped type reads as a complete one unless something says otherwise, so the
// property under test is not that the list is short. It is that total reports
// what is really there and truncated admits the difference.
func TestInstancesCapPerTypeAndSayThatTheyDid(t *testing.T) {
	f := newAppAccess(t, "mcap")
	f.declareScope(t, "project")
	f.declareCapability(t, "document:read")

	const written = service.MerchantInstanceCap + 12
	edges := make([]map[string]any, 0, written)
	for i := 0; i < written; i++ {
		edges = append(edges, map[string]any{
			"object_type": "document", "object_id": fmt.Sprintf("d%03d", i),
			"relation": "can_read", "subject_type": "app_user", "subject_id": "ana",
		})
	}
	f.writeEdges(t, edges...)

	_, out := f.get(t, "/access-instances")
	if cap, _ := out["cap"].(float64); int(cap) != service.MerchantInstanceCap {
		t.Errorf("cap = %v, want %d", out["cap"], service.MerchantInstanceCap)
	}

	docs := instancesByType(t, out)["document"]
	if got := len(labelsOf(docs)); got != service.MerchantInstanceCap {
		t.Errorf("drew %d documents, want the cap of %d", got, service.MerchantInstanceCap)
	}
	if total, _ := docs["total"].(float64); int(total) != written {
		t.Errorf("total = %v, want the true count %d", docs["total"], written)
	}
	if truncated, _ := docs["truncated"].(bool); !truncated {
		t.Error("a capped type that does not say so reads as a complete one")
	}

	// A type under the cap must not claim to be trimmed, or the notice appears
	// on every map and stops being read.
	if projects, ok := instancesByType(t, out)["app_user"]; ok {
		if truncated, _ := projects["truncated"].(bool); truncated {
			t.Error("a type under the cap was reported as truncated")
		}
	}
}
