package accessmodel_test

import (
	"context"
	"testing"
	"time"

	"github.com/gsarmaonline/kyc/core/reach"
	"github.com/gsarmaonline/kyc/internal/accessmodel"
)

// Namespace isolation is the tenancy boundary, and until now nothing held it in
// place but a parameter nobody had pinned.
//
// It is worth being precise about what does the holding. It is not a list of
// reserved names: a merchant declaring a scope kind called "global" would get
// global:x inside their own namespace, reaching nothing outside it, because no
// edge crosses. The isolation is structural, so what these tests assert is the
// structure rather than a rule.

func TestNamespacesCannotSeeEachOther(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()
	ctx := context.Background()
	now := time.Now().UTC()

	acme := accessmodel.MerchantNamespace("acme")
	other := accessmodel.MerchantNamespace("other")

	// The same edge, written three times, in three namespaces. Identical in
	// every column but one.
	for _, ns := range []string{acme, other, accessmodel.Namespace} {
		exec(t, pool, `
            INSERT INTO reach_edges (namespace, object_type, object_id, relation,
                                     subject_type, subject_id, subject_relation, source)
            VALUES ($1, 'project', 'shared', 'can_read', 'app_user', 'ana', '', 'test')
            ON CONFLICT DO NOTHING`, ns)
	}

	schema, err := accessmodel.MerchantSchema(accessmodel.MerchantModel{
		OrganisationID: "acme",
		ScopeKinds:     []string{"project"},
		CapabilityKeys: []string{"project:read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	e, err := reach.New(schema, accessmodel.NewResolverIn(db.Q(), acme))
	if err != nil {
		t.Fatal(err)
	}

	// Ana reaches it in her own namespace.
	d, err := e.Check(ctx, reach.Request{
		Subject:  reach.Node("app_user", "ana"),
		Action:   "read",
		Resource: reach.Node("project", "shared"),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Allowed {
		t.Fatalf("the edge in this namespace must be reachable, got %s", d.Reason)
	}

	// A subject that exists only in another namespace is unreachable here, and
	// unreachable rather than merely denied: the resource must be
	// indistinguishable from one that does not exist, or tenants become
	// enumerable by status code.
	exec(t, pool, `
        INSERT INTO reach_edges (namespace, object_type, object_id, relation,
                                 subject_type, subject_id, subject_relation, source)
        VALUES ($1, 'project', 'theirs', 'can_read', 'app_user', 'bob', '', 'test')
        ON CONFLICT DO NOTHING`, other)

	d, err = e.Check(ctx, reach.Request{
		Subject:  reach.Node("app_user", "bob"),
		Action:   "read",
		Resource: reach.Node("project", "theirs"),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if d.Allowed {
		t.Fatal("a walk reached across the namespace boundary")
	}
	if d.Reason != reach.ReasonUnreachable {
		t.Errorf("reason = %s, wanted unreachable so the resource stays invisible", d.Reason)
	}
}

// An edge pointing at a node that only exists in another namespace reaches
// nothing. Names are not addresses across the boundary: kyc's organisation:*
// star node is a different node from a merchant's, whatever it is called.
func TestAnEdgeCannotNameAnotherNamespacesNode(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()
	ctx := context.Background()

	acme := accessmodel.MerchantNamespace("acme")

	// A merchant writes the most dangerous edge they could: their customer as
	// holder of a role whose name matches one of KYC's own.
	exec(t, pool, `
        INSERT INTO reach_edges (namespace, object_type, object_id, relation,
                                 subject_type, subject_id, subject_relation, source)
        VALUES ($1, 'organisation', '*', 'can_read', 'app_user', 'ana', '', 'test')
        ON CONFLICT DO NOTHING`, acme)

	// Asked in KYC's namespace, that edge does not exist.
	e, err := accessmodel.NewEvaluator(db.Q())
	if err != nil {
		t.Fatal(err)
	}
	d, err := e.Check(ctx, reach.Request{
		Subject:  reach.Node("app_user", "ana"),
		Action:   "read",
		Resource: accessmodel.EveryArea("organisation"),
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if d.Allowed {
		t.Fatal("a merchant edge was visible inside KYC's namespace")
	}
}

// The reserved-name blacklist used to stand in for this. A merchant may now
// name a scope kind whatever they like, including the names KYC uses, because
// the name was never what kept them apart.
func TestAMerchantMayNameAScopeKindGlobal(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()
	ctx := context.Background()

	acme := accessmodel.MerchantNamespace("acme")
	exec(t, pool, `
        INSERT INTO reach_edges (namespace, object_type, object_id, relation,
                                 subject_type, subject_id, subject_relation, source)
        VALUES ($1, 'global', 'everything', 'can_read', 'app_user', 'ana', '', 'test')
        ON CONFLICT DO NOTHING`, acme)

	schema, err := accessmodel.MerchantSchema(accessmodel.MerchantModel{
		OrganisationID: "acme",
		ScopeKinds:     []string{"global"},
		CapabilityKeys: []string{"global:read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	e, err := reach.New(schema, accessmodel.NewResolverIn(db.Q(), acme))
	if err != nil {
		t.Fatal(err)
	}

	// It reaches exactly what it says inside their namespace.
	d, err := e.Check(ctx, reach.Request{
		Subject:  reach.Node("app_user", "ana"),
		Action:   "read",
		Resource: reach.Node("global", "everything"),
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if !d.Allowed {
		t.Fatalf("a merchant's own global type must work inside their namespace: %s", d.Reason)
	}

	// And nothing in KYC's namespace, which is the whole point.
	kyc, err := accessmodel.NewEvaluator(db.Q())
	if err != nil {
		t.Fatal(err)
	}
	for _, area := range []string{"organisation", "api_keys", "billing"} {
		d, err := kyc.Check(ctx, reach.Request{
			Subject:  reach.Node("app_user", "ana"),
			Action:   "read",
			Resource: accessmodel.EveryArea(area),
		}, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if d.Allowed {
			t.Errorf("naming a type %q reached KYC's %s", "global", area)
		}
	}
}
