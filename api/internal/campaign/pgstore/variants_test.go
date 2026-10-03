package pgstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/variants"
)

// Every Member sees how the table plays; the DM switches Rule Variants, and only to what each can be.
func TestTheDMSwitchesRuleVariants(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := open(t)
	_, d := npcs(t, pgstore.New(db.Pool()))
	s := &app.RuleVariants{Repo: pgstore.New(db.Pool()), Now: time.Now}
	values := func(who string) map[string]string {
		t.Helper()
		c := dmCaller
		if who == "player" {
			c = playerCaller
		}
		list, err := s.List(ctx, c, d.ID)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, v := range list {
			out[v.Variant.Slug] = v.Value
		}
		if len(list) != len(variants.Catalogue()) || list[0].Variant.Slug != variants.Flanking || list[0].Variant.Name != "Flanking" {
			t.Fatalf("the list = %+v", list)
		}
		return out
	}
	// A new Campaign plays every variant as the rules do without it.
	if got := values("player"); got[variants.Flanking] != variants.Off || got[variants.Rests] != variants.RestsStandard || got[variants.ShortRestCap] != variants.NoCap || got[variants.CriticalHits] != variants.CritDoubleDice {
		t.Fatalf("a new Campaign = %v", got)
	}
	if err := s.Set(ctx, dmCaller, d.ID, variants.Set{variants.Flanking: variants.On, variants.Rests: variants.RestsGritty, variants.ShortRestCap: "2"}); err != nil {
		t.Fatal(err)
	}
	if got := values("player"); got[variants.Flanking] != variants.On || got[variants.Rests] != variants.RestsGritty || got[variants.ShortRestCap] != "2" || got[variants.Morale] != variants.Off {
		t.Fatalf("after switching = %v", got)
	}
	// A later change touches only what it names.
	if err := s.Set(ctx, dmCaller, d.ID, variants.Set{variants.Flanking: variants.Off, variants.Morale: variants.On}); err != nil {
		t.Fatal(err)
	}
	if got := values("dm"); got[variants.Flanking] != variants.Off || got[variants.Rests] != variants.RestsGritty || got[variants.Morale] != variants.On {
		t.Fatalf("after a second change = %v", got)
	}
	// One bad choice among good ones changes nothing.
	for name, set := range map[string]variants.Set{
		"a value the variant cannot be": {variants.Morale: variants.Off, variants.Rests: "heroic"},
		"a variant that does not exist": {variants.Morale: variants.Off, "spell-points": variants.On},
	} {
		if err := s.Set(ctx, dmCaller, d.ID, set); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := s.Set(ctx, playerCaller, d.ID, variants.Set{variants.Morale: variants.Off}); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("a Player switches: %v", err)
	}
	if err := s.Set(ctx, stranger, d.ID, variants.Set{variants.Morale: variants.Off}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a stranger switches: %v", err)
	}
	if _, err := s.List(ctx, stranger, d.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a stranger lists: %v", err)
	}
	if got := values("dm"); got[variants.Morale] != variants.On || got[variants.Rests] != variants.RestsGritty {
		t.Fatalf("after the refusals = %v", got)
	}
	// Another Campaign of the same DM is not touched.
	campaigns, _ := service(t, pgstore.New(db.Pool()))
	elsewhere, err := campaigns.Create(ctx, dmCaller, app.CreateInput{Name: "Elsewhere", DisplayName: "Joris"})
	if err != nil {
		t.Fatal(err)
	}
	there, err := s.List(ctx, dmCaller, elsewhere.ID)
	if err != nil || there[0].Value != variants.Off || there[3].Variant.Slug != variants.Rests || there[3].Value != variants.RestsStandard {
		t.Fatalf("the other Campaign = %+v %v", there, err)
	}
	// An empty change is no change.
	if err := s.Set(ctx, dmCaller, d.ID, variants.Set{}); err != nil {
		t.Fatal(err)
	}

	// Every operation reports a database fault, and a change is kept whole or not at all.
	pgtest.EveryFault(t, func(fault *pgtest.Faulty) error {
		_, err := (&app.RuleVariants{Repo: pgstore.NewFaulty(db.Pool(), fault), Now: time.Now}).List(ctx, playerCaller, d.ID)
		if err != nil && !errors.Is(err, pgtest.ErrInjected) {
			t.Fatalf("list: %v", err)
		}
		return err
	})
	pgtest.EveryFault(t, func(fault *pgtest.Faulty) error {
		change := variants.Set{variants.Flanking: variants.On, variants.MassiveDamage: variants.On, variants.Rests: variants.RestsEpic}
		err := (&app.RuleVariants{Repo: pgstore.NewFaulty(db.Pool(), fault), Now: time.Now}).Set(ctx, dmCaller, d.ID, change)
		if err != nil && !errors.Is(err, pgtest.ErrInjected) {
			t.Fatalf("set: %v", err)
		}
		if got := values("dm"); err != nil && (got[variants.Flanking] != variants.Off || got[variants.MassiveDamage] != variants.Off || got[variants.Rests] != variants.RestsGritty) {
			t.Fatalf("a change that failed left half of itself behind: %v", got)
		}
		return err
	})
	if got := values("dm"); got[variants.Flanking] != variants.On || got[variants.MassiveDamage] != variants.On || got[variants.Rests] != variants.RestsEpic {
		t.Fatalf("after the faults = %v", got)
	}
}
