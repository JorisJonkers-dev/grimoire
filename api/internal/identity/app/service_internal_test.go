package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
)

var errBoom = errors.New("boom")

// fakeRepo answers from memory; fail names the one call that errors.
type fakeRepo struct {
	fail     string
	account  domain.Account
	hash     string
	invite   domain.Invite
	reused   bool
	linkedTo domain.AccountID
	nonce    string
	linkFor  *domain.AccountID
	unlinked bool
}

func (f *fakeRepo) err(call string) error {
	if f.fail == call {
		return errBoom
	}
	return nil
}

func (f *fakeRepo) InsertAccount(_ context.Context, a domain.Account, _ string) (domain.Account, error) {
	return a, f.err("insert account")
}

func (f *fakeRepo) AccountByUsername(context.Context, string) (domain.Account, string, error) {
	return f.account, f.hash, f.err("by username")
}

func (f *fakeRepo) AccountByEmail(context.Context, string) (domain.Account, error) {
	return f.account, f.err("by email")
}

func (f *fakeRepo) AccountBySubject(context.Context, string) (domain.Account, error) {
	return f.account, f.err("by subject")
}

func (f *fakeRepo) AccountByID(context.Context, domain.AccountID) (domain.Account, error) {
	return f.account, f.err("by id")
}

func (f *fakeRepo) SetPassword(context.Context, domain.AccountID, string) error {
	return f.err("set password")
}

func (f *fakeRepo) InsertInvite(context.Context, domain.Invite, []byte) error {
	return f.err("insert invite")
}

func (f *fakeRepo) InviteByToken(context.Context, []byte) (domain.Invite, error) {
	return f.invite, f.err("invite")
}

func (f *fakeRepo) UseInvite(context.Context, uuid.UUID, domain.AccountID, time.Time) (bool, error) {
	return !f.reused, f.err("use invite")
}

func (f *fakeRepo) InsertSession(context.Context, domain.Session, []byte, time.Time) error {
	return f.err("insert session")
}

func (f *fakeRepo) SessionSubject(context.Context, []byte, time.Time) (uuid.UUID, string, bool, error) {
	return uuid.Nil, f.account.Subject, f.account.Disabled, f.err("session")
}

func (f *fakeRepo) TouchSession(context.Context, uuid.UUID, time.Time) error { return nil }

func (f *fakeRepo) RevokeSession(context.Context, []byte, time.Time) error { return nil }

func (f *fakeRepo) InsertSignInLink(context.Context, []byte, domain.AccountID, time.Time, time.Time) error {
	return f.err("insert link")
}

func (f *fakeRepo) UseSignInLink(context.Context, []byte, time.Time) (domain.AccountID, error) {
	return f.linkedTo, f.err("use link")
}

func (f *fakeRepo) HasPassword(context.Context, domain.AccountID) (bool, error) {
	return f.hash != "", f.err("has password")
}

func (f *fakeRepo) UpdateProfile(context.Context, domain.AccountID, domain.ProfileChange) error {
	return f.err("update profile")
}

func (f *fakeRepo) SetAdmin(context.Context, domain.AccountID, bool) error { return f.err("set admin") }

func (f *fakeRepo) InsertOIDCRequest(context.Context, []byte, string, string, *domain.AccountID, time.Time, time.Time) error {
	return f.err("insert request")
}

func (f *fakeRepo) UseOIDCRequest(context.Context, []byte, time.Time) (string, string, *domain.AccountID, error) {
	return f.nonce, "verifier", f.linkFor, f.err("use request")
}

func (f *fakeRepo) LinkOf(context.Context, domain.AccountID) (domain.Link, error) {
	return domain.Link{}, f.err("link of")
}

func (f *fakeRepo) LinkedAccount(context.Context, string, string) (domain.AccountID, error) {
	if f.unlinked {
		return uuid.Nil, domain.ErrNotFound
	}
	return f.account.ID, f.err("linked account")
}

func (f *fakeRepo) InsertLink(context.Context, domain.AccountID, domain.Link) error {
	return f.err("insert oidc link")
}

func (f *fakeRepo) UpdateLink(context.Context, domain.AccountID, domain.Link) error {
	return f.err("update link")
}

func (f *fakeRepo) DeleteLink(context.Context, domain.AccountID) (bool, error) {
	return true, f.err("delete link")
}

func (f *fakeRepo) InsertPending(context.Context, []byte, domain.Claims, bool, time.Time, time.Time) error {
	return f.err("insert pending")
}

