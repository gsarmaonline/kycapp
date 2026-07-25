package service_test

import (
	"context"
	"encoding/json"

	"github.com/gsarmaonline/kyc/internal/jobs"
	"github.com/gsarmaonline/kyc/internal/service"
)

// syncEnqueuer processes automation jobs inline (no River / worker). For local e2e only.
type syncEnqueuer struct {
	svc *service.Service
}

func (e *syncEnqueuer) EnqueueAutomationEvent(ctx context.Context, orgID, trigger string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return e.svc.ProcessAutomationEvent(ctx, orgID, trigger, raw)
}

func (e *syncEnqueuer) EnqueueAutomationResume(ctx context.Context, in jobs.EnqueueResumeInput) error {
	raw, err := json.Marshal(in.Payload)
	if err != nil {
		return err
	}
	return e.svc.ResumeAutomation(ctx, in.OrganisationID, in.AutomationID, in.Trigger, raw, in.NextActionID)
}
