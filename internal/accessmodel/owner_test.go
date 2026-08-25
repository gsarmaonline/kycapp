package accessmodel_test

import (
	"testing"

	"github.com/gsarmaonline/kyc/core/reach"
	"github.com/gsarmaonline/kyc/internal/accessmodel"
)

// Ownership, which the grant store carried as constraint_kind = self_subject
// and answered by comparing two ids on the read path.
//
// As an edge it is one fact, and the walk answers "is this yours" with the
// comparison it already makes at the leaves for every other principal. The
// grant store needed a second mechanism for it; the graph needs none.

var ownedModel = accessmodel.MerchantModel{
	OrganisationID: "acme",
	ScopeKinds:     []string{"project"},
	CapabilityKeys: []string{"document:read", "document:write"},
}

func ownerEdge(doc, user string) reach.Edge {
	return reach.Edge{
		Object:   reach.Node("document", doc),
		Relation: "owner",
		Subject:  reach.Subject(reach.Node("app_user", user)),
	}
}

// One row, every customer, present and future. The grant store expressed this
// as a single self_subject grant; here it is a fact per resource, written by
// the merchant when the resource is created.
func TestOwnerReachesTheirOwnRow(t *testing.T) {
	e := merchantEval(t, ownedModel, ownerEdge("d1", "ana"))
	if !allows(t, e, reach.Node("app_user", "ana"), "read", reach.Node("document", "d1")) {
		t.Fatal("an owner must reach their own row")
	}
}

// The half that matters. Ownership of one row says nothing about anyone else's,
// which is the whole content of the constraint it replaces.
func TestOwnerReachesNobodyElsesRow(t *testing.T) {
	e := merchantEval(t, ownedModel, ownerEdge("d1", "ana"), ownerEdge("d2", "bo"))
	if allows(t, e, reach.Node("app_user", "ana"), "read", reach.Node("document", "d2")) {
		t.Fatal("an owner must not reach a row somebody else owns")
	}
}

// Ownership is not a grant at the container. Owning a document must not confer
// anything on the project it sits in, or "your own rows" would quietly become
// "the folder they are in".
func TestOwnerDoesNotClimbToTheContainer(t *testing.T) {
	model := accessmodel.MerchantModel{
		OrganisationID: "acme",
		ScopeKinds:     []string{"project"},
		CapabilityKeys: []string{"project:read", "document:read"},
	}
	e := merchantEval(t, model,
		ownerEdge("d1", "ana"),
		reach.Edge{
			Object:   reach.Node("document", "d1"),
			Relation: "parent",
			Subject:  reach.Subject(reach.Node("project", "apollo")),
		},
	)
	if !allows(t, e, reach.Node("app_user", "ana"), "read", reach.Node("document", "d1")) {
		t.Fatal("setup: the owner must reach their own document")
	}
	if allows(t, e, reach.Node("app_user", "ana"), "read", reach.Node("project", "apollo")) {
		t.Fatal("owning a row must not confer anything on its container")
	}
}

// Ownership confers every action the type answers, which is wider than the
// constraint it replaces: there, the role's capability set bounded it and a
// verb was withheld by leaving it out of the role.
//
// The grammar has no parentheses and associates strictly left to right, so
// "can_write + owner & self_write" would parse as "(can_write + owner) &
// self_write" and the intersection would swallow the union. Preserving the old
// bound would therefore mean changing the grammar.
//
// This test exists to make the widening visible rather than to endorse it. A
// merchant who does not want owners writing declares write on a type owners do
// not hold.
func TestOwnershipConfersEveryActionItsTypeAnswers(t *testing.T) {
	e := merchantEval(t, ownedModel, ownerEdge("d1", "ana"))
	if !allows(t, e, reach.Node("app_user", "ana"), "write", reach.Node("document", "d1")) {
		t.Fatal("ownership unions into every rule its type carries")
	}
}

// It stays inside its own type. Owning a document confers nothing on an
// invoice, even for an action both types answer.
func TestOwnershipDoesNotCrossTypes(t *testing.T) {
	model := accessmodel.MerchantModel{
		OrganisationID: "acme",
		ScopeKinds:     []string{"project"},
		CapabilityKeys: []string{"document:read", "invoice:read"},
	}
	e := merchantEval(t, model, ownerEdge("d1", "ana"))
	if allows(t, e, reach.Node("app_user", "ana"), "read", reach.Node("invoice", "i1")) {
		t.Fatal("ownership must not cross to another type")
	}
}

// A grant at the container still reaches a row somebody else owns. Ownership
// adds; it never subtracts, so it cannot be used to lock a row away from an
// administrator. That is the same rule every other edge follows.
func TestOwnershipAddsAndNeverSubtracts(t *testing.T) {
	e := merchantEval(t, ownedModel,
		ownerEdge("d1", "ana"),
		reach.Edge{
			Object:   reach.Node("document", "d1"),
			Relation: "parent",
			Subject:  reach.Subject(reach.Node("project", "apollo")),
		},
		reach.Edge{
			Object:   reach.Node("project", "apollo"),
			Relation: "can_read",
			Subject:  reach.Subject(reach.Node("app_user", "admin")),
		},
	)
	if !allows(t, e, reach.Node("app_user", "admin"), "read", reach.Node("document", "d1")) {
		t.Fatal("an ownership edge must not lock a container grant out")
	}
}

// Only a person owns a row. A group or a role owning one is a different idea
// from "this is mine", and admitting it would make ownership a second way to
// write a grant.
func TestOnlyAnAppUserMayOwn(t *testing.T) {
	schema, err := accessmodel.MerchantSchema(ownedModel)
	if err != nil {
		t.Fatal(err)
	}
	store := reach.NewMemoryStore()
	e, err := reach.New(schema, store)
	if err != nil {
		t.Fatal(err)
	}
	bad := reach.Edge{
		Object:   reach.Node("document", "d1"),
		Relation: "owner",
		Subject:  reach.Userset(reach.Node("role", "editor"), "holder"),
	}
	if err := e.CanWrite(t.Context(), reach.Node("app_user", "ana"), bad, reach.CarveNone, now); err == nil {
		t.Fatal("a role must not be a legal owner")
	}
}

// A vocabulary with no resource types has nothing to own, so the relation is
// not declared at all. The generator refuses inert declarations, so this is
// what keeps such a namespace loadable.
func TestOwnerIsAbsentWhenNothingCanBeOwned(t *testing.T) {
	schema, err := accessmodel.MerchantSchema(accessmodel.MerchantModel{
		OrganisationID: "acme",
		ScopeKinds:     []string{"project"},
		CapabilityKeys: []string{"project:read"},
	})
	if err != nil {
		t.Fatalf("a namespace with no resource types must stay loadable: %v", err)
	}
	if _, ok := schema.Relation("owner"); ok {
		t.Fatal("owner must not be declared where no type carries it")
	}
}