func (f *fakeRepo) UsePending(context.Context, []byte, time.Time) (domain.Claims, bool, error) {
	return domain.Claims{Subject: "estate", Email: "a@b.c"}, true, f.err("use pending")
}

func (f *fakeRepo) InTx(_ context.Context, fn func(Repository) error) error { return fn(f) }

type failingMail struct{}

func (failingMail) Send(context.Context, string, string, string) error { return errBoom }

func service(r *fakeRepo) *Service {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	return &Service{Repo: r, Mailer: failingMail{}, Passwords: Passwords{MemoryKiB: 64, Time: 1, Threads: 1}, Now: func() time.Time { return now }, Admins: map[string]bool{"root": true}}
}

func TestServiceErrorPaths(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	open := domain.Invite{ID: uuid.New(), ExpiresAt: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)}
	setup := domain.Setup{Username: "aria", Nickname: "Aria", Email: "a@b.c", Password: "0123456789"}
	if _, _, err := service(&fakeRepo{fail: "insert invite"}).CreateInvite(ctx, "root", 1, false); !errors.Is(err, errBoom) {
		t.Errorf("insert invite = %v", err)
	}
	if _, err := service(&fakeRepo{fail: "invite"}).Invite(ctx, "x"); !errors.Is(err, errBoom) {
		t.Errorf("invite = %v", err)
	}
	for call, r := range map[string]*fakeRepo{
		"invite":         {fail: "invite"},
		"insert account": {fail: "insert account", invite: open},
		"use invite":     {fail: "use invite", invite: open},
		"insert session": {fail: "insert session", invite: open},
	} {
		if _, _, err := service(r).Accept(ctx, "x", setup, "ua"); !errors.Is(err, errBoom) {
			t.Errorf("accept failing at %s = %v", call, err)
		}
	}
	if _, _, err := service(&fakeRepo{invite: open, reused: true}).Accept(ctx, "x", setup, "ua"); !errors.Is(err, domain.ErrExpired) {
		t.Errorf("an invite another request used = %v", err)
	}
	long := string(make([]byte, 400))
	if _, s, err := service(&fakeRepo{invite: open}).Accept(ctx, "x", setup, long); err != nil || s == "" {
		t.Errorf("a long user agent = %v", err)
	}
	if err := service(&fakeRepo{account: domain.Account{Nickname: "Aria", Email: "a@b.c"}, fail: "insert link"}).RequestLink(ctx, "a@b.c"); !errors.Is(err, errBoom) {
		t.Errorf("insert link = %v", err)
	}
	if err := service(&fakeRepo{account: domain.Account{Nickname: "Aria", Email: "a@b.c"}}).RequestLink(ctx, "a@b.c"); !errors.Is(err, errBoom) {
		t.Errorf("a failing mailer = %v", err)
	}
	if err := service(&fakeRepo{account: domain.Account{Disabled: true}}).RequestLink(ctx, "a@b.c"); err != nil {
		t.Errorf("a disabled Account gets no link = %v", err)
	}
	if _, _, err := service(&fakeRepo{account: domain.Account{Disabled: true}}).UseLink(ctx, "x", "ua"); !errors.Is(err, domain.ErrExpired) {
		t.Errorf("a disabled Account's link = %v", err)
	}
	if _, ok := service(&fakeRepo{account: domain.Account{Subject: "s", Disabled: true}}).Resolve(ctx, "x"); ok {
		t.Error("a disabled Account's session resolves")
	}
	if err := service(&fakeRepo{fail: "by subject"}).SetPassword(ctx, "s", "0123456789"); !errors.Is(err, errBoom) {
		t.Errorf("set password without an Account = %v", err)
	}
	if err := service(&fakeRepo{fail: "set password"}).SetPassword(ctx, "s", "0123456789"); !errors.Is(err, errBoom) {
		t.Errorf("set password failing = %v", err)
	}
	if service(&fakeRepo{account: domain.Account{Admin: true, Disabled: true}}).IsAdmin(ctx, "someone") {
		t.Error("a disabled Admin keeps Admin powers")
	}
	hash, _ := service(&fakeRepo{}).Passwords.Hash("0123456789")
	if _, _, err := service(&fakeRepo{account: domain.Account{Disabled: true}, hash: hash}).SignIn(ctx, "aria", "0123456789", "ua"); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Errorf("a disabled Account signs in = %v", err)
	}
	if !errors.Is(firstErr(errBoom, domain.ErrExpired), errBoom) || !errors.Is(firstErr(nil, domain.ErrExpired), domain.ErrExpired) {
		t.Error("firstErr")
	}
}
