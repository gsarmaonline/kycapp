package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

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

// WorkerHooks wires process / resume / schedule handlers into the worker client.
type WorkerHooks struct {
	Process      ProcessFunc
	Resume       ResumeFunc
	ScheduleTick ScheduleTickFunc
}

// NewWorkerClient creates a River client that processes automation jobs.
func NewWorkerClient(pool *pgxpool.Pool, hooks WorkerHooks) (*Client, error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, &AutomationEventWorker{Process: hooks.Process})
	river.AddWorker(workers, &AutomationResumeWorker{Resume: hooks.Resume})
	river.AddWorker(workers, &ScheduleTickWorker{Tick: hooks.ScheduleTick})

	periodic := []*river.PeriodicJob{
		river.NewPeriodicJob(
			river.PeriodicInterval(time.Minute),
			func() (river.JobArgs, *river.InsertOpts) {
				return ScheduleTickArgs{At: time.Now().UTC()}, nil
			},
			&river.PeriodicJobOpts{RunOnStart: false},
		),
	}

	inner, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 4},
		},
		Workers:      workers,
		PeriodicJobs: periodic,
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

// AutomationResumeWorker handles AutomationResumeArgs.
type AutomationResumeWorker struct {
	river.WorkerDefaults[AutomationResumeArgs]
	Resume ResumeFunc
}

func (w *AutomationResumeWorker) Work(ctx context.Context, job *river.Job[AutomationResumeArgs]) error {
	if w.Resume == nil {
		return fmt.Errorf("automation resume func not configured")
	}
	return w.Resume(ctx, job.Args.OrganisationID, job.Args.AutomationID, job.Args.Trigger, job.Args.Payload, job.Args.NextActionID)
}

// ScheduleTickWorker handles ScheduleTickArgs.
type ScheduleTickWorker struct {
	river.WorkerDefaults[ScheduleTickArgs]
	Tick ScheduleTickFunc
}

func (w *ScheduleTickWorker) Work(ctx context.Context, job *river.Job[ScheduleTickArgs]) error {
	if w.Tick == nil {
		return fmt.Errorf("schedule tick func not configured")
	}
	at := job.Args.At
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return w.Tick(ctx, at)
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

// EnqueueAutomationResume schedules continuation after a delay.
func (c *Client) EnqueueAutomationResume(ctx context.Context, in EnqueueResumeInput) error {
	raw, err := json.Marshal(in.Payload)
	if err != nil {
		return err
	}
	_, err = c.inner.Insert(ctx, AutomationResumeArgs{
		OrganisationID: in.OrganisationID,
		AutomationID:   in.AutomationID,
		Trigger:        in.Trigger,
		Payload:        raw,
		NextActionID:   in.NextActionID,
	}, &river.InsertOpts{ScheduledAt: in.RunAt})
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
