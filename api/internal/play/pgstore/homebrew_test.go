package pgstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	libraryapp "github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	librarydomain "github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	librarypg "github.com/JorisJonkers-dev/grimoire/api/internal/library/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
)

// A Campaign's homebrew spells build into Effects for play; a design the rules no longer run is left
// out rather than stopping the Session.
func TestHomebrewSpellsBuildForPlay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	store := pgstore.New(tb.pool)
	lib := &libraryapp.Service{
		Repo: librarypg.New(tb.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(tb.pool)}, Now: time.Now, Surfaces: store.SurfaceKinds,
	}
	design := spellbuild.Design{
		Targeting: spellbuild.Targeting{Shape: "sphere", SizeFt: 10, RangeFt: 30}, Save: "dexterity", Duration: spellbuild.Duration{Unit: "rounds", Amount: 2},
		CastingTime: spellbuild.CastingTime{Kind: "action"}, Components: spellbuild.Components{Verbal: true},
		Parts: []spellbuild.Part{{Type: "damage", When: "start_of_turn", Dice: "1d4", DamageType: "acid"}},
	}
	var ids []string
	for _, name := range []string{"Acid Pool", "Broken", "Odd"} {
		e, err := lib.Create(ctx, dm, librarydomain.Draft{Kind: "spell", Name: name})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := lib.SaveSpell(ctx, dm, e.ID, design); err != nil {
			t.Fatal(err)
		}
		if _, err := lib.Link(ctx, dm, tb.campaign, e.ID); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, e.ID.String())
	}
	for id, design := range map[string]string{ids[1]: `{"targeting":{"shape":"blob","sizeFt":5}}`, ids[2]: `{"parts":"many"}`} {
		if _, err := tb.pool.Exec(ctx, "UPDATE library.entries SET design = $2 WHERE id = $1", id, design); err != nil {
			t.Fatal(err)
		}
	}
	brewed, err := store.Homebrew(ctx, tb.campaign)
	if err != nil || len(brewed) != 1 || brewed[0].Definition.Slug != spellbuild.Slug(ids[0]) || brewed[0].Surface == nil {
		t.Fatalf("homebrew = %+v %v", brewed, err)
	}
	pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
		_, err := pgstore.NewFaulty(tb.pool, f).Homebrew(ctx, tb.campaign)
		if err != nil && !errors.Is(err, pgtest.ErrInjected) {
			t.Fatal(err)
		}
		return err
	})
}
