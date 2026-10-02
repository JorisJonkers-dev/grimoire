package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
)

// RFC 6238's SHA-1 test vectors, cut to six digits.
func TestTOTPVectors(t *testing.T) {
	t.Parallel()
	secret := totpEncoding.EncodeToString([]byte("12345678901234567890"))
	for at, want := range map[int64]string{59: "287082", 1111111109: "081804", 1111111111: "050471", 1234567890: "005924", 2000000000: "279037"} {
		if got := TOTP(secret, time.Unix(at, 0)); got != want {
			t.Errorf("TOTP at %d = %s, want %s", at, got, want)
		}
	}
	now := time.Unix(1111111111, 0)
	if matchTOTP(secret, TOTP(secret, now.Add(-30*time.Second)), now) == 0 || matchTOTP(secret, TOTP(secret, now.Add(30*time.Second)), now) == 0 {
		t.Error("one step of drift either way is allowed")
	}
	if matchTOTP(secret, TOTP(secret, now.Add(-61*time.Second)), now) != 0 {
		t.Error("two steps back is too far")
	}
	if TOTP("not base32!", now) != "" || matchTOTP("not base32!", "123456", now) != 0 {
		t.Error("a broken secret matches nothing")
	}
	for code, want := range map[string]bool{"123456": true, "12345": false, "12345a": false, "abcde-fghij": false} {
		if isTOTPCode(code) != want {
			t.Errorf("isTOTPCode(%q)", code)
		}
	}
	if normalCode(" AbCdE-fGhIj ") != "abcdefghij" {
		t.Error("normalCode")
	}
}

func TestTwoStepErrorPaths(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	secret := totpEncoding.EncodeToString([]byte("12345678901234567890"))
	on := func(fail string) *fakeRepo { return &fakeRepo{fail: fail, secret: secret, confirmed: true} }
	hash, _ := service(&fakeRepo{}).Passwords.Hash("0123456789")
	for call, r := range map[string]*fakeRepo{
		"totp":             {fail: "totp", hash: hash},
		"insert challenge": {fail: "insert challenge", hash: hash, secret: secret, confirmed: true},
	} {
		if _, err := service(r).SignIn(ctx, "aria", "0123456789", "ua"); !errors.Is(err, errBoom) {
			t.Errorf("sign in failing at %s = %v", call, err)
		}
	}
	code := func(s *Service) string { return TOTP(secret, s.Now()) }
	for call, r := range map[string]*fakeRepo{"use step": on("use step"), "use challenge": on("use challenge"), "by id": on("by id"), "insert session": on("insert session")} {
		s := service(r)
		if _, _, err := s.PassTwoStep(ctx, "c", code(s), "ua"); !errors.Is(err, errBoom) {
			t.Errorf("pass failing at %s = %v", call, err)
		}
	}
	disabled := on("")
	disabled.account.Disabled = true
	if s := service(disabled); true {
		if _, _, err := s.PassTwoStep(ctx, "c", code(s), "ua"); !errors.Is(err, domain.ErrUnauthenticated) {
			t.Errorf("a disabled Account passes = %v", err)
		}
	}
	if _, _, err := service(&fakeRepo{}).PassTwoStep(ctx, "c", "123456", "ua"); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Errorf("a code without an app = %v", err)
	}
	if _, _, err := service(&fakeRepo{secret: secret}).PassTwoStep(ctx, "c", "123456", "ua"); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Errorf("a code for an unconfirmed app = %v", err)
	}
	if _, _, err := service(on("use code")).PassTwoStep(ctx, "c", "abcde-fghij", "ua"); !errors.Is(err, errBoom) {
		t.Errorf("a recovery code failing = %v", err)
	}
	for call, r := range map[string]*fakeRepo{"by subject": {fail: "by subject"}, "start totp": {fail: "start totp"}} {
		if _, err := service(r).BeginTwoStep(ctx, "s"); !errors.Is(err, errBoom) {
			t.Errorf("begin failing at %s = %v", call, err)
		}
	}
	pending := func(fail string) *fakeRepo { return &fakeRepo{fail: fail, secret: secret} }
	for call, r := range map[string]*fakeRepo{"by subject": pending("by subject"), "totp": pending("totp"), "confirm totp": pending("confirm totp"), "replace codes": pending("replace codes"), "strengthen": pending("strengthen")} {
		s := service(r)
		if _, err := s.ConfirmTwoStep(ctx, "s", code(s), "session"); !errors.Is(err, errBoom) {
			t.Errorf("confirm failing at %s = %v", call, err)
		}
	}
	if s := service(on("")); true {
		if _, err := s.ConfirmTwoStep(ctx, "s", code(s), ""); !errors.Is(err, domain.ErrConflict) {
			t.Errorf("confirm while on = %v", err)
		}
	}
	if s := service(pending("")); true {
		if codes, err := s.ConfirmTwoStep(ctx, "s", code(s), ""); err != nil || len(codes) != RecoveryCodes {
			t.Errorf("confirm from forward-auth = %v %v", codes, err)
		}
	}
	for call, r := range map[string]*fakeRepo{"by subject": on("by subject"), "totp": on("totp"), "delete totp": on("delete totp")} {
		s := service(r)
		if err := s.DisableTwoStep(ctx, "s", code(s)); !errors.Is(err, errBoom) {
			t.Errorf("disable failing at %s = %v", call, err)
		}
	}
	if s := service(on("replace codes")); true {
		if _, err := s.ResetRecoveryCodes(ctx, "s", code(s)); !errors.Is(err, errBoom) {
			t.Errorf("reset failing = %v", err)
		}
	}
	if _, err := service(on("")).ResetRecoveryCodes(ctx, "s", "000000"); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Errorf("reset with a wrong code = %v", err)
	}
	for call, r := range map[string]*fakeRepo{"totp": {fail: "totp"}, "codes left": on("codes left")} {
		if _, err := service(r).Me(ctx, "s"); !errors.Is(err, errBoom) {
			t.Errorf("me failing at %s = %v", call, err)
		}
	}
	strong := service(&fakeRepo{account: domain.Account{ID: uuid.New(), Admin: true}})
	if strong.IsAdmin(ctx, "s") {
		t.Error("an Admin without a strong session holds powers")
	}
	strong.Strong = func(context.Context) bool { return true }
	if !strong.IsAdmin(ctx, "s") {
		t.Error("an Admin with a strong session lacks powers")
	}
}
