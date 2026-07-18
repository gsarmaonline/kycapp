package httpserver

import (
	"context"
	"net/http"
	"strings"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/authn"
)

type ctxKey int

const actorCtxKey ctxKey = 1

// ActorFrom returns the authenticated actor label, if any.
func ActorFrom(ctx context.Context) string {
	if p, ok := authn.FromContext(ctx); ok {
		return p.ActorLabel()
	}
	v, _ := ctx.Value(actorCtxKey).(string)
	return v
}

func unauthorizedErr() error {
	return apperr.Unauthorized("missing or invalid bearer token")
}

func rateLimitedErr() error {
	return apperr.RateLimited("rate limit exceeded")
}

func isPublicAPIPath(path string) bool {
	if strings.HasPrefix(path, "/v1/public/") {
		return true
	}
	if strings.HasPrefix(path, "/v1/billing/webhooks/") {
		return true
	}
	switch path {
	case "/v1/auth/providers",
		"/v1/auth/google",
		"/v1/auth/google/callback",
		"/v1/auth/dev-login":
		return true
	default:
		return false
	}
}

func authMiddleware(
	authenticate func(ctx context.Context, token string) (authn.Principal, bool),
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		if isPublicAPIPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			writeError(w, unauthorizedErr())
			return
		}
		principal, ok := authenticate(r.Context(), token)
		if !ok {
			writeError(w, unauthorizedErr())
			return
		}
		ctx := authn.WithPrincipal(r.Context(), principal)
		ctx = context.WithValue(ctx, actorCtxKey, principal.ActorLabel())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(h string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, prefix))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func auditMiddleware(record func(ctx context.Context, actor, method, path string, status int, orgID string) error, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isMutating(r.Method) || !strings.HasPrefix(r.URL.Path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		actor := ActorFrom(r.Context())
		if actor == "" {
			actor = "anonymous"
		}
		orgID := r.PathValue("id")
		if !strings.Contains(r.URL.Path, "/organisations/") {
			orgID = ""
		}
		_ = record(r.Context(), actor, r.Method, r.URL.Path, rec.status, orgID)
	})
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func authRateLimitMiddleware(limiter *rateLimiter, next http.Handler) http.Handler {
	if limiter == nil || limiter.limit <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/google", "/v1/auth/google/callback", "/v1/auth/dev-login":
			key := "auth:" + r.RemoteAddr
			if !limiter.allow(key) {
				writeError(w, rateLimitedErr())
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
