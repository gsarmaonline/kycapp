package observability

import (
	"context"
	"fmt"
	"strings"
)

// NewFromURL returns a Postgres-backed Store, or Noop when URL is empty.
func NewFromURL(ctx context.Context, databaseURL string) (Store, error) {
	u := strings.TrimSpace(databaseURL)
	if u == "" {
		return NewNoop(), nil
	}
	s, err := OpenPostgres(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("observability store: %w", err)
	}
	return s, nil
}
