package jobs

import (
	"testing"
	"time"

	"github.com/gsarmaonline/kyc/core/automations"
)

func TestCronPresetsDueViaAutomationsHelper(t *testing.T) {
	hourly := time.Date(2026, 7, 24, 15, 0, 0, 0, time.UTC)
	due, err := automations.CronDueAt("0 * * * *", "UTC", hourly)
	if err != nil || !due {
		t.Fatalf("hourly due=%v err=%v", due, err)
	}
	daily := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	due, err = automations.CronDueAt("0 0 * * *", "UTC", daily)
	if err != nil || !due {
		t.Fatalf("daily due=%v err=%v", due, err)
	}
	mid := time.Date(2026, 7, 24, 15, 30, 0, 0, time.UTC)
	due, err = automations.CronDueAt("0 * * * *", "UTC", mid)
	if err != nil || due {
		t.Fatalf("mid should not fire: due=%v err=%v", due, err)
	}
}
