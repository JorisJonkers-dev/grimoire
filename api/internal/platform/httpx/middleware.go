// Package httpx holds HTTP middleware shared by the inbound adapters.
package httpx

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// IdentityHeader carries the subject set by the platform's forward-auth.
const IdentityHeader = "X-User-Id"

// DevIdentity fills in a fixed identity when none is present. Only wired up in local development.
func DevIdentity(subject string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(IdentityHeader) == "" {
			r.Header.Set(IdentityHeader, subject)
		}
		next.ServeHTTP(w, r)
	})
}

// RateLimiter is a fixed-window limiter keyed by identity, or by client address when anonymous.
type RateLimiter struct {
	Limit  int
	Window time.Duration
	Now    func() time.Time
	Exempt map[string]bool

	mu      sync.Mutex
	windows map[string]*window
}

type window struct {
	start time.Time
	count int
}

// Wrap applies the limiter to next, answering 429 once a key exceeds its budget.
func (l *RateLimiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if l.Exempt[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		remaining, reset, ok := l.take(key(r))
		h := w.Header()
		h.Set("RateLimit-Limit", strconv.Itoa(l.Limit))
		h.Set("RateLimit-Remaining", strconv.Itoa(remaining))
		h.Set("RateLimit-Reset", strconv.Itoa(reset))
		if !ok {
			WriteProblem(w, http.StatusTooManyRequests, "Too many requests", "Slow down and try again shortly.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *RateLimiter) take(k string) (remaining, resetSeconds int, ok bool) {
	now := l.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.windows == nil {
		l.windows = map[string]*window{}
	}
	win := l.windows[k]
	if win == nil || now.Sub(win.start) >= l.Window {
		win = &window{start: now, count: 0}
		l.windows[k] = win
	}
	resetSeconds = int(win.start.Add(l.Window).Sub(now).Round(time.Second) / time.Second)
	if win.count >= l.Limit {
		return 0, resetSeconds, false
	}
	win.count++
	return l.Limit - win.count, resetSeconds, true
}

func key(r *http.Request) string {
	if id := r.Header.Get(IdentityHeader); id != "" {
		return "id:" + id
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return "ip:" + host
}
