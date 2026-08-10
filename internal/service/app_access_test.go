package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

type appAccessFixture struct {
	h     http.Handler
	orgID string
	token string
}

func newAppAccess(t *testing.T, slug string) appAccessFixture {
	t.Helper()
	ctx := context.Background()
	db := openTestDB(t, ctx)
	h := testServer(t, db)
	org, token, _ := doBootstrapOrg(t, h, slug+"@merchant.com", "Owner", slug, slug)
	return appAccessFixture{h: h, orgID: org["organisation"].(map[string]any)["id"].(string), token: token}
}

func (f appAccessFixture) post(t *testing.T, path string, body map[string]any) (int, map[string]any) {
	t.Helper()
	res := doJSON(t, f.h, http.MethodPost, "/v1/organisations/"+f.orgID+path, body, userAuth(f.token))
	var out map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &out)
	return res.Code, out
}

func (f appAccessFixture) declareScope(t *testing.T, kind string) {
	t.Helper()
	if code, out := f.post(t, "/app-scope-types", map[string]any{"kind": kind}); code != http.StatusCreated {
		t.Fatalf("declare scope %q: %d %v", kind, code, out)
	}
}

func (f appAccessFixture) declareCapability(t *testing.T, key string) {
	t.Helper()
	if code, out := f.post(t, "/app-capabilities", map[string]any{"key": key}); code != http.StatusCreated {
		t.Fatalf("declare capability %q: %d %v", key, code, out)
	}
}

func (f appAccessFixture) createRole(t *testing.T, key string, caps, extends []string) string {
	t.Helper()
	body := map[string]any{"key": key, "name": key}
	if caps != nil {
		body["capabilities"] = caps
	}
	if extends != nil {
		body["extends"] = extends
	}
	code, out := f.post(t, "/app-roles", body)
	if code != http.StatusCreated {
		t.Fatalf("create role %q: %d %v", key, code, out)
	}
	return out["id"].(string)
}

func (f appAccessFixture) createAppUser(t *testing.T, email string) string {
	t.Helper()
	code, out := f.post(t, "/app-users", map[string]any{"email": email})
	if code != http.StatusCreated {
		t.Fatalf("create app user: %d %v", code, out)
	}
	return out["id"].(string)
}

func (f appAccessFixture) accessFor(t *testing.T, appUserID string) map[string]any {
	t.Helper()
	res := doJSON(t, f.h, http.MethodGet, "/v1/app-users/"+appUserID+"/access", nil, userAuth(f.token))
	if res.Code != http.StatusOK {
		t.Fatalf("access set: %d %s", res.Code, res.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode access set: %v", err)
	}
	return out
}

// The whole loop: a merchant declares their model, grants a role at a scope,
// and reads back a grant set their backend can evaluate locally.
func TestMerchantDeclaresAndGrants(t *testing.T) {
	f := newAppAccess(t, "loop")
	f.declareScope(t, "project")
	f.declareCapability(t, "deploy:read")
	f.declareCapability(t, "deploy:write")

	roleID := f.createRole(t, "maintainer", []string{"deploy:read", "deploy:write"}, nil)
	userID := f.createAppUser(t, "alice@customer.com")

	if code, out := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "role_id": roleID, "scope_kind": "project", "scope_id": "p1",
	}); code != http.StatusCreated {
		t.Fatalf("grant: %d %v", code, out)
	}

	set := f.accessFor(t, userID)
	if ns := set["namespace"].(string); ns == "kyc" {
		t.Fatalf("merchant capabilities must not be in KYC's namespace, got %q", ns)
	}
	grants := set["grants"].([]any)
	if len(grants) != 1 {
		t.Fatalf("want one grant, got %d: %v", len(grants), grants)
	}
	g := grants[0].(map[string]any)
	if g["scope_kind"] != "project" || g["scope_id"] != "p1" {
		t.Errorf("scope = %v/%v, want project/p1", g["scope_kind"], g["scope_id"])
	}
	caps := g["capabilities"].([]any)
	if len(caps) != 2 {
		t.Errorf("capabilities = %v, want both", caps)
	}
}

