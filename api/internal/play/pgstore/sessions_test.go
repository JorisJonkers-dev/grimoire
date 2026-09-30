package pgstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

type closed struct{ ids []domain.SessionID }

func (c *closed) Close(id domain.SessionID) { c.ids = append(c.ids, id) }

func sessions(tb table, repo app.SessionRepository, c *closed) *app.Sessions {
	return &app.Sessions{Repo: repo, Members: pgstore.CampaignMembers{Store: campaignpg.New(tb.pool)}, Live: c, Now: time.Now}
}

func TestSessionsStartListEnd(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	c := &closed{}
	s := sessions(tb, pgstore.New(tb.pool), c)
	one, err := s.Start(ctx, dm, tb.campaign)
	if err != nil || one.Number != 1 || one.Status != domain.SessionLive || one.GridRadius != 10 {
		t.Fatalf("start = %+v %v", one, err)
	}
	two, _ := s.Start(ctx, dm, tb.campaign)
	if two.Number != 2 {
		t.Fatalf("second = %+v", two)
	}
	list, err := s.List(ctx, player, tb.campaign)
	if err != nil || len(list) != 2 || list[0].Number != 2 {
		t.Fatalf("list = %+v %v", list, err)
	}
	if got, err := s.Get(ctx, player, tb.campaign, one.ID); err != nil || got.ID != one.ID {
		t.Fatalf("get = %+v %v", got, err)
	}
	ended, err := s.End(ctx, dm, tb.campaign, one.ID)
	if err != nil || ended.Status != domain.SessionEnded || len(c.ids) != 1 {
		t.Fatalf("end = %+v %v", ended, err)
	}
	if _, err := s.End(ctx, dm, tb.campaign, one.ID); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("end twice = %v", err)
	}
	checkSessionRefusals(t, s, tb, one.ID, two.ID)
}

func checkSessionRefusals(t *testing.T, s *app.Sessions, tb table, one, two domain.SessionID) {
	t.Helper()
	ctx := context.Background()
	refused := map[string]error{}
	_, refused["player start"] = s.Start(ctx, player, tb.campaign)
	_, refused["player end"] = s.End(ctx, player, tb.campaign, two)
	for name, err := range refused {
		if !errors.Is(err, apperr.ErrForbidden) {
			t.Errorf("%s: %v", name, err)
		}
	}
	missing := map[string]error{}
	_, missing["stranger start"] = s.Start(ctx, stranger, tb.campaign)
	_, missing["stranger get"] = s.Get(ctx, stranger, tb.campaign, one)
	_, missing["stranger list"] = s.List(ctx, stranger, tb.campaign)
	_, missing["unknown"] = s.Get(ctx, dm, tb.campaign, domain.SessionID(uuid.New()))
	_, _, missing["load"] = pgstore.New(tb.pool).Load(ctx, domain.SessionID(uuid.New()))
	for name, err := range missing {
		if !errors.Is(err, apperr.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestEverySessionDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	base := sessions(tb, pgstore.New(tb.pool), &closed{})
	live1, _ := base.Start(ctx, dm, tb.campaign)
	store := pgstore.New(tb.pool)
	seq, tok, err := store.Apply(ctx, live1, live.Change{Kind: domain.ActionTokenPlaced, Token: domain.Token{Label: "A", Kind: domain.TokenEnemy}}, tb.dmMember(t), dm, time.Now())
	if err != nil || seq != 1 {
		t.Fatalf("apply = %d %v", seq, err)
	}
	gone, err := pgxpool.New(ctx, tb.pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	gone.Close()
	if _, err := (pgstore.Owner{Pool: gone}).Acquire(ctx, live1.ID); err == nil {
		t.Fatal("acquire on a closed pool")
	}
	ops := map[string]func(s *app.Sessions, repo *pgstore.Store) error{
		"start": func(s *app.Sessions, _ *pgstore.Store) error { _, err := s.Start(ctx, dm, tb.campaign); return err },
		"list":  func(s *app.Sessions, _ *pgstore.Store) error { _, err := s.List(ctx, dm, tb.campaign); return err },
		"end": func(s *app.Sessions, _ *pgstore.Store) error {
			fresh, err := base.Start(ctx, dm, tb.campaign)
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.End(ctx, dm, tb.campaign, fresh.ID)
			return err
		},
		"load": func(_ *app.Sessions, repo *pgstore.Store) error { _, _, err := repo.Load(ctx, live1.ID); return err },
		"apply": func(_ *app.Sessions, repo *pgstore.Store) error {
			_, _, err := repo.Apply(ctx, live1, live.Change{Kind: domain.ActionTokenMoved, Token: tok}, tb.dmMember(t), dm, time.Now())
			return err
		},
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			repo := pgstore.NewFaulty(tb.pool, f)
			err := op(sessions(tb, repo, &closed{}), repo)
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}
