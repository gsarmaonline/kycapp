package httpserver

import (
	"strings"
	"testing"

	"github.com/gsarmaonline/kyc/internal/service"
)

// tableForTest builds the route table with a non-nil service, so the full set is
// returned rather than just the health checks.
func tableForTest(t *testing.T) []route {
	t.Helper()
	// A service with no store is enough: building the table only needs the
	// handler methods to exist, not a database behind them.
	s := New(nil, Options{Service: service.New(nil)})
	table := s.routeTable()
	if len(table) < 100 {
		t.Fatalf("route table looks truncated: %d routes", len(table))
	}
	return table
}

// The reason this table exists. A new handler that forgets its gate is the
// highest-consequence mistake in this package and was previously silent: the
// route registered, the handler ran, and nothing said the organisation had gone
// unchecked.
//
// Every route reaching organisation data must say how it is guarded. The
// declaration is not the enforcement, but an undeclared route now fails here
// instead of shipping.
func TestEveryOrgScopedRouteDeclaresAuthorisation(t *testing.T) {
	for _, rt := range tableForTest(t) {
		if !strings.Contains(rt.Pattern, "/organisations/{id}") {
			continue
		}
		if rt.Auth.Kind == authPublic {
			// Only the logo is deliberately public, and it lives under
			// /v1/public/ so it is obvious in the path.
			if !strings.HasPrefix(rt.Pattern, "/v1/public/") {
				t.Errorf("%s %s is public but not under /v1/public/", rt.Method, rt.Pattern)
			}
			continue
		}
		// Platform is a stricter requirement than any organisation gate, not a
		// weaker one, and some organisation-scoped routes are deliberately
		// platform-only: assigning a plan and overriding entitlements. That
		// wall is what stops a merchant granting their own organisation an
		// entitlement, so it must stay platform rather than become org-scoped.
		if rt.Auth.Kind == authPlatform {
			continue
		}
		if !rt.IsOrgScoped() {
			t.Errorf("%s %s addresses an organisation but declares %q",
				rt.Method, rt.Pattern, rt.Auth.Kind)
		}
	}
}

// A declaration of "" would pass every other check while meaning nothing, so
// the zero value must never appear.
func TestNoRouteHasAnEmptyAuthKind(t *testing.T) {
	for _, rt := range tableForTest(t) {
		if rt.Auth.Kind == "" {
			t.Errorf("%s %s declares no authorisation", rt.Method, rt.Pattern)
		}
	}
}

// Kinds that gate on a named capability must name one, or the declaration says
// less than it appears to.
func TestPermissionKindsCarryAPermission(t *testing.T) {
	for _, rt := range tableForTest(t) {
		switch rt.Auth.Kind {
		case authPlatform, authOrgPermission, authOrgFromResource:
			if rt.Auth.Permission == "" {
				t.Errorf("%s %s declares %q with no permission", rt.Method, rt.Pattern, rt.Auth.Kind)
			}
		}
	}
}

// Registration must stay one-to-one with the table. A duplicate pattern is a
// silent override in ServeMux and would take a route out of service.
func TestRoutePatternsAreUnique(t *testing.T) {
	seen := map[string]string{}
	for _, rt := range tableForTest(t) {
		key := rt.Method + " " + rt.Pattern
		if prev, dup := seen[key]; dup {
			t.Errorf("%s registered twice (%s and again)", key, prev)
			continue
		}
		seen[key] = rt.Pattern
	}
}

// Every route needs a handler. A nil one panics at request time rather than at
// startup, which is the worst moment to find out.
func TestEveryRouteHasAHandler(t *testing.T) {
	for _, rt := range tableForTest(t) {
		if rt.Handler == nil {
			t.Errorf("%s %s has no handler", rt.Method, rt.Pattern)
		}
	}
}
