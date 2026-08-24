package accessmodel_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/gsarmaonline/kyc/core/reach"
	"github.com/gsarmaonline/kyc/internal/accessmodel"
	"github.com/gsarmaonline/kyc/internal/service"
)

// These tests are the migration's evidence. Each one names a behaviour the
// system needs and shows this model expresses it.

var now = time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

func build(t *testing.T, edges ...reach.Edge) *reach.Evaluator {
	t.Helper()
	schema, err := accessmodel.Load()
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	store := reach.NewMemoryStore()
	if err := store.Write(edges...); err != nil {
		t.Fatalf("write edges: %v", err)
	}
	e, err := reach.New(schema, store)
	if err != nil {
		t.Fatalf("new evaluator: %v", err)
	}
	return e
}

func at(obj reach.NodeRef, relation string, subject reach.SubjectRef) reach.Edge {
	return reach.Edge{Object: obj, Relation: relation, Subject: subject}
}

func until(e reach.Edge, when time.Time) reach.Edge {
	e.ExpiresAt = &when
	return e
}

func can(t *testing.T, e *reach.Evaluator, subject reach.NodeRef, key, orgID string) bool {
	t.Helper()
	p, ok := accessmodel.Permissions[key]
	if !ok {
		t.Fatalf("permission %q is not in the projection table", key)
	}
	d, err := e.Check(context.Background(), reach.Request{
		Subject:  subject,
		Action:   p.Action,
		Resource: accessmodel.Area(p.Type, orgID),
	}, now)
	if err != nil {
		t.Fatalf("check %s %s in %s: %v", subject, key, orgID, err)
	}
	return d.Allowed
}

func mustHold(t *testing.T, e *reach.Evaluator, subject reach.NodeRef, key, orgID string) {
	t.Helper()
	if !can(t, e, subject, key, orgID) {
		t.Fatalf("%s should hold %s in %s", subject, key, orgID)
	}
}

func mustNotHold(t *testing.T, e *reach.Evaluator, subject reach.NodeRef, key, orgID string) {
	t.Helper()
	if can(t, e, subject, key, orgID) {
		t.Fatalf("%s should not hold %s in %s", subject, key, orgID)
	}
}

// --- The migration contract ---

func TestEveryPermissionKeyMaps(t *testing.T) {
	// The current system boots from this registry. A key added on either side
	// without the other must fail here rather than silently lose a gate.
	current := service.KYCCapabilities.Keys()

	var projected []string
	for k := range accessmodel.Permissions {
		projected = append(projected, k)
	}
	sort.Strings(projected)
	sort.Strings(current)

	if len(projected) != len(current) {
		t.Errorf("projection has %d keys, the registry has %d", len(projected), len(current))
	}
	for _, key := range current {
		if _, ok := accessmodel.Permissions[key]; !ok {
			t.Errorf("permission %q has no projection", key)
		}
	}
	for _, key := range projected {
		if !service.KYCCapabilities.Has(key) {
			t.Errorf("projection carries %q, which the registry does not", key)
		}
	}
}

func TestEveryProjectionResolvesInTheSchema(t *testing.T) {
	schema := accessmodel.MustLoad()
	for key, p := range accessmodel.Permissions {
		typ, ok := schema.Type(p.Type)
		if !ok {
			t.Errorf("%s: type %q is not declared", key, p.Type)
			continue
		}
		if _, ok := typ.Rules[p.Action]; !ok {
			t.Errorf("%s: type %q has no rule %q", key, p.Type, p.Action)
		}
		if !schema.HasAction(p.Action) {
			t.Errorf("%s: action %q is not in the vocabulary", key, p.Action)
		}
	}
}

func TestSchemaCarriesNoInertDeclarations(t *testing.T) {
	if _, err := accessmodel.Load(); err != nil {
		t.Fatal(err)
	}
}

// --- Membership and roles ---

