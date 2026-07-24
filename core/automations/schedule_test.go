package automations

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNormalizeScheduleTrigger(t *testing.T) {
	trig, params := NormalizeScheduleTrigger("schedule.hourly", nil)
	if trig != TriggerScheduleCron || params[ParamCronExpr] != "0 * * * *" {
		t.Fatalf("got %s %#v", trig, params)
	}
	trig, params = NormalizeScheduleTrigger("schedule.cron", map[string]string{
		ParamCronExpr: "0 9 * * 1-5",
		ParamTimezone: "Australia/Sydney",
	})
	if trig != TriggerScheduleCron || params[ParamTimezone] != "Australia/Sydney" {
		t.Fatalf("got %s %#v", trig, params)
	}
}

func TestCronDueAt(t *testing.T) {
	hourly := time.Date(2026, 7, 24, 15, 0, 30, 0, time.UTC)
	due, err := CronDueAt("0 * * * *", "UTC", hourly)
	if err != nil || !due {
		t.Fatalf("hourly due=%v err=%v", due, err)
	}
	mid := time.Date(2026, 7, 24, 15, 30, 0, 0, time.UTC)
	due, err = CronDueAt("0 * * * *", "UTC", mid)
	if err != nil || due {
		t.Fatalf("mid should not be due: due=%v err=%v", due, err)
	}

	// 2026-07-24 23:00 UTC = 2026-07-25 09:00 Australia/Sydney (AEST, UTC+10)
	sydneyNine := time.Date(2026, 7, 24, 23, 0, 0, 0, time.UTC)
	due, err = CronDueAt("0 9 * * *", "Australia/Sydney", sydneyNine)
	if err != nil || !due {
		t.Fatalf("sydney 9am due=%v err=%v", due, err)
	}
}

func TestValidateCreateScheduleCron(t *testing.T) {
	spec, err := ValidateCreate(
		"schedule.hourly",
		json.RawMessage(`{"all":[]}`),
		json.RawMessage(`[{"type":"call_webhook","params":{"webhook_id":"w1"}}]`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Trigger != TriggerScheduleCron || spec.TriggerParams[ParamCronExpr] != "0 * * * *" {
		t.Fatalf("%#v", spec)
	}
	spec, err = ValidateCreate(
		TriggerScheduleCron,
		json.RawMessage(`{"all":[]}`),
		json.RawMessage(`[{"type":"call_webhook","params":{"webhook_id":"w1"}}]`),
		json.RawMessage(`{"expr":"0 9 * * 1-5","timezone":"Australia/Sydney"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if spec.TriggerParams[ParamCronExpr] != "0 9 * * 1-5" {
		t.Fatalf("%#v", spec)
	}
}
