package featureflags

import (
	"fmt"
	"testing"
)

func TestBucketSticky(t *testing.T) {
	a := Bucket("new_checkout", "user_1")
	b := Bucket("new_checkout", "user_1")
	if a != b {
		t.Fatalf("bucket not sticky: %d vs %d", a, b)
	}
	if a < 0 || a > 99 {
		t.Fatalf("bucket out of range: %d", a)
	}
	c := Bucket("new_checkout", "user_2")
	if a == c {
		_ = c
	}
	d := Bucket("other_flag", "user_1")
	if a == d {
		t.Fatal("expected different flags to hash differently for same subject")
	}
}

func TestInRollout(t *testing.T) {
	if InRollout("f", "s", 0) {
		t.Fatal("0% should be off")
	}
	if !InRollout("f", "s", 100) {
		t.Fatal("100% should be on")
	}
	if InRollout("f", "", 50) {
		t.Fatal("empty subject should be off for partial rollout")
	}
	on := 0
	for i := 0; i < 200; i++ {
		if InRollout("flag_x", fmt.Sprintf("user_%d", i), 50) {
			on++
		}
	}
	if on < 60 || on > 140 {
		t.Fatalf("expected ~50%% of 200 subjects, got %d", on)
	}
}

func TestEvaluate(t *testing.T) {
	enabled, reason := Evaluate(false, 100, "f", "s", "include")
	if enabled || reason != ReasonDisabled {
		t.Fatalf("kill switch should win: %v %s", enabled, reason)
	}
	enabled, reason = Evaluate(true, 0, "f", "s", "include")
	if !enabled || reason != ReasonOverrideOn {
		t.Fatalf("include override: %v %s", enabled, reason)
	}
	enabled, reason = Evaluate(true, 100, "f", "s", "exclude")
	if enabled || reason != ReasonOverrideOff {
		t.Fatalf("exclude override: %v %s", enabled, reason)
	}
	enabled, reason = Evaluate(true, 100, "f", "s", "")
	if !enabled || reason != ReasonFull {
		t.Fatalf("full rollout: %v %s", enabled, reason)
	}
	enabled, reason = Evaluate(true, 0, "f", "s", "")
	if enabled || reason != ReasonOff {
		t.Fatalf("zero rollout: %v %s", enabled, reason)
	}
}