func TestRolePermissionsKeepTheirGranularity(t *testing.T) {
	// A role holding members:invite and not members:remove stays that way.
	// One relation per action is what preserves that.
	e := build(t,
		at(accessmodel.Area("members", "acme"), accessmodel.GrantRelation("invite"), reach.Userset(reach.Node("role", "acme_ops"), "holder")),
		at(reach.Node("role", "acme_ops"), "holder", reach.Subject(reach.Node("user", "u9"))),
	)
	u9 := reach.Node("user", "u9")
	mustHold(t, e, u9, "members:invite", "acme")
	mustNotHold(t, e, u9, "members:remove", "acme")
	mustNotHold(t, e, u9, "members:read", "acme")
}

func TestMembershipDoesNotLeakAcrossOrganisations(t *testing.T) {
	e := build(t,
		at(accessmodel.Area("api_keys", "acme"), accessmodel.GrantRelation("manage"), reach.Userset(reach.Node("role", "acme_ops"), "holder")),
		at(reach.Node("role", "acme_ops"), "holder", reach.Subject(reach.Node("user", "u9"))),
	)
	u9 := reach.Node("user", "u9")
	mustHold(t, e, u9, "api_keys:manage", "acme")
	mustNotHold(t, e, u9, "api_keys:manage", "globex")
}

func TestGroupNestingReachesThroughARole(t *testing.T) {
	e := build(t,
		at(accessmodel.Area("billing", "acme"), accessmodel.GrantRelation("manage"), reach.Userset(reach.Node("role", "acme_finance"), "holder")),
		at(reach.Node("role", "acme_finance"), "holder", reach.Userset(reach.Node("group", "finance"), "member_of")),
		at(reach.Node("group", "finance"), "member_of", reach.Subject(reach.Node("group", "ap"))),
		at(reach.Node("group", "ap"), "member_of", reach.Subject(reach.Node("user", "u9"))),
	)
	mustHold(t, e, reach.Node("user", "u9"), "billing:manage", "acme")
}

func TestMembershipExpiryStopsAccessWithNoJobRunning(t *testing.T) {
	lapsed := build(t,
		at(accessmodel.Area("billing", "acme"), accessmodel.GrantRelation("manage"), reach.Userset(reach.Node("role", "acme_ops"), "holder")),
		until(at(reach.Node("role", "acme_ops"), "holder", reach.Subject(reach.Node("user", "u9"))), now.Add(-time.Second)),
	)
	mustNotHold(t, lapsed, reach.Node("user", "u9"), "billing:manage", "acme")

	live := build(t,
		at(accessmodel.Area("billing", "acme"), accessmodel.GrantRelation("manage"), reach.Userset(reach.Node("role", "acme_ops"), "holder")),
		until(at(reach.Node("role", "acme_ops"), "holder", reach.Subject(reach.Node("user", "u9"))), now.Add(time.Hour)),
	)
	mustHold(t, live, reach.Node("user", "u9"), "billing:manage", "acme")
}

// --- Global reach ---

func TestGlobalReachIsAnEdgeOnAStarNode(t *testing.T) {
	// Platform-org membership becomes the same edge written on the star node.
	// There is no flag and no privileged role name.
	e := build(t,
		at(accessmodel.EveryArea("api_keys"), accessmodel.GrantRelation("manage"), reach.Userset(reach.Node("role", "platform_admin"), "holder")),
		at(reach.Node("role", "platform_admin"), "holder", reach.Subject(reach.Node("user", "staff"))),
	)
	staff := reach.Node("user", "staff")
	mustHold(t, e, staff, "api_keys:manage", "acme")
	mustHold(t, e, staff, "api_keys:manage", "globex")
	mustHold(t, e, staff, "api_keys:manage", "a-tenant-created-tomorrow")
}

func TestReadOnlyStaffStayReadOnly(t *testing.T) {
	// Staff must not short-circuit. A support role carries exactly what it was
	// granted, everywhere.
	e := build(t,
		at(accessmodel.EveryArea("api_keys"), accessmodel.GrantRelation("read"), reach.Userset(reach.Node("role", "platform_support"), "holder")),
		at(accessmodel.EveryArea("billing"), accessmodel.GrantRelation("read"), reach.Userset(reach.Node("role", "platform_support"), "holder")),
		at(reach.Node("role", "platform_support"), "holder", reach.Subject(reach.Node("user", "sup"))),
	)
	sup := reach.Node("user", "sup")
	mustHold(t, e, sup, "api_keys:read", "acme")
	mustHold(t, e, sup, "billing:read", "globex")
	mustNotHold(t, e, sup, "api_keys:manage", "acme")
	mustNotHold(t, e, sup, "billing:manage", "acme")
	mustNotHold(t, e, sup, "app_users:write", "acme")
}

