package store_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMigrationFilesEmbedded(t *testing.T) {
	files, err := store.MigrationFiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"000001_init.up.sql",
		"000001_init.down.sql",
		"000002_seed.up.sql",
		"000002_seed.down.sql",
	} {
		if !slices.Contains(files, want) {
			t.Fatalf("missing migration file %q in %v", want, files)
		}
	}
}

func TestMigrateAndSeed(t *testing.T) {
	ctx := context.Background()
	dsn := startPostgres(t, ctx)

	db, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(db.Close)

	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	count, err := db.PermissionCount(ctx)
	if err != nil {
		t.Fatalf("permission count: %v", err)
	}
	if count != 9 {
		t.Fatalf("permission count = %d, want 9", count)
	}

	exists, err := db.PlanExists(ctx, "trial")
	if err != nil {
		t.Fatalf("plan exists: %v", err)
	}
	if !exists {
		t.Fatal("trial plan should be seeded")
	}

	if err := db.Migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func startPostgres(t *testing.T, ctx context.Context) string {
	t.Helper()

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
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() {
		terminateCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := container.Terminate(terminateCtx); err != nil {
			t.Logf("terminate postgres: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	return dsn
}
