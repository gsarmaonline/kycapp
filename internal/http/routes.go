package httpserver

import "net/http"

// The route table.
//
// Registration used to be 151 bare mux.HandleFunc calls, which meant two things
// were impossible. Go's ServeMux cannot list what has been registered, so
// nothing could be asserted about the routes as a set. And authorisation lives
// in two layers — most handlers gate themselves, a few gate inside the service —
// so whether a route was guarded could not be answered by looking at it.
//
// Each route now declares how it is authorised, and TestEveryRouteDeclaresAuth
// checks that no organisation-scoped route is left undeclared. The declaration
// is documentation that fails the build when it stops being true.
//
// It does not replace the check the handler performs. Making it authoritative
// is the next step: routes marked orgPermission take the organisation straight
// from the path, so a middleware could apply them from this table and the
// handler's first four lines could go.

type route struct {
	Method  string
	Pattern string
	Handler http.HandlerFunc
	Auth    authRule
}

type authKind string

const (
	// authPublic reaches the handler with no principal at all. Some of these
	// carry their own secret instead, such as a webhook signature.
	authPublic authKind = "public"
	// authPrincipal requires any authenticated caller and nothing more.
	authPrincipal authKind = "principal"
	// authUser requires a human session and refuses every API key.
	authUser authKind = "user"
	// authPlatform requires a named capability at global scope.
	authPlatform authKind = "platform"
	// authOrgMember requires reach into the organisation named in the path.
	authOrgMember authKind = "org_member"
	// authOrgMemberAnyStatus is authOrgMember for a lifecycle route: a
	// suspended organisation stays visible to its own members, so the state can
	// be seen and acted on rather than the tenant simply vanishing.
	authOrgMemberAnyStatus authKind = "org_member_any_status"
	// authOrgPermission requires a named permission in the organisation named
	// in the path. These are the routes a middleware could gate from this
	// table, because the organisation is known before the handler runs.
	authOrgPermission authKind = "org_permission"
	// authOrgPermissionAnyStatus is authOrgPermission that also accepts a
	// suspended or archived organisation.
	//
	// Lifecycle routes need it. Status is settable through PATCH, so without it
	// suspending a tenant made every route on that tenant return 404, including
	// the route that would restore it: a one-way door out of "suspend for
	// non-payment" into deletion. Read, update and delete therefore work on any
	// status, and everything else stays active-only, which is what suspension
	// is for.
	authOrgPermissionAnyStatus authKind = "org_permission_any_status"
	// authOrgFromResource requires a permission in an organisation that is only
	// known after loading the resource, so the handler must gate itself. The
	// permission is recorded here for the reader, not enforced from here.
	authOrgFromResource authKind = "org_from_resource"
	// authOrgFromBody requires reach into an organisation named in the request
	// body, so neither the path nor a loaded resource identifies it. The check
	// endpoints work this way, which is why they can never be gated from the
	// route table.
	authOrgFromBody authKind = "org_from_body"
	// authInService means the gate lives in the service call rather than the
	// handler. Recorded so an ungated route is distinguishable from one guarded
	// somewhere else.
	authInService authKind = "in_service"
)

type authRule struct {
	Kind authKind
	// Permission is the capability required, for the kinds that take one.
	Permission string
}

func public() authRule      { return authRule{Kind: authPublic} }
func principal() authRule   { return authRule{Kind: authPrincipal} }
func user() authRule        { return authRule{Kind: authUser} }
func inService() authRule   { return authRule{Kind: authInService} }
func orgFromBody() authRule { return authRule{Kind: authOrgFromBody} }

func platform(perm string) authRule {
	return authRule{Kind: authPlatform, Permission: perm}
}

func orgMember() authRule          { return authRule{Kind: authOrgMember} }
func orgMemberAnyStatus() authRule { return authRule{Kind: authOrgMemberAnyStatus} }

func orgPermission(perm string) authRule {
	return authRule{Kind: authOrgPermission, Permission: perm}
}

func orgPermissionAnyStatus(perm string) authRule {
	return authRule{Kind: authOrgPermissionAnyStatus, Permission: perm}
}

func orgFromResource(perm string) authRule {
	return authRule{Kind: authOrgFromResource, Permission: perm}
}

// IsOrgScoped reports whether the pattern addresses one organisation, either
// directly or through a resource that belongs to one. Used by the test that
// every such route declares how it is guarded.
func (r route) IsOrgScoped() bool {
	switch r.Auth.Kind {
	case authOrgMember, authOrgMemberAnyStatus, authOrgPermission, authOrgPermissionAnyStatus,
		authOrgFromResource, authOrgFromBody, authInService:
		return true
	default:
		return false
	}
}

// gate applies the rules the table can enforce.
//
// Only the kinds where the organisation is known before the handler runs, which
// is every route taking it straight from the path. The rest still gate
// themselves, because the organisation is not knowable until the resource is
// loaded or the body is read.
//
// This is the reason the table declares rather than merely documents: for these
// routes the declaration is now the enforcement, so a route cannot be gated
// differently from what it claims.
func (s *Server) gate(rule authRule, next http.HandlerFunc) http.HandlerFunc {
	switch rule.Kind {
	case authOrgPermission:
		return func(w http.ResponseWriter, r *http.Request) {
			if _, err := s.svc.RequireOrgPermission(r.Context(), r.PathValue("id"), rule.Permission); err != nil {
				writeError(w, err)
				return
			}
			next(w, r)
		}
	case authOrgPermissionAnyStatus:
		return func(w http.ResponseWriter, r *http.Request) {
			if _, err := s.svc.RequireOrgPermissionAnyStatus(r.Context(), r.PathValue("id"), rule.Permission); err != nil {
				writeError(w, err)
				return
			}
			next(w, r)
		}
	case authOrgMemberAnyStatus:
		return func(w http.ResponseWriter, r *http.Request) {
			if _, err := s.svc.RequireOrgMemberAnyStatus(r.Context(), r.PathValue("id")); err != nil {
				writeError(w, err)
				return
			}
			next(w, r)
		}
	case authOrgMember:
		return func(w http.ResponseWriter, r *http.Request) {
			if _, err := s.svc.RequireOrgMember(r.Context(), r.PathValue("id")); err != nil {
				writeError(w, err)
				return
			}
			next(w, r)
		}
	default:
		return next
	}
}

// EnforcedFromTable reports whether the gate applies this rule, as opposed to
// the handler applying it. Used by the test that checks the two never disagree.
func (r authRule) EnforcedFromTable() bool {
	switch r.Kind {
	case authOrgPermission, authOrgPermissionAnyStatus, authOrgMember, authOrgMemberAnyStatus:
		return true
	default:
		return false
	}
}