// --- Organisation lifecycle ---

func TestSuspendedOrgIsInvisibleToItsMembersButNotToPlatform(t *testing.T) {
	// A suspended tenant must stay visible to staff, or suspension becomes a
	// one-way door: the route that would restore it returns 404 too.
	e := build(t,
		at(reach.Node("organisation", "acme"), "belongs", reach.Userset(reach.Node("role", "acme_ops"), "holder")),
		at(reach.Node("role", "acme_ops"), "holder", reach.Subject(reach.Node("user", "u9"))),
		at(reach.Node("organisation", "acme"), "suspended", reach.Userset(reach.Node("role", "acme_ops"), "holder")),
		at(reach.Node("organisation", "acme"), "oversees", reach.Userset(reach.Node("role", "platform_admin"), "holder")),
		at(reach.Node("role", "platform_admin"), "holder", reach.Subject(reach.Node("user", "staff"))),
	)
	mustNotHold(t, e, reach.Node("user", "u9"), "organisation:member", "acme")
	mustHold(t, e, reach.Node("user", "staff"), "organisation:member", "acme")
}

func TestActiveOrgIsVisibleToItsMembers(t *testing.T) {
	e := build(t,
		at(reach.Node("organisation", "acme"), "belongs", reach.Userset(reach.Node("role", "acme_ops"), "holder")),
		at(reach.Node("role", "acme_ops"), "holder", reach.Subject(reach.Node("user", "u9"))),
	)
	mustHold(t, e, reach.Node("user", "u9"), "organisation:member", "acme")
}

// --- API keys ---

func TestKeyIsAnOrdinaryPrincipal(t *testing.T) {
	// A key carries no mechanism of its own. What it reaches is whatever edges
	// name it, so "what can this key do?" is answered by reading them rather
	// than by simulating a derivation.
	e := build(t,
		at(accessmodel.Area("billing", "acme"), accessmodel.GrantRelation("read"), reach.Subject(reach.Node("key", "k4"))),
		at(reach.Node("key", "k4"), "owner", reach.Subject(reach.Node("user", "u9"))),
	)
	k4 := reach.Node("key", "k4")
	mustHold(t, e, k4, "billing:read", "acme")
	mustNotHold(t, e, k4, "billing:manage", "acme")
	mustNotHold(t, e, k4, "api_keys:manage", "acme")
	mustNotHold(t, e, k4, "billing:read", "globex")
}

func TestOwningAKeyConfersNothingToIt(t *testing.T) {
	// The owner edge exists so a departing person's keys can be found and
	// swept. It is lifecycle, not authority.
	e := build(t,
		at(accessmodel.Area("api_keys", "acme"), accessmodel.GrantRelation("manage"), reach.Userset(reach.Node("role", "acme_ops"), "holder")),
		at(reach.Node("role", "acme_ops"), "holder", reach.Subject(reach.Node("user", "u9"))),
		at(reach.Node("key", "k4"), "owner", reach.Subject(reach.Node("user", "u9"))),
	)
	mustHold(t, e, reach.Node("user", "u9"), "api_keys:manage", "acme")
	mustNotHold(t, e, reach.Node("key", "k4"), "api_keys:manage", "acme")
}

func TestRevokingAKeyIsDeletingItsEdges(t *testing.T) {
	store := reach.NewMemoryStore()
	grant := at(accessmodel.Area("billing", "acme"), accessmodel.GrantRelation("read"), reach.Subject(reach.Node("key", "k4")))
	store.MustWrite(grant)
	e, err := reach.New(accessmodel.MustLoad(), store)
	if err != nil {
		t.Fatal(err)
	}
	k4 := reach.Node("key", "k4")
	mustHold(t, e, k4, "billing:read", "acme")

	store.Delete(grant)
	mustNotHold(t, e, k4, "billing:read", "acme")
}

