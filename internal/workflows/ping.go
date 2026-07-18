package workflows

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const TaskQueue = "kyc"

// PingWorkflow is a smoke-test workflow: activity returns "pong:<name>".
func PingWorkflow(ctx workflow.Context, name string) (string, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	})
	var result string
	if err := workflow.ExecuteActivity(ctx, PingActivity, name).Get(ctx, &result); err != nil {
		return "", err
	}
	return result, nil
}

// PingActivity is the activity registered for PingWorkflow.
func PingActivity(_ context.Context, name string) (string, error) {
	if name == "" {
		name = "world"
	}
	return fmt.Sprintf("pong:%s", name), nil
}

// Register attaches KYC workflows and activities to a Temporal worker.
func Register(w worker.Worker) {
	w.RegisterWorkflow(PingWorkflow)
	w.RegisterActivity(PingActivity)
}
