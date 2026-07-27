package service

import (
	"context"
	"errors"
	"strings"

	"github.com/gsarmaonline/kyc/core/featureflags"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type FeatureFlagView struct {
	Flag      sqlc.FeatureFlag
	Overrides []sqlc.FeatureFlagOverride
}

type CreateFeatureFlagInput struct {
	Key                string
	Description        string
	Enabled            *bool
	RolloutPercentage  *int32
}

type UpdateFeatureFlagInput struct {
	Description       *string
	Enabled           *bool
	RolloutPercentage *int32
}

type FeatureFlagOverrideInput struct {
	SubjectID string
	Effect    string
}

type SetFeatureFlagOverridesInput struct {
	Overrides []FeatureFlagOverrideInput
}

type CheckFeatureFlagResult struct {
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason"`
}

func (s *Service) CreateFeatureFlag(ctx context.Context, orgID string, in CreateFeatureFlagInput) (FeatureFlagView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return FeatureFlagView{}, err
	}
	key := strings.TrimSpace(in.Key)
	if key == "" {
		return FeatureFlagView{}, apperr.Validation("key is required")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	pct := int32(0)
	if in.RolloutPercentage != nil {
		pct = *in.RolloutPercentage
		if pct < 0 || pct > 100 {
			return FeatureFlagView{}, apperr.Validation("rollout_percentage must be between 0 and 100")
		}
	}
	flag, err := s.db.Q().CreateFeatureFlag(ctx, sqlc.CreateFeatureFlagParams{
		ID:                 ids.New(),
		OrganisationID:     orgID,
		Key:                key,
		Description:        strings.TrimSpace(in.Description),
		Enabled:            enabled,
		RolloutPercentage:  pct,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			return FeatureFlagView{}, apperr.Conflict("feature flag key already exists")
		}
		return FeatureFlagView{}, err
	}
	return FeatureFlagView{Flag: flag, Overrides: []sqlc.FeatureFlagOverride{}}, nil
}

func (s *Service) ListFeatureFlags(ctx context.Context, orgID string) ([]FeatureFlagView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return nil, err
	}
	flags, err := s.db.Q().ListFeatureFlagsByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]FeatureFlagView, 0, len(flags))
	for _, f := range flags {
		view, err := s.featureFlagView(ctx, f)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, nil
}

func (s *Service) GetFeatureFlag(ctx context.Context, id string) (FeatureFlagView, error) {
	flag, err := s.db.Q().GetFeatureFlag(ctx, id)
	if err != nil {
		return FeatureFlagView{}, mapNotFound(err, "feature flag not found")
	}
	return s.featureFlagView(ctx, flag)
}

func (s *Service) UpdateFeatureFlag(ctx context.Context, id string, in UpdateFeatureFlagInput) (FeatureFlagView, error) {
	existing, err := s.GetFeatureFlag(ctx, id)
	if err != nil {
		return FeatureFlagView{}, err
	}
	params := sqlc.UpdateFeatureFlagParams{
		ID:             id,
		OrganisationID: existing.Flag.OrganisationID,
	}
	if in.Description != nil {
		params.Description = pgtype.Text{String: strings.TrimSpace(*in.Description), Valid: true}
	}
	if in.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *in.Enabled, Valid: true}
	}
	if in.RolloutPercentage != nil {
		if *in.RolloutPercentage < 0 || *in.RolloutPercentage > 100 {
			return FeatureFlagView{}, apperr.Validation("rollout_percentage must be between 0 and 100")
		}
		params.RolloutPercentage = pgtype.Int4{Int32: *in.RolloutPercentage, Valid: true}
	}
	flag, err := s.db.Q().UpdateFeatureFlag(ctx, params)
	if err != nil {
		return FeatureFlagView{}, mapNotFound(err, "feature flag not found")
	}
	return s.featureFlagView(ctx, flag)
}

