package app

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238's default, the one every authenticator app expects
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
)

// TwoStepRepository persists authenticator apps, recovery codes and sign-ins waiting for a second step.
type TwoStepRepository interface {
	StartTOTP(ctx context.Context, account domain.AccountID, secret string, now time.Time) (bool, error)
	TOTPFactor(ctx context.Context, account domain.AccountID) (string, bool, error)
	ConfirmTOTP(ctx context.Context, account domain.AccountID, step int64, now time.Time) (bool, error)
	UseTOTPStep(ctx context.Context, account domain.AccountID, step int64) (bool, error)
	DeleteTOTP(ctx context.Context, account domain.AccountID) error
	ReplaceRecoveryCodes(ctx context.Context, account domain.AccountID, hashes [][]byte) error
	UseRecoveryCode(ctx context.Context, account domain.AccountID, hash []byte, now time.Time) (bool, error)
	RecoveryCodesLeft(ctx context.Context, account domain.AccountID) (int, error)
	InsertChallenge(ctx context.Context, tokenHash []byte, account domain.AccountID, now, expires time.Time) error
	TryChallenge(ctx context.Context, tokenHash []byte, now time.Time, maxAttempts int) (domain.AccountID, error)
	UseChallenge(ctx context.Context, tokenHash []byte, now time.Time) error
	StrengthenSession(ctx context.Context, tokenHash []byte, account domain.AccountID) error
}

// Two-step limits: how long a sign-in waits for its code, how many wrong codes it takes, and how many
// recovery codes an Account holds.
const (
	ChallengeTTL      = 5 * time.Minute
	ChallengeAttempts = 5
	RecoveryCodes     = 10
)

// totpStep is RFC 6238's 30-second time step.
const totpStep = 30

// totpAt is the six-digit code for a time step (RFC 6238 with SHA-1).
func totpAt(secret []byte, step int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step)) //nolint:gosec // time steps are positive
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	code := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", code%1_000_000)
}

// TOTP is the code an authenticator app shows for a secret at a moment.
func TOTP(secret string, at time.Time) string {
	key, err := totpEncoding.DecodeString(secret)
	if err != nil {
		return ""
	}
	return totpAt(key, at.Unix()/totpStep)
}

// matchTOTP is the time step a code belongs to, allowing one step of clock drift either way; zero when
// it matches none.
func matchTOTP(secret, code string, now time.Time) int64 {
	key, err := totpEncoding.DecodeString(secret)
	if err != nil {
		return 0
	}
	step := now.Unix() / totpStep
	for _, s := range []int64{step - 1, step, step + 1} {
		if hmac.Equal([]byte(totpAt(key, s)), []byte(code)) {
			return s
		}
	}
	return 0
}

var totpEncoding = base32.StdEncoding.WithPadding(base32.NoPadding) //nolint:gochecknoglobals // an immutable codec

// recoveryEncoding writes recovery codes in lowercase letters and digits that are hard to misread.
var recoveryEncoding = base32.NewEncoding("abcdefghjkmnpqrstuvwxyz123456789").WithPadding(base32.NoPadding) //nolint:gochecknoglobals // an immutable codec

// normalCode is a recovery code as typed, without case, spaces or dashes.
func normalCode(code string) string {
	return strings.NewReplacer("-", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(code)))
}

func isTOTPCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// firstStep finishes a sign-in whose first step passed: a session, or a challenge when two-step is on.
func (s *Service) firstStep(ctx context.Context, a domain.Account, userAgent string) (domain.SignedIn, error) {
	out := domain.SignedIn{Account: a, Session: "", Challenge: ""}
	_, confirmed, err := s.Repo.TOTPFactor(ctx, a.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return out, err
	}
	now := s.Now()
	if !confirmed {
		out.Session, err = startSession(ctx, s.Repo, a.ID, userAgent, now, false)
		return out, err
	}
	token, hash, err := newToken()
	if err != nil {
		return out, err
	}
	out.Challenge = token
	return out, s.Repo.InsertChallenge(ctx, hash, a.ID, now, now.Add(ChallengeTTL))
}

// PassTwoStep answers a sign-in's challenge with a code from the authenticator app or a recovery code,
// and signs in with a strong session. A challenge takes a few wrong codes, then expires.
func (s *Service) PassTwoStep(ctx context.Context, challenge, code, userAgent string) (domain.Account, string, error) {
	now := s.Now()
	hash := HashToken(challenge)
	id, err := s.Repo.TryChallenge(ctx, hash, now, ChallengeAttempts)
	if err != nil {
		return domain.Account{}, "", domain.ErrExpired
	}
	ok, err := s.checkCode(ctx, id, code, now)
	if err != nil {
		return domain.Account{}, "", err
	}
	if !ok {
		return domain.Account{}, "", domain.ErrUnauthenticated
	}
	if err := s.Repo.UseChallenge(ctx, hash, now); err != nil {
		return domain.Account{}, "", err
	}
	a, err := s.Repo.AccountByID(ctx, id)
	if err != nil {
		return domain.Account{}, "", err
	}
	if a.Disabled {
		return domain.Account{}, "", domain.ErrUnauthenticated
	}
	session, err := startSession(ctx, s.Repo, id, userAgent, now, true)
	return a, session, err
}

