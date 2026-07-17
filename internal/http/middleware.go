package httpserver

import (
	"context"
	"net/http"
	"strings"

	"github.com/gsarmaonline/kyc/internal/apperr"
)

type ctxKey int

const actorCtxKey ctxKey = 1

// ActorFrom returns the authenticated actor label, if any.
func ActorFrom(ctx context.Context) string {
	v, _ := ctx.Value(actorCtxKey).(string)
	return v
}

func withActor(ctx context.Context, actor string) context.Context {
	return context.WithValue(ctx, actorCtxKey, actor)
}

func rateLimitedErr() error {
	return apperr.RateLimited("rate limit exceeded")
}

func unauthorizedErr() error {
	return apperr.Unauthorized("missing or invalid bearer token")
}

func authMiddleware(authenticate func(ctx context.Context, token string) (string, bool), required bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			if required {
				writeError(w, unauthorizedErr())
				return
			}
			next.ServeHTTP(w, r.WithContext(withActor(r.Context(), "anonymous")))
			return
		}
		actor, ok := authenticate(r.Context(), token)
		if !ok {
			writeError(w, unauthorizedErr())
			return
		}
		next.ServeHTTP(w, r.WithContext(withActor(r.Context(), actor)))
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
		// Best-effort; never fail the request on audit write.
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
