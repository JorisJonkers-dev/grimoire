package pgstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/pgstore"
)

func account(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(), `INSERT INTO identity.accounts (id, subject, username, nickname, email, created_at)
		VALUES ($1, $2, $2, $2, $2 || '@example.org', now())`, id, name); err != nil {
		t.Fatal(err)
	}
	return id
}

// Every Friends operation reports a database fault at any of its calls instead of half-applying.
func TestEveryFriendsDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	pool := db.Pool()
	account(t, pool, "aria")
	bram := account(t, pool, "bram")
	cara := account(t, pool, "cara")
	base := &app.Service{Repo: pgstore.New(pool), Now: time.Now}
	for _, step := range []func() error{
		func() error { return base.Request(ctx, "bram", "aria") },
		func() error { return base.Request(ctx, "cara", "aria") },
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	page, _ := base.Friends(ctx, "aria")
	fromBram, fromCara := page.Incoming[0].ID, page.Incoming[1].ID
	if page.Incoming[0].Person.ID != bram {
		fromBram, fromCara = fromCara, fromBram
	}
	ops := map[string]func(s *app.Service) error{
		"friends": func(s *app.Service) error { _, err := s.Friends(ctx, "aria"); return err },
		"request": func(s *app.Service) error { return s.Request(ctx, "aria", "cara") },
		"accept":  func(s *app.Service) error { return s.Accept(ctx, "aria", fromBram) },
		"unfriend": func(s *app.Service) error {
			return s.Unfriend(ctx, "aria", bram)
		},
		"decline": func(s *app.Service) error { return s.Decline(ctx, "aria", fromCara, true) },
		"unblock": func(s *app.Service) error { return s.Unblock(ctx, "aria", cara) },
	}
	for _, name := range []string{"friends", "accept", "unfriend", "decline", "unblock", "request"} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := ops[name](&app.Service{Repo: pgstore.NewFaulty(pool, f), Now: time.Now})
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
	if err := base.Request(ctx, "bram", "cara"); err != nil {
		t.Fatal(err)
	}
	sent, _ := base.Friends(ctx, "bram")
	pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
		err := (&app.Service{Repo: pgstore.NewFaulty(pool, f), Now: time.Now}).Cancel(ctx, "bram", sent.Outgoing[0].ID)
		if err != nil && !errors.Is(err, pgtest.ErrInjected) {
			t.Fatalf("cancel: %v", err)
		}
		return err
	})
}
