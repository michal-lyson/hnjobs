package api

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

type ipWindow struct {
	count int
	start time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	windows map[string]*ipWindow
	limit   int
	window  time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		windows: make(map[string]*ipWindow),
		limit:   limit,
		window:  window,
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	w, ok := rl.windows[ip]
	if !ok || now.Sub(w.start) > rl.window {
		rl.windows[ip] = &ipWindow{count: 1, start: now}
		return true
	}
	w.count++
	return w.count <= rl.limit
}

// cleanup removes stale entries every 5 minutes to prevent unbounded memory growth.
func (rl *rateLimiter) cleanup() {
	for range time.Tick(5 * time.Minute) {
		rl.mu.Lock()
		for ip, w := range rl.windows {
			if time.Since(w.start) > rl.window {
				delete(rl.windows, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *rateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := realIP(r)
		if !rl.allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded"}`)) //nolint
			return
		}
		next.ServeHTTP(w, r)
	})
}

// realIP extracts the client IP, respecting X-Forwarded-For set by the proxy.
func realIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first (leftmost) IP — the original client
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// Strip port from RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
