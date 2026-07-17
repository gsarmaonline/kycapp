package httpserver

import (
	"net/http"
	"sync"
	"time"
)

// rateLimiter is a simple per-key fixed window counter.
type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	hits    map[string]int
	resetAt map[string]time.Time
	now     func() time.Time
}

func newRateLimiter(perMinute int) *rateLimiter {
	return &rateLimiter{
		limit:   perMinute,
		window:  time.Minute,
		hits:    make(map[string]int),
		resetAt: make(map[string]time.Time),
		now:     time.Now,
	}
}

func (r *rateLimiter) allow(key string) bool {
	if r == nil || r.limit <= 0 {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	if until, ok := r.resetAt[key]; !ok || now.After(until) {
		r.resetAt[key] = now.Add(r.window)
		r.hits[key] = 1
		return true
	}
	if r.hits[key] >= r.limit {
		return false
	}
	r.hits[key]++
	return true
}

func rateLimitMiddleware(limiter *rateLimiter, next http.Handler) http.Handler {
	if limiter == nil || limiter.limit <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/authz/check" || r.URL.Path == "/v1/entitlements/check" {
			key := ActorFrom(r.Context())
			if key == "" {
				key = r.RemoteAddr
			}
			if !limiter.allow(key) {
				writeError(w, rateLimitedErr())
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
