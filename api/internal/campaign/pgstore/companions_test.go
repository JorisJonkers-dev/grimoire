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
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

// anyCreature has every creature anyone asks for, unless it cannot be asked.
type anyCreature struct{ down bool }

func (c anyCreature) Creature(context.Context, domain.CampaignID, string) error {
	if c.down {
		return errors.New("compendium down")
	}
	return nil
}

// Every Companion operation reports a database fault at any of its calls, and a Companion shows the
// hit points it kept.
func TestEveryCompanionDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := open(t)
	_, d := npcs(t, pgstore.New(db.Pool()))
	svc := func(repo app.CompanionRepository) *app.Companions {
		return &app.Companions{Repo: repo, Creatures: anyCreature{}, Now: time.Now}
	}
	base := svc(pgstore.New(db.Pool()))
	me, err := pgstore.New(db.Pool()).Membership(ctx, d.ID, dmCaller.Subject)
	if err != nil {
		t.Fatal(err)
	}
	in := app.CompanionInput{Name: "Fang", Kind: domain.KindCompanion, MonsterSlug: "wolf", Controller: &me.ID, SharesXP: true, Notes: "Bites."}
	a, err := base.Create(ctx, dmCaller, d.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	doomed, _ := base.Create(ctx, dmCaller, d.ID, in)
	if _, err := db.Pool().Exec(ctx, "UPDATE campaign.companions SET hp_current = 4 WHERE id = $1", a.ID); err != nil {
		t.Fatal(err)
	}
	list, err := base.List(ctx, dmCaller, d.ID)
	if err != nil || len(list) != 2 || list[0].HP == nil && list[1].HP == nil || list[0].Controller == nil || *list[0].Controller != me.ID || !list[0].SharesXP || list[0].Notes != "Bites." {
		t.Fatalf("list = %+v, %v", list, err)
	}
	// Given another creature, a Companion starts again at that creature's full hit points; kept as it
	// is, it keeps what it had.
	same := in
	same.Name = "Old Fang"
	if _, err := base.Update(ctx, dmCaller, d.ID, a.ID, same); err != nil {
		t.Fatal(err)
	}
	hp := func() *int {
		t.Helper()
		all, err := base.List(ctx, dmCaller, d.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range all {
			if c.ID == a.ID {
				return c.HP
			}
		}
		t.Fatal("the Companion is gone")
		return nil
	}
	if got := hp(); got == nil || *got != 4 {
		t.Fatalf("hit points after a new name = %v", got)
	}
	other := same
	other.MonsterSlug = "bear"
	if _, err := base.Update(ctx, dmCaller, d.ID, a.ID, other); err != nil {
		t.Fatal(err)
	}
	if got := hp(); got != nil {
		t.Fatalf("hit points after a new creature = %d", *got)
	}
	// An ally is a companion or a hireling, and nothing else.
	pet := in
	pet.Kind = "pet"
	if _, err := base.Create(ctx, dmCaller, d.ID, pet); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("an ally of a kind there is not = %v", err)
	}
	// A bestiary that cannot be asked is a fault, not a creature that is not there.
	down := &app.Companions{Repo: pgstore.New(db.Pool()), Creatures: anyCreature{down: true}, Now: time.Now}
	var rule *apperr.RuleError
	if _, err := down.Create(ctx, dmCaller, d.ID, in); err == nil || errors.As(err, &rule) {
		t.Fatalf("a Companion made while the bestiary is down = %v", err)
	}
	ops := map[string]func(s *app.Companions) error{
		"create": func(s *app.Companions) error { _, err := s.Create(ctx, dmCaller, d.ID, in); return err },
		"update": func(s *app.Companions) error { _, err := s.Update(ctx, dmCaller, d.ID, a.ID, in); return err },
		"delete": func(s *app.Companions) error { return s.Delete(ctx, dmCaller, d.ID, doomed.ID) },
		"list":   func(s *app.Companions) error { _, err := s.List(ctx, dmCaller, d.ID); return err },
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(svc(pgstore.NewFaulty(db.Pool(), f)))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}
