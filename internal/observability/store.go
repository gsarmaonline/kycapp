package observability

import (
	"context"
	"encoding/json"
	"time"
)

// Stable action IDs for activity_events.
const (
	ActionOrgCreated                = "org.created"
	ActionAPIKeyCreated             = "api_key.created"
	ActionAPIKeyRevoked             = "api_key.revoked"
	ActionSubscriptionCreated       = "subscription.created"
	ActionSubscriptionUpdated       = "subscription.updated"
	ActionSubscriptionStatusChanged = "subscription.status_changed"
)

// Meter keys for usage_counters.
const (
	MeterEntitlementCheck = "entitlement.check"
)

// Activity is a semantic org timeline event (denormalized for the obs DB).
type Activity struct {
	ID               string
	OrganisationID   string
	OrganisationSlug string
	OrganisationName string
	ActorType        string
	ActorID          string
	ActorLabel       string
	Action           string
	ResourceType     string
	ResourceID       string
	Summary          string
	Payload          map[string]any
	CreatedAt        time.Time
}

// UsageDelta increments a period counter.
type UsageDelta struct {
	OrganisationID string
	MeterKey       string
	PeriodStart    time.Time
	Dim1Key        string
	Dim1Value      string
	Dim2Key        string
	Dim2Value      string
	Delta          int64
}

// UsageRow is a rolled-up meter reading.
type UsageRow struct {
	OrganisationID string
	MeterKey       string
	PeriodStart    time.Time
	Dim1Key        string
	Dim1Value      string
	Dim2Key        string
	Dim2Value      string
	Count          int64
	UpdatedAt      time.Time
}

// ListActivityOpts controls activity listing.
type ListActivityOpts struct {
	Limit int32
}

// ListUsageOpts controls usage listing (UTC day periods by default).
type ListUsageOpts struct {
	From time.Time // inclusive period_start
	To   time.Time // exclusive period_start
}

// Store is the observability persistence port (separate database).
type Store interface {
	Ping(ctx context.Context) error
	Close()
	RecordActivity(ctx context.Context, a Activity) error
	ListActivity(ctx context.Context, orgID string, opts ListActivityOpts) ([]Activity, error)
	IncrUsage(ctx context.Context, d UsageDelta) error
	ListUsage(ctx context.Context, orgID string, opts ListUsageOpts) ([]UsageRow, error)
	DeleteOrganisation(ctx context.Context, orgID string) error
}

// DayPeriodStart truncates t to UTC midnight.
func DayPeriodStart(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

func payloadJSON(m map[string]any) (json.RawMessage, error) {
	if m == nil {
		return json.RawMessage(`{}`), nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return b, nil
}
