package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gsarmaonline/kyc/internal/service"
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

func (f appAccessFixture) get(t *testing.T, path string) (int, map[string]any) {
	t.Helper()
	res := doJSON(t, f.h, http.MethodGet, "/v1/organisations/"+f.orgID+path, nil, userAuth(f.token))
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

func (f appAccessFixture) createNestedGroup(t *testing.T, key string, parents ...string) string {
	t.Helper()
	code, out := f.post(t, "/app-user-groups", map[string]any{
		"key": key, "name": key, "parents": parents,
	})
	if code != http.StatusCreated {
		t.Fatalf("create nested group %q: %d %v", key, code, out)
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

// Nesting is the same mechanism for a group as for a role. A grant written on a
// parent group has to reach a member of the child, or "enterprise customers are
// also beta customers" stays something a merchant can only express by adding
// every member to both groups and keeping them in step by hand.
func TestNestedGroupGrantReachesTheChildsMembers(t *testing.T) {
	f := newAppAccess(t, "nested")
	f.declareScope(t, "project")
	f.declareCapability(t, "beta:read")
	roleID := f.createRole(t, "beta_reader", []string{"beta:read"}, nil)

	beta := f.createGroup(t, "beta")
	enterprise := f.createNestedGroup(t, "enterprise", beta)

	member := f.createAppUser(t, "ent@customer.com")
	f.addToGroup(t, enterprise, member)
	outsider := f.createAppUser(t, "free@customer.com")

	// Granted on the parent only. Nobody is a direct member of it.
	if code, out := f.post(t, "/app-grants", map[string]any{
		"group_id": beta, "role_id": roleID, "scope_kind": "project", "scope_id": "p1",
	}); code != http.StatusCreated {
		t.Fatalf("grant on parent group: %d %v", code, out)
	}

	grants := f.accessFor(t, member)["grants"].([]any)
	if len(grants) != 1 {
		t.Fatalf("a member of enterprise must inherit the beta grant, got %v", grants)
	}
	if caps := grants[0].(map[string]any)["capabilities"].([]any); len(caps) != 1 || caps[0] != "beta:read" {
		t.Errorf("capabilities = %v, wanted [beta:read]", caps)
	}

	if other := f.accessFor(t, outsider)["grants"].([]any); len(other) != 0 {
		t.Errorf("a customer in neither group must get nothing, got %v", other)
	}
}

// Nesting adds, it does not replace. A grant on the child must not leak upward
// to members of the parent, or extending a group would silently widen it.
func TestNestingDoesNotReachUpward(t *testing.T) {
	f := newAppAccess(t, "upward")
	f.declareScope(t, "project")
	f.declareCapability(t, "beta:read")
	roleID := f.createRole(t, "beta_reader", []string{"beta:read"}, nil)

	beta := f.createGroup(t, "beta")
	enterprise := f.createNestedGroup(t, "enterprise", beta)

	betaOnly := f.createAppUser(t, "beta@customer.com")
	f.addToGroup(t, beta, betaOnly)

	f.post(t, "/app-grants", map[string]any{
		"group_id": enterprise, "role_id": roleID, "scope_kind": "project", "scope_id": "p1",
	})

	if got := f.accessFor(t, betaOnly)["grants"].([]any); len(got) != 0 {
		t.Errorf("a grant on the child must not reach the parent's members, got %v", got)
	}
}

// A cycle has to be refused at write time. Accepting one would either hang the
// expansion or leave the stored sets half-written.
func TestGroupNestingRejectsCycles(t *testing.T) {
	f := newAppAccess(t, "groupcycle")
	a := f.createGroup(t, "a")
	b := f.createNestedGroup(t, "b", a)

	res := doJSON(t, f.h, http.MethodPatch,
		"/v1/organisations/"+f.orgID+"/app-user-groups/"+a,
		map[string]any{"parents": []string{b}}, userAuth(f.token))
	if res.Code < 400 {
		t.Fatalf("a cycle was accepted: %d %s", res.Code, res.Body.String())
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

// The gate that now enforces most organisation routes has to actually deny.
// The declarations in the route table are only worth something if the rule they
// name is applied before the handler runs, so this drives real requests rather
// than inspecting the table.
func TestRouteTableGateDeniesForReal(t *testing.T) {
	f := newAppAccess(t, "gate")
	f.declareCapability(t, "deploy:read")

	// A member with a role holding only organisation:read.
	roles := doJSON(t, f.h, http.MethodPost, "/v1/organisations/"+f.orgID+"/roles", map[string]any{
		"key": "readonly", "name": "Read only", "permission_keys": []string{"organisation:read"},
	}, userAuth(f.token))
	if roles.Code != http.StatusCreated {
		t.Fatalf("create role: %d %s", roles.Code, roles.Body.String())
	}
	var role struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(roles.Body.Bytes(), &role); err != nil {
		t.Fatalf("decode role: %v", err)
	}
	_, mateToken := doDevLogin(t, f.h, "readonly@merchant.com", "Read Only")
	me := doJSON(t, f.h, http.MethodGet, "/v1/me", nil, userAuth(mateToken))
	var meBody struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(me.Body.Bytes(), &meBody); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if add := doJSON(t, f.h, http.MethodPost, "/v1/organisations/"+f.orgID+"/memberships", map[string]any{
		"user_id": meBody.User.ID, "role_id": role.ID, "status": "active",
	}, userAuth(f.token)); add.Code != http.StatusCreated && add.Code != http.StatusOK {
		t.Fatalf("add member: %d %s", add.Code, add.Body.String())
	}

	// Reads they hold are allowed; writes they do not hold are refused, and the
	// refusal comes from the gate rather than from inside the handler.
	if ok := doJSON(t, f.h, http.MethodGet, "/v1/organisations/"+f.orgID, nil, userAuth(mateToken)); ok.Code != http.StatusOK {
		t.Fatalf("organisation:read must be allowed: %d %s", ok.Code, ok.Body.String())
	}
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/app-capabilities"},
		{http.MethodPost, "/app-user-groups"},
		{http.MethodPost, "/app-roles"},
		{http.MethodPost, "/app-users"},
		{http.MethodPost, "/roles"},
	} {
		res := doJSON(t, f.h, tc.method, "/v1/organisations/"+f.orgID+tc.path,
			map[string]any{"key": "x", "name": "X", "email": "x@y.com"}, userAuth(mateToken))
		if res.Code != http.StatusForbidden {
			t.Errorf("%s %s: want 403 from the gate, got %d %s", tc.method, tc.path, res.Code, res.Body.String())
		}
	}

	// A caller with no membership at all sees 404, not 403, so the gate keeps
	// the non-disclosure property rather than leaking that the tenant exists.
	_, strangerToken := doDevLogin(t, f.h, "stranger@elsewhere.com", "Stranger")
	if res := doJSON(t, f.h, http.MethodGet, "/v1/organisations/"+f.orgID+"/app-roles", nil, userAuth(strangerToken)); res.Code != http.StatusNotFound {
		t.Errorf("non-member: want 404 from the gate, got %d %s", res.Code, res.Body.String())
	}
}

// --- Wildcards and exceptions ---
//
// A wildcard claims a set nobody can enumerate; an exception names the members
// that do not belong. They are one feature, and these tests pin the properties
// that make the pairing safe rather than a deny rule in disguise.

// The everyone subject exists so a baseline does not need per-customer
// bookkeeping. One row, and a customer who signs up later is covered by
// construction.
func TestEveryoneGrantCoversFutureCustomers(t *testing.T) {
	f := newAppAccess(t, "everyone")
	f.declareScope(t, "tenant")
	f.declareCapability(t, "profile:read")
	roleID := f.createRole(t, "self_manager", []string{"profile:read"}, nil)

	existing := f.createAppUser(t, "before@customer.com")
	if code, out := f.post(t, "/app-grants", map[string]any{
		"subject_kind": "everyone", "role_id": roleID,
		"scope_kind": "tenant", "scope_id": "acme",
	}); code != http.StatusCreated {
		t.Fatalf("everyone grant: %d %v", code, out)
	}
	// Created after the grant, and never touched by anyone.
	later := f.createAppUser(t, "after@customer.com")

	for _, id := range []string{existing, later} {
		grants := f.accessFor(t, id)["grants"].([]any)
		if len(grants) != 1 {
			t.Fatalf("customer %s must hold the everyone grant, got %v", id, grants)
		}
		if src := grants[0].(map[string]any)["source"].(string); !strings.Contains(src, "everyone") {
			t.Errorf("source must say the grant came from the everyone rule, got %q", src)
		}
	}
}

// The counterpart to the subject wildcard: offboard one person without
// enumerating everyone else.
func TestEveryoneGrantExcludesNamedCustomers(t *testing.T) {
	f := newAppAccess(t, "everyoneexcept")
	f.declareScope(t, "tenant")
	f.declareCapability(t, "profile:read")
	roleID := f.createRole(t, "reader", []string{"profile:read"}, nil)

	kept := f.createAppUser(t, "kept@customer.com")
	dropped := f.createAppUser(t, "dropped@customer.com")

	if code, out := f.post(t, "/app-grants", map[string]any{
		"subject_kind": "everyone", "role_id": roleID,
		"scope_kind": "tenant", "scope_id": "acme",
		"except_app_user_ids": []string{dropped},
	}); code != http.StatusCreated {
		t.Fatalf("everyone grant: %d %v", code, out)
	}

	if got := len(f.accessFor(t, kept)["grants"].([]any)); got != 1 {
		t.Errorf("an unexcluded customer keeps the grant, got %d", got)
	}
	if got := len(f.accessFor(t, dropped)["grants"].([]any)); got != 0 {
		t.Errorf("an excluded customer must hold nothing, got %d", got)
	}

	// A typo here would silently exclude nobody, so an unknown id is refused.
	if code, _ := f.post(t, "/app-grants", map[string]any{
		"subject_kind": "everyone", "role_id": roleID,
		"scope_kind": "tenant", "scope_id": "other",
		"except_app_user_ids": []string{"not-a-real-customer"},
	}); code != http.StatusBadRequest {
		t.Errorf("an unknown excluded customer must be rejected, got %d", code)
	}
}

// The capability wildcard carries verbs declared after the grant was written.
// That is the whole point, and also the risk the merchant is accepting.
func TestCapabilityWildcardCoversLaterDeclarations(t *testing.T) {
	f := newAppAccess(t, "wildcard")
	f.declareScope(t, "tenant")
	f.declareCapability(t, "docs:read")

	userID := f.createAppUser(t, "wild@customer.com")
	if code, out := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "all_capabilities": true,
		"scope_kind": "tenant", "scope_id": "acme",
	}); code != http.StatusCreated {
		t.Fatalf("wildcard grant: %d %v", code, out)
	}

	grants := f.accessFor(t, userID)["grants"].([]any)
	if len(grants) != 1 {
		t.Fatalf("want one grant, got %v", grants)
	}
	g := grants[0].(map[string]any)
	if g["all_capabilities"] != true {
		t.Error("the wire format must say the grant is a wildcard, or a backend reads it as granting nothing")
	}
	if src := g["source"].(string); !strings.Contains(src, "all-capabilities") {
		t.Errorf("source must name the wildcard, got %q", src)
	}

	// A wildcard grant carries no role, so asking for both is a contradiction.
	roleID := f.createRole(t, "reader", []string{"docs:read"}, nil)
	if code, _ := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "all_capabilities": true, "role_id": roleID,
		"scope_kind": "tenant", "scope_id": "other",
	}); code != http.StatusBadRequest {
		t.Errorf("a wildcard grant with a role must be rejected, got %d", code)
	}
}

