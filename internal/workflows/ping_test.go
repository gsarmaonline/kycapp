package workflows

import (
	"context"
	"testing"
)

func TestPingActivity(t *testing.T) {
	got, err := PingActivity(context.Background(), "kyc")
	if err != nil {
		t.Fatal(err)
	}
	if got != "pong:kyc" {
		t.Fatalf("got %q", got)
	}
}