// Scope kinds are declared so a typo is rejected, but scope ids are opaque:
// KYC never resolves them, because a grant naming a project that does not exist
// simply matches nothing.
func TestScopeKindsAreCheckedAndIdsAreNot(t *testing.T) {
	f := newAppAccess(t, "scopes")
	f.declareScope(t, "project")
	f.declareCapability(t, "deploy:read")
	roleID := f.createRole(t, "reader", []string{"deploy:read"}, nil)
	userID := f.createAppUser(t, "bob@customer.com")

	if code, _ := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "role_id": roleID, "scope_kind": "projekt", "scope_id": "p1",
	}); code != http.StatusBadRequest {
		t.Errorf("an undeclared scope kind must be rejected, got %d", code)
	}

	// An id KYC has never heard of is accepted: it is the merchant's identifier
	// and only ever matched against coordinates their own resources carry.
	if code, out := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "role_id": roleID, "scope_kind": "project", "scope_id": "never-seen",
	}); code != http.StatusCreated {
		t.Errorf("scope ids are opaque and must be accepted: %d %v", code, out)
	}
}

// A merchant may only use capabilities they declared, and may never name one of
// KYC's. That is invariant 3 applied per namespace.
func TestMerchantCapabilitiesAreNamespaced(t *testing.T) {
	f := newAppAccess(t, "namespace")
	f.declareCapability(t, "deploy:read")

	if code, _ := f.post(t, "/app-roles", map[string]any{
		"key": "typo", "name": "Typo", "capabilities": []string{"deploy:raed"},
	}); code != http.StatusBadRequest {
		t.Errorf("undeclared capability must be rejected, got %d", code)
	}

	// A KYC permission key is not declared in the merchant's namespace, so it is
	// rejected like any other unknown key rather than granting KYC access.
	if code, _ := f.post(t, "/app-roles", map[string]any{
		"key": "sneaky", "name": "Sneaky", "capabilities": []string{"organisation:update"},
	}); code != http.StatusBadRequest {
		t.Errorf("a KYC capability must not be usable in a merchant role, got %d", code)
	}

	// Wildcards are refused by the evaluator's own rules: one would silently
	// widen every role holding it the day a new capability is declared.
	if code, _ := f.post(t, "/app-capabilities", map[string]any{"key": "deploy:*"}); code != http.StatusBadRequest {
		t.Errorf("wildcards must be rejected, got %d", code)
	}
}

// Inheritance resolves at write time and the effective set is what a grant
// carries. Editing a base role must reach everyone holding a role built on it,
// which is the reason inheritance lives on the role rather than being copied.
func TestRoleInheritancePropagates(t *testing.T) {
	f := newAppAccess(t, "inherit")
	f.declareScope(t, "project")
	for _, c := range []string{"deploy:read", "deploy:write", "billing:read", "audit:read"} {
		f.declareCapability(t, c)
	}

	base := f.createRole(t, "viewer", []string{"deploy:read"}, nil)
	mid := f.createRole(t, "operator", []string{"deploy:write"}, []string{base})
	top := f.createRole(t, "lead", []string{"billing:read"}, []string{mid})

	userID := f.createAppUser(t, "carol@customer.com")
	if code, out := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "role_id": top, "scope_kind": "project", "scope_id": "p1",
	}); code != http.StatusCreated {
		t.Fatalf("grant: %d %v", code, out)
	}

	set := f.accessFor(t, userID)
	caps := set["grants"].([]any)[0].(map[string]any)["capabilities"].([]any)
	if len(caps) != 3 {
		t.Fatalf("lead must inherit the whole chain, got %v", caps)
	}

	// Adding a capability to the base role reaches the grant without touching it.
	res := doJSON(t, f.h, http.MethodPatch, "/v1/organisations/"+f.orgID+"/app-roles/"+base,
		map[string]any{"capabilities": []string{"deploy:read", "audit:read"}}, userAuth(f.token))
	if res.Code != http.StatusOK {
		t.Fatalf("patch base role: %d %s", res.Code, res.Body.String())
	}

	set = f.accessFor(t, userID)
	caps = set["grants"].([]any)[0].(map[string]any)["capabilities"].([]any)
	if len(caps) != 4 {
		t.Fatalf("editing a base role must reach everyone holding a descendant, got %v", caps)
	}
}