func TestAKeyExpiresWithItsEdge(t *testing.T) {
	e := build(t,
		until(at(accessmodel.Area("billing", "acme"), accessmodel.GrantRelation("read"), reach.Subject(reach.Node("key", "k4"))),
			now.Add(-time.Second)),
	)
	mustNotHold(t, e, reach.Node("key", "k4"), "billing:read", "acme")
}

func TestAKeyCannotBeIssuedBeyondItsCreator(t *testing.T) {
	// The bound on a machine credential is the subset rule at write time. It
	// cannot exceed the person who minted it, and the check is the same walk
	// that answers an ordinary request.
	e := build(t,
		at(accessmodel.Area("billing", "acme"), accessmodel.GrantRelation("read"), reach.Userset(reach.Node("role", "acme_finance"), "holder")),
		at(reach.Node("role", "acme_finance"), "holder", reach.Subject(reach.Node("user", "u9"))),
	)
	ctx := context.Background()
	u9 := reach.Node("user", "u9")

	within := at(accessmodel.Area("billing", "acme"), accessmodel.GrantRelation("read"), reach.Subject(reach.Node("key", "k4")))
	if err := e.CanWrite(ctx, u9, within, reach.CarveNone, now); err != nil {
		t.Fatalf("issuing a key inside the creator's reach was refused: %v", err)
	}

	// u9 holds billing:read, not billing:manage.
	beyond := at(accessmodel.Area("billing", "acme"), accessmodel.GrantRelation("manage"), reach.Subject(reach.Node("key", "k4")))
	if err := e.CanWrite(ctx, u9, beyond, reach.CarveNone, now); err == nil {
		t.Fatal("a key was issued beyond its creator's reach")
	}

	// And never across tenants.
	elsewhere := at(accessmodel.Area("billing", "globex"), accessmodel.GrantRelation("read"), reach.Subject(reach.Node("key", "k4")))
	if err := e.CanWrite(ctx, u9, elsewhere, reach.CarveNone, now); err == nil {
		t.Fatal("a key was issued into another tenant")
	}
}

// --- Delegation ---

func TestAnOrgAdminCannotIssueGlobalReach(t *testing.T) {
	e := build(t,
		at(accessmodel.Area("api_keys", "acme"), accessmodel.GrantRelation("manage"), reach.Userset(reach.Node("role", "acme_ops"), "holder")),
		at(reach.Node("role", "acme_ops"), "holder", reach.Subject(reach.Node("user", "u9"))),
	)
	ctx := context.Background()

	// Within their own organisation: allowed.
	inside := at(accessmodel.Area("api_keys", "acme"), accessmodel.GrantRelation("manage"), reach.Subject(reach.Node("user", "u12")))
	if err := e.CanWrite(ctx, reach.Node("user", "u9"), inside, reach.CarveNone, now); err != nil {
		t.Fatalf("delegation inside the tenant was refused: %v", err)
	}

	// Another tenant: refused.
	elsewhere := at(accessmodel.Area("api_keys", "globex"), accessmodel.GrantRelation("manage"), reach.Subject(reach.Node("user", "u12")))
	if err := e.CanWrite(ctx, reach.Node("user", "u9"), elsewhere, reach.CarveNone, now); err == nil {
		t.Fatal("delegation into another tenant was allowed")
	}

	// Every tenant at once: refused, because u9 does not reach the star node.
	everywhere := at(accessmodel.EveryArea("api_keys"), accessmodel.GrantRelation("manage"), reach.Subject(reach.Node("user", "u12")))
	if err := e.CanWrite(ctx, reach.Node("user", "u9"), everywhere, reach.CarveNone, now); err == nil {
		t.Fatal("an org admin issued global reach")
	}
}

func TestRootOfTrustCanSeedAnEmptyStore(t *testing.T) {
	// The system has to be recoverable when the store is empty or freshly
	// restored, which is the whole reason the carve-out exists.
	e := build(t)
	seed := at(accessmodel.EveryArea("api_keys"), accessmodel.GrantRelation("manage"), reach.Userset(reach.Node("role", "platform_admin"), "holder"))
	if err := e.CanWrite(context.Background(), reach.Node("user", "none"), seed, reach.CarveRootOfTrust, now); err != nil {
		t.Fatalf("root of trust could not seed an empty store: %v", err)
	}
}
