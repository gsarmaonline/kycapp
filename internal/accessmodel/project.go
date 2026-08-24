package accessmodel

import (
	"context"
	_ "embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

//go:embed projection.sql
var projectionSQL string

// Project writes the current authorisation model into the edge table.
//
// It is idempotent: every statement is ON CONFLICT DO NOTHING, so running it
// twice changes nothing and running it again at cutover picks up whatever was
// written in between. It never deletes, so a row removed from the source tables
// is not removed here; that is what the differential run is for, and what the
// eventual cutover replaces.
//
// It is deliberately a function rather than a data migration. A projection
// buried in a schema change cannot be re-run and cannot be tested, and this one
// has to be both.
func Project(ctx context.Context, tx pgx.Tx) error {
	resources := ProjectedResources()
	for i, stmt := range projectionStatements() {
		// Only the statements that place a permission take the resource list.
		// Handing an argument to one that has no placeholder is an error, not a
		// no-op, so the presence of $1 decides.
		var args []any
		if strings.Contains(stripSQLComments(stmt), "$1") {
			args = append(args, resources)
		}
		if _, err := tx.Exec(ctx, stmt, args...); err != nil {
			return fmt.Errorf("accessmodel: projection statement %d: %w", i+1, err)
		}
	}
	return nil
}

// projectionStatements splits the embedded file on statement boundaries.
//
// Each statement takes the same one parameter, so they cannot simply be sent as
// one string: pgx would treat the batch as a single unnamed statement and
// refuse the repeated placeholder.
func projectionStatements() []string {
	var out []string
	for _, raw := range strings.Split(projectionSQL, ";") {
		if strings.TrimSpace(stripSQLComments(raw)) == "" {
			continue
		}
		out = append(out, raw+";")
	}
	return out
}

func stripSQLComments(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// ProjectedResources is the set of permission resources the projection knows
// how to place, taken from the mapping rather than repeated in SQL. A resource
// outside this set is skipped, and TestProjectionCoversEveryPermission fails
// rather than letting the omission pass silently.
func ProjectedResources() []string {
	seen := map[string]struct{}{}
	for key := range Permissions {
		resource, _, ok := strings.Cut(key, ":")
		if !ok {
			continue
		}
		seen[resource] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}
