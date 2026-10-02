package pgstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
)

// Every read and write reports a lost database instead of answering as if nothing were there.
func TestEveryDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	s := pgstore.New(db.Pool())
	db.Close()
	id, now, hash := uuid.New(), time.Now(), make([]byte, 32)
	ops := map[string]func() error{
		"list accounts":  func() error { _, err := s.ListAccounts(ctx); return err },
		"unused invites": func() error { _, err := s.UnusedInvites(ctx); return err },
		"events":         func() error { _, err := s.Events(ctx, id); return err },
		"revoke all":     func() error { return s.RevokeEverything(ctx, id, now) },
		"live counts":    func() error { _, _, err := s.LiveCounts(ctx, id, now); return err },
		"tokens":         func() error { _, err := s.AccessTokens(ctx, id, now); return err },
		"delete totp":    func() error { return s.DeleteTOTP(ctx, id) },
		"replace codes":  func() error { return s.ReplaceRecoveryCodes(ctx, id, [][]byte{hash}) },
		"by id":          func() error { _, err := s.AccountByID(ctx, id); return err },
		"insert": func() error {
			_, err := s.InsertAccount(ctx, domain.Account{ID: id, Subject: "s", CreatedAt: now}, "")
			return err
		},
		"in tx": func() error { return s.InTx(ctx, func(app.Repository) error { return nil }) },
	}
	for name, op := range ops {
		if op() == nil {
			t.Errorf("%s answered without a database", name)
		}
	}
}
