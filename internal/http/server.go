package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/authn"
	"github.com/gsarmaonline/kyc/internal/service"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

// DBPinger is satisfied by the store (and easy to fake in tests).
type DBPinger interface {
	Ping(ctx context.Context) error
}

// Server serves HTTP endpoints for the KYC API.
type Server struct {
	db                  DBPinger
	svc                 *service.Service
	mux                 *http.ServeMux
	now                 func() time.Time
	corsOrigin          string
	apiTokens           []string
	platformAdminEmails []string
	checkRatePerMinute  int
	authRatePerMinute   int
	google              *service.GoogleOAuth
	oauthStateSecret    string
	appOrigin           string
	authDevLogin        bool
}

// Options configures the HTTP server.
type Options struct {
	Service              *service.Service
	CORSOrigin           string
	APITokens            []string
	PlatformAdminEmails  []string
	CheckRateLimitPerMin int
	AuthRateLimitPerMin  int
	GoogleClientID       string
	GoogleClientSecret   string
	OAuthRedirectURL     string
	OAuthStateSecret     string
	AppOrigin            string
	AuthDevLogin         bool
}

// New constructs an HTTP server.
func New(db DBPinger, opts Options) *Server {
	s := &Server{
		db:                  db,
		svc:                 opts.Service,
		mux:                 http.NewServeMux(),
		now:                 time.Now,
		corsOrigin:          opts.CORSOrigin,
		apiTokens:           opts.APITokens,
		platformAdminEmails: opts.PlatformAdminEmails,
		checkRatePerMinute:  opts.CheckRateLimitPerMin,
		authRatePerMinute:   opts.AuthRateLimitPerMin,
		google:              service.NewGoogleOAuth(opts.GoogleClientID, opts.GoogleClientSecret, opts.OAuthRedirectURL),
		oauthStateSecret:    opts.OAuthStateSecret,
		appOrigin:           opts.AppOrigin,
		authDevLogin:        opts.AuthDevLogin,
	}
	if s.appOrigin == "" {
		s.appOrigin = "http://localhost:8080"
	}
	if s.oauthStateSecret == "" {
		s.oauthStateSecret = "dev-insecure-oauth-state"
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)

	if s.svc == nil {
		return
	}

	s.mux.HandleFunc("GET /v1/auth/providers", s.handleAuthProviders)
	s.mux.HandleFunc("GET /v1/auth/google", s.handleGoogleStart)
	s.mux.HandleFunc("GET /v1/auth/google/callback", s.handleGoogleCallback)
	s.mux.HandleFunc("POST /v1/auth/dev-login", s.handleDevLogin)
	s.mux.HandleFunc("POST /v1/auth/logout", s.handleLogout)
	s.mux.HandleFunc("GET /v1/me", s.handleMe)

	s.mux.HandleFunc("POST /v1/organisations", s.handleCreateOrganisation)
	s.mux.HandleFunc("GET /v1/organisations", s.handleListOrganisations)
	s.mux.HandleFunc("GET /v1/organisations/{id}", s.handleGetOrganisation)
	s.mux.HandleFunc("PATCH /v1/organisations/{id}", s.handlePatchOrganisation)
	s.mux.HandleFunc("POST /v1/organisations/{id}/archive", s.handleArchiveOrganisation)
	s.mux.HandleFunc("GET /v1/organisations/{id}/roles", s.handleListRoles)
	s.mux.HandleFunc("POST /v1/organisations/{id}/roles", s.handleCreateRole)
	s.mux.HandleFunc("POST /v1/organisations/{id}/memberships", s.handleCreateMembership)
	s.mux.HandleFunc("GET /v1/organisations/{id}/memberships", s.handleListMemberships)

	s.mux.HandleFunc("POST /v1/users", s.handleCreateUser)
	s.mux.HandleFunc("GET /v1/users", s.handleListUsers)
	s.mux.HandleFunc("GET /v1/users/{id}", s.handleGetUser)
	s.mux.HandleFunc("PATCH /v1/users/{id}", s.handlePatchUser)
	s.mux.HandleFunc("GET /v1/users/{id}/memberships", s.handleListUserMemberships)

	s.mux.HandleFunc("POST /v1/memberships/{id}/accept", s.handleAcceptMembership)
	s.mux.HandleFunc("PATCH /v1/memberships/{id}", s.handlePatchMembership)
	s.mux.HandleFunc("DELETE /v1/memberships/{id}", s.handleRevokeMembership)

	s.mux.HandleFunc("GET /v1/permissions", s.handleListPermissions)
	s.mux.HandleFunc("GET /v1/permissions/{key}", s.handleGetPermission)
	s.mux.HandleFunc("PATCH /v1/roles/{id}", s.handlePatchRole)
	s.mux.HandleFunc("POST /v1/authz/check", s.handleAuthzCheck)

	s.mux.HandleFunc("POST /v1/plans", s.handleCreatePlan)
	s.mux.HandleFunc("GET /v1/plans", s.handleListPlans)
	s.mux.HandleFunc("GET /v1/plans/{id}", s.handleGetPlan)
	s.mux.HandleFunc("PUT /v1/plans/{id}/entitlements", s.handleSetPlanEntitlements)
	s.mux.HandleFunc("POST /v1/entitlements", s.handleCreateEntitlement)
	s.mux.HandleFunc("GET /v1/entitlements", s.handleListEntitlementsCatalog)
	s.mux.HandleFunc("PUT /v1/organisations/{id}/subscription", s.handleUpsertSubscription)
	s.mux.HandleFunc("GET /v1/organisations/{id}/subscription", s.handleGetSubscription)
	s.mux.HandleFunc("PUT /v1/organisations/{id}/entitlements", s.handleSetOrgEntitlements)
	s.mux.HandleFunc("GET /v1/organisations/{id}/entitlements", s.handleGetOrgEntitlements)
	s.mux.HandleFunc("POST /v1/entitlements/check", s.handleEntitlementsCheck)

	s.mux.HandleFunc("POST /v1/api-keys", s.handleCreateAPIKey)
	s.mux.HandleFunc("GET /v1/api-keys", s.handleListAPIKeys)
	s.mux.HandleFunc("DELETE /v1/api-keys/{id}", s.handleRevokeAPIKey)
	s.mux.HandleFunc("GET /v1/audit-events", s.handleListAuditEvents)

	s.mux.HandleFunc("POST /v1/organisations/{id}/attribute-definitions", s.handleCreateAttributeDefinition)
	s.mux.HandleFunc("GET /v1/organisations/{id}/attribute-definitions", s.handleListAttributeDefinitions)
	s.mux.HandleFunc("PATCH /v1/attribute-definitions/{id}", s.handlePatchAttributeDefinition)
	s.mux.HandleFunc("POST /v1/organisations/{id}/app-users", s.handleCreateAppUser)
	s.mux.HandleFunc("GET /v1/organisations/{id}/app-users", s.handleListAppUsers)
	s.mux.HandleFunc("GET /v1/app-users/{id}", s.handleGetAppUser)
	s.mux.HandleFunc("PATCH /v1/app-users/{id}", s.handlePatchAppUser)

	s.mux.HandleFunc("POST /v1/organisations/{id}/email-templates", s.handleCreateEmailTemplate)
	s.mux.HandleFunc("GET /v1/organisations/{id}/email-templates", s.handleListEmailTemplates)
	s.mux.HandleFunc("GET /v1/email-templates/{id}", s.handleGetEmailTemplate)
	s.mux.HandleFunc("PATCH /v1/email-templates/{id}", s.handlePatchEmailTemplate)
}