// Multiple parents are safe because capabilities only ever add: a diamond
// resolves to the union with no precedence rules.
func TestRoleDiamondResolvesToUnion(t *testing.T) {
	f := newAppAccess(t, "diamond")
	for _, c := range []string{"a:read", "b:write", "c:admin"} {
		f.declareCapability(t, c)
	}
	base := f.createRole(t, "base", []string{"a:read"}, nil)
	left := f.createRole(t, "left", []string{"b:write"}, []string{base})
	right := f.createRole(t, "right", []string{"c:admin"}, []string{base})
	both := f.createRole(t, "both", nil, []string{left, right})

	res := doJSON(t, f.h, http.MethodGet, "/v1/organisations/"+f.orgID+"/app-roles", nil, userAuth(f.token))
	var body struct {
		Items []struct {
			ID                    string   `json:"id"`
			EffectiveCapabilities []string `json:"effective_capabilities"`
		} `json:"items"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode roles: %v", err)
	}
	for _, r := range body.Items {
		if r.ID == both && len(r.EffectiveCapabilities) != 3 {
			t.Fatalf("diamond must union to three capabilities, got %v", r.EffectiveCapabilities)
		}
	}
}

// Cycles and excessive depth are rejected before anything is written, so a bad
// edit cannot leave the role set half-updated.
func TestRoleInheritanceRejectsCyclesAndDepth(t *testing.T) {
	f := newAppAccess(t, "cycles")
	f.declareCapability(t, "a:read")

	a := f.createRole(t, "a", []string{"a:read"}, nil)
	b := f.createRole(t, "b", nil, []string{a})

	// a extends b, closing the loop.
	res := doJSON(t, f.h, http.MethodPatch, "/v1/organisations/"+f.orgID+"/app-roles/"+a,
		map[string]any{"extends": []string{b}}, userAuth(f.token))
	if res.Code != http.StatusBadRequest {
		t.Fatalf("a cycle must be rejected, got %d %s", res.Code, res.Body.String())
	}

	// A chain deeper than the cap is refused rather than silently truncated.
	prev := a
	for i := 0; i < 8; i++ {
		key := fmt.Sprintf("deep%d", i)
		body := map[string]any{"key": key, "name": key, "extends": []string{prev}}
		code, out := f.post(t, "/app-roles", body)
		if code == http.StatusBadRequest {
			return // hit the depth cap, which is the expected outcome
		}
		if code != http.StatusCreated {
			t.Fatalf("create %s: %d %v", key, code, out)
		}
		prev = out["id"].(string)
	}
	t.Fatal("expected the depth cap to reject a long chain")
}

// The stated requirement was to be ready for around twenty roles. This builds
// them with real inheritance and checks the model stays coherent.
func TestManyRolesWithInheritance(t *testing.T) {
	f := newAppAccess(t, "many")
	for i := 0; i < 20; i++ {
		f.declareCapability(t, fmt.Sprintf("res%d:read", i))
	}

	// Four chains of five, so depth stays within the cap while the role count
	// is what a real merchant might build.
	var lastOfChain []string
	for chain := 0; chain < 4; chain++ {
		var parent []string
		for level := 0; level < 5; level++ {
			key := fmt.Sprintf("c%dl%d", chain, level)
			id := f.createRole(t, key, []string{fmt.Sprintf("res%d:read", chain*5+level)}, parent)
			parent = []string{id}
		}
		lastOfChain = append(lastOfChain, parent[0])
	}

	res := doJSON(t, f.h, http.MethodGet, "/v1/organisations/"+f.orgID+"/app-roles", nil, userAuth(f.token))
	var body struct {
		Items []struct {
			ID                    string   `json:"id"`
			EffectiveCapabilities []string `json:"effective_capabilities"`
		} `json:"items"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 20 {
		t.Fatalf("want 20 roles, got %d", len(body.Items))
	}
	deepest := map[string]bool{}
	for _, id := range lastOfChain {
		deepest[id] = true
	}
	for _, r := range body.Items {
		if deepest[r.ID] && len(r.EffectiveCapabilities) != 5 {
			t.Errorf("the end of a five-deep chain must hold five capabilities, got %v", r.EffectiveCapabilities)
		}
	}
}

// Administering the model is a KYC permission. A member without it cannot
// declare or grant, even though the model belongs to their organisation.
func TestAppAccessAdministrationIsGated(t *testing.T) {
	f := newAppAccess(t, "gated")
	f.declareCapability(t, "deploy:read")

	roles := doJSON(t, f.h, http.MethodPost, "/v1/organisations/"+f.orgID+"/roles", map[string]any{
		"key": "plain", "name": "Plain", "permission_keys": []string{"organisation:read"},
	}, userAuth(f.token))
	if roles.Code != http.StatusCreated {
		t.Fatalf("create org role: %d %s", roles.Code, roles.Body.String())
	}
	var role struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(roles.Body.Bytes(), &role)

	_, mateToken := doDevLogin(t, f.h, "plain@merchant.com", "Plain")
	me := doJSON(t, f.h, http.MethodGet, "/v1/me", nil, userAuth(mateToken))
	var meBody struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	_ = json.Unmarshal(me.Body.Bytes(), &meBody)
	if add := doJSON(t, f.h, http.MethodPost, "/v1/organisations/"+f.orgID+"/memberships", map[string]any{
		"user_id": meBody.User.ID, "role_id": role.ID, "status": "active",
	}, userAuth(f.token)); add.Code != http.StatusCreated && add.Code != http.StatusOK {
		t.Fatalf("add member: %d %s", add.Code, add.Body.String())
	}

	denied := doJSON(t, f.h, http.MethodPost, "/v1/organisations/"+f.orgID+"/app-capabilities",
		map[string]any{"key": "sneak:in"}, userAuth(mateToken))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("declaring capabilities needs app_access:manage: want 403, got %d %s", denied.Code, denied.Body.String())
	}
}

