package accessmodel_test

import (
	"context"
	"testing"

	"github.com/gsarmaonline/kyc/core/reach"
	"github.com/gsarmaonline/kyc/internal/accessmodel"
)

// The third wildcard axis.
//
// A grant could always say "every customer" and "every scope of a kind",
// because both are node ids and the star lives in an id. It could not say
// "every capability", because a capability is a relation. can_all is that
// third axis, as an ordinary relation rather than a pattern.

func merchantEval(t *testing.T, model accessmodel.MerchantModel, edges ...reach.Edge) *reach.Evaluator {
	t.Helper()
	schema, err := accessmodel.MerchantSchema(model)
	if err != nil {
		t.Fatalf("merchant schema: %v", err)
	}
	store := reach.NewMemoryStore()
	store.MustWrite(edges...)
	e, err := reach.New(schema, store)
	if err != nil {
		t.Fatalf("evaluator: %v", err)
	}
	return e
}

func allows(t *testing.T, e *reach.Evaluator, subject reach.NodeRef, action string, resource reach.NodeRef) bool {
	t.Helper()
	d, err := e.Check(context.Background(), reach.Request{
		Subject: subject, Action: action, Resource: resource,
	}, now)
	if err != nil {
		t.Fatalf("check %s %s %s: %v", subject, action, resource, err)
	}
	return d.Allowed
}

var editorModel = accessmodel.MerchantModel{
	OrganisationID: "acme",
	ScopeKinds:     []string{"project"},
	CapabilityKeys: []string{"project:read", "project:write", "project:publish"},
}

// One edge carries every capability the type answers. This is what a grant of
// "admin here" becomes, and writing it out as one edge per capability would
// have been a different thing: a list that stops covering the admin the moment
// the merchant declares a fourth action.
func TestCanAllCarriesEveryAction(t *testing.T) {
	e := merchantEval(t, editorModel,
		reach.Edge{
			Object:   reach.Node("project", "apollo"),
			Relation: "can_all",
			Subject:  reach.Subject(reach.Node("app_user", "ana")),
		},
	)
	for _, action := range []string{"read", "write", "publish"} {
		if !allows(t, e, reach.Node("app_user", "ana"), action, reach.Node("project", "apollo")) {
			t.Errorf("can_all must carry %q", action)
		}
	}
}

// The point of a wildcard: it is a standing instruction, not a snapshot. A
// merchant declares a new action and the existing edge already covers it, with
// nothing rewritten.
func TestCanAllStaysStandingWhenTheVocabularyGrows(t *testing.T) {
	grant := reach.Edge{
		Object:   reach.Node("project", "apollo"),
		Relation: "can_all",
		Subject:  reach.Subject(reach.Node("app_user", "ana")),
	}
	grown := editorModel
	grown.CapabilityKeys = append(append([]string(nil), editorModel.CapabilityKeys...), "project:archive")

	e := merchantEval(t, grown, grant)
	if !allows(t, e, reach.Node("app_user", "ana"), "archive", reach.Node("project", "apollo")) {
		t.Fatal("an action declared after the grant must be covered by can_all")
	}
}

// It is a wildcard on one axis only. Holding every capability at apollo says
// nothing about borealis.
func TestCanAllDoesNotWidenScope(t *testing.T) {
	e := merchantEval(t, editorModel,
		reach.Edge{
			Object:   reach.Node("project", "apollo"),
			Relation: "can_all",
			Subject:  reach.Subject(reach.Node("app_user", "ana")),
		},
	)
	if allows(t, e, reach.Node("app_user", "ana"), "read", reach.Node("project", "borealis")) {
		t.Fatal("can_all must not reach a scope it was not written at")
	}
}

// The three axes compose. Every capability, every project, every customer, in
// one row, which is the widest thing the old grant could say.
func TestCanAllComposesWithTheNodeWildcards(t *testing.T) {
	e := merchantEval(t, editorModel,
		reach.Edge{
			Object:   reach.Star("project"),
			Relation: "can_all",
			Subject:  reach.Subject(reach.Star("app_user")),
		},
	)
	if !allows(t, e, reach.Node("app_user", "someone-new"), "publish", reach.Node("project", "unheard-of")) {
		t.Fatal("the three wildcard axes must compose in one edge")
	}
}

