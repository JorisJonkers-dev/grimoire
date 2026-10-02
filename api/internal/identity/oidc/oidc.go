// Package oidc is the OIDC provider an Account can link its login to (ADR-0008): authorization code
// flow with PKCE and a nonce, the ID token verified against the issuer's keys.
package oidc

import (
	"context"
	"errors"
	"net/http"
	"sync"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
)

// Config is how to reach the provider. RolesClaim is the ID token claim that lists the login's roles.
type Config struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	RolesClaim   string
	HTTP         *http.Client
}

// Provider discovers the issuer on first use, so a provider that is down never stops Grimoire starting.
type Provider struct {
	cfg Config
	mu  sync.Mutex
	op  *gooidc.Provider
}

var _ app.Provider = (*Provider)(nil)

// New makes a Provider.
func New(cfg Config) *Provider {
	if cfg.HTTP == nil {
		cfg.HTTP = http.DefaultClient
	}
	return &Provider{cfg: cfg, mu: sync.Mutex{}, op: nil}
}

func (p *Provider) discover(ctx context.Context) (*gooidc.Provider, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.op != nil {
		return p.op, nil
	}
	op, err := gooidc.NewProvider(gooidc.ClientContext(ctx, p.cfg.HTTP), p.cfg.Issuer)
	if err != nil {
		return nil, err
	}
	p.op = op
	return op, nil
}

func (p *Provider) oauth(op *gooidc.Provider) *oauth2.Config {
	return &oauth2.Config{
		ClientID: p.cfg.ClientID, ClientSecret: p.cfg.ClientSecret, Endpoint: op.Endpoint(), RedirectURL: p.cfg.RedirectURL,
		Scopes: []string{gooidc.ScopeOpenID, "profile", "email"},
	}
}

// AuthURL is where to send the browser to sign in.
func (p *Provider) AuthURL(ctx context.Context, state, nonce, verifier string) (string, error) {
	op, err := p.discover(ctx)
	if err != nil {
		return "", err
	}
	return p.oauth(op).AuthCodeURL(state, gooidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), nil
}

var errNoIDToken = errors.New("oidc: no id_token in the token response")

// Exchange trades a code for an ID token and reads what it vouches for.
func (p *Provider) Exchange(ctx context.Context, code, verifier string) (domain.Claims, error) {
	op, err := p.discover(ctx)
	if err != nil {
		return domain.Claims{}, err
	}
	ctx = gooidc.ClientContext(ctx, p.cfg.HTTP)
	tok, err := p.oauth(op).Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return domain.Claims{}, err
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok {
		return domain.Claims{}, errNoIDToken
	}
	idt, err := op.Verifier(&gooidc.Config{ClientID: p.cfg.ClientID}).Verify(ctx, raw)
	if err != nil {
		return domain.Claims{}, err
	}
	var all map[string]any
	if err := idt.Claims(&all); err != nil {
		return domain.Claims{}, err
	}
	return domain.Claims{
		Issuer: idt.Issuer, Subject: idt.Subject, Email: text(all["email"]), Username: text(all["preferred_username"]),
		Name: text(all["name"]), Roles: texts(all[p.cfg.RolesClaim]), Nonce: idt.Nonce,
	}, nil
}

func text(v any) string {
	s, _ := v.(string)
	return s
}

func texts(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, x := range list {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