func (s *Service) DeleteFeatureFlag(ctx context.Context, id string) error {
	existing, err := s.GetFeatureFlag(ctx, id)
	if err != nil {
		return err
	}
	return s.db.Q().DeleteFeatureFlag(ctx, sqlc.DeleteFeatureFlagParams{
		ID:             id,
		OrganisationID: existing.Flag.OrganisationID,
	})
}

func (s *Service) SetFeatureFlagOverrides(ctx context.Context, id string, in SetFeatureFlagOverridesInput) (FeatureFlagView, error) {
	existing, err := s.GetFeatureFlag(ctx, id)
	if err != nil {
		return FeatureFlagView{}, err
	}
	seen := make(map[string]struct{}, len(in.Overrides))
	for _, o := range in.Overrides {
		subj := strings.TrimSpace(o.SubjectID)
		if subj == "" {
			return FeatureFlagView{}, apperr.Validation("override subject_id is required")
		}
		effect := strings.TrimSpace(o.Effect)
		switch effect {
		case "include", "exclude":
		default:
			return FeatureFlagView{}, apperr.Validation("override effect must be include or exclude")
		}
		if _, ok := seen[subj]; ok {
			return FeatureFlagView{}, apperr.Validation("duplicate override subject_id")
		}
		seen[subj] = struct{}{}
	}
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		if err := q.DeleteFeatureFlagOverrides(ctx, id); err != nil {
			return err
		}
		for _, o := range in.Overrides {
			if err := q.UpsertFeatureFlagOverride(ctx, sqlc.UpsertFeatureFlagOverrideParams{
				FeatureFlagID: id,
				SubjectID:     strings.TrimSpace(o.SubjectID),
				Effect:        strings.TrimSpace(o.Effect),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return FeatureFlagView{}, err
	}
	return s.GetFeatureFlag(ctx, existing.Flag.ID)
}

func (s *Service) CheckFeatureFlag(ctx context.Context, orgID, flagKey, subjectID string) (CheckFeatureFlagResult, error) {
	flagKey = strings.TrimSpace(flagKey)
	subjectID = strings.TrimSpace(subjectID)
	if orgID == "" || flagKey == "" {
		return CheckFeatureFlagResult{}, apperr.Validation("organisation_id and flag are required")
	}
	org, err := s.GetOrganisation(ctx, orgID)
	if err != nil {
		return CheckFeatureFlagResult{}, err
	}
	if org.Status != "active" {
		return CheckFeatureFlagResult{Enabled: false, Reason: featureflags.ReasonDisabled}, nil
	}
	flag, err := s.db.Q().GetFeatureFlagByOrgKey(ctx, sqlc.GetFeatureFlagByOrgKeyParams{
		OrganisationID: orgID,
		Key:            flagKey,
	})
	if err != nil {
		return CheckFeatureFlagResult{}, mapNotFound(err, "feature flag not found")
	}
	if flag.Enabled && flag.RolloutPercentage > 0 && flag.RolloutPercentage < 100 && subjectID == "" {
		return CheckFeatureFlagResult{}, apperr.Validation("subject_id is required for percentage rollouts")
	}
	overrideEffect := ""
	if subjectID != "" {
		row, err := s.db.Q().GetFeatureFlagOverride(ctx, sqlc.GetFeatureFlagOverrideParams{
			FeatureFlagID: flag.ID,
			SubjectID:     subjectID,
		})
		if err == nil {
			overrideEffect = row.Effect
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return CheckFeatureFlagResult{}, err
		}
	}
	enabled, reason := featureflags.Evaluate(flag.Enabled, int(flag.RolloutPercentage), flag.Key, subjectID, overrideEffect)
	return CheckFeatureFlagResult{Enabled: enabled, Reason: reason}, nil
}

func (s *Service) featureFlagView(ctx context.Context, flag sqlc.FeatureFlag) (FeatureFlagView, error) {
	overrides, err := s.db.Q().ListFeatureFlagOverrides(ctx, flag.ID)
	if err != nil {
		return FeatureFlagView{}, err
	}
	return FeatureFlagView{Flag: flag, Overrides: overrides}, nil
}