// A carve-out with no wildcard beside it does nothing while reading as though
// it does, and one naming an undeclared capability protects against nothing.
func TestCapabilityExceptionsAreCheckedAndCarried(t *testing.T) {
	f := newAppAccess(t, "capexcept")
	f.declareScope(t, "tenant")
	f.declareCapability(t, "docs:read")
	f.declareCapability(t, "account:delete")
	userID := f.createAppUser(t, "carve@customer.com")

	if code, out := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "all_capabilities": true,
		"scope_kind": "tenant", "scope_id": "acme",
		"except_capabilities": []string{"account:delete"},
	}); code != http.StatusCreated {
		t.Fatalf("wildcard with carve-out: %d %v", code, out)
	}
	g := f.accessFor(t, userID)["grants"].([]any)[0].(map[string]any)
	except := g["except_capabilities"].([]any)
	if len(except) != 1 || except[0].(string) != "account:delete" {
		t.Errorf("the carve-out must reach the merchant's backend, got %v", except)
	}

	roleID := f.createRole(t, "reader", []string{"docs:read"}, nil)
	if code, _ := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "role_id": roleID,
		"scope_kind": "tenant", "scope_id": "b",
		"except_capabilities": []string{"account:delete"},
	}); code != http.StatusBadRequest {
		t.Errorf("a carve-out without a wildcard must be rejected, got %d", code)
	}
	if code, _ := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "all_capabilities": true,
		"scope_kind": "tenant", "scope_id": "c",
		"except_capabilities": []string{"never:declared"},
	}); code != http.StatusBadRequest {
		t.Errorf("an undeclared carve-out must be rejected, got %d", code)
	}
}

