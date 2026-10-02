package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
)

func TestScopesAreCleaned(t *testing.T) {
	t.Parallel()
	for in, want := range map[string][]string{
		"play read":       {"read", "play"},
		"read read build": {"read", "build"},
		"read admin":      nil,
		"":                nil,
	} {
		if got := cleanScopes(strings.Fields(in)); !slices.Equal(got, want) && (len(got) != 0 || want != nil) {
			t.Errorf("cleanScopes(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestAccessTokenErrorPaths(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for call, r := range map[string]*fakeRepo{"by subject": {fail: "by subject"}, "insert token": {fail: "insert token"}} {
		if _, _, err := service(r).MintToken(ctx, "s", "Agent", []string{"read"}, 7); !errors.Is(err, errBoom) {
			t.Errorf("mint failing at %s = %v", call, err)
		}
	}
	if _, _, err := service(&fakeRepo{}).MintToken(ctx, "s", "Agent", []string{"read"}, MaxTokenDays+1); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("a token for over a year = %v", err)
	}
	for call, r := range map[string]*fakeRepo{"by subject": {fail: "by subject"}, "tokens": {fail: "tokens"}} {
		if _, err := service(r).AccessTokens(ctx, "s"); !errors.Is(err, errBoom) {
			t.Errorf("list failing at %s = %v", call, err)
		}
	}
	for call, r := range map[string]*fakeRepo{"by subject": {fail: "by subject"}, "revoke token": {fail: "revoke token"}} {
		if err := service(r).RevokeToken(ctx, "s", uuid.New()); !errors.Is(err, errBoom) {
			t.Errorf("revoke failing at %s = %v", call, err)
		}
	}
	if _, _, ok := service(&fakeRepo{account: domain.Account{Subject: "s", Disabled: true}}).ResolveToken(ctx, TokenPrefix+"x"); ok {
		t.Error("a disabled Account's token resolves")
	}
	if _, _, ok := service(&fakeRepo{account: domain.Account{Subject: "s"}}).ResolveToken(ctx, "not-ours"); ok {
		t.Error("another service's bearer resolves")
	}
	if subject, scopes, ok := service(&fakeRepo{account: domain.Account{Subject: "s"}}).ResolveToken(ctx, TokenPrefix+"x"); !ok || subject != "s" || scopes[0] != "read" {
		t.Error("a live token does not resolve")
	}
}
