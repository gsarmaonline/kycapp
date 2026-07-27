package observability

import "context"

// Noop is a no-op Store used when OBSERVABILITY_DATABASE_URL is unset.
type Noop struct{}

func NewNoop() *Noop { return &Noop{} }

func (n *Noop) Ping(context.Context) error { return nil }

func (n *Noop) Close() {}

func (n *Noop) RecordActivity(context.Context, Activity) error { return nil }

func (n *Noop) ListActivity(context.Context, string, ListActivityOpts) ([]Activity, error) {
	return nil, nil
}

func (n *Noop) IncrUsage(context.Context, UsageDelta) error { return nil }

func (n *Noop) ListUsage(context.Context, string, ListUsageOpts) ([]UsageRow, error) {
	return nil, nil
}

func (n *Noop) DeleteOrganisation(context.Context, string) error { return nil }
