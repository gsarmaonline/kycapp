package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/obsstore"
	"github.com/gsarmaonline/kyc/internal/obsstore/sqlc"
)

// PostgresStore implements Store against the observability database.
type PostgresStore struct {
	db *obsstore.Store
}

// OpenPostgres connects, migrates, and returns a Store.
func OpenPostgres(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	db, err := obsstore.Open(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.Ping(ctx)
}

func (s *PostgresStore) Close() {
	s.db.Close()
}

func (s *PostgresStore) RecordActivity(ctx context.Context, a Activity) error {
	if strings.TrimSpace(a.OrganisationID) == "" {
		return fmt.Errorf("organisation_id is required")
	}
	if strings.TrimSpace(a.Action) == "" {
		return fmt.Errorf("action is required")
	}
	id := strings.TrimSpace(a.ID)
	if id == "" {
		id = ids.New()
	}
	raw, err := payloadJSON(a.Payload)
	if err != nil {
		return fmt.Errorf("payload: %w", err)
	}
	_, err = s.db.Q().InsertActivityEvent(ctx, sqlc.InsertActivityEventParams{
		ID:               id,
		OrganisationID:   a.OrganisationID,
		OrganisationSlug: a.OrganisationSlug,
		OrganisationName: a.OrganisationName,
		ActorType:        a.ActorType,
		ActorID:          a.ActorID,
		ActorLabel:       a.ActorLabel,
		Action:           a.Action,
		ResourceType:     a.ResourceType,
		ResourceID:       a.ResourceID,
		Summary:          a.Summary,
		Payload:          raw,
	})
	return err
}

func (s *PostgresStore) ListActivity(ctx context.Context, orgID string, opts ListActivityOpts) ([]Activity, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Q().ListActivityEventsByOrg(ctx, sqlc.ListActivityEventsByOrgParams{
		OrganisationID: orgID,
		Limit:          limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Activity, 0, len(rows))
	for _, r := range rows {
		out = append(out, activityFromRow(r))
	}
	return out, nil
}

func (s *PostgresStore) IncrUsage(ctx context.Context, d UsageDelta) error {
	if strings.TrimSpace(d.OrganisationID) == "" || strings.TrimSpace(d.MeterKey) == "" {
		return fmt.Errorf("organisation_id and meter_key are required")
	}
	delta := d.Delta
	if delta == 0 {
		delta = 1
	}
	period := d.PeriodStart
	if period.IsZero() {
		period = DayPeriodStart(time.Now().UTC())
	} else {
		period = DayPeriodStart(period)
	}
	_, err := s.db.Q().IncrUsageCounter(ctx, sqlc.IncrUsageCounterParams{
		OrganisationID: d.OrganisationID,
		MeterKey:       d.MeterKey,
		PeriodStart:    period,
		Dim1Key:        d.Dim1Key,
		Dim1Value:      d.Dim1Value,
		Dim2Key:        d.Dim2Key,
		Dim2Value:      d.Dim2Value,
		Count:          delta,
	})
	return err
}

func (s *PostgresStore) ListUsage(ctx context.Context, orgID string, opts ListUsageOpts) ([]UsageRow, error) {
	from := opts.From
	to := opts.To
	if from.IsZero() && to.IsZero() {
		now := time.Now().UTC()
		to = DayPeriodStart(now).AddDate(0, 0, 1)
		from = to.AddDate(0, 0, -30)
	} else {
		if !from.IsZero() {
			from = DayPeriodStart(from)
		}
		if !to.IsZero() {
			to = DayPeriodStart(to)
		}
		if to.IsZero() {
			to = DayPeriodStart(time.Now().UTC()).AddDate(0, 0, 1)
		}
		if from.IsZero() {
			from = to.AddDate(0, 0, -30)
		}
	}
	rows, err := s.db.Q().ListUsageCountersByOrgPeriod(ctx, sqlc.ListUsageCountersByOrgPeriodParams{
		OrganisationID: orgID,
		FromPeriod:     from,
		ToPeriod:       to,
	})
	if err != nil {
		return nil, err
	}
	out := make([]UsageRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, UsageRow{
			OrganisationID: r.OrganisationID,
			MeterKey:       r.MeterKey,
			PeriodStart:    r.PeriodStart,
			Dim1Key:        r.Dim1Key,
			Dim1Value:      r.Dim1Value,
			Dim2Key:        r.Dim2Key,
			Dim2Value:      r.Dim2Value,
			Count:          r.Count,
			UpdatedAt:      r.UpdatedAt,
		})
	}
	return out, nil
}

func (s *PostgresStore) DeleteOrganisation(ctx context.Context, orgID string) error {
	if err := s.db.Q().DeleteActivityEventsByOrg(ctx, orgID); err != nil {
		return err
	}
	return s.db.Q().DeleteUsageCountersByOrg(ctx, orgID)
}

func activityFromRow(r sqlc.ActivityEvent) Activity {
	var payload map[string]any
	if len(r.Payload) > 0 {
		_ = json.Unmarshal(r.Payload, &payload)
	}
	return Activity{
		ID:               r.ID,
		OrganisationID:   r.OrganisationID,
		OrganisationSlug: r.OrganisationSlug,
		OrganisationName: r.OrganisationName,
		ActorType:        r.ActorType,
		ActorID:          r.ActorID,
		ActorLabel:       r.ActorLabel,
		Action:           r.Action,
		ResourceType:     r.ResourceType,
		ResourceID:       r.ResourceID,
		Summary:          r.Summary,
		Payload:          payload,
		CreatedAt:        r.CreatedAt,
	}
}
