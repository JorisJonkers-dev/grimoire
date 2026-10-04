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
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/itembuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
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
		return &app.Service{Repo: repo, Members: members, Now: time.Now, Admins: everyone{}, Surfaces: playpg.New(pool).SurfaceKinds}
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
		"restore": func(s *app.Service) error { _, err := s.Restore(ctx, dm, fresh(), 1); return err },
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
		"pin":         func(s *app.Service) error { _, err := s.Pin(ctx, dm, campaign, fresh(), &one); return err },
		"collections": func(s *app.Service) error { _, err := s.Collections(ctx, dm); return err },
		"collect": func(s *app.Service) error {
			col, err := s.CreateCollection(ctx, dm, "Fey", "")
			if err == nil {
				_, err = s.UpdateCollection(ctx, dm, col.ID, "Fey", "Fey magic", []uuid.UUID{fresh(), fresh()})
			}
			return err
		},
		"campaign collections": func(s *app.Service) error { _, err := s.CampaignCollections(ctx, dm, campaign); return err },
		"switch": func(s *app.Service) error {
			col, err := base.CreateCollection(ctx, dm, "Fey", "")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Switch(ctx, dm, campaign, col.ID, true); err != nil {
				return err
			}
			_, err = s.Switch(ctx, dm, campaign, col.ID, false)
			return err
		},
		"propose": func(s *app.Service) error {
			_, err := s.Propose(ctx, dm, campaign, draft, "note", nil)
			return err
		},
		"proposals": func(s *app.Service) error { _, err := s.Proposals(ctx, dm, campaign); return err },
		"proposal": func(s *app.Service) error {
			e := fresh()
			p, err := base.Propose(ctx, dm, campaign, draft, "", &e)
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.Proposal(ctx, dm, campaign, p.ID)
			return err
		},
		"review": func(s *app.Service) error {
			first, err := base.Propose(ctx, dm, campaign, draft, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Review(ctx, dm, campaign, first.ID, app.RequestChanges, "more", nil); err != nil {
				return err
			}
			if _, err := s.Resubmit(ctx, dm, campaign, first.ID, draft, "done"); err != nil {
				return err
			}
			if _, err := s.Review(ctx, dm, campaign, first.ID, app.Approve, "", nil); err != nil {
				return err
			}
			e := fresh()
			change, err := base.Propose(ctx, dm, campaign, draft, "", &e)
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.Review(ctx, dm, campaign, change.ID, app.Approve, "", nil)
			return err
		},
		"shared": func(s *app.Service) error { _, err := s.Shared(ctx, "npc"); return err },
		"share": func(s *app.Service) error {
			if _, err := s.Share(ctx, dm, fresh(), "mine"); err != nil {
				return err
			}
			_, err := s.Submissions(ctx, dm)
			return err
		},
		"review share": func(s *app.Service) error {
			x, err := base.Share(ctx, dm, fresh(), "")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.AllSubmissions(ctx, dm); err != nil {
				return err
			}
			_, err = s.ReviewSubmission(ctx, dm, x.ID, true, true, "own words", "thanks")
			return err
		},
		"export": func(s *app.Service) error {
			if _, err := s.Export(ctx, dm, nil, nil); err != nil {
				return err
			}
			col, err := base.CreateCollection(ctx, dm, "Out", "")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Export(ctx, dm, &col.ID, nil); err != nil {
				return err
			}
			e := fresh()
			_, err = s.Export(ctx, dm, nil, &e)
			return err
		},
		"import": func(s *app.Service) error {
			_, err := s.Import(ctx, dm, []domain.Incoming{{Key: "a", Kind: "npc", Name: "Odo"}}, []domain.ExportedCollection{{Name: "In", Entries: []string{"a"}}})
			return err
		},
		"spell build": func(s *app.Service) error {
			e, err := base.Create(ctx, dm, domain.Draft{Kind: "spell", Name: "Glow"})
			if err != nil {
				t.Fatal(err)
			}
			d := spellbuild.Design{
				Targeting: spellbuild.Targeting{Shape: "sphere", SizeFt: 10, RangeFt: 30}, Duration: spellbuild.Duration{Unit: "instant"},
				CastingTime: spellbuild.CastingTime{Kind: "action"}, Components: spellbuild.Components{Verbal: true},
			}
			if _, err := s.SaveSpell(ctx, dm, e.ID, d); err != nil {
				return err
			}
			_, err = s.Spell(ctx, dm, e.ID)
			return err
		},
		"item build": func(s *app.Service) error {
			e, err := base.Create(ctx, dm, domain.Draft{Kind: "item", Name: "Charm"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.SaveItem(ctx, dm, e.ID, itembuild.Design{Kind: "ring", Rarity: "common", Properties: []itembuild.Property{}}); err != nil {
				return err
			}
			_, err = s.Item(ctx, dm, e.ID)
			return err
		},
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

// everyone is an Admin.
type everyone struct{}

func (everyone) IsAdmin(context.Context, string) bool { return true }
