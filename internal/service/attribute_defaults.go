package service

import (
	"context"
	"encoding/json"

	"github.com/gsarmaonline/kyc/core/userattributes"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
)

// EnsureDefaultAttributeDefinitions seeds system attribute definitions for an org (idempotent).
func (s *Service) EnsureDefaultAttributeDefinitions(ctx context.Context, orgID string) error {
	return ensureDefaultAttributeDefinitions(ctx, s.db.Q(), orgID)
}

func ensureDefaultAttributeDefinitions(ctx context.Context, q *sqlc.Queries, orgID string) error {
	for _, spec := range userattributes.Defaults() {
		_, err := q.GetAttributeDefinitionByOrgKey(ctx, sqlc.GetAttributeDefinitionByOrgKeyParams{
			OrganisationID: orgID,
			Key:            spec.Key,
		})
		if err == nil {
			continue
		}
		if err != pgx.ErrNoRows {
			return err
		}
		enumRaw := json.RawMessage("[]")
		if len(spec.EnumValues) > 0 {
			raw, err := json.Marshal(spec.EnumValues)
			if err != nil {
				return err
			}
			enumRaw = raw
		}
		_, err = q.CreateAttributeDefinition(ctx, sqlc.CreateAttributeDefinitionParams{
			ID:             ids.New(),
			OrganisationID: orgID,
			Key:            spec.Key,
			Label:          spec.Label,
			Description:    spec.Description,
			ValueType:      spec.ValueType,
			Section:        spec.Section,
			SortOrder:      spec.SortOrder,
			Required:       spec.Required,
			EnumValues:     enumRaw,
			IsPii:          spec.IsPII,
			Status:         "active",
			IsSystem:       true,
		})
		if err != nil && !store.IsUniqueViolation(err) {
			return err
		}
	}
	return nil
}
