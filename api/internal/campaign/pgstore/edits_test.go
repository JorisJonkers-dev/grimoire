package pgstore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func TestEditsFeedTheActivityAndPlanUndos(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := open(t)
	repo := pgstore.New(db.Pool())
	n, d := npcs(t, repo)
	s := app.NewService(repo)
	agent := caller.MCP(dmCaller.Subject, "prep-agent")
	a, _ := n.Create(ctx, agent, d.ID, morvain())
	_, _ = n.Update(ctx, agent, d.ID, a.ID, app.NPCInput{Name: "Morvain", Disposition: "neutral"})
	_, _ = n.Create(ctx, dmCaller, d.ID, morvain())

	feed, err := s.Activity(ctx, dmCaller, d.ID)
	if err != nil || len(feed) != 2 || feed[0].No != 2 || !feed[0].Latest || feed[1].Latest || feed[0].Client != "prep-agent" || feed[0].Name != "Morvain" {
		t.Fatalf("activity %+v, %v", feed, err)
	}
	if _, _, err := s.UndoPlan(ctx, dmCaller, d.ID, feed[1].RevisionID); err == nil || err.Error() != "Undo the later changes to Morvain first." {
		t.Fatalf("older change: %v", err)
	}
	if e, no, err := s.UndoPlan(ctx, dmCaller, d.ID, feed[0].RevisionID); err != nil || no != 1 || e.EntityID != uuid.UUID(a.ID) {
		t.Fatalf("plan %+v %d %v", e, no, err)
	}
	latest, err := s.LatestEdit(ctx, dmCaller, d.ID, domain.EntityNPC, uuid.UUID(a.ID))
	if err != nil || latest.RevisionID != feed[0].RevisionID {
		t.Fatalf("latest %+v %v", latest, err)
	}
	if _, err := s.LatestEdit(ctx, dmCaller, d.ID, domain.EntityNPC, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("latest of nothing: %v", err)
	}
	for name, op := range map[string]func() error{
		"activity": func() error { _, err := s.Activity(ctx, playerCaller, d.ID); return err },
		"latest": func() error {
			_, err := s.LatestEdit(ctx, playerCaller, d.ID, domain.EntityNPC, uuid.UUID(a.ID))
			return err
		},
		"undo": func() error { _, _, err := s.UndoPlan(ctx, playerCaller, d.ID, feed[0].RevisionID); return err },
	} {
		if err := op(); !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("%s for a player: %v", name, err)
		}
	}
	for name, op := range map[string]func(s *app.Service) error{
		"activity": func(s *app.Service) error { _, err := s.Activity(ctx, dmCaller, d.ID); return err },
		"undo": func(s *app.Service) error {
			_, _, err := s.UndoPlan(ctx, dmCaller, d.ID, feed[0].RevisionID)
			return err
		},
	} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(app.NewService(pgstore.NewFaulty(db.Pool(), f)))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}
