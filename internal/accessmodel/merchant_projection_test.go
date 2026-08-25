package accessmodel_test

import (
	"context"
	"strings"
	"testing"

	"github.com/gsarmaonline/kyc/core/reach"
	"github.com/gsarmaonline/kyc/internal/accessmodel"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Closing the seam.
//
// A merchant's roles, their inheritance, their groups, who is in them and every
// grant lived in app_* tables, and nothing projected any of it into reach_edges.
// POST /check therefore could not see a single one. Only the vocabulary crossed
// over, because MerchantSchema reads app_scope_types and app_capabilities to
// generate the schema; none of the data did.
//
// KYC had built this exact bridge for its own tier and stopped: projection.sql
// does roles, role_permissions, role_extends and memberships, and hard-codes
// 'kyc' in every statement. These tests are the evidence the merchant side
// crosses too. Each seeds through the tables a merchant's operator writes, runs
// the projection, and asks the evaluator what the merchant's backend would ask.

const mOrg = "m_acme"

// seedMerchant writes the rows the customer-access pages write: a vocabulary, a
// role, and a customer.
func seedMerchant(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	exec(t, pool, `INSERT INTO organisations (id, name, slug, status)
		VALUES ($1, $1, $1, 'active') ON CONFLICT (id) DO NOTHING`, mOrg)
	exec(t, pool, `INSERT INTO app_scope_types (id, organisation_id, kind)
		VALUES ('st_project', $1, 'project') ON CONFLICT DO NOTHING`, mOrg)
	// resource and action are written; key derives from them, so it cannot be
	// inserted directly and cannot disagree with its own halves.
	for _, key := range []string{"document:read", "document:write"} {
		resource, action, _ := strings.Cut(key, ":")
		exec(t, pool, `INSERT INTO app_capabilities (id, organisation_id, resource, action)
			VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`, "cap_"+key, mOrg, resource, action)
	}
	for _, id := range []string{"ana", "bo"} {
		exec(t, pool, `INSERT INTO app_users (id, organisation_id, email, status)
			VALUES ($1, $2, $1 || '@customer.test', 'active') ON CONFLICT (id) DO NOTHING`, id, mOrg)
	}
}

func seedMerchantRole(t *testing.T, pool *pgxpool.Pool, id string, own, effective []string) {
	t.Helper()
	exec(t, pool, `INSERT INTO app_roles (id, organisation_id, key, name, own_capabilities, effective_capabilities)
		VALUES ($1, $2, $1, $1, $3, $4) ON CONFLICT (id) DO NOTHING`, id, mOrg, own, effective)
}

func grantToUser(t *testing.T, pool *pgxpool.Pool, id, userID, roleID, scopeID string) {
	t.Helper()
	exec(t, pool, `INSERT INTO app_grants (id, organisation_id, app_user_id, role_id, scope_kind, scope_id, subject_kind)
		VALUES ($1, $2, $3, $4, 'project', $5, 'app_user') ON CONFLICT DO NOTHING`,
		id, mOrg, userID, roleID, scopeID)
}

func projectMerchant(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		return accessmodel.ProjectMerchant(ctx, tx, mOrg)
	})
	if err != nil {
		t.Fatalf("project merchant: %v", err)
	}
}

