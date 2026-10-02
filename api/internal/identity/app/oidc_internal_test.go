package app

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
)

// fakeProvider vouches for one login, or fails.
type fakeProvider struct {
	claims domain.Claims
	fail   bool
}

func (p fakeProvider) AuthURL(context.Context, string, string, string) (string, error) {
	if p.fail {
		return "", errBoom
	}
	return "https://idp.example/authorize", nil
}

func (p fakeProvider) Exchange(context.Context, string, string) (domain.Claims, error) {
	if p.fail {
		return domain.Claims{}, errBoom
	}
	return p.claims, nil
}

func oidcService(r *fakeRepo, p fakeProvider) *Service {
	s := service(r)
	s.OIDC, s.Grant, s.AdminRole = p, "grimoire", "admin"
	return s
}

func TestOIDCErrorPaths(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	login := fakeProvider{claims: domain.Claims{Subject: "estate", Email: "a@b.c", Roles: []string{"grimoire"}, Nonce: "n"}}
	for call, r := range map[string]*fakeRepo{"by subject": {fail: "by subject"}, "insert request": {fail: "insert request"}} {
		if _, _, err := oidcService(r, login).StartOIDC(ctx, "someone"); !errors.Is(err, errBoom) {
			t.Errorf("start failing at %s = %v", call, err)
		}
	}
	if _, _, err := oidcService(&fakeRepo{}, fakeProvider{fail: true}).StartOIDC(ctx, ""); !errors.Is(err, errBoom) {
		t.Errorf("a provider that is down = %v", err)
	}
	if _, err := oidcService(&fakeRepo{nonce: "other"}, login).FinishOIDC(ctx, "c", "s", "ua"); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Errorf("a replayed nonce = %v", err)
	}
	id := uuid.New()
	for call, r := range map[string]*fakeRepo{
		"linked account": {fail: "linked account", nonce: "n"},
		"by id":          {fail: "by id", nonce: "n"},
		"update link":    {fail: "update link", nonce: "n"},
		"insert session": {fail: "insert session", nonce: "n"},
		"link by id":     {fail: "by id", nonce: "n", linkFor: &id},
		"insert link":    {fail: "insert oidc link", nonce: "n", linkFor: &id},
	} {
		if _, err := oidcService(r, login).FinishOIDC(ctx, "c", "s", "ua"); !errors.Is(err, errBoom) {
			t.Errorf("finish failing at %s = %v", call, err)
		}
	}
	if _, err := oidcService(&fakeRepo{fail: "insert pending", nonce: "n", unlinked: true}, login).FinishOIDC(ctx, "c", "s", "ua"); !errors.Is(err, errBoom) {
		t.Errorf("a waiting login failing = %v", err)
	}
	admin := fakeProvider{claims: domain.Claims{Subject: "estate", Roles: []string{"admin"}, Nonce: "n"}}
	if _, err := oidcService(&fakeRepo{fail: "set admin", nonce: "n"}, admin).FinishOIDC(ctx, "c", "s", "ua"); !errors.Is(err, errBoom) {
		t.Errorf("promoting failing = %v", err)
	}
	if _, err := oidcService(&fakeRepo{nonce: "n", account: domain.Account{Disabled: true}}, login).FinishOIDC(ctx, "c", "s", "ua"); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Errorf("a disabled Account = %v", err)
	}
	for call, r := range map[string]*fakeRepo{
		"insert account": {fail: "insert account"},
		"insert link":    {fail: "insert oidc link"},
		"insert session": {fail: "insert session"},
	} {
		if _, _, err := oidcService(r, login).CreateFromOIDC(ctx, "t", "mira", "Mira", "ua"); !errors.Is(err, errBoom) {
			t.Errorf("create failing at %s = %v", call, err)
		}
	}
	hash, _ := service(&fakeRepo{}).Passwords.Hash("0123456789")
	for call, r := range map[string]*fakeRepo{
		"use pending":    {fail: "use pending", hash: hash},
		"insert link":    {fail: "insert oidc link", hash: hash},
		"set admin":      {fail: "set admin", hash: hash},
		"insert session": {fail: "insert session", hash: hash},
	} {
		_, _, err := oidcService(r, login).LinkFromOIDC(ctx, "t", "aria", "0123456789", "ua")
		if want := map[bool]error{true: domain.ErrExpired, false: errBoom}[call == "use pending"]; !errors.Is(err, want) {
			t.Errorf("link failing at %s = %v", call, err)
		}
	}
	if _, _, err := oidcService(&fakeRepo{hash: hash, account: domain.Account{Disabled: true}}, login).LinkFromOIDC(ctx, "t", "aria", "0123456789", "ua"); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Errorf("a disabled Account links = %v", err)
	}
	if _, _, err := oidcService(&fakeRepo{}, login).LinkFromOIDC(ctx, "t", "nobody", "x", "ua"); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Errorf("an unknown Username links = %v", err)
	}
	for call, r := range map[string]*fakeRepo{
		"by subject":   {fail: "by subject"},
		"has password": {fail: "has password"},
		"delete link":  {fail: "delete link", hash: hash},
	} {
		if err := oidcService(r, login).Unlink(ctx, "s"); !errors.Is(err, errBoom) {
			t.Errorf("unlink failing at %s = %v", call, err)
		}
	}
	for call, r := range map[string]*fakeRepo{"has password": {fail: "has password"}, "link of": {fail: "link of"}, "by subject": {fail: "by subject"}} {
		if _, err := service(r).Me(ctx, "s"); !errors.Is(err, errBoom) {
			t.Errorf("me failing at %s = %v", call, err)
		}
	}
	for call, r := range map[string]*fakeRepo{"by subject": {fail: "by subject"}, "update profile": {fail: "update profile"}} {
		if _, err := service(r).UpdateProfile(ctx, "s", domain.ProfileChange{Username: "aria", Nickname: "Aria", Email: "a@b.c"}); !errors.Is(err, errBoom) {
			t.Errorf("update profile failing at %s = %v", call, err)
		}
	}
	if !oidcService(&fakeRepo{}, login).OIDCEnabled() || service(&fakeRepo{}).OIDCEnabled() {
		t.Error("OIDCEnabled")
	}
}
