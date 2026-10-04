package httpx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpx"
)

var ok = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(r.Header.Get(httpx.IdentityHeader)))
})

func TestDevIdentityFillsMissingSubject(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	httpx.DevIdentity("dev", ok).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if rec.Body.String() != "dev" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestDevIdentityKeepsExistingSubject(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set(httpx.IdentityHeader, "real")
	rec := httptest.NewRecorder()
	httpx.DevIdentity("dev", ok).ServeHTTP(rec, req)
	if rec.Body.String() != "real" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func do(h http.Handler, path, subject, remote string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	req.RemoteAddr = remote
	if subject != "" {
		req.Header.Set(httpx.IdentityHeader, subject)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRateLimiterCountsDownThenRejects(t *testing.T) {
	t.Parallel()
	c := &clock{t: time.Unix(1_000, 0)}
	l := &httpx.RateLimiter{Limit: 2, Window: time.Minute, Now: c.now}
	h := l.Wrap(ok)

	first := do(h, "/api", "a", "1.2.3.4:5")
	if first.Code != 200 || first.Header().Get("RateLimit-Remaining") != "1" || first.Header().Get("RateLimit-Limit") != "2" || first.Header().Get("RateLimit-Reset") != "60" {
		t.Fatalf("first: %d %v", first.Code, first.Header())
	}
	c.t = c.t.Add(10 * time.Second)
	if second := do(h, "/api", "a", "1.2.3.4:5"); second.Code != 200 || second.Header().Get("RateLimit-Remaining") != "0" || second.Header().Get("RateLimit-Reset") != "50" {
		t.Fatalf("second: %d %v", second.Code, second.Header())
	}
	third := do(h, "/api", "a", "1.2.3.4:5")
	if third.Code != http.StatusTooManyRequests || third.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("third: %d %v", third.Code, third.Header())
	}
	var p map[string]any
	if err := json.Unmarshal(third.Body.Bytes(), &p); err != nil || p["status"] != float64(429) {
		t.Fatalf("problem = %v %v", p, err)
	}
	if other := do(h, "/api", "b", "1.2.3.4:5"); other.Code != 200 {
		t.Fatalf("other subject limited: %d", other.Code)
	}
	c.t = c.t.Add(time.Minute)
	if again := do(h, "/api", "a", "1.2.3.4:5"); again.Code != 200 {
		t.Fatalf("window did not reset: %d", again.Code)
	}
}

func TestRateLimiterKeysAnonymousByAddress(t *testing.T) {
	t.Parallel()
	l := &httpx.RateLimiter{Limit: 1, Window: time.Minute, Now: time.Now}
	h := l.Wrap(ok)
	if r := do(h, "/api", "", "9.9.9.9:1"); r.Code != 200 {
		t.Fatalf("first: %d", r.Code)
	}
	if r := do(h, "/api", "", "9.9.9.9:2"); r.Code != http.StatusTooManyRequests {
		t.Fatalf("same host, other port: %d", r.Code)
	}
	if r := do(h, "/api", "", "no-port"); r.Code != 200 {
		t.Fatalf("unparseable address: %d", r.Code)
	}
}

func TestRateLimiterExemptsProbes(t *testing.T) {
	t.Parallel()
	l := &httpx.RateLimiter{Limit: 1, Window: time.Minute, Now: time.Now, Exempt: map[string]bool{"/healthz": true}}
	h := l.Wrap(ok)
	for range 3 {
		if r := do(h, "/healthz", "", "1.1.1.1:1"); r.Code != 200 || r.Header().Get("RateLimit-Limit") != "" {
			t.Fatalf("probe limited: %d", r.Code)
		}
	}
}

func TestRateLimiterLogsTheFirstRefusalOfEachWindow(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	l := &httpx.RateLimiter{Limit: 1, Window: time.Minute, Now: time.Now, Log: slog.New(slog.NewTextHandler(&out, nil))}
	h := l.Wrap(ok)
	for range 3 {
		do(h, "/api/x", "alice", "1.1.1.1:1")
	}
	if n := strings.Count(out.String(), "rate limit reached"); n != 1 || !strings.Contains(out.String(), "key=id:alice") {
		t.Fatalf("log = %q", out.String())
	}
	quiet := (&httpx.RateLimiter{Limit: 1, Window: time.Minute, Now: time.Now}).Wrap(ok)
	do(quiet, "/api/x", "bob", "1.1.1.1:1")
	if r := do(quiet, "/api/x", "bob", "1.1.1.1:1"); r.Code != http.StatusTooManyRequests {
		t.Fatalf("without a logger = %d", r.Code)
	}
}

// from makes an anonymous request from a peer address with an X-Forwarded-For header.
func from(h http.Handler, peer, forwarded string) int {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api", nil)
	r.RemoteAddr = peer
	if forwarded != "" {
		r.Header.Set("X-Forwarded-For", forwarded)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec.Code
}

// Behind trusted reverse proxies every anonymous reader has a budget of their own, by the address the
// outermost trusted proxy saw them at, and cannot claim another's by writing the header themselves.
func TestRateLimiterKeysAnonymousByTheClientBehindTrustedProxies(t *testing.T) {
	t.Parallel()
	const proxy, busy = "10.0.0.1:443", http.StatusTooManyRequests
	one := (&httpx.RateLimiter{Limit: 1, Window: time.Minute, Now: time.Now, ProxyHops: 1}).Wrap(ok)
	if a, b := from(one, proxy, "203.0.113.7"), from(one, proxy, "198.51.100.9"); a != 200 || b != 200 {
		t.Fatalf("two readers behind one proxy: %d %d", a, b)
	}
	// The first reader is past their budget, whatever they put in front of what the proxy appends.
	for _, forwarded := range []string{"203.0.113.7", "1.2.3.4, 203.0.113.7", " 5.6.7.8 ,9.9.9.9 , 203.0.113.7 "} {
		if got := from(one, proxy, forwarded); got != busy {
			t.Fatalf("%q: %d", forwarded, got)
		}
	}
	// IPv6 readers are told apart too.
	if a, b := from(one, proxy, "2001:db8::1"), from(one, proxy, "2001:db8::1"); a != 200 || b != busy {
		t.Fatalf("an IPv6 reader: %d %d", a, b)
	}
	// With nothing usable from the proxy the peer address is the key, as with no proxy at all.
	if got := from(one, "10.0.0.2:443", ""); got != 200 {
		t.Fatalf("no header: %d", got)
	}
	for _, forwarded := range []string{"", "not an address", "203.0.113.200, nonsense", ","} {
		if got := from(one, "10.0.0.2:443", forwarded); got != busy {
			t.Fatalf("an unusable header %q is keyed by the peer: %d", forwarded, got)
		}
	}

	// Two proxies deep, the reader is the second from the right; the first proxy's own address is not.
	two := (&httpx.RateLimiter{Limit: 1, Window: time.Minute, Now: time.Now, ProxyHops: 2}).Wrap(ok)
	if a, b := from(two, proxy, "203.0.113.7, 172.16.0.1"), from(two, proxy, "198.51.100.9, 172.16.0.1"); a != 200 || b != 200 {
		t.Fatalf("two readers behind two proxies: %d %d", a, b)
	}
	if got := from(two, proxy, "6.6.6.6, 203.0.113.7, 172.16.0.2"); got != busy {
		t.Fatalf("the same reader through another edge: %d", got)
	}
	// Fewer entries than proxies: something reached the server past the outer proxy, so it is keyed by its peer.
	if a, b := from(two, "10.0.0.3:443", "203.0.113.99"), from(two, "10.0.0.3:443", "203.0.113.98"); a != 200 || b != busy {
		t.Fatalf("a short header: %d %d", a, b)
	}

	// With no proxy trusted the header counts for nothing: everyone from one peer shares its budget.
	none := (&httpx.RateLimiter{Limit: 1, Window: time.Minute, Now: time.Now}).Wrap(ok)
	if a, b := from(none, proxy, "203.0.113.7"), from(none, proxy, "198.51.100.9"); a != 200 || b != busy {
		t.Fatalf("an untrusted header: %d %d", a, b)
	}
	// A signed-in caller is keyed by who they are, wherever they come from.
	if a, b := do(one, "/api", "aria", proxy), do(one, "/api", "aria", "10.9.9.9:1"); a.Code != 200 || b.Code != busy {
		t.Fatalf("a signed-in caller: %d %d", a.Code, b.Code)
	}
}

// with makes an anonymous request from a peer with several X-Forwarded-For header lines.
func with(h http.Handler, peer string, lines ...string) int {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api", nil)
	r.RemoteAddr = peer
	for _, line := range lines {
		r.Header.Add("X-Forwarded-For", line)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec.Code
}

// A proxy may add a header line of its own instead of appending to the caller's: the lines read as
// one list, so the caller's own line never stands in for what the proxy saw.
func TestRateLimiterReadsEveryForwardedLine(t *testing.T) {
	t.Parallel()
	const proxy, busy = "10.0.0.1:443", http.StatusTooManyRequests
	one := (&httpx.RateLimiter{Limit: 1, Window: time.Minute, Now: time.Now, ProxyHops: 1}).Wrap(ok)
	if got := with(one, proxy, "203.0.113.7"); got != 200 {
		t.Fatalf("first: %d", got)
	}
	for _, forged := range []string{"1.1.1.1", "2.2.2.2", "3.3.3.3, 4.4.4.4"} {
		if got := with(one, proxy, forged, "203.0.113.7"); got != busy {
			t.Fatalf("a line of the caller's own (%q) before the proxy's: %d", forged, got)
		}
	}
	two := (&httpx.RateLimiter{Limit: 1, Window: time.Minute, Now: time.Now, ProxyHops: 2}).Wrap(ok)
	if a, b := with(two, proxy, "9.9.9.9", "203.0.113.7", "172.16.0.1"), with(two, proxy, "8.8.8.8, 203.0.113.7", "172.16.0.2"); a != 200 || b != busy {
		t.Fatalf("two proxies, a line each: %d %d", a, b)
	}
}

// Whoever holds an IPv6 network holds every address in it: they are one caller, by their /64. An IPv4
// address written the IPv6 way is the IPv4 caller it is.
func TestRateLimiterKeysIPv6ByNetwork(t *testing.T) {
	t.Parallel()
	const proxy, busy = "10.0.0.1:443", http.StatusTooManyRequests
	one := (&httpx.RateLimiter{Limit: 1, Window: time.Minute, Now: time.Now, ProxyHops: 1}).Wrap(ok)
	if got := from(one, proxy, "2001:db8:aa:bb::1"); got != 200 {
		t.Fatalf("first: %d", got)
	}
	for _, same := range []string{"2001:db8:aa:bb::2", "2001:db8:aa:bb:ffff:ffff:ffff:ffff", "2001:DB8:AA:BB:1:2:3:4"} {
		if got := from(one, proxy, same); got != busy {
			t.Fatalf("%s is the same network: %d", same, got)
		}
	}
	if got := from(one, proxy, "2001:db8:aa:bc::1"); got != 200 {
		t.Fatalf("the network next door: %d", got)
	}
	if a, b := from(one, proxy, "192.0.2.44"), from(one, proxy, "::ffff:192.0.2.44"); a != 200 || b != busy {
		t.Fatalf("an IPv4 caller written both ways: %d %d", a, b)
	}
	if got := from(one, proxy, "192.0.2.45"); got != 200 {
		t.Fatalf("IPv4 callers stay apart by address: %d", got)
	}

	// With no proxy the peer is keyed the same way.
	none := (&httpx.RateLimiter{Limit: 1, Window: time.Minute, Now: time.Now}).Wrap(ok)
	if a, b, c := from(none, "[2001:db8:1:2::a]:5000", ""), from(none, "[2001:db8:1:2::b]:5001", ""), from(none, "[2001:db8:1:3::a]:5000", ""); a != 200 || b != busy || c != 200 {
		t.Fatalf("IPv6 peers: %d %d %d", a, b, c)
	}
	// A zone names the interface the address was seen on, not another caller.
	if a, b, c := from(none, "[fe80::1%eth0]:1", ""), from(none, "[fe80::2%eth1]:1", ""), from(none, "[fe80:0:0:1::1%eth0]:1", ""); a != 200 || b != busy || c != 200 {
		t.Fatalf("zoned peers: %d %d %d", a, b, c)
	}
	if a, b := from(none, "192.0.2.1:1", ""), from(none, "[::ffff:192.0.2.1]:2", ""); a != 200 || b != busy {
		t.Fatalf("an IPv4 peer written both ways: %d %d", a, b)
	}
}
