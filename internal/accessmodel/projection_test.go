package accessmodel_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gsarmaonline/kyc/core/reach"
	"github.com/gsarmaonline/kyc/internal/accessmodel"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// The projection is the whole basis for trusting the new engine, so it is
// exercised against a real Postgres rather than a fake. Each test seeds the
// current model's tables, runs the projection, and then asks the evaluator the
// same questions the current gates ask.

func openDB(t *testing.T) *store.Store {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("kyc"),
		postgres.WithUsername("kyc"),
		postgres.WithPassword("kyc"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(func() {
		stop, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := container.Terminate(stop); err != nil {
			t.Logf("terminate postgres: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	db, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(db.Close)
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// project runs the projection in its own transaction, as a caller would.
func project(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		return accessmodel.Project(ctx, tx)
	})
	if err != nil {
		t.Fatalf("project: %v", err)
	}
}

func exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %s: %v", strings.SplitN(strings.TrimSpace(sql), "\n", 2)[0], err)
	}
}

// platformOrgID returns the organisation membership of which is what makes
// someone staff. Global reach is derived from it and never stored, so a test
// that wants staff reach seeds a role there rather than setting a flag.
func platformOrgID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`SELECT platform_organisation_id FROM system_state WHERE id = 1`).Scan(&id); err != nil {
		t.Skipf("platform organisation not seeded: %v", err)
	}
	if id == "" {
		t.Skip("platform organisation not seeded")
	}
	return id
}

// seedTenant creates an organisation, a role holding the named permissions, and
// a member of it. It mirrors what signup and role management write today.
func seedTenant(t *testing.T, pool *pgxpool.Pool, orgID, roleID, userID string, permissionKeys ...string) {
	t.Helper()
	exec(t, pool, `INSERT INTO organisations (id, name, slug, status)
		VALUES ($1, $1, $1, 'active') ON CONFLICT (id) DO NOTHING`, orgID)
	exec(t, pool, `INSERT INTO users (id, email, name, status)
		VALUES ($1, $1 || '@example.test', $1, 'active') ON CONFLICT (id) DO NOTHING`, userID)
	exec(t, pool, `INSERT INTO roles (id, organisation_id, key, name)
		VALUES ($1, $2, $1, $1) ON CONFLICT (id) DO NOTHING`, roleID, orgID)
	for _, key := range permissionKeys {
		exec(t, pool, `INSERT INTO role_permissions (role_id, permission_id)
			SELECT $1, p.id FROM permissions p WHERE p.key = $2
			ON CONFLICT DO NOTHING`, roleID, key)
	}
	exec(t, pool, `INSERT INTO memberships (id, organisation_id, user_id, role_id, status)
		VALUES ($1, $2, $3, $4, 'active') ON CONFLICT (id) DO NOTHING`,
		"m_"+roleID+"_"+userID, orgID, userID, roleID)
}

func evaluator(t *testing.T, db *store.Store) *reach.Evaluator {
	t.Helper()
	e, err := accessmodel.NewEvaluatorFrom(db.Q(), accessmodel.SourceEdges)
	if err != nil {
		t.Fatalf("evaluator: %v", err)
	}
	return e
}

