// Package httpx holds HTTP middleware shared by the inbound adapters.
package httpx

import (
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
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
	// ProxyHops is how many reverse proxies in front of the server are trusted to append the address
	// they saw to X-Forwarded-For. With none, an anonymous caller is keyed by the peer address, which
	// behind a proxy is the proxy's and is shared by everyone. ProxyHops stays 0 unless the server can
	// only be reached through that many proxies: a caller who reaches it directly writes the header.
	ProxyHops int
	// Log, when set, records the first request each key has refused in a window.
	Log *slog.Logger

	mu      sync.Mutex
	windows map[string]*window
}

type window struct {
	start  time.Time
	count  int
	warned bool
}

// Wrap applies the limiter to next, answering 429 once a key exceeds its budget.
func (l *RateLimiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if l.Exempt[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		k := l.key(r)
		remaining, reset, ok := l.take(k)
		h := w.Header()
		h.Set("RateLimit-Limit", strconv.Itoa(l.Limit))
		h.Set("RateLimit-Remaining", strconv.Itoa(remaining))
		h.Set("RateLimit-Reset", strconv.Itoa(reset))
		if !ok {
			l.warn(k, r.URL.Path)
			WriteProblem(w, http.StatusTooManyRequests, "Too many requests", "Slow down and try again shortly.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// warn logs a key's first refusal in its window, so a flood of 429s leaves one line, not none.
func (l *RateLimiter) warn(k, path string) {
	l.mu.Lock()
	win := l.windows[k]
	first := !win.warned
	win.warned = true
	l.mu.Unlock()
	if first && l.Log != nil {
		l.Log.Warn("rate limit reached", "key", k, "path", path, "limit", l.Limit)
	}
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

func (l *RateLimiter) key(r *http.Request) string {
	if id := r.Header.Get(IdentityHeader); id != "" {
		return "id:" + id
	}
	if client, ok := forwardedClient(r.Header.Get("X-Forwarded-For"), l.ProxyHops); ok {
		return "ip:" + client
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return "ip:" + host
}

// forwardedClient is the address the outermost of hops trusted proxies saw its caller at: each proxy
// appends the address it saw, so that is the entry hops from the right. Whatever a caller wrote into
// the header themselves lies further left and is never read. It reports false with no proxy trusted,
// with fewer entries than proxies, or when that entry is no address.
func forwardedClient(header string, hops int) (string, bool) {
	parts := strings.Split(header, ",")
	if hops < 1 || len(parts) < hops {
		return "", false
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(parts[len(parts)-hops]))
	if err != nil {
		return "", false
	}
	return addr.String(), true
}
