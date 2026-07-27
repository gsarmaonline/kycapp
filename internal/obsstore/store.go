package obsstore

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
	"github.com/gsarmaonline/kyc/internal/obsstore/sqlc"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store is the observability Postgres persistence layer (separate database).
type Store struct {
	pool        *pgxpool.Pool
	databaseURL string
	q           *sqlc.Queries
}

// Open connects to the observability Postgres database.
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect observability postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping observability postgres: %w", err)
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

// Migrate applies embedded observability migrations.
func (s *Store) Migrate() error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("obs migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, toMigrateURL(s.databaseURL))
	if err != nil {
		return fmt.Errorf("obs migrate init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("obs migrate up: %w", err)
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
