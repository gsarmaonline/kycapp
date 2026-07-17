package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/service"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
)

// DBPinger is satisfied by the store (and easy to fake in tests).
type DBPinger interface {
	Ping(ctx context.Context) error
}

// Server serves HTTP endpoints for the KYC API.
type Server struct {
	db         DBPinger
	svc        *service.Service
	mux        *http.ServeMux
	now        func() time.Time
	corsOrigin string
}

// Options configures the HTTP server.
type Options struct {
	Service    *service.Service
	CORSOrigin string
}

// New constructs an HTTP server.
func New(db DBPinger, opts Options) *Server {
	s := &Server{
		db:         db,
		svc:        opts.Service,
		mux:        http.NewServeMux(),
		now:        time.Now,
		corsOrigin: opts.CORSOrigin,
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

	s.mux.HandleFunc("POST /v1/signup", s.handleSignup)

	s.mux.HandleFunc("POST /v1/organisations", s.handleCreateOrganisation)
	s.mux.HandleFunc("GET /v1/organisations", s.handleListOrganisations)
	s.mux.HandleFunc("GET /v1/organisations/{id}", s.handleGetOrganisation)
	s.mux.HandleFunc("PATCH /v1/organisations/{id}", s.handlePatchOrganisation)
	s.mux.HandleFunc("POST /v1/organisations/{id}/archive", s.handleArchiveOrganisation)
	s.mux.HandleFunc("GET /v1/organisations/{id}/roles", s.handleListRoles)
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
}

// Handler returns the root handler (with optional CORS).
func (s *Server) Handler() http.Handler {
	h := http.Handler(s.mux)
	if s.corsOrigin != "" {
		h = corsMiddleware(s.corsOrigin, h)
	}
	return h
}

func corsMiddleware(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
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
		"id":         u.ID,
		"email":      u.Email,
		"name":       u.Name,
		"status":     u.Status,
		"created_at": u.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at": u.UpdatedAt.UTC().Format(time.RFC3339Nano),
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
