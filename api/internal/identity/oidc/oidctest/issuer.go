// Package oidctest is a fake OIDC issuer shaped like jorisjonkers.dev: discovery, keys, a token
// endpoint that checks the client and the PKCE verifier and signs ID tokens, and a sign-in page a
// browser can use. Go tests run it in process with New; the e2e suite runs it as cmd/fakeoidc, so
// nothing ever reaches the real provider.
package oidctest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// ClientID is the client the fake issuer expects unless told otherwise.
const ClientID = "grimoire"

// The claim that lists a login's roles, and the roles jorisjonkers.dev puts in it: the Grimoire grant
// and the estate admin role. They are the names Grimoire reads when its configuration names none.
const (
	RolesClaim = "roles"
	GrantRole  = "SERVICE_GRIMOIRE"
	AdminRole  = "ROLE_ADMIN"
)

// Login is someone the issuer vouches for.
type Login struct {
	Subject  string
	Email    string
	Username string
	Name     string
	Roles    []string
}

// Options configure a Provider.
type Options struct {
	// Issuer is the URL the provider is reached at and signs as.
	Issuer string
	// ClientID is the only client the provider serves; empty means ClientID.
	ClientID string
	// ClientSecret, when set, must accompany every token request: a confidential client, like Grimoire
	// against jorisjonkers.dev. Empty serves a public client.
	ClientSecret string
	// RedirectURL, when set, is the client's registered redirect URI: the sign-in page sends the browser
	// back nowhere else.
	RedirectURL string
	// People are whom the sign-in page knows, by Username.
	People []Login
}

type grant struct {
	login     Login
	nonce     string
	challenge string
	redirect  string
}

// Provider is the fake issuer as an http.Handler, with no server of its own.
type Provider struct {
	opts   Options
	key    *rsa.PrivateKey
	page   *template.Template
	mu     sync.Mutex
	grants map[string]grant
}

// NewProvider makes a Provider with a fresh signing key.
func NewProvider(o Options) (*Provider, error) {
	if o.ClientID == "" {
		o.ClientID = ClientID
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return &Provider{opts: o, key: key, page: signInPage(), mu: sync.Mutex{}, grants: map[string]grant{}}, nil
}

// Handler serves discovery, keys, the sign-in page and the token endpoint.
func (p *Provider) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("GET /jwks", p.keys)
	mux.HandleFunc("GET /authorize", p.showSignIn)
	mux.HandleFunc("POST /authorize", p.signIn)
	mux.HandleFunc("POST /token", p.token)
	return mux
}

// Issuer is a fake issuer running for the length of a test.
type Issuer struct {
	*httptest.Server

	p *Provider
}

// New starts a fake issuer for a public client and no people, for a test that authorizes its own logins.
func New(t *testing.T) *Issuer {
	t.Helper()
	return NewWith(t, Options{Issuer: "", ClientID: ClientID, ClientSecret: "", RedirectURL: "", People: nil})
}

// NewWith starts a fake issuer with the given options; its Issuer is always the server's own URL.
func NewWith(t *testing.T, o Options) *Issuer {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	o.Issuer = "http://" + srv.Listener.Addr().String()
	p, err := NewProvider(o)
	if err != nil {
		t.Fatal(err)
	}
	srv.Config.Handler = p.Handler()
	srv.Start()
	t.Cleanup(srv.Close)
	return &Issuer{Server: srv, p: p}
}

// Authorize plays the provider's sign-in page: it reads the state, nonce and PKCE challenge from an
// authorization URL and returns the code and state the browser comes back with.
func (i *Issuer) Authorize(t *testing.T, authURL string, l Login) (string, string) {
	t.Helper()
	code, state, err := i.p.Grant(authURL, l)
	if err != nil {
		t.Fatalf("authorization URL %s: %v", authURL, err)
	}
	return code, state
}

var errBadRequest = errors.New("oidctest: not an authorization request for this client with an S256 PKCE challenge")

// Grant authorizes l for an authorization request and returns the code and the state the browser
// takes back to the client.
func (p *Provider) Grant(authURL string, l Login) (string, string, error) {
	u, err := url.Parse(authURL)
	if err != nil {
		return "", "", err
	}
	q := u.Query()
	if q.Get("client_id") != p.opts.ClientID || q.Get("response_type") != "code" || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		return "", "", errBadRequest
	}
	code := rand.Text()
	p.mu.Lock()
	p.grants[code] = grant{login: l, nonce: q.Get("nonce"), challenge: q.Get("code_challenge"), redirect: q.Get("redirect_uri")}
	p.mu.Unlock()
	return code, q.Get("state"), nil
}

func (p *Provider) discovery(w http.ResponseWriter, _ *http.Request) {
	iss := p.opts.Issuer
	writeJSON(w, map[string]any{
		"issuer": iss, "authorization_endpoint": iss + "/authorize", "token_endpoint": iss + "/token",
		"jwks_uri": iss + "/jwks", "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"}, "code_challenge_methods_supported": []string{"S256"},
		"scopes_supported": []string{"openid", "profile", "email"}, "claims_supported": []string{"sub", "email", "preferred_username", "name", RolesClaim},
	})
}

func (p *Provider) keys(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &p.key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
}

// client reports whether a token request comes from the client, its secret too when it has one, in
// the Authorization header or the form.
func (p *Provider) client(r *http.Request) bool {
	id, secret, ok := r.BasicAuth()
	if ok {
		id, _ = url.QueryUnescape(id)
		secret, _ = url.QueryUnescape(secret)
	} else {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if id != p.opts.ClientID {
		return false
	}
	return p.opts.ClientSecret == "" || subtle.ConstantTimeCompare([]byte(secret), []byte(p.opts.ClientSecret)) == 1
}

func (p *Provider) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if !p.client(r) {
		w.Header().Set("WWW-Authenticate", `Basic realm="fake-oidc"`)
		oauthError(w, http.StatusUnauthorized, "invalid_client")
		return
	}
	p.mu.Lock()
	g, ok := p.grants[r.PostForm.Get("code")]
	delete(p.grants, r.PostForm.Get("code"))
	p.mu.Unlock()
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	redirect := r.PostForm.Get("redirect_uri")
	if !ok || base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge || (redirect != "" && redirect != g.redirect) {
		oauthError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	now := time.Now()
	claims := map[string]any{
		"iss": p.opts.Issuer, "sub": g.login.Subject, "aud": p.opts.ClientID, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
		"nonce": g.nonce, "email": g.login.Email, "preferred_username": g.login.Username, "name": g.login.Name, RolesClaim: g.login.Roles,
	}
	writeJSON(w, map[string]any{"access_token": "at", "token_type": "Bearer", "expires_in": 3600, "id_token": p.sign(claims)})
}

func (p *Provider) sign(claims map[string]any) string {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: p.key}, (&jose.SignerOptions{}).WithHeader("kid", "k1"))
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

func oauthError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
