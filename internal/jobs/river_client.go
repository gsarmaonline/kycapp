package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// Client wraps River for insert-only (API) or worker use.
type Client struct {
	inner *river.Client[pgx.Tx]
}

// NewInsertClient creates a River client that only inserts jobs (no workers).
func NewInsertClient(pool *pgxpool.Pool) (*Client, error) {
	inner, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		return nil, err
	}
	return &Client{inner: inner}, nil
}

// NewWorkerClient creates a River client that processes automation jobs.
func NewWorkerClient(pool *pgxpool.Pool, process ProcessFunc) (*Client, error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, &AutomationEventWorker{Process: process})
	inner, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 4},
		},
		Workers: workers,
	})
	if err != nil {
		return nil, err
	}
	return &Client{inner: inner}, nil
}

// ProcessFunc runs when an automation_event job is worked.
type ProcessFunc func(ctx context.Context, orgID, trigger string, payload json.RawMessage) error

// AutomationEventWorker handles AutomationEventArgs.
type AutomationEventWorker struct {
	river.WorkerDefaults[AutomationEventArgs]
	Process ProcessFunc
}

func (w *AutomationEventWorker) Work(ctx context.Context, job *river.Job[AutomationEventArgs]) error {
	if w.Process == nil {
		return fmt.Errorf("automation process func not configured")
	}
	return w.Process(ctx, job.Args.OrganisationID, job.Args.Trigger, job.Args.Payload)
}

// EnqueueAutomationEvent implements service.Enqueuer.
func (c *Client) EnqueueAutomationEvent(ctx context.Context, orgID, trigger string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = c.inner.Insert(ctx, AutomationEventArgs{
		OrganisationID: orgID,
		Trigger:        trigger,
		Payload:        raw,
	}, nil)
	return err
}

// Start begins working jobs (worker clients only).
func (c *Client) Start(ctx context.Context) error {
	return c.inner.Start(ctx)
}

// Stop gracefully stops the client.
func (c *Client) Stop(ctx context.Context) error {
	return c.inner.Stop(ctx)
}

// LogInsertErrors is a helper for optional enqueue logging.
func LogInsertErrors(err error) {
	if err != nil {
		slog.Error("river insert", "err", err)
	}
}
