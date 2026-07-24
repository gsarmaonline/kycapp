package automations

import (
	"fmt"
	"strings"
	"time"
)

func init() {
	RegisterActionHandler(delayHandler{})
}

const ActionDelay = "delay"

type delayHandler struct{}

func (delayHandler) Type() string { return ActionDelay }

func (delayHandler) Info() ActionInfo {
	return ActionInfo{
		Type:  ActionDelay,
		Label: "Delay",
		Description: "Wait for a duration, then continue on the success path. " +
			"Uses a scheduled background job (does not block a worker).",
		Params: []ActionParam{
			{Key: "duration", Label: "Duration (e.g. 5m, 1h, 24h)", Required: true},
		},
		Requires: nil,
	}
}

func (delayHandler) Requires() []string { return nil }

func (delayHandler) Validate(params map[string]any) error {
	_, err := ParseDelayDuration(params)
	return err
}

// ParseDelayDuration reads params["duration"] (Go duration string).
// Min 1s, max 30 days.
func ParseDelayDuration(params map[string]any) (time.Duration, error) {
	raw, err := RequireStringParam(params, "duration")
	if err != nil {
		return 0, err
	}
	d, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("duration must be a Go duration (e.g. 5m, 1h, 24h)")
	}
	if d < time.Second {
		return 0, fmt.Errorf("duration must be at least 1s")
	}
	if d > 30*24*time.Hour {
		return 0, fmt.Errorf("duration must be at most 30 days")
	}
	return d, nil
}
