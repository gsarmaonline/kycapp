package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/gsarmaonline/kyc/core/organisation"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service implements Phase 2 domain operations.
type Service struct {
	db *store.Store
}

func New(db *store.Store) *Service {
	return &Service{db: db}
}

// --- Signup ---

type SignupInput struct {
	UserEmail        string
	UserName         string
	OrganisationName string
	OrganisationSlug string
	PlanKey          string
}

type SignupResult struct {
	User         sqlc.User         `json:"user"`
	Organisation sqlc.Organisation `json:"organisation"`
	Membership   sqlc.Membership   `json:"membership"`
	Subscription sqlc.Subscription `json:"subscription"`
}

func (s *Service) Signup(ctx context.Context, in SignupInput, idempotencyKey string) (SignupResult, int, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return SignupResult{}, 0, apperr.Validation("Idempotency-Key is required")
	}
	if err := validateSignup(in); err != nil {
		return SignupResult{}, 0, err
	}
	if in.PlanKey == "" {
		in.PlanKey = "trial"
	}
	if in.OrganisationSlug == "" {
		in.OrganisationSlug = organisation.Slugify(in.OrganisationName)
	}
	in.UserEmail = strings.ToLower(strings.TrimSpace(in.UserEmail))

	reqHash := hashJSON(in)

	if existing, err := s.db.Q().GetIdempotencyKey(ctx, idempotencyKey); err == nil {
		if existing.RequestHash != reqHash {
			return SignupResult{}, 0, apperr.IdempotencyConflict("Idempotency-Key reused with different request")
		}
		var out SignupResult
		if err := json.Unmarshal(existing.ResponseBody, &out); err != nil {
			return SignupResult{}, 0, err
		}
		return out, int(existing.ResponseStatus), nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return SignupResult{}, 0, err
	}

	var result SignupResult
	err := s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		plan, err := q.GetPlanByKey(ctx, in.PlanKey)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.Validation("unknown plan_key")
			}
			return err
		}

		user, err := q.GetUserByEmail(ctx, in.UserEmail)
		if errors.Is(err, pgx.ErrNoRows) {
			user, err = q.CreateUser(ctx, sqlc.CreateUserParams{
				ID:     ids.New(),
				Email:  in.UserEmail,
				Name:   strings.TrimSpace(in.UserName),
				Status: "active",
			})
			if err != nil {
				if store.IsUniqueViolation(err) {
					return apperr.Conflict("email already exists")
				}
				return err
			}
		} else if err != nil {
			return err
		}

		org, err := q.CreateOrganisation(ctx, sqlc.CreateOrganisationParams{
			ID:     ids.New(),
			Name:   strings.TrimSpace(in.OrganisationName),
			Slug:   in.OrganisationSlug,
			Status: "active",
		})
		if err != nil {
			if store.IsUniqueViolation(err) {
				return apperr.Conflict("organisation slug already exists")
			}
			return err
		}

		ownerRole, _, _, err := seedSystemRoles(ctx, q, org.ID)
		if err != nil {
			return err
		}

		membership, err := q.CreateMembership(ctx, sqlc.CreateMembershipParams{
			ID:             ids.New(),
			OrganisationID: org.ID,
			UserID:         user.ID,
			RoleID:         ownerRole.ID,
			Status:         "active",
		})
		if err != nil {
			if store.IsUniqueViolation(err) {
				return apperr.Conflict("user already a member of organisation")
			}
			return err
		}

		sub, err := q.CreateSubscription(ctx, sqlc.CreateSubscriptionParams{
			ID:               ids.New(),
			OrganisationID:   org.ID,
			PlanID:           plan.ID,
			Status:           "trialing",
			CurrentPeriodEnd: pgtype.Timestamptz{},
		})
		if err != nil {
			return err
		}

		result = SignupResult{
			User:         user,
			Organisation: org,
			Membership:   membership,
			Subscription: sub,
		}

		body, err := json.Marshal(result)
		if err != nil {
			return err
		}
		_, err = q.InsertIdempotencyKey(ctx, sqlc.InsertIdempotencyKeyParams{
			Key:            idempotencyKey,
			RequestHash:    reqHash,
			ResponseStatus: 201,
			ResponseBody:   body,
		})
		if store.IsUniqueViolation(err) {
			return err // caller will re-read
		}
		return err
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			existing, getErr := s.db.Q().GetIdempotencyKey(ctx, idempotencyKey)
			if getErr == nil && existing.RequestHash == reqHash {
				var out SignupResult
				if jsonErr := json.Unmarshal(existing.ResponseBody, &out); jsonErr == nil {
					return out, int(existing.ResponseStatus), nil
				}
			}
			if getErr == nil {
				return SignupResult{}, 0, apperr.IdempotencyConflict("Idempotency-Key reused with different request")
			}
		}
		return SignupResult{}, 0, err
	}
	return result, 201, nil
}

func validateSignup(in SignupInput) error {
	if strings.TrimSpace(in.UserEmail) == "" || !strings.Contains(in.UserEmail, "@") {
		return apperr.Validation("user.email is required")
	}
	if strings.TrimSpace(in.UserName) == "" {
		return apperr.Validation("user.name is required")
	}
	if strings.TrimSpace(in.OrganisationName) == "" {
		return apperr.Validation("organisation.name is required")
	}
	return nil
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

	// Owner + admin get all permissions.
	for _, roleID := range []string{owner.ID, admin.ID} {
		for _, pid := range permIDs {
			if err = q.AddRolePermission(ctx, sqlc.AddRolePermissionParams{RoleID: roleID, PermissionID: pid}); err != nil {
				return
			}
		}
	}

	// Member: read-only subset by key convention — attach organisation:read, members:read, roles:read, billing:read.
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

func hashJSON(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
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
