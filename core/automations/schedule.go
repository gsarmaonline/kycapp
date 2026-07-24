package automations

import (
	"fmt"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/core/resources"
	"github.com/robfig/cron/v3"
)

const DefaultTimezone = "UTC"

// TriggerScheduleCron is the unified org schedule trigger.
var TriggerScheduleCron = resources.LifecycleTrigger(resources.Schedule, resources.ScheduleCron)

// LegacyScheduleExprs maps removed schedule.* presets to standard 5-field cron (UTC).
var LegacyScheduleExprs = map[string]string{
	resources.LifecycleTrigger(resources.Schedule, resources.ScheduleHourly): "0 * * * *",
	resources.LifecycleTrigger(resources.Schedule, resources.ScheduleDaily):  "0 0 * * *",
	resources.LifecycleTrigger(resources.Schedule, resources.ScheduleWeekly): "0 0 * * 1",
}

// SchedulePresets are UI shortcuts that fill expr (still stored as schedule.cron).
var SchedulePresets = []struct {
	Key   string
	Label string
	Expr  string
}{
	{Key: "hourly", Label: "Hourly", Expr: "0 * * * *"},
	{Key: "daily", Label: "Daily (midnight)", Expr: "0 0 * * *"},
	{Key: "weekly", Label: "Weekly (Mon midnight)", Expr: "0 0 * * 1"},
	{Key: "weekdays_9am", Label: "Weekdays 09:00", Expr: "0 9 * * 1-5"},
}

// NormalizeScheduleTrigger rewrites legacy schedule.hourly|daily|weekly into
// schedule.cron + expr / timezone. Other triggers are unchanged.
func NormalizeScheduleTrigger(trigger string, params map[string]string) (string, map[string]string) {
	trigger = strings.TrimSpace(trigger)
	out := map[string]string{}
	for k, v := range params {
		out[k] = v
	}
	if expr, ok := LegacyScheduleExprs[trigger]; ok {
		out[ParamCronExpr] = expr
		if strings.TrimSpace(out[ParamTimezone]) == "" {
			out[ParamTimezone] = DefaultTimezone
		}
		return TriggerScheduleCron, out
	}
	return trigger, out
}

// ValidateCronExpr checks a standard 5-field cron expression.
func ValidateCronExpr(expr string) error {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return fmt.Errorf("cron expr is required")
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(expr); err != nil {
		return fmt.Errorf("invalid cron expr %q: %w", expr, err)
	}
	return nil
}

// ValidateTimezone accepts IANA names (or UTC).
func ValidateTimezone(tz string) error {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return nil
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("invalid timezone %q", tz)
	}
	return nil
}

// CronDueAt reports whether expr fires at the given instant in tz (minute resolution).
func CronDueAt(expr, timezone string, at time.Time) (bool, error) {
	if err := ValidateCronExpr(expr); err != nil {
		return false, err
	}
	tz := strings.TrimSpace(timezone)
	if tz == "" {
		tz = DefaultTimezone
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return false, fmt.Errorf("invalid timezone %q", tz)
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(strings.TrimSpace(expr))
	if err != nil {
		return false, err
	}
	local := at.In(loc)
	floor := time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), local.Minute(), 0, 0, loc)
	prev := sched.Next(floor.Add(-time.Second))
	return prev.Equal(floor), nil
}