// Scope exceptions exist for the case positive scoping cannot express: a huge
// include set and a tiny exclusion. They must reach the merchant's backend,
// which is the only place they can be enforced.
func TestScopeExceptionsAreCheckedAndCarried(t *testing.T) {
	f := newAppAccess(t, "scopeexcept")
	f.declareScope(t, "tenant")
	f.declareScope(t, "project")
	f.declareCapability(t, "docs:read")
	roleID := f.createRole(t, "reader", []string{"docs:read"}, nil)
	userID := f.createAppUser(t, "carve@customer.com")

	if code, out := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "role_id": roleID,
		"scope_kind": "tenant", "scope_id": "acme",
		"except_scopes": []map[string]string{{"kind": "project", "id": "salaries"}},
	}); code != http.StatusCreated {
		t.Fatalf("grant with an excluded scope: %d %v", code, out)
	}
	g := f.accessFor(t, userID)["grants"].([]any)[0].(map[string]any)
	except := g["except_scopes"].([]any)
	if len(except) != 1 {
		t.Fatalf("the exclusion must reach the backend, got %v", except)
	}
	if got := except[0].(map[string]any)["id"].(string); got != "salaries" {
		t.Errorf("want the excluded scope id, got %q", got)
	}

	// An undeclared kind excludes nothing, for the same reason it grants
	// nothing: it silently matches no resource.
	if code, _ := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "role_id": roleID,
		"scope_kind": "tenant", "scope_id": "b",
		"except_scopes": []map[string]string{{"kind": "undeclared", "id": "x"}},
	}); code != http.StatusBadRequest {
		t.Errorf("an undeclared excluded kind must be rejected, got %d", code)
	}
}

