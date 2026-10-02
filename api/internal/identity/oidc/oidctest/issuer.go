// Package oidctest is a fake OIDC issuer for tests: discovery, keys, and a token endpoint that checks
// the PKCE verifier and signs ID tokens for the logins a test authorizes.
package oidctest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// ClientID is the client the fake issuer expects.
const ClientID = "grimoire"

// Login is someone the issuer vouches for.
type Login struct {
	Subject  string
	Email    string
	Username string
	Name     string
	Roles    []string
}

type grant struct {
	login     Login
	nonce     string
	challenge string
}

// Issuer is a running fake issuer.
type Issuer struct {
	*httptest.Server
	key    *rsa.PrivateKey
	mu     sync.Mutex
	grants map[string]grant
}

// New starts a fake issuer for the length of a test.
func New(t *testing.T) *Issuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	i := &Issuer{Server: nil, key: key, mu: sync.Mutex{}, grants: map[string]grant{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", i.discovery)
	mux.HandleFunc("GET /jwks", i.keys)
	mux.HandleFunc("POST /token", i.token)
	i.Server = httptest.NewServer(mux)
	t.Cleanup(i.Close)
	return i
}

func (i *Issuer) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer": i.URL, "authorization_endpoint": i.URL + "/authorize", "token_endpoint": i.URL + "/token",
		"jwks_uri": i.URL + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (i *Issuer) keys(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &i.key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
}

// Authorize plays the provider's sign-in page: it reads the state, nonce and PKCE challenge from an
// authorization URL and returns the code and state the browser comes back with.
func (i *Issuer) Authorize(t *testing.T, authURL string, l Login) (string, string) {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("client_id") != ClientID || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("authorization URL = %s", authURL)
	}
	code := rand.Text()
	i.mu.Lock()
	i.grants[code] = grant{login: l, nonce: q.Get("nonce"), challenge: q.Get("code_challenge")}
	i.mu.Unlock()
	return code, q.Get("state")
}

func (i *Issuer) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	i.mu.Lock()
	g, ok := i.grants[r.PostForm.Get("code")]
	delete(i.grants, r.PostForm.Get("code"))
	i.mu.Unlock()
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if !ok || base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
		return
	}
	now := time.Now()
	claims := map[string]any{
		"iss": i.URL, "sub": g.login.Subject, "aud": ClientID, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
		"nonce": g.nonce, "email": g.login.Email, "preferred_username": g.login.Username, "name": g.login.Name, "roles": g.login.Roles,
	}
	writeJSON(w, map[string]any{"access_token": "at", "token_type": "Bearer", "expires_in": 3600, "id_token": i.sign(claims)})
}

func (i *Issuer) sign(claims map[string]any) string {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: i.key}, (&jose.SignerOptions{}).WithHeader("kid", "k1"))
	if err != nil {
		panic(err)
	}
	body, _ := json.Marshal(claims)
	sig, err := signer.Sign(body)
	if err != nil {
		panic(err)
	}
	out, err := sig.CompactSerialize()
	if err != nil {
		panic(err)
	}
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
