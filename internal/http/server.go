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
	obs                 DBPinger
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
	Observability        DBPinger // optional; when set, /readyz also pings obs DB
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
		obs:                 opts.Observability,
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
	for _, rt := range s.routeTable() {
		s.mux.HandleFunc(rt.Method+" "+rt.Pattern, s.gate(rt.Auth, rt.Handler))
	}
}

// routeTable is every route the server serves, with how each is authorised.
// See routes.go for what the declarations mean and why they exist.
//
// Health checks are registered even without a service, so the container has
// something to probe while the rest is unavailable.
func (s *Server) routeTable() []route {
	table := []route{
		{"GET", "/healthz", s.handleHealthz, public()},
		{"GET", "/readyz", s.handleReadyz, public()},
	}
	if s.svc == nil {
		return table
	}
	return append(table, []route{
		{"GET", "/v1/auth/providers", s.handleAuthProviders, public()},
		{"GET", "/v1/auth/google", s.handleGoogleStart, public()},
		{"GET", "/v1/auth/google/callback", s.handleGoogleCallback, public()},
		{"POST", "/v1/auth/dev-login", s.handleDevLogin, public()},
		{"POST", "/v1/auth/logout", s.handleLogout, user()},
		{"GET", "/v1/me", s.handleMe, user()},
		{"POST", "/v1/organisations", s.handleCreateOrganisation, principal()},
		{"GET", "/v1/organisations", s.handleListOrganisations, principal()},
		{"GET", "/v1/organisations/{id}", s.handleGetOrganisation, orgMemberAnyStatus()},
		{"PATCH", "/v1/organisations/{id}", s.handlePatchOrganisation, orgPermissionAnyStatus("organisation:update")},
		{"GET", "/v1/organisations/{id}/onboarding", s.handleGetOrgOnboarding, inService()},
		{"PATCH", "/v1/organisations/{id}/onboarding", s.handlePatchOrgOnboarding, inService()},
		{"GET", "/v1/organisations/{id}/activity", s.handleListOrgActivity, orgPermission("activity:read")},
		{"GET", "/v1/organisations/{id}/usage", s.handleListOrgUsage, orgPermission("usage:read")},
		{"POST", "/v1/organisations/{id}/archive", s.handleArchiveOrganisation, orgPermissionAnyStatus("organisation:update")},
		{"DELETE", "/v1/organisations/{id}", s.handleDeleteOrganisation, orgPermissionAnyStatus("organisation:update")},
		{"GET", "/v1/organisations/{id}/integrations", s.handleListOrgIntegrations, orgPermission("organisation:update")},
		{"PUT", "/v1/organisations/{id}/integrations/stripe", s.handleUpsertStripeIntegration, orgPermission("organisation:update")},
		{"GET", "/v1/organisations/{id}/integrations/stripe/catalog", s.handleListStripeCatalog, orgPermission("organisation:update")},
		{"POST", "/v1/organisations/{id}/integrations/stripe/import", s.handleImportStripeCatalog, orgPermission("organisation:update")},
		{"POST", "/v1/organisations/{id}/integrations/stripe/sync", s.handleSyncProductPlansToStripe, orgPermission("organisation:update")},
		{"DELETE", "/v1/organisations/{id}/integrations/{provider}", s.handleDeleteOrgIntegration, orgPermission("organisation:update")},
		{"GET", "/v1/organisations/{id}/databases", s.handleListOrgDatabases, orgPermission("organisation:update")},
		{"POST", "/v1/organisations/{id}/databases", s.handleCreateOrgDatabase, orgPermission("organisation:update")},
		{"GET", "/v1/organisations/{id}/databases/{dbId}", s.handleGetOrgDatabase, orgPermission("organisation:update")},
		{"PATCH", "/v1/organisations/{id}/databases/{dbId}", s.handlePatchOrgDatabase, orgPermission("organisation:update")},
		{"POST", "/v1/organisations/{id}/databases/{dbId}/check", s.handleCheckOrgDatabase, orgPermission("organisation:update")},
		{"POST", "/v1/organisations/{id}/databases/{dbId}/disconnect", s.handleDisconnectOrgDatabase, orgPermission("organisation:update")},
		{"DELETE", "/v1/organisations/{id}/databases/{dbId}", s.handleDeleteOrgDatabase, orgPermission("organisation:update")},
		{"GET", "/v1/organisations/{id}/webhooks", s.handleListOrgWebhooks, orgPermission("organisation:update")},
		{"POST", "/v1/organisations/{id}/webhooks", s.handleCreateOrgWebhook, orgPermission("organisation:update")},
		{"GET", "/v1/organisations/{id}/webhooks/{webhookId}", s.handleGetOrgWebhook, orgPermission("organisation:update")},
		{"PATCH", "/v1/organisations/{id}/webhooks/{webhookId}", s.handlePatchOrgWebhook, orgPermission("organisation:update")},
		{"DELETE", "/v1/organisations/{id}/webhooks/{webhookId}", s.handleDeleteOrgWebhook, orgPermission("organisation:update")},
		{"GET", "/v1/organisations/{id}/inbound-webhooks", s.handleListInboundWebhooks, orgPermission("organisation:update")},
		{"POST", "/v1/organisations/{id}/inbound-webhooks", s.handleCreateInboundWebhook, orgPermission("organisation:update")},
		{"GET", "/v1/organisations/{id}/inbound-webhooks/{hookId}", s.handleGetInboundWebhook, orgPermission("organisation:update")},
		{"PATCH", "/v1/organisations/{id}/inbound-webhooks/{hookId}", s.handlePatchInboundWebhook, orgPermission("organisation:update")},
		{"DELETE", "/v1/organisations/{id}/inbound-webhooks/{hookId}", s.handleDeleteInboundWebhook, orgPermission("organisation:update")},
		{"POST", "/v1/hooks/inbound/{hookId}", s.handleInboundWebhook, public()},
		{"POST", "/v1/hooks/inbound/{hookId}/{token}", s.handleInboundWebhookWithPathToken, public()},
		{"POST", "/v1/organisations/{id}/api-keys", s.handleCreateOrgAPIKey, orgPermission("api_keys:manage")},
		{"GET", "/v1/organisations/{id}/api-keys", s.handleListOrgAPIKeys, orgPermission("api_keys:read")},
		{"POST", "/v1/organisations/{id}/branding/logo", s.handleUploadOrganisationLogo, orgPermission("organisation:update")},
		{"DELETE", "/v1/organisations/{id}/branding/logo", s.handleDeleteOrganisationLogo, orgPermission("organisation:update")},
		{"GET", "/v1/public/organisations/{id}/branding/logo", s.handlePublicOrganisationLogo, public()},
		{"GET", "/v1/organisations/{id}/roles", s.handleListRoles, orgPermission("roles:read")},
		{"POST", "/v1/organisations/{id}/roles", s.handleCreateRole, orgPermission("roles:manage")},
		{"POST", "/v1/organisations/{id}/memberships", s.handleCreateMembership, orgPermission("members:invite")},
		{"GET", "/v1/organisations/{id}/memberships", s.handleListMemberships, orgPermission("members:read")},
		{"POST", "/v1/users", s.handleCreateUser, platform("members:invite")},
		{"GET", "/v1/users", s.handleListUsers, platform("members:read")},
		{"GET", "/v1/users/{id}", s.handleGetUser, selfOrPlatform("members:read")},
		{"PATCH", "/v1/users/{id}", s.handlePatchUser, selfOrPlatform("members:invite")},
		{"GET", "/v1/users/{id}/memberships", s.handleListUserMemberships, selfOrPlatform("members:read")},
		{"POST", "/v1/memberships/{id}/accept", s.handleAcceptMembership, user()},
		{"GET", "/v1/memberships/{id}", s.handleGetMembership, orgFromResource("members:read")},
		{"GET", "/v1/memberships/{id}/access", s.handleExplainMembershipAccess, inService()},
		{"GET", "/v1/organisations/{id}/operator-roles", s.handleListOperatorRoles, inService()},
		{"PATCH", "/v1/memberships/{id}", s.handlePatchMembership, orgFromResource("members:invite")},
		{"DELETE", "/v1/memberships/{id}", s.handleRevokeMembership, orgFromResource("members:remove")},
		{"GET", "/v1/permissions", s.handleListPermissions, principal()},
		{"GET", "/v1/permissions/{key}", s.handleGetPermission, principal()},
		{"GET", "/v1/roles/{id}", s.handleGetRole, orgFromResource("roles:read")},
		{"PATCH", "/v1/roles/{id}", s.handlePatchRole, orgFromResource("roles:manage")},
		{"DELETE", "/v1/roles/{id}", s.handleDeleteRole, orgFromResource("roles:manage")},
		{"POST", "/v1/authz/check", s.handleAuthzCheck, orgFromBody()},
		{"POST", "/v1/plans", s.handleCreatePlan, platform("billing:manage")},
		{"GET", "/v1/plans", s.handleListPlans, principal()},
		{"GET", "/v1/plans/{id}", s.handleGetPlan, principal()},
		{"PUT", "/v1/plans/{id}/entitlements", s.handleSetPlanEntitlements, platform("billing:manage")},
		{"POST", "/v1/entitlements", s.handleCreateEntitlement, platform("billing:manage")},
		{"GET", "/v1/entitlements", s.handleListEntitlementsCatalog, principal()},
		{"PUT", "/v1/organisations/{id}/subscription", s.handleUpsertSubscription, platform("billing:manage")},
		{"GET", "/v1/organisations/{id}/subscription", s.handleGetSubscription, orgPermission("billing:read")},
		{"PUT", "/v1/organisations/{id}/entitlements", s.handleSetOrgEntitlements, platform("billing:manage")},
		{"GET", "/v1/organisations/{id}/entitlements", s.handleGetOrgEntitlements, orgPermission("billing:read")},
		{"POST", "/v1/entitlements/check", s.handleEntitlementsCheck, orgFromBody()},
		{"PUT", "/v1/plans/{id}/price", s.handleUpsertPlanPrice, platform("billing:manage")},
		{"GET", "/v1/plans/{id}/prices", s.handleListPlanPrices, principal()},
		{"POST", "/v1/organisations/{id}/billing/checkout", s.handleBillingCheckout, orgPermission("billing:manage")},
		{"POST", "/v1/organisations/{id}/billing/portal", s.handleBillingPortal, orgPermission("billing:manage")},
		{"POST", "/v1/billing/webhooks/{provider}", s.handleBillingWebhook, public()},
		{"POST", "/v1/api-keys", s.handleCreateAPIKey, platform("api_keys:manage")},
		{"GET", "/v1/api-keys", s.handleListAPIKeys, platform("api_keys:read")},
		{"DELETE", "/v1/api-keys/{id}", s.handleRevokeAPIKey, orgFromResource("api_keys:manage")},
		{"GET", "/v1/organisations/{id}/app-scope-types", s.handleListAppScopeTypes, orgPermission("app_access:read")},
		{"POST", "/v1/organisations/{id}/app-scope-types", s.handleCreateAppScopeType, orgPermission("app_access:manage")},
		{"DELETE", "/v1/organisations/{id}/app-scope-types/{typeId}", s.handleDeleteAppScopeType, orgPermission("app_access:manage")},
		{"GET", "/v1/organisations/{id}/app-capabilities", s.handleListAppCapabilities, orgPermission("app_access:read")},
		{"POST", "/v1/organisations/{id}/app-capabilities", s.handleCreateAppCapability, orgPermission("app_access:manage")},
		{"DELETE", "/v1/organisations/{id}/app-capabilities/{capId}", s.handleDeleteAppCapability, orgPermission("app_access:manage")},
		// The merchant graph. Edges are their product's own facts, so writing
		// them is app_access:manage, and asking is app_access:read.
		{"POST", "/v1/organisations/{id}/edges", s.handleWriteMerchantEdges, orgPermission("app_access:manage")},
		{"DELETE", "/v1/organisations/{id}/edges", s.handleDeleteMerchantEdge, orgPermission("app_access:manage")},
		{"POST", "/v1/organisations/{id}/check", s.handleCheckMerchant, orgPermission("app_access:read")},
		// The reverse index. POST rather than GET because both take a body of
		// several fields, and a listing page is not a cacheable URL anyway.
		{"POST", "/v1/organisations/{id}/list-objects", s.handleListMerchantObjects, orgPermission("app_access:read")},
		{"POST", "/v1/organisations/{id}/list-subjects", s.handleListMerchantSubjects, orgPermission("app_access:read")},
		{"GET", "/v1/organisations/{id}/access-schema", s.handleMerchantSchema, orgPermission("app_access:read")},
		{"GET", "/v1/organisations/{id}/access-instances", s.handleMerchantInstances, orgPermission("app_access:read")},
		{"GET", "/v1/organisations/{id}/app-capability-templates", s.handleListCapabilityTemplates, orgPermission("app_access:read")},
		{"POST", "/v1/organisations/{id}/app-capability-templates/apply", s.handleApplyCapabilityTemplate, orgPermission("app_access:manage")},
		{"GET", "/v1/organisations/{id}/app-roles", s.handleListAppRoles, orgPermission("app_access:read")},
		{"POST", "/v1/organisations/{id}/app-roles", s.handleCreateAppRole, orgPermission("app_access:manage")},
		{"PATCH", "/v1/organisations/{id}/app-roles/{roleId}", s.handlePatchAppRole, orgPermission("app_access:manage")},
		{"DELETE", "/v1/organisations/{id}/app-roles/{roleId}", s.handleDeleteAppRole, orgPermission("app_access:manage")},
		{"POST", "/v1/organisations/{id}/app-grants", s.handleCreateAppGrant, orgPermission("app_access:manage")},
		{"DELETE", "/v1/organisations/{id}/app-grants/{grantId}", s.handleDeleteAppGrant, orgPermission("app_access:manage")},
		{"GET", "/v1/organisations/{id}/app-scope-types/{typeId}", s.handleGetAppScopeType, orgPermission("app_access:read")},
		{"PATCH", "/v1/organisations/{id}/app-scope-types/{typeId}", s.handlePatchAppScopeType, orgPermission("app_access:manage")},
		{"GET", "/v1/organisations/{id}/app-capabilities/{capId}", s.handleGetAppCapability, orgPermission("app_access:read")},
		{"PATCH", "/v1/organisations/{id}/app-capabilities/{capId}", s.handlePatchAppCapability, orgPermission("app_access:manage")},
		{"GET", "/v1/organisations/{id}/app-roles/{roleId}", s.handleGetAppRole, orgPermission("app_access:read")},
		{"GET", "/v1/organisations/{id}/app-user-groups/{groupId}", s.handleGetAppUserGroup, orgPermission("app_access:read")},
		{"GET", "/v1/organisations/{id}/app-grants", s.handleListAppGrants, orgPermission("app_access:read")},
		{"GET", "/v1/organisations/{id}/app-user-groups", s.handleListAppUserGroups, orgPermission("app_access:read")},
		{"POST", "/v1/organisations/{id}/app-user-groups", s.handleCreateAppUserGroup, orgPermission("app_access:manage")},
		{"PATCH", "/v1/organisations/{id}/app-user-groups/{groupId}", s.handlePatchAppUserGroup, orgPermission("app_access:manage")},
		{"DELETE", "/v1/organisations/{id}/app-user-groups/{groupId}", s.handleDeleteAppUserGroup, orgPermission("app_access:manage")},
		{"GET", "/v1/organisations/{id}/app-user-groups/{groupId}/members", s.handleListAppUserGroupMembers, orgPermission("app_access:read")},
		{"POST", "/v1/organisations/{id}/app-user-groups/{groupId}/members", s.handleAddAppUserGroupMember, orgPermission("app_access:manage")},
		{"DELETE", "/v1/organisations/{id}/app-user-groups/{groupId}/members/{appUserId}", s.handleRemoveAppUserGroupMember, orgPermission("app_access:manage")},
		{"GET", "/v1/app-users/{id}/access", s.handleAppUserAccess, orgFromResource("app_access:read")},
		{"GET", "/v1/app-users/{id}/groups", s.handleAppUserGroups, orgFromResource("app_access:read")},
		{"POST", "/v1/recovery-credentials", s.handleCreateRecoveryCredential, platform("api_keys:manage")},
		{"GET", "/v1/recovery-credentials", s.handleListRecoveryCredentials, platform("api_keys:read")},
		{"DELETE", "/v1/recovery-credentials/{id}", s.handleRevokeRecoveryCredential, platform("api_keys:manage")},
		{"GET", "/v1/audit-events", s.handleListAuditEvents, platform("activity:read")},
		{"POST", "/v1/organisations/{id}/attribute-definitions", s.handleCreateAttributeDefinition, orgPermission("attributes:manage")},
		{"GET", "/v1/organisations/{id}/attribute-definitions", s.handleListAttributeDefinitions, orgPermission("attributes:read")},
		{"GET", "/v1/attribute-definitions/{id}", s.handleGetAttributeDefinition, orgFromResource("attributes:read")},
		{"PATCH", "/v1/attribute-definitions/{id}", s.handlePatchAttributeDefinition, orgFromResource("attributes:manage")},
		{"DELETE", "/v1/attribute-definitions/{id}", s.handleDeleteAttributeDefinition, orgFromResource("attributes:manage")},
		{"POST", "/v1/organisations/{id}/app-users", s.handleCreateAppUser, orgPermission("app_users:write")},
		{"PUT", "/v1/organisations/{id}/app-users/ingest", s.handleIngestAppUser, orgPermission("app_users:write")},
		{"GET", "/v1/organisations/{id}/app-users", s.handleListAppUsers, orgPermission("app_users:read")},
		{"GET", "/v1/app-users/{id}", s.handleGetAppUser, orgFromResource("app_users:read")},
		{"PATCH", "/v1/app-users/{id}", s.handlePatchAppUser, orgFromResource("app_users:write")},
		{"DELETE", "/v1/app-users/{id}", s.handleDeleteAppUser, orgFromResource("app_users:write")},
		{"POST", "/v1/organisations/{id}/email-templates", s.handleCreateEmailTemplate, orgPermission("email_templates:manage")},
		{"GET", "/v1/organisations/{id}/email-templates", s.handleListEmailTemplates, orgPermission("email_templates:read")},
		{"GET", "/v1/email-templates/{id}", s.handleGetEmailTemplate, orgFromResource("email_templates:read")},
		{"PATCH", "/v1/email-templates/{id}", s.handlePatchEmailTemplate, orgFromResource("email_templates:manage")},
		{"DELETE", "/v1/email-templates/{id}", s.handleDeleteEmailTemplate, orgFromResource("email_templates:manage")},
		{"POST", "/v1/organisations/{id}/automations", s.handleCreateAutomation, orgPermission("automations:manage")},
		{"GET", "/v1/organisations/{id}/automations/catalog", s.handleAutomationCatalog, orgPermission("automations:read")},
		{"GET", "/v1/organisations/{id}/automations", s.handleListAutomations, orgPermission("automations:read")},
		{"GET", "/v1/organisations/{id}/automation-runs", s.handleListAutomationRuns, orgPermission("automations:read")},
		{"GET", "/v1/automations/{id}", s.handleGetAutomation, orgFromResource("automations:read")},
		{"PATCH", "/v1/automations/{id}", s.handlePatchAutomation, orgFromResource("automations:manage")},
		{"DELETE", "/v1/automations/{id}", s.handleDeleteAutomation, orgFromResource("automations:manage")},
		{"POST", "/v1/organisations/{id}/product-features", s.handleCreateProductFeature, orgPermission("product_features:manage")},
		{"GET", "/v1/organisations/{id}/product-features", s.handleListProductFeatures, orgPermission("product_features:read")},
		{"GET", "/v1/product-features/{id}", s.handleGetProductFeature, orgFromResource("product_features:read")},
		{"PATCH", "/v1/product-features/{id}", s.handlePatchProductFeature, orgFromResource("product_features:manage")},
		{"PUT", "/v1/product-features/{id}/overrides", s.handleSetProductFeatureOverrides, orgFromResource("product_features:manage")},
		{"DELETE", "/v1/product-features/{id}", s.handleDeleteProductFeature, orgFromResource("product_features:manage")},
		{"POST", "/v1/organisations/{id}/product-plans", s.handleCreateProductPlan, orgPermission("product_features:manage")},
		{"GET", "/v1/organisations/{id}/product-plans", s.handleListProductPlans, orgPermission("product_features:read")},
		{"GET", "/v1/product-plans/{id}", s.handleGetProductPlan, orgFromResource("product_features:read")},
		{"PATCH", "/v1/product-plans/{id}", s.handlePatchProductPlan, orgFromResource("product_features:manage")},
		{"PUT", "/v1/product-plans/{id}/features", s.handleSetProductPlanFeatures, orgFromResource("product_features:manage")},
		{"PUT", "/v1/product-plans/{id}/price", s.handleUpsertProductPlanPrice, orgFromResource("product_features:manage")},
		{"GET", "/v1/product-plans/{id}/prices", s.handleListProductPlanPrices, orgFromResource("product_features:read")},
		{"DELETE", "/v1/product-plans/{id}", s.handleDeleteProductPlan, orgFromResource("product_features:manage")},
		{"PUT", "/v1/organisations/{id}/product-plan", s.handleSetActiveProductPlan, orgPermission("product_features:manage")},
		{"GET", "/v1/organisations/{id}/product-plan", s.handleGetActiveProductPlan, orgPermission("product_features:read")},
	}...)
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key, Authorization, X-KYC-Webhook-Secret")
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
	if s.obs != nil {
		if err := s.obs.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "not_ready",
				"error":  "observability database unreachable",
			})
			return
		}
	}
	writeJSON(w, http.StatusOK, healthResponse{
		Status: "ok",
		Time:   s.now().UTC().Format(time.RFC3339),
	})
}