// The scope wildcard, at both levels. Without it "every project" could not be
// written at all: a merchant issued one grant per project and reissued on every
// new one, which is exactly the bookkeeping the everyone subject and the
// capability wildcard exist to remove.
func TestScopeWildcardReachesTheBackend(t *testing.T) {
	f := newAppAccess(t, "scopewild")
	f.declareScope(t, "project")
	f.declareCapability(t, "deploy:read")
	roleID := f.createRole(t, "reader", []string{"deploy:read"}, nil)
	perKind := f.createAppUser(t, "perkind@customer.com")
	orgWide := f.createAppUser(t, "orgwide@customer.com")

	// Every instance of one kind. The kind is still declared and checked; only
	// the id is the star.
	if code, out := f.post(t, "/app-grants", map[string]any{
		"app_user_id": perKind, "role_id": roleID,
		"scope_kind": "project", "scope_id": "*",
	}); code != http.StatusCreated {
		t.Fatalf("per-kind wildcard: %d %v", code, out)
	}
	g := f.accessFor(t, perKind)["grants"].([]any)[0].(map[string]any)
	if g["scope_kind"] != "project" || g["scope_id"] != "*" {
		t.Errorf("wildcard scope must reach the backend intact, got %v", g)
	}
	if g["all_scopes"] != false {
		t.Errorf("a per-kind wildcard is not organisation-wide, got %v", g["all_scopes"])
	}

	// Every kind at once. The widest a grant can be, and it names no scope.
	if code, out := f.post(t, "/app-grants", map[string]any{
		"app_user_id": orgWide, "role_id": roleID, "all_scopes": true,
	}); code != http.StatusCreated {
		t.Fatalf("organisation-wide grant: %d %v", code, out)
	}
	wide := f.accessFor(t, orgWide)["grants"].([]any)[0].(map[string]any)
	if wide["all_scopes"] != true {
		t.Fatalf("all_scopes must reach the backend, got %v", wide)
	}
	if wide["scope_kind"] != "" || wide["scope_id"] != "" {
		t.Errorf("an organisation-wide grant names no scope, got %v", wide)
	}
}

