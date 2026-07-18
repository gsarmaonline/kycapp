package httpserver

import (
	"context"
	"net/http"
	"time"

	"github.com/gsarmaonline/kyc/internal/authn"
	"github.com/gsarmaonline/kyc/internal/service"
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
	s.mux.HandleFunc("POST /v1/organisations/{id}/branding/logo", s.handleUploadOrganisationLogo)
	s.mux.HandleFunc("DELETE /v1/organisations/{id}/branding/logo", s.handleDeleteOrganisationLogo)
	s.mux.HandleFunc("GET /v1/public/organisations/{id}/branding/logo", s.handlePublicOrganisationLogo)
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
	s.mux.HandleFunc("GET /v1/memberships/{id}", s.handleGetMembership)
	s.mux.HandleFunc("PATCH /v1/memberships/{id}", s.handlePatchMembership)
	s.mux.HandleFunc("DELETE /v1/memberships/{id}", s.handleRevokeMembership)

	s.mux.HandleFunc("GET /v1/permissions", s.handleListPermissions)
	s.mux.HandleFunc("GET /v1/permissions/{key}", s.handleGetPermission)
	s.mux.HandleFunc("GET /v1/roles/{id}", s.handleGetRole)
	s.mux.HandleFunc("PATCH /v1/roles/{id}", s.handlePatchRole)
	s.mux.HandleFunc("DELETE /v1/roles/{id}", s.handleDeleteRole)
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
	s.mux.HandleFunc("GET /v1/attribute-definitions/{id}", s.handleGetAttributeDefinition)
	s.mux.HandleFunc("PATCH /v1/attribute-definitions/{id}", s.handlePatchAttributeDefinition)
	s.mux.HandleFunc("DELETE /v1/attribute-definitions/{id}", s.handleDeleteAttributeDefinition)
	s.mux.HandleFunc("POST /v1/organisations/{id}/app-users", s.handleCreateAppUser)
	s.mux.HandleFunc("GET /v1/organisations/{id}/app-users", s.handleListAppUsers)
	s.mux.HandleFunc("GET /v1/app-users/{id}", s.handleGetAppUser)
	s.mux.HandleFunc("PATCH /v1/app-users/{id}", s.handlePatchAppUser)
	s.mux.HandleFunc("DELETE /v1/app-users/{id}", s.handleDeleteAppUser)

	s.mux.HandleFunc("POST /v1/organisations/{id}/email-templates", s.handleCreateEmailTemplate)
	s.mux.HandleFunc("GET /v1/organisations/{id}/email-templates", s.handleListEmailTemplates)
	s.mux.HandleFunc("GET /v1/email-templates/{id}", s.handleGetEmailTemplate)
	s.mux.HandleFunc("PATCH /v1/email-templates/{id}", s.handlePatchEmailTemplate)
	s.mux.HandleFunc("DELETE /v1/email-templates/{id}", s.handleDeleteEmailTemplate)

	s.mux.HandleFunc("POST /v1/organisations/{id}/automations", s.handleCreateAutomation)
	s.mux.HandleFunc("GET /v1/organisations/{id}/automations", s.handleListAutomations)
	s.mux.HandleFunc("GET /v1/organisations/{id}/automation-runs", s.handleListAutomationRuns)
	s.mux.HandleFunc("GET /v1/automations/{id}", s.handleGetAutomation)
	s.mux.HandleFunc("PATCH /v1/automations/{id}", s.handlePatchAutomation)
	s.mux.HandleFunc("DELETE /v1/automations/{id}", s.handleDeleteAutomation)
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