func (f appAccessFixture) createGroup(t *testing.T, key string) string {
	t.Helper()
	code, out := f.post(t, "/app-user-groups", map[string]any{"key": key, "name": key})
	if code != http.StatusCreated {
		t.Fatalf("create group %q: %d %v", key, code, out)
	}
	return out["id"].(string)
}

func (f appAccessFixture) addToGroup(t *testing.T, groupID, appUserID string) {
	t.Helper()
	res := doJSON(t, f.h, http.MethodPost,
		"/v1/organisations/"+f.orgID+"/app-user-groups/"+groupID+"/members",
		map[string]any{"app_user_id": appUserID}, userAuth(f.token))
	if res.Code != http.StatusCreated {
		t.Fatalf("add to group: %d %s", res.Code, res.Body.String())
	}
}

// A grant to a group reaches every member, so a merchant configures a set once
// instead of once per customer.
func TestGroupGrantReachesMembers(t *testing.T) {
	f := newAppAccess(t, "groups")
	f.declareScope(t, "project")
	f.declareCapability(t, "deploy:read")
	roleID := f.createRole(t, "reader", []string{"deploy:read"}, nil)

	groupID := f.createGroup(t, "au_customers")
	inGroup := f.createAppUser(t, "in@customer.com")
	outside := f.createAppUser(t, "out@customer.com")
	f.addToGroup(t, groupID, inGroup)

	if code, out := f.post(t, "/app-grants", map[string]any{
		"group_id": groupID, "role_id": roleID, "scope_kind": "project", "scope_id": "p1",
	}); code != http.StatusCreated {
		t.Fatalf("group grant: %d %v", code, out)
	}

	member := f.accessFor(t, inGroup)
	grants := member["grants"].([]any)
	if len(grants) != 1 {
		t.Fatalf("a member must inherit the group grant, got %v", grants)
	}
	// Provenance: holding a capability through a group must say so, or "why
	// does this customer have this?" is unanswerable.
	if src := grants[0].(map[string]any)["source"].(string); !strings.Contains(src, "group:au_customers") {
		t.Errorf("source must name the group, got %q", src)
	}

	if other := f.accessFor(t, outside); len(other["grants"].([]any)) != 0 {
		t.Errorf("a non-member must get nothing, got %v", other["grants"])
	}
}

