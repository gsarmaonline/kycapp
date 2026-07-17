package service

import (
	"context"
	"strings"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// RoleView is a role plus its permission keys.
type RoleView struct {
	Role           sqlc.Role
	PermissionKeys []string
}

type CreateRoleInput struct {
	Key            string
	Name           string
	Description    string
	PermissionKeys []string
}

type UpdateRoleInput struct {
	Name           *string
	Description    *string
	PermissionKeys *[]string
}

type AuthzCheckInput struct {
	OrganisationID string
	UserID         string
	Permission     string
	Resource       string
	Action         string
}

func (s *Service) ListPermissions(ctx context.Context, category, resource string) ([]sqlc.Permission, error) {
	return s.db.Q().ListPermissionsFiltered(ctx, sqlc.ListPermissionsFilteredParams{
		Category: textArg(category),
		Resource: textArg(resource),
	})
}

func (s *Service) GetPermission(ctx context.Context, key string) (sqlc.Permission, error) {
	p, err := s.db.Q().GetPermissionByKey(ctx, key)
	return p, mapNotFound(err, "permission not found")
}

func (s *Service) GetRole(ctx context.Context, id string) (RoleView, error) {
	role, err := s.db.Q().GetRole(ctx, id)
	if err != nil {
		return RoleView{}, mapNotFound(err, "role not found")
	}
	keys, err := s.db.Q().ListPermissionKeysByRole(ctx, role.ID)
	if err != nil {
		return RoleView{}, err
	}
	return RoleView{Role: role, PermissionKeys: keys}, nil
}

func (s *Service) ListRoles(ctx context.Context, orgID string) ([]RoleView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return nil, err
	}
	roles, err := s.db.Q().ListRolesByOrganisation(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]RoleView, 0, len(roles))
	for _, role := range roles {
		keys, err := s.db.Q().ListPermissionKeysByRole(ctx, role.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, RoleView{Role: role, PermissionKeys: keys})
	}
	return out, nil
}

func (s *Service) CreateRole(ctx context.Context, orgID string, in CreateRoleInput) (RoleView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return RoleView{}, err
	}
	key := strings.TrimSpace(in.Key)
	name := strings.TrimSpace(in.Name)
	if key == "" {
		return RoleView{}, apperr.Validation("key is required")
	}
	if name == "" {
		return RoleView{}, apperr.Validation("name is required")
	}
	if len(in.PermissionKeys) == 0 {
		return RoleView{}, apperr.Validation("permission_keys is required")
	}

	permRows, err := s.db.Q().ListPermissionIDsByKeys(ctx, uniqueStrings(in.PermissionKeys))
	if err != nil {
		return RoleView{}, err
	}
	keys := uniqueStrings(in.PermissionKeys)
	if len(permRows) != len(keys) {
		return RoleView{}, apperr.Validation("one or more permission_keys are unknown")
	}

	var role sqlc.Role
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		var err error
		role, err = q.CreateRole(ctx, sqlc.CreateRoleParams{
			ID:             ids.New(),
			OrganisationID: orgID,
			Key:            key,
			Name:           name,
			Description:    strings.TrimSpace(in.Description),
			IsSystem:       false,
		})
		if err != nil {
			if store.IsUniqueViolation(err) {
				return apperr.Conflict("role key already exists")
			}
			return err
		}
		for _, row := range permRows {
			if err := q.AddRolePermission(ctx, sqlc.AddRolePermissionParams{
				RoleID:       role.ID,
				PermissionID: row.ID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return RoleView{}, err
	}
	return RoleView{Role: role, PermissionKeys: keys}, nil
}

func (s *Service) UpdateRole(ctx context.Context, roleID string, in UpdateRoleInput) (RoleView, error) {
	role, err := s.db.Q().GetRole(ctx, roleID)
	if err != nil {
		return RoleView{}, mapNotFound(err, "role not found")
	}

	if role.Key == "owner" && in.PermissionKeys != nil {
		return RoleView{}, apperr.Validation("owner role permissions cannot be changed")
	}
	if in.PermissionKeys != nil && len(*in.PermissionKeys) == 0 {
		return RoleView{}, apperr.Validation("permission_keys cannot be empty")
	}

	var permRows []sqlc.ListPermissionIDsByKeysRow
	var keys []string
	if in.PermissionKeys != nil {
		keys = uniqueStrings(*in.PermissionKeys)
		permRows, err = s.db.Q().ListPermissionIDsByKeys(ctx, keys)
		if err != nil {
			return RoleView{}, err
		}
		if len(permRows) != len(keys) {
			return RoleView{}, apperr.Validation("one or more permission_keys are unknown")
		}
	}

	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		params := sqlc.UpdateRoleParams{ID: roleID}
		if in.Name != nil {
			params.Name = pgtype.Text{String: strings.TrimSpace(*in.Name), Valid: true}
		}
		if in.Description != nil {
			params.Description = pgtype.Text{String: strings.TrimSpace(*in.Description), Valid: true}
		}
		updated, err := q.UpdateRole(ctx, params)
		if err != nil {
			return mapNotFound(err, "role not found")
		}
		role = updated

		if in.PermissionKeys != nil {
			if err := q.DeleteRolePermissions(ctx, roleID); err != nil {
				return err
			}
			for _, row := range permRows {
				if err := q.AddRolePermission(ctx, sqlc.AddRolePermissionParams{
					RoleID:       roleID,
					PermissionID: row.ID,
				}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return RoleView{}, err
	}

	keys, err = s.db.Q().ListPermissionKeysByRole(ctx, role.ID)
	if err != nil {
		return RoleView{}, err
	}
	return RoleView{Role: role, PermissionKeys: keys}, nil
}

func (s *Service) CheckAuthz(ctx context.Context, in AuthzCheckInput) (bool, error) {
	if strings.TrimSpace(in.OrganisationID) == "" || strings.TrimSpace(in.UserID) == "" {
		return false, apperr.Validation("organisation_id and user_id are required")
	}

	key := strings.TrimSpace(in.Permission)
	if key == "" {
		resource := strings.TrimSpace(in.Resource)
		action := strings.TrimSpace(in.Action)
		if resource == "" || action == "" {
			return false, apperr.Validation("permission or resource+action is required")
		}
		p, err := s.db.Q().GetPermissionByResourceAction(ctx, sqlc.GetPermissionByResourceActionParams{
			Resource: resource,
			Action:   action,
		})
		if err != nil {
			return false, mapNotFound(err, "permission not found")
		}
		key = p.Key
	} else {
		if _, err := s.GetPermission(ctx, key); err != nil {
			return false, err
		}
	}

	allowed, err := s.db.Q().CheckUserPermission(ctx, sqlc.CheckUserPermissionParams{
		OrganisationID: in.OrganisationID,
		UserID:         in.UserID,
		PermissionKey:  key,
	})
	return allowed, err
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