// checkCode spends a code: a six-digit one from the confirmed authenticator app, never twice, or an
// unused recovery code.
func (s *Service) checkCode(ctx context.Context, id domain.AccountID, code string, now time.Time) (bool, error) {
	code = normalCode(code)
	if !isTOTPCode(code) {
		return s.Repo.UseRecoveryCode(ctx, id, HashToken(code), now)
	}
	secret, confirmed, err := s.Repo.TOTPFactor(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	if err != nil || !confirmed {
		return false, err
	}
	step := matchTOTP(secret, code, now)
	if step == 0 {
		return false, nil
	}
	return s.Repo.UseTOTPStep(ctx, id, step)
}

// TwoStepSetup is a new authenticator secret, and the otpauth URI an app scans to add it.
type TwoStepSetup struct {
	Secret string
	URI    string
}

// BeginTwoStep starts adding an authenticator app to the signed-in Account; a second call replaces an
// unconfirmed secret, and an Account with two-step on must turn it off first.
func (s *Service) BeginTwoStep(ctx context.Context, subject string) (TwoStepSetup, error) {
	a, err := s.Repo.AccountBySubject(ctx, subject)
	if err != nil {
		return TwoStepSetup{}, err
	}
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return TwoStepSetup{}, err
	}
	secret := totpEncoding.EncodeToString(raw)
	started, err := s.Repo.StartTOTP(ctx, a.ID, secret, s.Now())
	if err != nil {
		return TwoStepSetup{}, err
	}
	if !started {
		return TwoStepSetup{}, domain.ErrConflict
	}
	q := url.Values{"secret": {secret}, "issuer": {"Grimoire"}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	return TwoStepSetup{Secret: secret, URI: "otpauth://totp/Grimoire:" + url.PathEscape(a.Username) + "?" + q.Encode()}, nil
}

// ConfirmTwoStep turns two-step on with a first code from the app and returns the recovery codes,
// shown only now. The session that confirmed it becomes strong.
func (s *Service) ConfirmTwoStep(ctx context.Context, subject, code, session string) ([]string, error) {
	a, err := s.Repo.AccountBySubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	secret, confirmed, err := s.Repo.TOTPFactor(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	if confirmed {
		return nil, domain.ErrConflict
	}
	now := s.Now()
	step := matchTOTP(secret, normalCode(code), now)
	if step == 0 {
		return nil, domain.ErrUnauthenticated
	}
	var codes []string
	err = s.Repo.InTx(ctx, func(r Repository) error {
		if ok, err := r.ConfirmTOTP(ctx, a.ID, step, now); err != nil || !ok {
			return firstErr(err, domain.ErrConflict)
		}
		if codes, err = newRecoveryCodes(ctx, r, a.ID); err != nil {
			return err
		}
		if err := s.record(ctx, r, a.ID, subject, domain.EventTwoStepOn, ""); err != nil {
			return err
		}
		if session == "" {
			return nil
		}
		return r.StrengthenSession(ctx, HashToken(session), a.ID)
	})
	return codes, err
}

// DisableTwoStep turns two-step off with a current code; its recovery codes go with it.
func (s *Service) DisableTwoStep(ctx context.Context, subject, code string) error {
	a, err := s.withCode(ctx, subject, code)
	if err != nil {
		return err
	}
	if err := s.Repo.DeleteTOTP(ctx, a.ID); err != nil {
		return err
	}
	return s.record(ctx, s.Repo, a.ID, subject, domain.EventTwoStepOff, "")
}

// ResetRecoveryCodes replaces the recovery codes, with a current code, and returns the new ones.
func (s *Service) ResetRecoveryCodes(ctx context.Context, subject, code string) ([]string, error) {
	a, err := s.withCode(ctx, subject, code)
	if err != nil {
		return nil, err
	}
	return newRecoveryCodes(ctx, s.Repo, a.ID)
}

func (s *Service) withCode(ctx context.Context, subject, code string) (domain.Account, error) {
	a, err := s.Repo.AccountBySubject(ctx, subject)
	if err != nil {
		return a, err
	}
	ok, err := s.checkCode(ctx, a.ID, code, s.Now())
	if err != nil {
		return a, err
	}
	if !ok {
		return a, domain.ErrUnauthenticated
	}
	return a, nil
}

func newRecoveryCodes(ctx context.Context, r Repository, id domain.AccountID) ([]string, error) {
	codes := make([]string, 0, RecoveryCodes)
	hashes := make([][]byte, 0, RecoveryCodes)
	for range RecoveryCodes {
		raw := make([]byte, 7)
		if _, err := rand.Read(raw); err != nil {
			return nil, err
		}
		c := recoveryEncoding.EncodeToString(raw)[:10]
		codes = append(codes, c[:5]+"-"+c[5:])
		hashes = append(hashes, HashToken(c))
	}
	return codes, r.ReplaceRecoveryCodes(ctx, id, hashes)
}

// twoStepOf reports whether two-step is on for an Account, and how many recovery codes it has left.
func (s *Service) twoStepOf(ctx context.Context, id domain.AccountID) (bool, int, error) {
	_, confirmed, err := s.Repo.TOTPFactor(ctx, id)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && !confirmed) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	left, err := s.Repo.RecoveryCodesLeft(ctx, id)
	return true, left, err
}
