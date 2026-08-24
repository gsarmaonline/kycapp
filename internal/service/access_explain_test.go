package service

import (
	"strings"
	"testing"

	"github.com/gsarmaonline/kyc/core/reach"
)

// A path is the most useful thing the engine produces and the leakiest, so the
// filter is worth testing on its own: it is pure, and every case below is a
// name a tenant admin must not be handed.

func acmeVisible() nodeSet {
	return nodeSet{
		"user:ana":          {},
		"organisation:acme": {},
		"billing:acme":      {},
		"role:acme_member":  {},
		"role:acme_admin":   {},
		"api_keys:acme":     {},
	}
}

func TestFilterPathKeepsNodesInsideTheOrganisation(t *testing.T) {
	path := []reach.Step{
		{
			Object:   reach.Node("billing", "acme"),
			Relation: "can_read",
			Subject:  reach.Userset(reach.Node("role", "acme_member"), "holder"),
		},
		{
			Object:   reach.Node("role", "acme_member"),
			Relation: "holder",
			Subject:  reach.Subject(reach.Node("user", "ana")),
		},
	}
	got := filterPath(path, acmeVisible(), false)
	if len(got) != 2 {
		t.Fatalf("got %d hops, wanted 2", len(got))
	}
	for i, hop := range got {
		if hop.Redacted {
			t.Errorf("hop %d was redacted: %+v", i, hop)
		}
	}
	if got[0].Subject != "role:acme_member#holder" {
		t.Errorf("subject = %q, wanted the userset intact", got[0].Subject)
	}
}

func TestFilterPathRedactsForeignRoles(t *testing.T) {
	// A platform role reaching a tenant is exactly the disclosure to prevent:
	// its name tells a tenant admin that staff access exists and who holds it.
	path := []reach.Step{{
		Object:   reach.Node("billing", "acme"),
		Relation: "can_read",
		Subject:  reach.Userset(reach.Node("role", "platform_support"), "holder"),
	}}
	got := filterPath(path, acmeVisible(), false)
	if !got[0].Redacted {
		t.Fatal("a foreign role was not redacted")
	}
	if strings.Contains(got[0].Subject, "platform_support") {
		t.Errorf("subject %q still names the foreign role", got[0].Subject)
	}
	// The relation survives, because it is schema vocabulary rather than a
	// tenant's data, and dropping it would make the hop unreadable.
	if got[0].Relation != "can_read" {
		t.Errorf("relation = %q, wanted it kept", got[0].Relation)
	}
	// The object is this organisation's own, so it stays named.
	if got[0].Object != "billing:acme" {
		t.Errorf("object = %q, wanted billing:acme kept", got[0].Object)
	}
}

func TestFilterPathRedactsStarNodes(t *testing.T) {
	// organisation:* is the platform's reach over every tenant. It is never in
	// the visible set, but it gets its own check so a future change to how the
	// set is built cannot quietly expose it.
	path := []reach.Step{{
		Object:   reach.Star("organisation"),
		Relation: "oversees",
		Subject:  reach.Userset(reach.Node("role", "platform_admin"), "holder"),
	}}
	got := filterPath(path, acmeVisible(), false)
	if !got[0].Redacted {
		t.Fatal("a star node was not redacted")
	}
	if strings.Contains(got[0].Object, "*") {
		t.Errorf("object %q still names the star node", got[0].Object)
	}
}

func TestFilterPathKeepsTheUsersetRelationWhenRedacting(t *testing.T) {
	// Withholding the node while keeping "#holder" says a role is involved
	// without saying which. Dropping it too would make the hop look like a
	// direct grant to a principal, which is a different shape of fact.
	path := []reach.Step{{
		Object:   reach.Node("billing", "acme"),
		Relation: "can_manage",
		Subject:  reach.Userset(reach.Node("role", "elsewhere"), "holder"),
	}}
	got := filterPath(path, acmeVisible(), false)
	if want := RedactedNode + "#holder"; got[0].Subject != want {
		t.Errorf("subject = %q, wanted %q", got[0].Subject, want)
	}
}

func TestFilterPathPreservesLength(t *testing.T) {
	// Redaction must not drop hops. A four-hop route rendered as three would
	// misdescribe the model, and the length is itself information a viewer is
	// entitled to.
	path := []reach.Step{
		{Object: reach.Node("billing", "acme"), Relation: "can_read", Subject: reach.Userset(reach.Node("role", "x"), "holder")},
		{Object: reach.Node("role", "x"), Relation: "holder", Subject: reach.Userset(reach.Node("role", "y"), "holder")},
		{Object: reach.Node("role", "y"), Relation: "holder", Subject: reach.Subject(reach.Node("user", "zed"))},
	}
	got := filterPath(path, acmeVisible(), false)
	if len(got) != len(path) {
		t.Fatalf("got %d hops, wanted %d", len(got), len(path))
	}
}

func TestFilterPathRedactsNothingForAnUnrestrictedViewer(t *testing.T) {
	// Staff already reach every tenant, so a redaction would withhold names
	// they are entitled to and make the answer worse rather than safer.
	path := []reach.Step{{
		Object:   reach.Star("organisation"),
		Relation: "oversees",
		Subject:  reach.Userset(reach.Node("role", "platform_admin"), "holder"),
	}}
	got := filterPath(path, nodeSet{}, true)
	if got[0].Redacted {
		t.Fatal("an unrestricted viewer got a redacted hop")
	}
	if got[0].Object != "organisation:*" || got[0].Subject != "role:platform_admin#holder" {
		t.Errorf("names were altered: %+v", got[0])
	}
}

func TestFilterPathReturnsEmptySliceNotNil(t *testing.T) {
	// A denial has no path. It must serialise as [] rather than null, so the
	// client can map over it without a guard on every render.
	got := filterPath(nil, acmeVisible(), false)
	if got == nil {
		t.Fatal("filterPath returned nil")
	}
	if len(got) != 0 {
		t.Fatalf("got %d hops, wanted 0", len(got))
	}
}