func holds(t *testing.T, e *reach.Evaluator, subject reach.NodeRef, permissionKey, orgID string) bool {
	t.Helper()
	p, ok := accessmodel.Permissions[permissionKey]
	if !ok {
		t.Fatalf("permission %q is not in the projection table", permissionKey)
	}
	d, err := e.Check(context.Background(), reach.Request{
		Subject:  subject,
		Action:   p.Action,
		Resource: accessmodel.Area(p.Type, orgID),
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("check %s %s in %s: %v", subject, permissionKey, orgID, err)
	}
	return d.Allowed
}

func assertHolds(t *testing.T, e *reach.Evaluator, subject reach.NodeRef, key, orgID string, want bool) {
	t.Helper()
	if got := holds(t, e, subject, key, orgID); got != want {
		t.Errorf("%s holds %s in %s = %v, wanted %v", subject, key, orgID, got, want)
	}
}

// --- The projection ---

func TestProjectionReproducesOrgScopedPermissions(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedTenant(t, pool, "acme", "acme_ops", "u9", "api_keys:manage", "billing:read")
	seedTenant(t, pool, "globex", "globex_ops", "u12", "api_keys:manage")
	project(t, pool)

	e := evaluator(t, db)
	u9 := reach.Node("user", "u9")

	assertHolds(t, e, u9, "api_keys:manage", "acme", true)
	assertHolds(t, e, u9, "billing:read", "acme", true)
	// Granularity survives: the role holds billing:read and not billing:manage.
	assertHolds(t, e, u9, "billing:manage", "acme", false)
	// And nothing leaks across the tenancy boundary.
	assertHolds(t, e, u9, "api_keys:manage", "globex", false)
	assertHolds(t, e, reach.Node("user", "u12"), "api_keys:manage", "acme", false)
}

func TestProjectionMakesEveryMemberReachTheirOrganisation(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	// A role with no permissions at all still confers reach.
	seedTenant(t, pool, "acme", "acme_guest", "u9")
	project(t, pool)

	e := evaluator(t, db)
	assertHolds(t, e, reach.Node("user", "u9"), "organisation:member", "acme", true)
	assertHolds(t, e, reach.Node("user", "nobody"), "organisation:member", "acme", false)
}

func TestProjectionWritesGlobalReachOnTheStarNode(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	exec(t, pool, `INSERT INTO organisations (id, name, slug, status)
		VALUES ('acme', 'acme', 'acme', 'active') ON CONFLICT (id) DO NOTHING`)
	seedTenant(t, pool, platformOrgID(t, pool), "platform_support", "sup", "api_keys:read")
	project(t, pool)

	e := evaluator(t, db)
	sup := reach.Node("user", "sup")

	// One edge on the star node reaches every tenant, including ones that do
	// not exist yet.
	assertHolds(t, e, sup, "api_keys:read", "acme", true)
	assertHolds(t, e, sup, "api_keys:read", "a-tenant-created-tomorrow", true)
	// And a read-only support role stays read-only, everywhere.
	assertHolds(t, e, sup, "api_keys:manage", "acme", false)
	assertHolds(t, e, sup, "billing:manage", "acme", false)
}

func TestProjectionCarriesMembershipExpiry(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedTenant(t, pool, "acme", "acme_ops", "u9", "billing:manage")
	exec(t, pool, `UPDATE memberships SET expires_at = now() - interval '1 hour' WHERE user_id = 'u9'`)
	project(t, pool)

	e := evaluator(t, db)
	assertHolds(t, e, reach.Node("user", "u9"), "billing:manage", "acme", false)
}

func TestProjectionHidesASuspendedTenantFromItsMembers(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedTenant(t, pool, "acme", "acme_ops", "u9", "billing:read")
	exec(t, pool, `UPDATE organisations SET status = 'suspended' WHERE id = 'acme'`)
	project(t, pool)

	e := evaluator(t, db)
	assertHolds(t, e, reach.Node("user", "u9"), "organisation:member", "acme", false)
}

func TestProjectionKeepsASuspendedTenantVisibleToPlatformStaff(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedTenant(t, pool, "acme", "acme_ops", "u9", "billing:read")
	seedTenant(t, pool, platformOrgID(t, pool), "platform_admin", "staff", "organisation:read")
	exec(t, pool, `UPDATE organisations SET status = 'suspended' WHERE id = 'acme'`)
	project(t, pool)

	e := evaluator(t, db)
	// Suspension must not be a one-way door: the route that would restore the
	// tenant has to stay reachable.
	assertHolds(t, e, reach.Node("user", "staff"), "organisation:member", "acme", true)
	assertHolds(t, e, reach.Node("user", "u9"), "organisation:member", "acme", false)
}

// --- API keys ---

func TestProjectionMaterialisesAnUnscopedKeyAsItsOwnersReach(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedTenant(t, pool, "acme", "acme_ops", "u9", "api_keys:manage", "billing:read")
	exec(t, pool, `INSERT INTO api_keys (id, name, key_prefix, key_hash, user_id, organisation_id, scopes)
		VALUES ('k4', 'k4', 'kyc_', 'hash_k4', 'u9', 'acme', '{}')`)
	project(t, pool)

	e := evaluator(t, db)
	k4 := reach.Node("key", "k4")

	assertHolds(t, e, k4, "api_keys:manage", "acme", true)
	assertHolds(t, e, k4, "billing:read", "acme", true)
	// It cannot exceed its owner.
	assertHolds(t, e, k4, "billing:manage", "acme", false)
	assertHolds(t, e, k4, "api_keys:manage", "globex", false)
}

func TestProjectionNarrowsAScopedKey(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedTenant(t, pool, "acme", "acme_ops", "u9", "api_keys:manage", "billing:read")
	exec(t, pool, `INSERT INTO api_keys (id, name, key_prefix, key_hash, user_id, organisation_id, scopes)
		VALUES ('k7', 'k7', 'kyc_', 'hash_k7', 'u9', 'acme', ARRAY['billing:read'])`)
	project(t, pool)

	e := evaluator(t, db)
	k7 := reach.Node("key", "k7")

	// The scope list is applied once, here, rather than re-derived per request.
	assertHolds(t, e, k7, "billing:read", "acme", true)
	assertHolds(t, e, k7, "api_keys:manage", "acme", false)
}

func TestProjectionGivesARevokedKeyNothing(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedTenant(t, pool, "acme", "acme_ops", "u9", "billing:read")
	exec(t, pool, `INSERT INTO api_keys (id, name, key_prefix, key_hash, user_id, organisation_id, scopes, revoked_at)
		VALUES ('kx', 'kx', 'kyc_', 'hash_kx', 'u9', 'acme', '{}', now())`)
	project(t, pool)

	e := evaluator(t, db)
	assertHolds(t, e, reach.Node("key", "kx"), "billing:read", "acme", false)
}

func TestProjectionGivesAnOwnerlessKeyNothing(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedTenant(t, pool, "acme", "acme_ops", "u9", "billing:read")
	exec(t, pool, `INSERT INTO api_keys (id, name, key_prefix, key_hash, organisation_id, scopes)
		VALUES ('ko', 'ko', 'kyc_', 'hash_ko', 'acme', '{}')`)
	project(t, pool)

	e := evaluator(t, db)
	assertHolds(t, e, reach.Node("key", "ko"), "billing:read", "acme", false)
}

func TestOwnerEdgeIsWrittenButConfersNothing(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedTenant(t, pool, "acme", "acme_ops", "u9", "api_keys:manage")
	exec(t, pool, `INSERT INTO api_keys (id, name, key_prefix, key_hash, user_id, organisation_id, scopes)
		VALUES ('k4', 'k4', 'kyc_', 'hash_k4', 'u9', 'acme', ARRAY['billing:read'])`)
	project(t, pool)

	// The sweep can find it, which is the whole reason the edge exists.
	res := accessmodel.NewResolverFrom(db.Q(), accessmodel.SourceEdges)
	edges, err := res.EdgesForSubject(context.Background(), reach.Node("user", "u9"))
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, e := range edges {
		if e.Object == reach.Node("key", "k4") && e.Relation == "owner" {
			found = true
		}
	}
	if !found {
		t.Fatal("owner edge was not written, so a departing person's keys could not be swept")
	}

	// And it grants nothing: the key holds only its scope.
	e := evaluator(t, db)
	assertHolds(t, e, reach.Node("key", "k4"), "api_keys:manage", "acme", false)
}

// --- Properties of the projection itself ---

func TestProjectionIsIdempotent(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedTenant(t, pool, "acme", "acme_ops", "u9", "api_keys:manage", "billing:read")
	project(t, pool)
	first := countEdges(t, pool)

	project(t, pool)
	if second := countEdges(t, pool); second != first {
		t.Fatalf("re-running the projection changed the row count: %d then %d", first, second)
	}
}

func TestProjectionCoversEveryRolePermissionRow(t *testing.T) {
	db := openDB(t)
	pool := db.Pool()

	seedTenant(t, pool, "acme", "acme_ops", "u9")
	// Give the role every permission the catalog holds, so a resource the
	// projection cannot place shows up as a missing row rather than silently.
	exec(t, pool, `INSERT INTO role_permissions (role_id, permission_id)
		SELECT 'acme_ops', p.id FROM permissions p ON CONFLICT DO NOTHING`)
	project(t, pool)

	var unplaced int
	err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM role_permissions rp
		JOIN roles r ON r.id = rp.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE r.id = 'acme_ops'
		  AND NOT EXISTS (
		    SELECT 1 FROM reach_edges e
		    WHERE e.subject_type = 'role' AND e.subject_id = r.id
		      AND e.relation = 'can_' || p.action
		      AND e.object_type = CASE p.resource WHEN 'roles' THEN 'org_roles' ELSE p.resource END
		  )`).Scan(&unplaced)
	if err != nil {
		t.Fatal(err)
	}
	if unplaced != 0 {
		t.Errorf("%d role_permissions rows were not projected; the mapping in schema.go is incomplete", unplaced)
	}
}

func TestEdgeTableRefusesAStarType(t *testing.T) {
	db := openDB(t)
	// Reach over every type at once stays outside the data, where it can be
	// counted. The database refuses it even if application code asks.
	_, err := db.Pool().Exec(context.Background(),
		`INSERT INTO reach_edges (namespace, object_type, object_id, relation, subject_type, subject_id)
		 VALUES ('kyc', '*', 'x', 'can_read', 'user', 'u9')`)
	if err == nil {
		t.Fatal("the edge table accepted a star type")
	}
}

func countEdges(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM reach_edges WHERE namespace = 'kyc'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