// merchantEvaluator builds the evaluator the merchant's own check endpoint
// builds: their derived schema, over their namespace, reading written edges.
func merchantEvaluator(t *testing.T, db *store.Store, caps ...string) *reach.Evaluator {
	t.Helper()
	if len(caps) == 0 {
		caps = []string{"document:read", "document:write"}
	}
	schema, err := accessmodel.MerchantSchema(accessmodel.MerchantModel{
		OrganisationID: mOrg,
		ScopeKinds:     []string{"project"},
		CapabilityKeys: caps,
	})
	if err != nil {
		t.Fatal(err)
	}
	e, err := reach.New(schema, accessmodel.NewResolverIn(db.Q(), accessmodel.MerchantNamespace(mOrg)))
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func mAllows(t *testing.T, e *reach.Evaluator, subject, action, resType, resID string) bool {
	t.Helper()
	d, err := e.Check(context.Background(), reach.Request{
		Subject:  reach.Node("app_user", subject),
		Action:   action,
		Resource: reach.Node(resType, resID),
	}, now)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	return d.Allowed
}

// containment is the merchant's own fact: this document lives in that project.
// Without it no walk can arrive at the document, which is why the merchant has
// to write these whatever else changes.
func containment(t *testing.T, pool *pgxpool.Pool, doc, project string) {
	t.Helper()
	exec(t, pool, `INSERT INTO reach_edges
		(namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation, source)
		VALUES ($1, 'document', $2, 'parent', 'project', $3, '', 'merchant') ON CONFLICT DO NOTHING`,
		accessmodel.MerchantNamespace(mOrg), doc, project)
}

// The claim the whole seam is about: a grant issued on the Grants page reaches
// POST /check. Before the projection this was false, and nothing said so.
func TestAGrantIssuedThroughTheGrantStoreReachesTheGraph(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedMerchant(t, pool)
	seedMerchantRole(t, pool, "editor", []string{"document:read"}, []string{"document:read"})
	grantToUser(t, pool, "g1", "ana", "editor", "apollo")
	containment(t, pool, "d1", "apollo")
	projectMerchant(t, pool)

	e := merchantEvaluator(t, db)
	if !mAllows(t, e, "ana", "read", "document", "d1") {
		t.Fatal("a grant in app_grants must reach the graph")
	}
	// And it stays a grant rather than becoming a blanket one.
	if mAllows(t, e, "bo", "read", "document", "d1") {
		t.Fatal("the projection must not widen a grant to other customers")
	}
	if mAllows(t, e, "ana", "write", "document", "d1") {
		t.Fatal("a role carrying only read must not confer write")
	}
}

// Role inheritance crosses as a userset, so the chain resolves in the walk and
// editing a base role reaches everything built on it.
func TestRoleInheritanceCrossesToTheGraph(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedMerchant(t, pool)
	seedMerchantRole(t, pool, "viewer", []string{"document:read"}, []string{"document:read"})
	seedMerchantRole(t, pool, "editor", []string{"document:write"}, []string{"document:read", "document:write"})
	exec(t, pool, `INSERT INTO app_role_extends (role_id, parent_id)
		VALUES ('editor', 'viewer') ON CONFLICT DO NOTHING`)
	// The grant names the parent; the holder holds the child.
	exec(t, pool, `INSERT INTO app_grants (id, organisation_id, app_user_id, role_id, scope_kind, scope_id, subject_kind)
		VALUES ('g_v', $1, 'ana', 'viewer', 'project', 'apollo', 'app_user') ON CONFLICT DO NOTHING`, mOrg)
	containment(t, pool, "d1", "apollo")
	projectMerchant(t, pool)

	e := merchantEvaluator(t, db)
	if !mAllows(t, e, "ana", "read", "document", "d1") {
		t.Fatal("the direct grant must reach")
	}
}

// Groups and their nesting cross too. A grant on a parent group reaches a
// member of a child, which is the whole reason nesting exists.
func TestGroupMembershipAndNestingCrossToTheGraph(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedMerchant(t, pool)
	seedMerchantRole(t, pool, "editor", []string{"document:read"}, []string{"document:read"})
	for _, g := range []string{"eng", "backend"} {
		exec(t, pool, `INSERT INTO app_user_groups (id, organisation_id, key, name)
			VALUES ($1, $2, $1, $1) ON CONFLICT (id) DO NOTHING`, g, mOrg)
	}
	// backend nests inside eng, and ana is only in backend.
	exec(t, pool, `INSERT INTO app_user_group_extends (group_id, parent_id)
		VALUES ('backend', 'eng') ON CONFLICT DO NOTHING`)
	exec(t, pool, `INSERT INTO app_user_group_members (group_id, app_user_id)
		VALUES ('backend', 'ana') ON CONFLICT DO NOTHING`)
	// The grant is made to the outer group.
	exec(t, pool, `INSERT INTO app_grants (id, organisation_id, group_id, role_id, scope_kind, scope_id, subject_kind)
		VALUES ('g_grp', $1, 'eng', 'editor', 'project', 'apollo', 'group') ON CONFLICT DO NOTHING`, mOrg)
	containment(t, pool, "d1", "apollo")
	projectMerchant(t, pool)

	e := merchantEvaluator(t, db)
	if !mAllows(t, e, "ana", "read", "document", "d1") {
		t.Fatal("a grant on a parent group must reach a member of a child")
	}
	if mAllows(t, e, "bo", "read", "document", "d1") {
		t.Fatal("a group grant must not reach somebody outside the group")
	}
}

// An everyone grant crosses as the subject star, so it covers tomorrow's
// signups without a row per customer.
func TestEveryoneGrantCrossesAsTheSubjectStar(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedMerchant(t, pool)
	seedMerchantRole(t, pool, "reader", []string{"document:read"}, []string{"document:read"})
	exec(t, pool, `INSERT INTO app_grants (id, organisation_id, role_id, scope_kind, scope_id, subject_kind)
		VALUES ('g_all', $1, 'reader', 'project', 'apollo', 'everyone') ON CONFLICT DO NOTHING`, mOrg)
	containment(t, pool, "d1", "apollo")
	projectMerchant(t, pool)

	e := merchantEvaluator(t, db)
	if !mAllows(t, e, "someone-new", "read", "document", "d1") {
		t.Fatal("an everyone grant must cover a customer who did not exist when it was written")
	}
}

// all_capabilities crosses as can_all rather than as a list, so it keeps its
// standing property: an action declared later is already covered.
func TestWildcardGrantCrossesAsCanAll(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedMerchant(t, pool)
	// all_capabilities carries no role: the wildcard *is* the capability set.
	exec(t, pool, `INSERT INTO app_grants
		(id, organisation_id, app_user_id, scope_kind, scope_id, subject_kind, all_capabilities)
		VALUES ('g_wild', $1, 'ana', 'project', 'apollo', 'app_user', true) ON CONFLICT DO NOTHING`, mOrg)
	containment(t, pool, "d1", "apollo")
	projectMerchant(t, pool)

	// The evaluator is built with an action the grant was never told about.
	e := merchantEvaluator(t, db, "document:read", "document:write", "document:archive")
	if !mAllows(t, e, "ana", "archive", "document", "d1") {
		t.Fatal("a wildcard grant must cover an action declared after it was written")
	}
}

// all_scopes crosses as the object star: every project, present and future.
func TestAllScopesCrossesAsTheObjectStar(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedMerchant(t, pool)
	seedMerchantRole(t, pool, "reader", []string{"document:read"}, []string{"document:read"})
	// all_scopes names no kind and no id: it is every kind at once, the ceiling
	// of the organisation. The projection fans it out to the star node of every
	// kind the merchant has declared.
	exec(t, pool, `INSERT INTO app_grants
		(id, organisation_id, app_user_id, role_id, scope_kind, scope_id, subject_kind, all_scopes)
		VALUES ('g_any', $1, 'ana', 'reader', '', '', 'app_user', true) ON CONFLICT DO NOTHING`, mOrg)
	containment(t, pool, "d9", "a-project-nobody-named")
	projectMerchant(t, pool)

	e := merchantEvaluator(t, db)
	if !mAllows(t, e, "ana", "read", "document", "d9") {
		t.Fatal("an all_scopes grant must reach a scope it never named")
	}
}

// A self_subject grant has no edge form that can be derived. It said "your own
// rows", and KYC never learned which rows exist, let alone who owns them. It is
// skipped rather than mistranslated, and ownership becomes an owner edge the
// merchant writes when it creates the resource.
func TestSelfSubjectGrantsAreSkippedRatherThanMistranslated(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedMerchant(t, pool)
	seedMerchantRole(t, pool, "self", []string{"document:read"}, []string{"document:read"})
	exec(t, pool, `INSERT INTO app_grants
		(id, organisation_id, role_id, scope_kind, scope_id, subject_kind, constraint_kind)
		VALUES ('g_self', $1, 'self', 'project', 'apollo', 'everyone', 'self_subject') ON CONFLICT DO NOTHING`, mOrg)
	containment(t, pool, "d1", "apollo")
	projectMerchant(t, pool)

	e := merchantEvaluator(t, db)
	// Translating it as an ordinary everyone grant would have handed every
	// customer every document in the project. That is the mistranslation this
	// guards against.
	if mAllows(t, e, "bo", "read", "document", "d1") {
		t.Fatal("a self_subject grant must not project as a blanket everyone grant")
	}

	// Ownership is available, as a fact the merchant writes.
	exec(t, pool, `INSERT INTO reach_edges
		(namespace, object_type, object_id, relation, subject_type, subject_id, subject_relation, source)
		VALUES ($1, 'document', 'd1', 'owner', 'app_user', 'ana', '', 'merchant') ON CONFLICT DO NOTHING`,
		accessmodel.MerchantNamespace(mOrg))
	if !mAllows(t, e, "ana", "read", "document", "d1") {
		t.Fatal("an owner edge must reach the owner's own row")
	}
	if mAllows(t, e, "bo", "read", "document", "d1") {
		t.Fatal("an owner edge must reach nobody else")
	}
}

// Re-running converges. A merchant re-syncing after a failure is the first
// thing that happens when one goes wrong.
func TestMerchantProjectionIsIdempotent(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedMerchant(t, pool)
	seedMerchantRole(t, pool, "editor", []string{"document:read"}, []string{"document:read"})
	grantToUser(t, pool, "g1", "ana", "editor", "apollo")
	containment(t, pool, "d1", "apollo")

	projectMerchant(t, pool)
	projectMerchant(t, pool)

	e := merchantEvaluator(t, db)
	if !mAllows(t, e, "ana", "read", "document", "d1") {
		t.Fatal("running the projection twice must not break it")
	}
}

// One merchant's model must never appear in another's namespace, which is the
// boundary the whole tier rests on.
func TestMerchantProjectionStaysInItsOwnNamespace(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedMerchant(t, pool)
	seedMerchantRole(t, pool, "editor", []string{"document:read"}, []string{"document:read"})
	grantToUser(t, pool, "g1", "ana", "editor", "apollo")
	containment(t, pool, "d1", "apollo")
	projectMerchant(t, pool)

	var leaked int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM reach_edges WHERE namespace <> $1 AND source LIKE 'app_%'`,
		accessmodel.MerchantNamespace(mOrg)).Scan(&leaked); err != nil {
		t.Fatal(err)
	}
	if leaked != 0 {
		t.Fatalf("the merchant projection wrote %d rows outside its namespace", leaked)
	}
}
