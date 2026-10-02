package app

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
)

func TestAdminErrorPaths(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	id := uuid.New()
	for call, r := range map[string]*fakeRepo{"list accounts": {fail: "list accounts"}, "unused invites": {fail: "unused invites"}} {
		if _, _, err := service(r).AdminAccounts(ctx, "root"); !errors.Is(err, errBoom) {
			t.Errorf("list failing at %s = %v", call, err)
		}
	}
	for call, r := range map[string]*fakeRepo{"by id": {fail: "by id"}, "has password": {fail: "has password"}, "live counts": {fail: "live counts"}, "events": {fail: "events"}} {
		if _, err := service(r).AdminAccount(ctx, "root", id); !errors.Is(err, errBoom) {
			t.Errorf("detail failing at %s = %v", call, err)
		}
	}
	controls := map[string]func(*Service) error{
		"link":     func(s *Service) error { return s.SendSignInLink(ctx, "root", id) },
		"admin":    func(s *Service) error { return s.SetAdmin(ctx, "root", id, true) },
		"disable":  func(s *Service) error { return s.SetDisabled(ctx, "root", id, true) },
		"two-step": func(s *Service) error { return s.ResetTwoStep(ctx, "root", id) },
	}
	for name, run := range controls {
		if err := run(service(&fakeRepo{})); name != "link" && err != nil {
			t.Errorf("%s = %v", name, err)
		}
		if err := run(service(&fakeRepo{fail: "by id"})); !errors.Is(err, errBoom) {
			t.Errorf("%s without the Account = %v", name, err)
		}
		if err := run(service(&fakeRepo{fail: "insert event"})); !errors.Is(err, errBoom) && name != "link" {
			t.Errorf("%s failing to record = %v", name, err)
		}
		s := service(&fakeRepo{})
		s.Admins = map[string]bool{}
		if err := run(s); !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("%s by a non-Admin = %v", name, err)
		}
	}
	for call, r := range map[string]*fakeRepo{"set disabled": {fail: "set disabled"}, "revoke everything": {fail: "revoke everything"}} {
		if err := service(r).SetDisabled(ctx, "root", id, true); !errors.Is(err, errBoom) {
			t.Errorf("disable failing at %s = %v", call, err)
		}
	}
	if err := service(&fakeRepo{fail: "set disabled"}).SetDisabled(ctx, "root", id, false); !errors.Is(err, errBoom) {
		t.Errorf("enable failing = %v", err)
	}
	if err := service(&fakeRepo{fail: "set admin"}).SetAdmin(ctx, "root", id, true); !errors.Is(err, errBoom) {
		t.Errorf("make Admin failing = %v", err)
	}
	if err := service(&fakeRepo{fail: "delete totp"}).ResetTwoStep(ctx, "root", id); !errors.Is(err, errBoom) {
		t.Errorf("reset failing = %v", err)
	}
	for call, r := range map[string]*fakeRepo{"by subject": {fail: "by subject"}, "events": {fail: "events"}} {
		if _, err := service(r).History(ctx, "s"); !errors.Is(err, errBoom) {
			t.Errorf("history failing at %s = %v", call, err)
		}
	}
	if err := service(&fakeRepo{fail: "insert event"}).SetPassword(ctx, "s", "0123456789"); !errors.Is(err, errBoom) {
		t.Errorf("recording a password change failing = %v", err)
	}
}
