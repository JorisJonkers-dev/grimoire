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
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/conditionbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/itembuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
)

// A Campaign's homebrew spells build into Effects for play; a design the rules no longer run is left
// out rather than stopping the Session. Homebrew conditions and the Campaign's exhaustion come along.
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
	frost, err := lib.Create(ctx, dm, librarydomain.Draft{Kind: "condition", Name: "Frostbite"})
	if err != nil {
		t.Fatal(err)
	}
	cold := conditionbuild.Design{Icon: "snow", Color: "#7fa8dd", Ends: "rest", Stacks: true, MaxLevel: 3, PerLevel: conditionbuild.Penalty{D20: 1}, Parts: []conditionbuild.Part{}}
	if _, err := lib.SaveCondition(ctx, dm, frost.ID, cold); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Link(ctx, dm, tb.campaign, frost.ID); err != nil {
		t.Fatal(err)
	}
	brew, err := store.Homebrew(ctx, tb.campaign)
	if err != nil || len(brew.Spells) != 1 || brew.Spells[0].Definition.Slug != spellbuild.Slug(ids[0]) || brew.Spells[0].Surface == nil {
		t.Fatalf("homebrew = %+v %v", brew, err)
	}
	if len(brew.Conditions) != 1 || brew.Conditions[0].Icon != "snow" || brew.Conditions[0].Definition.Name != "Frostbite" || brew.Exhaustion.DeathAt != 6 {
		t.Fatalf("conditions %+v, exhaustion %+v", brew.Conditions, brew.Exhaustion)
	}
	if _, err := tb.pool.Exec(ctx, "UPDATE campaign.campaigns SET exhaustion_variant = 'grim' WHERE id = $1", tb.campaign); err != nil {
		t.Fatal(err)
	}
	if brew, _ := store.Homebrew(ctx, tb.campaign); brew.Exhaustion.DeathAt != 4 {
		t.Fatalf("grim exhaustion = %+v", brew.Exhaustion)
	}
	pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
		_, err := pgstore.NewFaulty(tb.pool, f).Homebrew(ctx, tb.campaign)
		if err != nil && !errors.Is(err, pgtest.ErrInjected) {
			t.Fatal(err)
		}
		return err
	})
}

// A Campaign's homebrew items read like any other item, by their slug; a design that no longer reads is
// left out.
func TestHomebrewItemsReadInPlay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	store := pgstore.New(tb.pool)
	lib := &libraryapp.Service{Repo: librarypg.New(tb.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(tb.pool)}, Now: time.Now}
	var slugs []string
	for _, name := range []string{"Ashwood Longbow", "Broken"} {
		e, err := lib.Create(ctx, dm, librarydomain.Draft{Kind: "item", Name: name})
		if err != nil {
			t.Fatal(err)
		}
		design := itembuild.Design{
			Kind: "weapon", Rarity: "rare", Attunement: &itembuild.Attunement{Kind: "class", Value: "ranger"}, Charges: &itembuild.Charges{Max: 3, On: "dawn", Dice: 1, Faces: 4},
			Properties: []itembuild.Property{{Type: "spell", Spell: "hunters-mark", Name: "Hunter's Mark", Level: 1, Cost: 1}},
		}
		if _, err := lib.SaveItem(ctx, dm, e.ID, design); err != nil {
			t.Fatal(err)
		}
		if _, err := lib.Link(ctx, dm, tb.campaign, e.ID); err != nil {
			t.Fatal(err)
		}
		slugs = append(slugs, spellbuild.Slug(e.ID.String()))
		if name == "Broken" {
			if _, err := tb.pool.Exec(ctx, "UPDATE library.entries SET design = '{\"properties\":\"x\"}' WHERE id = $1", e.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	items, err := store.Items(ctx, tb.campaign, append(slugs, "hb-000000000000"))
	bow := items[slugs[0]]
	if err != nil || len(items) != 1 || bow.Name != "Ashwood Longbow" || bow.Category != "weapon" || !bow.RequiresAttunement || bow.AttunementDetail != "Requires Attunement by a ranger" ||
		bow.MaxCharges != 3 || bow.RechargeOn != "dawn" || len(bow.Spells) != 1 || bow.Spells[0].Cost != 1 || len(bow.KnownCard) == 0 {
		t.Fatalf("items = %+v %v", items, err)
	}
	if plain, err := store.Items(ctx, tb.campaign, []string{"rope"}); err != nil || len(plain) != 0 {
		t.Fatalf("no homebrew asked for = %v %v", plain, err)
	}
	pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
		_, err := pgstore.NewFaulty(tb.pool, f).Items(ctx, tb.campaign, slugs)
		if err != nil && !errors.Is(err, pgtest.ErrInjected) {
			t.Fatal(err)
		}
		return err
	})
}
