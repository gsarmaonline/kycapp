package service

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/authn"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
)

const defaultPrimaryColor = "#1f4d3a"

// OnboardingStep is one Getting started setup step.
type OnboardingStep struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Done  bool   `json:"done"`
	Href  string `json:"href"`
}

// OnboardingView is the org dashboard onboarding payload.
type OnboardingView struct {
	Visible        bool             `json:"visible"`
	Dismissed      bool             `json:"dismissed"`
	CompletedCount int              `json:"completed_count"`
	TotalCount     int              `json:"total_count"`
	Steps          []OnboardingStep `json:"steps"`
}

type DismissOnboardingInput struct {
	Dismissed bool
}

// GetOrganisationOnboarding derives setup progress for the org dashboard.
// Members without organisation:update get visible=false and no steps.
func (s *Service) GetOrganisationOnboarding(ctx context.Context, orgID string) (OnboardingView, error) {
	p, err := s.RequireOrgMember(ctx, orgID)
	if err != nil {
		return OnboardingView{}, err
	}
	canManage, err := s.canManageOnboarding(ctx, p, orgID)
	if err != nil {
		return OnboardingView{}, err
	}
	if !canManage {
		return OnboardingView{Visible: false, Steps: []OnboardingStep{}}, nil
	}

	org, err := s.GetOrganisation(ctx, orgID)
	if err != nil {
		return OnboardingView{}, err
	}

	featureCount, err := s.db.Q().CountProductFeaturesByOrg(ctx, textArg(orgID))
	if err != nil {
		return OnboardingView{}, err
	}
	planCount, err := s.db.Q().CountProductPlansByOrg(ctx, orgID)
	if err != nil {
		return OnboardingView{}, err
	}
	automationCount, err := s.db.Q().CountAutomationsByOrg(ctx, orgID)
	if err != nil {
		return OnboardingView{}, err
	}
	appUserCount, err := s.db.Q().CountAppUsersByOrg(ctx, orgID)
	if err != nil {
		return OnboardingView{}, err
	}
	apiKeyCount, err := s.db.Q().CountActiveAPIKeysByOrg(ctx, textArg(orgID))
	if err != nil {
		return OnboardingView{}, err
	}
	capabilityCount, err := s.db.Q().CountAppCapabilitiesByOrg(ctx, orgID)
	if err != nil {
		return OnboardingView{}, err
	}

	steps := []OnboardingStep{
		{
			Key:   "branding",
			Label: "Brand your emails",
			Done:  brandingConfigured(org),
			Href:  "branding",
		},
		{
			Key:   "features",
			Label: "Define a product feature",
			Done:  featureCount > 0,
			Href:  "product-features",
		},
		{
			Key:   "plan",
			Label: "Package a product plan",
			Done:  planCount > 0,
			Href:  "product-plans",
		},
		{
			Key:   "automation",
			Label: "Add a lifecycle automation",
			Done:  automationCount > 0,
			Href:  "automations",
		},
		{
			Key:   "api_key",
			Label: "Create an API key",
			Done:  apiKeyCount > 0,
			Href:  "api-keys",
		},
		{
			Key:   "app_user",
			Label: "Create your first app user",
			Done:  appUserCount > 0,
			Href:  "users",
		},
		{
			// Customer access had five pages and no way in. Nothing is seeded
			// there, deliberately, because the vocabulary is the merchant's own
			// and a guessed default is worse than none. That leaves an empty
			// section nobody is pointed at, which this step fixes.
			//
			// Capabilities is the step rather than scope kinds or roles: a scope
			// kind grants nothing on its own, a role with no capabilities
			// carries nothing, and a grant needs both. Declaring the first
			// capability is where the section starts to mean something.
			Key:   "customer_access",
			Label: "Declare what your customers may do",
			Done:  capabilityCount > 0,
			Href:  "customer-capabilities",
		},
	}

	completed := 0
	for _, step := range steps {
		if step.Done {
			completed++
		}
	}
	dismissed := false
	row, err := s.db.Q().GetOrganisationOnboarding(ctx, orgID)
	if err == nil {
		dismissed = row.DismissedAt.Valid
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return OnboardingView{}, err
	}

	allDone := completed == len(steps)
	return OnboardingView{
		Visible:        !dismissed && !allDone,
		Dismissed:      dismissed,
		CompletedCount: completed,
		TotalCount:     len(steps),
		Steps:          steps,
	}, nil
}

// DismissOrganisationOnboarding hides the Getting started panel for the org.
func (s *Service) DismissOrganisationOnboarding(ctx context.Context, orgID string, in DismissOnboardingInput) (OnboardingView, error) {
	if _, err := s.RequireOrgPermission(ctx, orgID, "organisation:update"); err != nil {
		return OnboardingView{}, err
	}
	if !in.Dismissed {
		return OnboardingView{}, apperr.Validation("dismissed must be true")
	}
	if _, err := s.db.Q().UpsertOrganisationOnboardingDismissed(ctx, orgID); err != nil {
		return OnboardingView{}, err
	}
	return s.GetOrganisationOnboarding(ctx, orgID)
}

func (s *Service) canManageOnboarding(ctx context.Context, p authn.Principal, orgID string) (bool, error) {
	if p.IsPlatform() {
		return true, nil
	}
	if p.Kind == authn.KindService && p.OrganisationID == orgID {
		return len(p.Scopes) == 0 || slices.Contains(p.Scopes, "organisation:update"), nil
	}
	if p.Kind != authn.KindUser || p.UserID == "" {
		return false, nil
	}
	return s.CheckAuthz(ctx, AuthzCheckInput{
		OrganisationID: orgID,
		UserID:         p.UserID,
		Permission:     "organisation:update",
	})
}

func brandingConfigured(org sqlc.Organisation) bool {
	if strings.TrimSpace(org.LogoUrl) != "" {
		return true
	}
	if strings.TrimSpace(org.EmailFooter) != "" {
		return true
	}
	if strings.TrimSpace(org.AccentColor) != "" {
		return true
	}
	pc := strings.TrimSpace(org.PrimaryColor)
	return pc != "" && !strings.EqualFold(pc, defaultPrimaryColor)
}
