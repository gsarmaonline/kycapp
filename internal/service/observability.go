package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/gsarmaonline/kyc/internal/authn"
	"github.com/gsarmaonline/kyc/internal/observability"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

func (s *Service) SetObservability(obs observability.Store) {
	if obs == nil {
		s.obs = observability.NewNoop()
		return
	}
	s.obs = obs
}

func (s *Service) Observability() observability.Store {
	if s.obs == nil {
		return observability.NewNoop()
	}
	return s.obs
}

func actorFromCtx(ctx context.Context) (actorType, actorID, actorLabel string) {
	p, ok := authn.FromContext(ctx)
	if !ok {
		return "system", "", "system"
	}
	switch p.Kind {
	case authn.KindUser:
		return "user", p.UserID, p.ActorLabel()
	case authn.KindService:
		id := p.APIKeyID
		if id == "" {
			id = "service"
		}
		return "service", id, p.ActorLabel()
	default:
		return "system", "", p.ActorLabel()
	}
}

func (s *Service) orgSnapshot(ctx context.Context, orgID string) (slug, name string) {
	org, err := s.db.Q().GetOrganisation(ctx, orgID)
	if err != nil {
		return "", ""
	}
	return org.Slug, org.Name
}

// recordActivity best-effort writes to the observability DB.
func (s *Service) recordActivity(ctx context.Context, a observability.Activity) {
	if s.obs == nil {
		return
	}
	if a.OrganisationSlug == "" && a.OrganisationName == "" && a.OrganisationID != "" {
		a.OrganisationSlug, a.OrganisationName = s.orgSnapshot(ctx, a.OrganisationID)
	}
	if a.ActorType == "" && a.ActorLabel == "" {
		a.ActorType, a.ActorID, a.ActorLabel = actorFromCtx(ctx)
	}
	if err := s.obs.RecordActivity(ctx, a); err != nil {
		slog.Warn("observability record activity failed", "action", a.Action, "org_id", a.OrganisationID, "err", err)
	}
}

func (s *Service) incrUsage(ctx context.Context, d observability.UsageDelta) {
	if s.obs == nil {
		return
	}
	if err := s.obs.IncrUsage(ctx, d); err != nil {
		slog.Warn("observability incr usage failed", "meter", d.MeterKey, "org_id", d.OrganisationID, "err", err)
	}
}

func (s *Service) ListOrganisationActivity(ctx context.Context, orgID string, limit int32) ([]observability.Activity, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return nil, err
	}
	return s.Observability().ListActivity(ctx, orgID, observability.ListActivityOpts{Limit: limit})
}

func (s *Service) ListOrganisationUsage(ctx context.Context, orgID string, from, to time.Time) ([]observability.UsageRow, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return nil, err
	}
	return s.Observability().ListUsage(ctx, orgID, observability.ListUsageOpts{From: from, To: to})
}

func entitlementResultLabel(allowed bool) string {
	if allowed {
		return "allowed"
	}
	return "denied"
}

func activityForSubscription(org sqlc.Organisation, sub sqlc.Subscription, action, summary string, payload map[string]any) observability.Activity {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["plan_id"] = sub.PlanID
	payload["status"] = sub.Status
	return observability.Activity{
		OrganisationID:   org.ID,
		OrganisationSlug: org.Slug,
		OrganisationName: org.Name,
		Action:           action,
		ResourceType:     "subscription",
		ResourceID:       sub.ID,
		Summary:          summary,
		Payload:          payload,
	}
}
