package service

import (
	"context"
	"errors"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
)

// Bootstrap turns an empty database into one with a first staff member.
//
// The chain of trust, in order:
//
//  1. Break-glass (API_TOKENS) resolves from the environment before any query,
//     so it works when the database is empty, mis-seeded, or freshly restored.
//     It is the root of trust precisely because it is not data.
//  2. This path mints the first membership of the platform organisation, using
//     the role the migration nominated. That makes staff access ordinary data
//     from the very first grant.
//  3. Everything afterwards is normal delegation.
//
// Gated on a marker rather than on "are there zero staff", because counting
// would let revoking every staff membership reopen the door.

// isBootstrapped reports whether the first staff member has been minted.
// A missing system_state row is treated as bootstrapped: a database that cannot
// answer the question must not hand out platform access.
func (s *Service) isBootstrapped(ctx context.Context) bool {
	state, err := s.db.Q().GetSystemState(ctx)
	if err != nil {
		return true
	}
	return state.BootstrappedAt.Valid
}

// PlatformOrganisationID returns the organisation representing KYC itself, or
// empty when the platform organisation has not been seeded.
func (s *Service) PlatformOrganisationID(ctx context.Context) string {
	state, err := s.db.Q().GetSystemState(ctx)
	if err != nil || !state.PlatformOrganisationID.Valid {
		return ""
	}
	return state.PlatformOrganisationID.String
}

// BootstrapFirstStaff makes userID a member of the platform organisation with
// the nominated bootstrap role, exactly once.
//
// It is a no-op after the marker is set, and the marker is claimed in the same
// transaction as the membership, so concurrent logins cannot both succeed.
func (s *Service) BootstrapFirstStaff(ctx context.Context, userID string) error {
	return s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		// Claiming the marker first makes this the only caller that proceeds.
		state, err := q.MarkBootstrapped(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // already bootstrapped, or no system_state row
		}
		if err != nil {
			return err
		}
		if !state.PlatformOrganisationID.Valid || !state.BootstrapRoleID.Valid {
			return apperr.Validation("platform organisation is not seeded")
		}
		_, err = q.CreateMembership(ctx, sqlc.CreateMembershipParams{
			ID:             ids.New(),
			OrganisationID: state.PlatformOrganisationID.String,
			UserID:         userID,
			RoleID:         state.BootstrapRoleID.String,
			Status:         "active",
		})
		return err
	})
}
