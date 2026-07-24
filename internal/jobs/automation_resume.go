package jobs

import (
	"context"
	"encoding/json"
	"time"
)

// AutomationResumeArgs continues an automation after a delay action.
type AutomationResumeArgs struct {
	OrganisationID string          `json:"organisation_id"`
	AutomationID   string          `json:"automation_id"`
	Trigger        string          `json:"trigger"`
	Payload        json.RawMessage `json:"payload"`
	NextActionID   string          `json:"next_action_id"`
}

func (AutomationResumeArgs) Kind() string { return "automation_resume" }

// ScheduleTickArgs is a periodic tick that fires due org schedule triggers.
type ScheduleTickArgs struct {
	At time.Time `json:"at"`
}

func (ScheduleTickArgs) Kind() string { return "schedule_tick" }

// ResumeFunc continues a paused automation workflow.
type ResumeFunc func(ctx context.Context, orgID, automationID, trigger string, payload json.RawMessage, nextActionID string) error

// ScheduleTickFunc evaluates which schedule triggers are due and fires them.
type ScheduleTickFunc func(ctx context.Context, at time.Time) error

// DueScheduleTriggers returns schedule.* trigger IDs that should fire at t (UTC).
func DueScheduleTriggers(t time.Time) []string {
	t = t.UTC()
	var out []string
	if t.Minute() == 0 {
		out = append(out, "schedule.hourly")
	}
	if t.Minute() == 0 && t.Hour() == 0 {
		out = append(out, "schedule.daily")
	}
	if t.Minute() == 0 && t.Hour() == 0 && t.Weekday() == time.Monday {
		out = append(out, "schedule.weekly")
	}
	return out
}

// EnqueueResumeInput is passed to delayed resume inserts.
type EnqueueResumeInput struct {
	OrganisationID string
	AutomationID   string
	Trigger        string
	Payload        any
	NextActionID   string
	RunAt          time.Time
}