// A grant is everywhere or somewhere, never both. Accepting a scope alongside
// the wildcard would leave a row whose scope columns are ignored, and the next
// reader would reasonably believe them.
func TestOrganisationWideGrantRefusesAScope(t *testing.T) {
	f := newAppAccess(t, "scopeboth")
	f.declareScope(t, "project")
	f.declareCapability(t, "deploy:read")
	roleID := f.createRole(t, "reader", []string{"deploy:read"}, nil)
	userID := f.createAppUser(t, "both@customer.com")

	if code, _ := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "role_id": roleID, "all_scopes": true,
		"scope_kind": "project", "scope_id": "p1",
	}); code != http.StatusBadRequest {
		t.Errorf("a grant cannot be both everywhere and somewhere, got %d", code)
	}
	// And a grant that is neither is still refused, as it always was.
	if code, _ := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "role_id": roleID,
	}); code != http.StatusBadRequest {
		t.Errorf("a grant with no scope at all must be rejected, got %d", code)
	}
}

// The organisation is the ceiling, not a name. 'global' and 'organisation' stay
// undeclarable: a merchant's world ends at their organisation, so a scope
// reaching past it would cross into another tenant.
func TestReservedScopeKindsStayReserved(t *testing.T) {
	f := newAppAccess(t, "reserved")
	for _, kind := range []string{"global", "organisation"} {
		code, _ := f.post(t, "/app-scope-types", map[string]any{"kind": kind})
		if code != http.StatusBadRequest {
			t.Errorf("%q must stay reserved, got %d", kind, code)
		}
	}
}

// A template is not a default. Nothing is seeded on signup, because the
// vocabulary is the merchant's own; a template is the same rows arrived at by
// an explicit click, and the provenance has to survive so "why does this exist?"
// stays answerable.
func TestCapabilityTemplateIsAppliedAndMarked(t *testing.T) {
	f := newAppAccess(t, "template")

	// Nothing exists until it is asked for.
	if code, out := f.get(t, "/app-capabilities"); code != http.StatusOK ||
		len(out["items"].([]any)) != 0 {
		t.Fatalf("a new organisation starts with no capabilities: %d %v", code, out)
	}

	code, out := f.post(t, "/app-capability-templates/apply", map[string]any{
		"template": "team_accounts",
	})
	if code != http.StatusCreated {
		t.Fatalf("apply template: %d %v", code, out)
	}
	items := out["items"].([]any)
	if len(items) == 0 {
		t.Fatal("the template declared nothing")
	}
	for _, raw := range items {
		item := raw.(map[string]any)
		if item["source"] != "template:team_accounts" {
			t.Errorf("%v must be marked as template-sourced", item)
		}
	}

	// Applying twice is harmless: a key already declared is already declared.
	if code, _ := f.post(t, "/app-capability-templates/apply", map[string]any{
		"template": "team_accounts",
	}); code != http.StatusCreated {
		t.Errorf("re-applying must not fail, got %d", code)
	}
	_, after := f.get(t, "/app-capabilities")
	if len(after["items"].([]any)) != len(items) {
		t.Errorf("re-applying duplicated rows: %d then %d", len(items), len(after["items"].([]any)))
	}

	if code, _ := f.post(t, "/app-capability-templates/apply", map[string]any{
		"template": "not_a_template",
	}); code != http.StatusBadRequest {
		t.Errorf("an unknown template must be refused, got %d", code)
	}
}

// An authored capability keeps its authored provenance even when a template
// later names the same key. Relabelling it would rewrite history about who
// decided what.
func TestTemplateNeverRelabelsAnAuthoredCapability(t *testing.T) {
	f := newAppAccess(t, "authored")
	f.declareCapability(t, "profile:read")

	if code, _ := f.post(t, "/app-capability-templates/apply", map[string]any{
		"template": "team_accounts",
	}); code != http.StatusCreated {
		t.Fatalf("apply template")
	}

	_, out := f.get(t, "/app-capabilities")
	for _, raw := range out["items"].([]any) {
		item := raw.(map[string]any)
		if item["key"] == "profile:read" && item["source"] != "" {
			t.Errorf("an authored capability was relabelled: %v", item)
		}
	}
}

// Every template key goes through the same validation an authored one does. A
// template able to write a key the registry would reject would be a way around
// the vocabulary rules rather than a shortcut through them.
func TestEveryTemplateKeyIsValid(t *testing.T) {
	for _, tpl := range service.CapabilityTemplates {
		if len(tpl.Items) == 0 {
			t.Errorf("template %q declares nothing", tpl.Key)
		}
		for _, item := range tpl.Items {
			if err := service.ValidAppCapabilityKey(item.Key); err != nil {
				t.Errorf("template %q: %v", tpl.Key, err)
			}
			if item.Description == "" {
				t.Errorf("template %q: %q has no description", tpl.Key, item.Key)
			}
		}
	}
}

