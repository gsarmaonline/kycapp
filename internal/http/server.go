package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// DBPinger is satisfied by the store (and easy to fake in tests).
type DBPinger interface {
	Ping(ctx context.Context) error
}

// Server serves HTTP endpoints for the KYC API.
type Server struct {
	db     DBPinger
	mux    *http.ServeMux
	// now is overridable in tests
	now func() time.Time
}

// New constructs an HTTP server. db may be nil only for tests that never hit /healthz readiness.
func New(db DBPinger) *Server {
	s := &Server{
		db:  db,
		mux: http.NewServeMux(),
		now: time.Now,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)
}

// Handler returns the root handler.
func (s *Server) Handler() http.Handler {
	return s.mux
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
