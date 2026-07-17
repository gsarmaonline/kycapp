package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

var attrKeyRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

var allowedValueTypes = map[string]bool{
	"string": true, "number": true, "boolean": true, "date": true, "dropdown": true,
}

type CreateAttributeDefinitionInput struct {
	Key         string
	Label       string
	Description string
	ValueType   string
	Section     string
	SortOrder   int32
	Required    bool
	EnumValues  []string
	IsPII       bool
}

type UpdateAttributeDefinitionInput struct {
	Label       *string
	Description *string
	ValueType   *string
	Section     *string
	SortOrder   *int32
	Required    *bool
	EnumValues  *[]string
	IsPII       *bool
	Status      *string
}

type CreateAppUserInput struct {
	ExternalID  string
	Email       string
	DisplayName string
	Status      string
	Attributes  map[string]any
}

type UpdateAppUserInput struct {
	ExternalID  *string
	Email       *string
	DisplayName *string
	Status      *string
	Attributes  map[string]any // nil = leave unchanged; empty map clears
}

func (s *Service) CreateAttributeDefinition(ctx context.Context, orgID string, in CreateAttributeDefinitionInput) (sqlc.AttributeDefinition, error) {
	key := strings.TrimSpace(in.Key)
	label := strings.TrimSpace(in.Label)
	valueType := strings.TrimSpace(in.ValueType)
	section := strings.TrimSpace(in.Section)
	if key == "" || !attrKeyRE.MatchString(key) {
		return sqlc.AttributeDefinition{}, apperr.Validation("key must be lowercase snake_case (a-z, 0-9, _)")
	}
	if label == "" {
		return sqlc.AttributeDefinition{}, apperr.Validation("label is required")
	}
	if !allowedValueTypes[valueType] {
		return sqlc.AttributeDefinition{}, apperr.Validation("value_type must be string, number, boolean, date, or dropdown")
	}
	if section == "" {
		section = "general"
	}
	enumRaw, err := encodeOptionValues(valueType, in.EnumValues)
	if err != nil {
		return sqlc.AttributeDefinition{}, err
	}

	row, err := s.db.Q().CreateAttributeDefinition(ctx, sqlc.CreateAttributeDefinitionParams{
		ID:             ids.New(),
		OrganisationID: orgID,
		Key:            key,
		Label:          label,
		Description:    strings.TrimSpace(in.Description),
		ValueType:      valueType,
		Section:        section,
		SortOrder:      in.SortOrder,
		Required:       in.Required,
		EnumValues:     enumRaw,
		IsPii:          in.IsPII,
		Status:         "active",
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			return sqlc.AttributeDefinition{}, apperr.Conflict("attribute key already exists in organisation")
		}
		return sqlc.AttributeDefinition{}, err
	}
	return row, nil
}

func (s *Service) GetAttributeDefinition(ctx context.Context, id string) (sqlc.AttributeDefinition, error) {
	row, err := s.db.Q().GetAttributeDefinition(ctx, id)
	return row, mapNotFound(err, "attribute definition not found")
}

func (s *Service) ListAttributeDefinitions(ctx context.Context, orgID, status string) ([]sqlc.AttributeDefinition, error) {
	return s.db.Q().ListAttributeDefinitions(ctx, sqlc.ListAttributeDefinitionsParams{
		OrganisationID: orgID,
		Status:         textArg(status),
	})
}