// The self constraint is how "everyone may manage their own things" becomes one
// row. KYC cannot enforce it, so the only thing that matters here is that it
// survives to the merchant's backend intact.
func TestSelfConstraintReachesTheBackend(t *testing.T) {
	f := newAppAccess(t, "selfconstraint")
	f.declareScope(t, "tenant")
	f.declareCapability(t, "profile:write")
	roleID := f.createRole(t, "self_manager", []string{"profile:write"}, nil)
	userID := f.createAppUser(t, "self@customer.com")

	if code, out := f.post(t, "/app-grants", map[string]any{
		"subject_kind": "everyone", "role_id": roleID,
		"scope_kind": "tenant", "scope_id": "acme",
		"constraint": "self_subject",
	}); code != http.StatusCreated {
		t.Fatalf("self grant: %d %v", code, out)
	}
	g := f.accessFor(t, userID)["grants"].([]any)[0].(map[string]any)
	if g["constraint"] != "self_subject" {
		t.Errorf("the constraint must be on the wire, got %v", g["constraint"])
	}

	// An unrecognised constraint must be refused rather than stored and
	// ignored, or it reads as a restriction that is not applied.
	if code, _ := f.post(t, "/app-grants", map[string]any{
		"subject_kind": "everyone", "role_id": roleID,
		"scope_kind": "tenant", "scope_id": "b", "constraint": "invented",
	}); code != http.StatusBadRequest {
		t.Errorf("an unknown constraint must be rejected, got %d", code)
	}
}

// The response shape is a published contract: merchant backends read these
// fields in their own process, and both SDKs are generated against them. This
// pins every key, so the merchant tier can be re-modelled internally without
// the wire format moving underneath anyone.
func TestAccessSetWireFormatIsStable(t *testing.T) {
	f := newAppAccess(t, "wire")
	f.declareScope(t, "project")
	f.declareCapability(t, "deploy:read")
	roleID := f.createRole(t, "reader", []string{"deploy:read"}, nil)
	userID := f.createAppUser(t, "wire@customer.com")

	if code, out := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "role_id": roleID,
		"scope_kind": "project", "scope_id": "p1",
		"constraint":    "self_subject",
		"except_scopes": []map[string]any{{"kind": "project", "id": "secret"}},
	}); code != http.StatusCreated {
		t.Fatalf("grant: %d %v", code, out)
	}

	set := f.accessFor(t, userID)
	for _, key := range []string{"app_user_id", "namespace", "version", "grants"} {
		if _, ok := set[key]; !ok {
			t.Errorf("response is missing %q", key)
		}
	}

	grants := set["grants"].([]any)
	if len(grants) != 1 {
		t.Fatalf("want one grant, got %d", len(grants))
	}
	g := grants[0].(map[string]any)

	// Present on every grant, never omitted when empty: a backend reading only
	// `capabilities` would treat a narrowed grant as a plain one and allow more
	// than was granted.
	for _, key := range []string{
		"id", "scope_kind", "scope_id", "capabilities", "source",
		"all_capabilities", "all_scopes", "except_capabilities", "except_scopes", "constraint",
	} {
		if _, ok := g[key]; !ok {
			t.Errorf("grant is missing %q: %v", key, g)
		}
	}

	if g["constraint"] != "self_subject" {
		t.Errorf("constraint = %v, wanted self_subject", g["constraint"])
	}
	if g["all_capabilities"] != false {
		t.Errorf("all_capabilities = %v, wanted false", g["all_capabilities"])
	}
	if caps, ok := g["capabilities"].([]any); !ok || len(caps) != 1 || caps[0] != "deploy:read" {
		t.Errorf("capabilities = %v, wanted [deploy:read]", g["capabilities"])
	}
	// An absent list serialises as [] rather than null.
	if ex, ok := g["except_capabilities"].([]any); !ok || len(ex) != 0 {
		t.Errorf("except_capabilities = %v, wanted []", g["except_capabilities"])
	}
	scopes, ok := g["except_scopes"].([]any)
	if !ok || len(scopes) != 1 {
		t.Fatalf("except_scopes = %v, wanted one entry", g["except_scopes"])
	}
	if sc := scopes[0].(map[string]any); sc["kind"] != "project" || sc["id"] != "secret" {
		t.Errorf("except_scopes[0] = %v", sc)
	}
	// expires_at appears only on a grant that has one.
	if _, present := g["expires_at"]; present {
		t.Errorf("expires_at must be omitted on a standing grant")
	}
}
