// Package httpx holds HTTP plumbing shared by the inbound adapters.
package httpx

import "net/http"

// Health answers liveness and readiness probes. Readiness gains real checks once the API has
// dependencies to wait for.
func Health() http.Handler {
	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
	mux.HandleFunc("GET /healthz", ok)
	mux.HandleFunc("GET /readyz", ok)
	return mux
}