func (s *Service) UpdateAttributeDefinition(ctx context.Context, id string, in UpdateAttributeDefinitionInput) (sqlc.AttributeDefinition, error) {
	existing, err := s.GetAttributeDefinition(ctx, id)
	if err != nil {
		return sqlc.AttributeDefinition{}, err
	}

	params := sqlc.UpdateAttributeDefinitionParams{ID: id}
	if in.Label != nil {
		label := strings.TrimSpace(*in.Label)
		if label == "" {
			return sqlc.AttributeDefinition{}, apperr.Validation("label cannot be empty")
		}
		params.Label = pgtype.Text{String: label, Valid: true}
	}
	if in.Description != nil {
		params.Description = pgtype.Text{String: strings.TrimSpace(*in.Description), Valid: true}
	}
	valueType := existing.ValueType
	if in.ValueType != nil {
		valueType = strings.TrimSpace(*in.ValueType)
		if !allowedValueTypes[valueType] {
			return sqlc.AttributeDefinition{}, apperr.Validation("value_type must be string, number, boolean, date, or dropdown")
		}
		params.ValueType = pgtype.Text{String: valueType, Valid: true}
	}
	if in.Section != nil {
		section := strings.TrimSpace(*in.Section)
		if section == "" {
			section = "general"
		}
		params.Section = pgtype.Text{String: section, Valid: true}
	}
	if in.SortOrder != nil {
		params.SortOrder = pgtype.Int4{Int32: *in.SortOrder, Valid: true}
	}
	if in.Required != nil {
		params.Required = pgtype.Bool{Bool: *in.Required, Valid: true}
	}
	if in.IsPII != nil {
		params.IsPii = pgtype.Bool{Bool: *in.IsPII, Valid: true}
	}
	if in.Status != nil {
		status := strings.TrimSpace(*in.Status)
		if status != "active" && status != "archived" {
			return sqlc.AttributeDefinition{}, apperr.Validation("status must be active or archived")
		}
		params.Status = pgtype.Text{String: status, Valid: true}
	}
	if in.EnumValues != nil {
		raw, err := encodeOptionValues(valueType, *in.EnumValues)
		if err != nil {
			return sqlc.AttributeDefinition{}, err
		}
		params.EnumValues = raw
	}

	row, err := s.db.Q().UpdateAttributeDefinition(ctx, params)
	return row, err
}

func (s *Service) CreateAppUser(ctx context.Context, orgID string, in CreateAppUserInput) (sqlc.AppUser, error) {
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "disabled" && status != "archived" {
		return sqlc.AppUser{}, apperr.Validation("status must be active, disabled, or archived")
	}
	attrs := in.Attributes
	if attrs == nil {
		attrs = map[string]any{}
	}
	if err := s.validateAttributes(ctx, orgID, attrs, true); err != nil {
		return sqlc.AppUser{}, err
	}
	raw, err := json.Marshal(attrs)
	if err != nil {
		return sqlc.AppUser{}, apperr.Validation("invalid attributes")
	}

	row, err := s.db.Q().CreateAppUser(ctx, sqlc.CreateAppUserParams{
		ID:             ids.New(),
		OrganisationID: orgID,
		ExternalID:     textArg(in.ExternalID),
		Email:          textArg(strings.ToLower(strings.TrimSpace(in.Email))),
		DisplayName:    strings.TrimSpace(in.DisplayName),
		Status:         status,
		Attributes:     raw,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			return sqlc.AppUser{}, apperr.Conflict("app user email or external_id already exists in organisation")
		}
		return sqlc.AppUser{}, err
	}
	return row, nil
}

func (s *Service) GetAppUser(ctx context.Context, id string) (sqlc.AppUser, error) {
	row, err := s.db.Q().GetAppUser(ctx, id)
	return row, mapNotFound(err, "app user not found")
}

func (s *Service) ListAppUsers(ctx context.Context, orgID, status string) ([]sqlc.AppUser, error) {
	return s.db.Q().ListAppUsers(ctx, sqlc.ListAppUsersParams{
		OrganisationID: orgID,
		Status:         textArg(status),
	})
}

