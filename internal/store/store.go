package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store is the Postgres persistence layer.
type Store struct {
	pool        *pgxpool.Pool
	databaseURL string
	q           *sqlc.Queries
}

// Open connects to Postgres using databaseURL.
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{
		pool:        pool,
		databaseURL: databaseURL,
		q:           sqlc.New(pool),
	}, nil
}

// Close releases the connection pool.
func (s *Store) Close() {
	if s == nil || s.pool == nil {
		return
	}
	s.pool.Close()
}

// Ping checks database connectivity.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Q returns the sqlc queries bound to the pool.
func (s *Store) Q() *sqlc.Queries {
	return s.q
}

// Pool returns the underlying pgx pool (River, etc.).
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

// WithTx runs fn inside a database transaction.
func (s *Store) WithTx(ctx context.Context, fn func(q *sqlc.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := fn(sqlc.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// Migrate applies all embedded up migrations, then River's schema.
func (s *Store) Migrate() error {
	if err := migrateUp(s.databaseURL); err != nil {
		return err
	}
	return s.MigrateRiver(context.Background())
}

// MigrateRiver applies River queue tables (idempotent).
func (s *Store) MigrateRiver(ctx context.Context) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(s.pool), nil)
	if err != nil {
		return fmt.Errorf("river migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, &rivermigrate.MigrateOpts{}); err != nil {
		return fmt.Errorf("river migrate: %w", err)
	}
	return nil
}

func migrateUp(databaseURL string) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, toMigrateURL(databaseURL))
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func toMigrateURL(databaseURL string) string {
	u := strings.Trim(strings.TrimSpace(databaseURL), `"'`)
	scheme, rest, ok := strings.Cut(u, "://")
	if !ok {
		return u
	}
	switch strings.ToLower(scheme) {
	case "postgres", "postgresql", "pgx5":
		return "pgx5://" + rest
	default:
		return u
	}
}

// PermissionCount returns how many permissions are in the catalog (for smoke tests).
func (s *Store) PermissionCount(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM permissions`).Scan(&n)
	return n, err
}

// PlanExists reports whether a plan with the given key exists.
func (s *Store) PlanExists(ctx context.Context, key string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM plans WHERE key = $1)`, key).Scan(&exists)
	return exists, err
}

// IsUniqueViolation reports whether err is a Postgres unique_violation.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// MigrationFiles returns embedded migration filenames (for tests).
func MigrationFiles() ([]string, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