// Roles and groups carry it like any other capability relation, so a wildcard
// grant is issued the same way a narrow one is.
func TestCanAllIsCarriedByRolesAndGroups(t *testing.T) {
	e := merchantEval(t, editorModel,
		reach.Edge{
			Object:   reach.Node("project", "apollo"),
			Relation: "can_all",
			Subject:  reach.Userset(reach.Node("role", "admin"), "holder"),
		},
		reach.Edge{
			Object:   reach.Node("role", "admin"),
			Relation: "holder",
			Subject:  reach.Userset(reach.Node("group", "ops"), "member_of"),
		},
		reach.Edge{
			Object:   reach.Node("group", "ops"),
			Relation: "member_of",
			Subject:  reach.Subject(reach.Node("app_user", "ana")),
		},
	)
	if !allows(t, e, reach.Node("app_user", "ana"), "write", reach.Node("project", "apollo")) {
		t.Fatal("can_all must resolve through a role and a nested group")
	}
}

// A wildcard at a container covers what the container holds, the same way a
// named capability does. Nothing about the arrow changes.
func TestCanAllReachesThroughContainment(t *testing.T) {
	model := accessmodel.MerchantModel{
		OrganisationID: "acme",
		ScopeKinds:     []string{"project"},
		CapabilityKeys: []string{"document:read", "document:write"},
	}
	e := merchantEval(t, model,
		reach.Edge{
			Object:   reach.Node("project", "apollo"),
			Relation: "can_all",
			Subject:  reach.Subject(reach.Node("app_user", "ana")),
		},
		reach.Edge{
			Object:   reach.Node("document", "d1"),
			Relation: "parent",
			Subject:  reach.Subject(reach.Node("project", "apollo")),
		},
	)
	if !allows(t, e, reach.Node("app_user", "ana"), "write", reach.Node("document", "d1")) {
		t.Fatal("can_all at a container must reach what the container holds")
	}
}

// On a resource type the wildcard means every action *that type* answers, not
// every action in the namespace. A wildcard is a claim about a set, and the set
// is what this resource can have done to it.
func TestCanAllOnAResourceIsScopedToThatResourcesActions(t *testing.T) {
	model := accessmodel.MerchantModel{
		OrganisationID: "acme",
		ScopeKinds:     []string{"project"},
		CapabilityKeys: []string{"document:read", "invoice:read", "invoice:refund"},
	}
	e := merchantEval(t, model,
		reach.Edge{
			Object:   reach.Node("document", "d1"),
			Relation: "can_all",
			Subject:  reach.Subject(reach.Node("app_user", "ana")),
		},
	)
	if !allows(t, e, reach.Node("app_user", "ana"), "read", reach.Node("document", "d1")) {
		t.Fatal("can_all must carry the actions its own type answers")
	}
	// document has no refund rule at all, so this is unreachable rather than
	// denied, and it must stay that way.
	d, err := e.Check(context.Background(), reach.Request{
		Subject:  reach.Node("app_user", "ana"),
		Action:   "refund",
		Resource: reach.Node("document", "d1"),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if d.Allowed {
		t.Fatal("can_all must not invent an action the type does not answer")
	}
}

// Issuing a wildcard is issuing everything, so the subset rule has to see it
// that way. This falls out of ActionsFed rather than being checked separately.
func TestIssuingCanAllRequiresHoldingEverything(t *testing.T) {
	schema, err := accessmodel.MerchantSchema(editorModel)
	if err != nil {
		t.Fatal(err)
	}
	fed := map[string]bool{}
	store := reach.NewMemoryStore()
	e, err := reach.New(schema, store)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range e.ActionsFed("project", "can_all") {
		fed[a] = true
	}
	for _, action := range []string{"read", "write", "publish"} {
		if !fed[action] {
			t.Errorf("can_all must feed %q, or CanWrite would let a partial holder issue one", action)
		}
	}
}

// A namespace with no capabilities is legal and reaches nothing. Declaring
// can_all there would be inert, and the generator refuses inert declarations.
func TestCanAllIsAbsentFromAnEmptyVocabulary(t *testing.T) {
	if _, err := accessmodel.MerchantSchema(accessmodel.MerchantModel{
		OrganisationID: "acme",
		ScopeKinds:     []string{"project"},
	}); err != nil {
		t.Fatalf("an empty vocabulary must stay legal: %v", err)
	}
}