func (s *Service) UpdateAppUser(ctx context.Context, id string, in UpdateAppUserInput) (sqlc.AppUser, error) {
	existing, err := s.GetAppUser(ctx, id)
	if err != nil {
		return sqlc.AppUser{}, err
	}

	params := sqlc.UpdateAppUserParams{ID: id}
	if in.ExternalID != nil {
		params.ExternalID = textArg(*in.ExternalID)
	}
	if in.Email != nil {
		params.Email = textArg(strings.ToLower(strings.TrimSpace(*in.Email)))
	}
	if in.DisplayName != nil {
		params.DisplayName = pgtype.Text{String: strings.TrimSpace(*in.DisplayName), Valid: true}
	}
	if in.Status != nil {
		status := strings.TrimSpace(*in.Status)
		if status != "active" && status != "disabled" && status != "archived" {
			return sqlc.AppUser{}, apperr.Validation("status must be active, disabled, or archived")
		}
		params.Status = pgtype.Text{String: status, Valid: true}
	}
	if in.Attributes != nil {
		if err := s.validateAttributes(ctx, existing.OrganisationID, in.Attributes, true); err != nil {
			return sqlc.AppUser{}, err
		}
		raw, err := json.Marshal(in.Attributes)
		if err != nil {
			return sqlc.AppUser{}, apperr.Validation("invalid attributes")
		}
		params.Attributes = raw
	}

	row, err := s.db.Q().UpdateAppUser(ctx, params)
	if err != nil {
		if store.IsUniqueViolation(err) {
			return sqlc.AppUser{}, apperr.Conflict("app user email or external_id already exists in organisation")
		}
		return sqlc.AppUser{}, err
	}
	return row, nil
}

func (s *Service) validateAttributes(ctx context.Context, orgID string, attrs map[string]any, enforceRequired bool) error {
	defs, err := s.ListAttributeDefinitions(ctx, orgID, "active")
	if err != nil {
		return err
	}
	byKey := make(map[string]sqlc.AttributeDefinition, len(defs))
	for _, d := range defs {
		byKey[d.Key] = d
	}
	for key, val := range attrs {
		def, ok := byKey[key]
		if !ok {
			return apperr.Validation(fmt.Sprintf("unknown attribute %q", key))
		}
		if err := validateAttrValue(def, val); err != nil {
			return err
		}
	}
	if enforceRequired {
		for _, d := range defs {
			if !d.Required {
				continue
			}
			val, ok := attrs[d.Key]
			if !ok || val == nil || val == "" {
				return apperr.Validation(fmt.Sprintf("attribute %q is required", d.Key))
			}
		}
	}
	return nil
}

func validateAttrValue(def sqlc.AttributeDefinition, val any) error {
	if val == nil {
		return nil
	}
	switch def.ValueType {
	case "string":
		if _, ok := val.(string); !ok {
			return apperr.Validation(fmt.Sprintf("attribute %q must be a string", def.Key))
		}
	case "number":
		switch val.(type) {
		case float64, json.Number, int, int32, int64:
			// ok
		default:
			return apperr.Validation(fmt.Sprintf("attribute %q must be a number", def.Key))
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return apperr.Validation(fmt.Sprintf("attribute %q must be a boolean", def.Key))
		}
	case "date":
		s, ok := val.(string)
		if !ok {
			return apperr.Validation(fmt.Sprintf("attribute %q must be an ISO date string", def.Key))
		}
		if _, err := time.Parse("2006-01-02", s); err != nil {
			if _, err2 := time.Parse(time.RFC3339, s); err2 != nil {
				return apperr.Validation(fmt.Sprintf("attribute %q must be YYYY-MM-DD or RFC3339", def.Key))
			}
		}
	case "dropdown":
		s, ok := val.(string)
		if !ok {
			return apperr.Validation(fmt.Sprintf("attribute %q must be a string option value", def.Key))
		}
		var allowed []string
		_ = json.Unmarshal(def.EnumValues, &allowed)
		for _, a := range allowed {
			if a == s {
				return nil
			}
		}
		return apperr.Validation(fmt.Sprintf("attribute %q value is not in allowed options", def.Key))
	}
	return nil
}

func encodeOptionValues(valueType string, values []string) (json.RawMessage, error) {
	if valueType != "dropdown" {
		return json.RawMessage("[]"), nil
	}
	if len(values) == 0 {
		return nil, apperr.Validation("enum_values is required for dropdown value_type")
	}
	clean := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		clean = append(clean, v)
	}
	if len(clean) == 0 {
		return nil, apperr.Validation("enum_values is required for dropdown value_type")
	}
	raw, err := json.Marshal(clean)
	if err != nil {
		return nil, apperr.Validation("invalid enum_values")
	}
	return raw, nil
}