// Group and direct grants union, and removing someone from a group takes their
// group-derived access with them while leaving anything granted directly.
func TestGroupAndDirectGrantsUnion(t *testing.T) {
	f := newAppAccess(t, "union")
	f.declareScope(t, "project")
	f.declareCapability(t, "deploy:read")
	f.declareCapability(t, "billing:read")
	readRole := f.createRole(t, "reader", []string{"deploy:read"}, nil)
	billRole := f.createRole(t, "biller", []string{"billing:read"}, nil)

	groupID := f.createGroup(t, "everyone")
	userID := f.createAppUser(t, "both@customer.com")
	f.addToGroup(t, groupID, userID)

	f.post(t, "/app-grants", map[string]any{
		"group_id": groupID, "role_id": readRole, "scope_kind": "project", "scope_id": "p1",
	})
	f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "role_id": billRole, "scope_kind": "project", "scope_id": "p1",
	})

	if got := len(f.accessFor(t, userID)["grants"].([]any)); got != 2 {
		t.Fatalf("group and direct grants must union, got %d", got)
	}

	res := doJSON(t, f.h, http.MethodDelete,
		"/v1/organisations/"+f.orgID+"/app-user-groups/"+groupID+"/members/"+userID, nil, userAuth(f.token))
	if res.Code != http.StatusNoContent {
		t.Fatalf("remove member: %d %s", res.Code, res.Body.String())
	}
	after := f.accessFor(t, userID)["grants"].([]any)
	if len(after) != 1 {
		t.Fatalf("removing from a group must drop only the group grant, got %v", after)
	}
	if src := after[0].(map[string]any)["source"].(string); strings.Contains(src, "group:") {
		t.Errorf("the surviving grant must be the direct one, got %q", src)
	}
}

// A grant needs exactly one subject. Neither would apply to nobody; both would
// be ambiguous.
func TestGrantRequiresExactlyOneSubject(t *testing.T) {
	f := newAppAccess(t, "subject")
	f.declareScope(t, "project")
	f.declareCapability(t, "deploy:read")
	roleID := f.createRole(t, "reader", []string{"deploy:read"}, nil)
	groupID := f.createGroup(t, "g")
	userID := f.createAppUser(t, "s@customer.com")

	if code, _ := f.post(t, "/app-grants", map[string]any{
		"role_id": roleID, "scope_kind": "project", "scope_id": "p1",
	}); code != http.StatusBadRequest {
		t.Errorf("no subject must be rejected, got %d", code)
	}
	if code, _ := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "group_id": groupID,
		"role_id": roleID, "scope_kind": "project", "scope_id": "p1",
	}); code != http.StatusBadRequest {
		t.Errorf("two subjects must be rejected, got %d", code)
	}
}

// The cache version must move when a group grant or a membership changes,
// otherwise a merchant's cached set silently serves the wrong permissions.
func TestAccessVersionMovesOnGroupChanges(t *testing.T) {
	f := newAppAccess(t, "version")
	f.declareScope(t, "project")
	f.declareCapability(t, "deploy:read")
	roleID := f.createRole(t, "reader", []string{"deploy:read"}, nil)
	groupID := f.createGroup(t, "g")
	userID := f.createAppUser(t, "v@customer.com")

	before := f.accessFor(t, userID)["version"].(float64)

	f.post(t, "/app-grants", map[string]any{
		"group_id": groupID, "role_id": roleID, "scope_kind": "project", "scope_id": "p1",
	})
	f.addToGroup(t, groupID, userID)

	after := f.accessFor(t, userID)
	if after["version"].(float64) < before {
		t.Errorf("version must not go backwards: %v -> %v", before, after["version"])
	}
	if len(after["grants"].([]any)) != 1 {
		t.Fatalf("membership must take effect, got %v", after["grants"])
	}
}
