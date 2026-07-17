package service

import (
	"context"
	"errors"
	"strings"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service implements domain operations.
type Service struct {
	db *store.Store
}

func New(db *store.Store) *Service {
	return &Service{db: db}
}

func seedSystemRoles(ctx context.Context, q *sqlc.Queries, orgID string) (owner, admin, member sqlc.Role, err error) {
	permIDs, err := q.ListPermissionIDs(ctx)
	if err != nil {
		return
	}
	if len(permIDs) == 0 {
		err = apperr.Validation("permission catalog is empty")
		return
	}

	owner, err = q.CreateRole(ctx, sqlc.CreateRoleParams{
		ID: ids.New(), OrganisationID: orgID, Key: "owner", Name: "Owner",
		Description: "Full access", IsSystem: true,
	})
	if err != nil {
		return
	}
	admin, err = q.CreateRole(ctx, sqlc.CreateRoleParams{
		ID: ids.New(), OrganisationID: orgID, Key: "admin", Name: "Admin",
		Description: "Administrative access", IsSystem: true,
	})
	if err != nil {
		return
	}
	member, err = q.CreateRole(ctx, sqlc.CreateRoleParams{
		ID: ids.New(), OrganisationID: orgID, Key: "member", Name: "Member",
		Description: "Standard member", IsSystem: true,
	})
	if err != nil {
		return
	}

	for _, roleID := range []string{owner.ID, admin.ID} {
		for _, pid := range permIDs {
			if err = q.AddRolePermission(ctx, sqlc.AddRolePermissionParams{RoleID: roleID, PermissionID: pid}); err != nil {
				return
			}
		}
	}

	perms, err := q.ListPermissions(ctx)
	if err != nil {
		return
	}
	memberKeys := map[string]bool{
		"organisation:read": true,
		"members:read":      true,
		"roles:read":        true,
		"billing:read":      true,
	}
	for _, p := range perms {
		if memberKeys[p.Key] {
			if err = q.AddRolePermission(ctx, sqlc.AddRolePermissionParams{RoleID: member.ID, PermissionID: p.ID}); err != nil {
				return
			}
		}
	}
	return
}

func textArg(s string) pgtype.Text {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func mapNotFound(err error, msg string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound(msg)
	}
	return err
}
