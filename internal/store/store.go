package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store is the Postgres persistence layer.
type Store struct {
	pool        *pgxpool.Pool
	databaseURL string
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
	return &Store{pool: pool, databaseURL: databaseURL}, nil
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

// Migrate applies all embedded up migrations.
func (s *Store) Migrate() error {
	return migrateUp(s.databaseURL)
}

func migrateUp(databaseURL string) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}

	// migrate's pgx driver expects a pgx5:// scheme.
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
	// Accept postgres:// and postgresql://; rewrite to pgx5:// for the driver.
	switch {
	case len(databaseURL) >= 11 && databaseURL[:11] == "postgres://":
		return "pgx5://" + databaseURL[11:]
	case len(databaseURL) >= 14 && databaseURL[:14] == "postgresql://":
		return "pgx5://" + databaseURL[14:]
	case len(databaseURL) >= 7 && databaseURL[:7] == "pgx5://":
		return databaseURL
	default:
		return databaseURL
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
