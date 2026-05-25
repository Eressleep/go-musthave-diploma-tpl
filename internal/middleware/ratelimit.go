package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type rateWindow struct {
	count   int
	resetAt time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	requests map[string]*rateWindow
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, w time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*rateWindow),
		limit:    limit,
		window:   w,
	}

	go rl.cleanup()

	return rl
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr

		rl.mu.Lock()

		wnd, ok := rl.requests[ip]
		if !ok || time.Now().After(wnd.resetAt) {
			wnd = &rateWindow{resetAt: time.Now().Add(rl.window)}
			rl.requests[ip] = wnd
		}

		wnd.count++
		count := wnd.count
		rl.mu.Unlock()

		if count > rl.limit {
			w.Header().Set("Retry-After", "60")
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.limit))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, wnd := range rl.requests {
			if now.After(wnd.resetAt) {
				delete(rl.requests, ip)
			}
		}
		rl.mu.Unlock()
	}
}
