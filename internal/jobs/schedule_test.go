package jobs

import (
	"testing"
	"time"
)

func TestDueScheduleTriggers(t *testing.T) {
	hourly := time.Date(2026, 7, 24, 15, 0, 0, 0, time.UTC)
	got := DueScheduleTriggers(hourly)
	if len(got) != 1 || got[0] != "schedule.hourly" {
		t.Fatalf("hourly=%v", got)
	}

	daily := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC) // Friday
	got = DueScheduleTriggers(daily)
	if len(got) != 2 {
		t.Fatalf("daily=%v", got)
	}

	weekly := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC) // Monday
	got = DueScheduleTriggers(weekly)
	if len(got) != 3 {
		t.Fatalf("weekly=%v", got)
	}

	mid := time.Date(2026, 7, 24, 15, 30, 0, 0, time.UTC)
	if DueScheduleTriggers(mid) != nil && len(DueScheduleTriggers(mid)) != 0 {
		t.Fatalf("mid-hour should be empty: %v", DueScheduleTriggers(mid))
	}
}
