package pgstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/library/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	playpg "github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Every Library operation reports a database fault at any of its calls instead of half-applying.
func TestEveryLibraryDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	pool := db.Pool()
	dm := caller.UI("dm")
	camp, err := campaignapp.NewService(campaignpg.New(pool)).Create(ctx, dm, campaignapp.CreateInput{Name: "Morvain", DisplayName: "Joris"})
	if err != nil {
		t.Fatal(err)
	}
	campaign := uuid.UUID(camp.ID)
	members := playpg.CampaignMembers{Store: campaignpg.New(pool)}
	service := func(repo app.Repository) *app.Service {
		return &app.Service{Repo: repo, Members: members, Now: time.Now}
	}
	base := service(pgstore.New(pool))
	draft := domain.Draft{Kind: "npc", Name: "Odo", Fields: domain.Fields{"Mood": "cheery"}}
	fresh := func() uuid.UUID {
		t.Helper()
		e, err := base.Create(ctx, dm, draft)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := base.Link(ctx, dm, campaign, e.ID); err != nil {
			t.Fatal(err)
		}
		return e.ID
	}
	one := 1
	ops := map[string]func(s *app.Service) error{
		"entries": func(s *app.Service) error { _, err := s.Entries(ctx, dm, "npc"); return err },
		"create":  func(s *app.Service) error { _, err := s.Create(ctx, dm, draft); return err },
		"get":     func(s *app.Service) error { _, err := s.Get(ctx, dm, fresh()); return err },
		"update":  func(s *app.Service) error { _, err := s.Update(ctx, dm, fresh(), draft); return err },
		"linked":  func(s *app.Service) error { _, err := s.Linked(ctx, dm, campaign); return err },
		"link": func(s *app.Service) error {
			e, err := base.Create(ctx, dm, draft)
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.Link(ctx, dm, campaign, e.ID)
			return err
		},
		"unlink": func(s *app.Service) error { return s.Unlink(ctx, dm, campaign, fresh()) },
		"override": func(s *app.Service) error {
			_, err := s.Override(ctx, dm, campaign, fresh(), domain.Fields{"Mood": "grim"})
			return err
		},
		"pin": func(s *app.Service) error { _, err := s.Pin(ctx, dm, campaign, fresh(), &one); return err },
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(service(pgstore.NewFaulty(pool, f)))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}