// Handler returns the root handler with auth, audit, rate limit, and optional CORS.
func (s *Server) Handler() http.Handler {
	var h http.Handler = s.mux
	if s.svc != nil {
		h = auditMiddleware(s.svc.RecordAudit, h)
		h = rateLimitMiddleware(newRateLimiter(s.checkRatePerMinute), h)
		h = authMiddleware(func(ctx context.Context, token string) (authn.Principal, bool) {
			return s.svc.AuthenticateBearer(ctx, token, s.apiTokens, s.platformAdminEmails)
		}, h)
		h = authRateLimitMiddleware(newRateLimiter(s.authRatePerMinute), h)
	}
	if s.corsOrigin != "" {
		h = corsMiddleware(s.corsOrigin, h)
	}
	return h
}

func corsMiddleware(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type healthResponse struct {
	Status string `json:"status"`
	Time   string `json:"time"`
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status: "ok",
		Time:   s.now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not_ready",
			"error":  "database not configured",
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not_ready",
			"error":  "database unreachable",
		})
		return
	}
	writeJSON(w, http.StatusOK, healthResponse{
		Status: "ok",
		Time:   s.now().UTC().Format(time.RFC3339),
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		status := http.StatusBadRequest
		switch {
		case errors.Is(ae.Err, apperr.ErrNotFound):
			status = http.StatusNotFound
		case errors.Is(ae.Err, apperr.ErrConflict), errors.Is(ae.Err, apperr.ErrIdempotencyConflict):
			status = http.StatusConflict
		case errors.Is(ae.Err, apperr.ErrValidation):
			status = http.StatusBadRequest
		case errors.Is(ae.Err, apperr.ErrUnauthorized):
			status = http.StatusUnauthorized
		case errors.Is(ae.Err, apperr.ErrForbidden):
			status = http.StatusForbidden
		case errors.Is(ae.Err, apperr.ErrRateLimited):
			status = http.StatusTooManyRequests
		}
		writeJSON(w, status, map[string]any{
			"error": map[string]string{"code": ae.Code, "message": ae.Message},
		})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{
		"error": map[string]string{"code": "internal_error", "message": "internal server error"},
	})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func queryLimit(r *http.Request) int32 {
	v := r.URL.Query().Get("limit")
	if v == "" {
		return 50
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 50
	}
	return int32(n)
}

func orgJSON(o sqlc.Organisation) map[string]any {
	return map[string]any{
		"id":         o.ID,
		"name":       o.Name,
		"slug":       o.Slug,
		"status":     o.Status,
		"created_at": o.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at": o.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func userJSON(u sqlc.User) map[string]any {
	return map[string]any{
		"id":             u.ID,
		"email":          u.Email,
		"name":           u.Name,
		"status":         u.Status,
		"platform_admin": u.PlatformAdmin,
		"created_at":     u.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":     u.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func membershipJSON(m sqlc.Membership) map[string]any {
	return map[string]any{
		"id":              m.ID,
		"organisation_id": m.OrganisationID,
		"user_id":         m.UserID,
		"role_id":         m.RoleID,
		"status":          m.Status,
		"created_at":      m.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func subscriptionJSON(sub sqlc.Subscription) map[string]any {
	out := map[string]any{
		"id":              sub.ID,
		"organisation_id": sub.OrganisationID,
		"plan_id":         sub.PlanID,
		"status":          sub.Status,
	}
	if sub.CurrentPeriodEnd.Valid {
		out["current_period_end"] = sub.CurrentPeriodEnd.Time.UTC().Format(time.RFC3339Nano)
	} else {
		out["current_period_end"] = nil
	}
	return out
}

func permissionJSON(p sqlc.Permission) map[string]any {
	return map[string]any{
		"id":          p.ID,
		"key":         p.Key,
		"resource":    p.Resource,
		"action":      p.Action,
		"category":    p.Category,
		"description": p.Description,
		"is_system":   p.IsSystem,
	}
}

func roleJSON(role service.RoleView) map[string]any {
	return map[string]any{
		"id":              role.Role.ID,
		"organisation_id": role.Role.OrganisationID,
		"key":             role.Role.Key,
		"name":            role.Role.Name,
		"description":     role.Role.Description,
		"is_system":       role.Role.IsSystem,
		"permission_keys": role.PermissionKeys,
	}
}

func planJSON(p service.PlanView) map[string]any {
	return map[string]any{
		"id":               p.Plan.ID,
		"key":              p.Plan.Key,
		"name":             p.Plan.Name,
		"status":           p.Plan.Status,
		"entitlement_keys": p.EntitlementKeys,
	}
}

func entitlementJSON(e sqlc.Entitlement) map[string]any {
	return map[string]any{
		"id":          e.ID,
		"key":         e.Key,
		"description": e.Description,
	}
}

func authResultJSON(a service.AuthResult) map[string]any {
	return map[string]any{
		"token":      a.Token,
		"expires_at": a.ExpiresAt.UTC().Format(time.RFC3339Nano),
		"user":       userJSON(a.User),
	}
}

func attributeDefinitionJSON(d sqlc.AttributeDefinition) map[string]any {
	var enumValues []string
	if len(d.EnumValues) > 0 {
		_ = json.Unmarshal(d.EnumValues, &enumValues)
	}
	if enumValues == nil {
		enumValues = []string{}
	}
	return map[string]any{
		"id":              d.ID,
		"organisation_id": d.OrganisationID,
		"key":             d.Key,
		"label":           d.Label,
		"description":     d.Description,
		"value_type":      d.ValueType,
		"section":         d.Section,
		"sort_order":      d.SortOrder,
		"required":        d.Required,
		"enum_values":     enumValues,
		"is_pii":          d.IsPii,
		"status":          d.Status,
		"created_at":      d.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":      d.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func appUserJSON(u sqlc.AppUser) map[string]any {
	attrs := map[string]any{}
	if len(u.Attributes) > 0 {
		_ = json.Unmarshal(u.Attributes, &attrs)
	}
	out := map[string]any{
		"id":              u.ID,
		"organisation_id": u.OrganisationID,
		"display_name":    u.DisplayName,
		"status":          u.Status,
		"attributes":      attrs,
		"created_at":      u.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":      u.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if u.ExternalID.Valid {
		out["external_id"] = u.ExternalID.String
	} else {
		out["external_id"] = nil
	}
	if u.Email.Valid {
		out["email"] = u.Email.String
	} else {
		out["email"] = nil
	}
	return out
}

func emailTemplateJSON(t sqlc.EmailTemplate) map[string]any {
	return map[string]any{
		"id":              t.ID,
		"organisation_id": t.OrganisationID,
		"key":             t.Key,
		"name":            t.Name,
		"description":     t.Description,
		"subject":         t.Subject,
		"body_text":       t.BodyText,
		"body_html":       t.BodyHtml,
		"status":          t.Status,
		"is_system":       t.IsSystem,
		"created_at":      t.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":      t.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}
