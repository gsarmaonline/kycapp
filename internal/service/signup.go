package service

import (
	"context"
	"errors"
	"strings"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/jobs"
	"github.com/gsarmaonline/kyc/internal/mailer"
	"github.com/gsarmaonline/kyc/internal/observability"
	"github.com/gsarmaonline/kyc/internal/payments"
	"github.com/gsarmaonline/kyc/internal/store"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service implements domain operations.
type Service struct {
	db                 *store.Store
	obs                observability.Store
	uploadDir          string
	publicBaseURL      string
	enqueue            Enqueuer
	payments           payments.Processor
	mailer             mailer.Mailer
	checkoutSuccessURL string
	checkoutCancelURL  string
}

// Enqueuer inserts background jobs (River). Optional — nil skips enqueue.
type Enqueuer interface {
	EnqueueAutomationEvent(ctx context.Context, orgID, trigger string, payload any) error
	EnqueueAutomationResume(ctx context.Context, in jobs.EnqueueResumeInput) error
}

func New(db *store.Store) *Service {
	return &Service{
		db:            db,
		obs:           observability.NewNoop(),
		uploadDir:     "data/uploads",
		publicBaseURL: "http://localhost:8080",
		mailer:        mailer.NewNoop(),
	}
}

// SetEnqueuer attaches a job enqueue implementation (typically River).
func (s *Service) SetEnqueuer(e Enqueuer) {
	s.enqueue = e
}

// SetMailer configures transactional email delivery (default noop).
func (s *Service) SetMailer(m mailer.Mailer) {
	if m == nil {
		s.mailer = mailer.NewNoop()
		return
	}
	s.mailer = m
}

// ConfigureAssets sets local upload directory and public base URL for logos.
func (s *Service) ConfigureAssets(uploadDir, publicBaseURL string) {
	if strings.TrimSpace(uploadDir) != "" {
		s.uploadDir = uploadDir
	}
	if u := strings.TrimSpace(publicBaseURL); u != "" {
		s.publicBaseURL = strings.TrimRight(u, "/")
	}
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
		"organisation:read":     true,
		"members:read":          true,
		"roles:read":            true,
		"billing:read":          true,
		"attributes:read":       true,
		"app_users:read":        true,
		"email_templates:read":  true,
		"automations:read":      true,
		"product_features:read": true,
		"activity:read":         true,
		"usage:read":            true,
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
