package pgstore_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

var (
	dm       = caller.UI("dm")
	player   = caller.UI("player")
	stranger = caller.UI("stranger")
)

type table struct {
	pool     *pgxpool.Pool
	campaign uuid.UUID
	dmID     uuid.UUID
	playerID uuid.UUID
}

func setup(t *testing.T) table {
	t.Helper()
	ctx := context.Background()
	store, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	camp := campaignapp.NewService(campaignpg.New(store.Pool()))
	d, err := camp.Create(ctx, dm, campaignapp.CreateInput{Name: "Dice", DisplayName: "Joris"})
	if err != nil {
		t.Fatal(err)
	}
	inv, _ := camp.CreateInvite(ctx, dm, d.ID)
	if _, err := camp.AcceptInvite(ctx, player, inv.Token, "Ireena"); err != nil {
		t.Fatal(err)
	}
	home, _ := camp.Get(ctx, dm, d.ID)
	return table{pool: store.Pool(), campaign: uuid.UUID(d.ID), dmID: uuid.UUID(home.Members[0].ID), playerID: uuid.UUID(home.Members[1].ID)}
}

func rolls(tb table, repo app.Repository) *app.Rolls {
	seed := uint64(0)
	return &app.Rolls{
		Repo: repo, Members: pgstore.CampaignMembers{Store: campaignpg.New(tb.pool)},
		Seed:   func() uint64 { seed++; return seed },
		Source: func(s uint64) dice.Source { return rng.New(s) },
		Now:    func() time.Time { return time.Date(2026, 9, 30, 22, 0, 0, 0, time.UTC) },
	}
}

func attack() app.RollInput {
	return app.RollInput{
		Purpose: " Longsword attack ", Notation: "2d20kh1 + 1d4", Labels: map[int]string{0: "Advantage: Help", 1: "Bless"},
		Modifiers: []domain.Modifier{{Label: "Strength", Value: 3}, {Label: "Proficiency", Value: 2}},
	}
}

func TestManualAndAutoDiceResolve(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	r := rolls(tb, pgstore.New(tb.pool))
	roll, err := r.Create(ctx, player, tb.campaign, attack())
	if err != nil || roll.Purpose != "Longsword attack" || roll.Notation != "2d20kh1+1d4" || len(roll.Dice) != 3 ||
		roll.Status != domain.StatusPending || roll.Roller.Name != "Ireena" || roll.Labels[1] != "Bless" {
		t.Fatalf("created = %+v %v", roll, err)
	}
	roll, err = r.SetDie(ctx, player, tb.campaign, roll.ID, 0, app.Fill{Value: 7})
	if err != nil || roll.Dice[0].Value != 7 || roll.Dice[0].Mode != domain.ModeManual || roll.Dice[0].Kept {
		t.Fatalf("manual = %+v %v", roll.Dice, err)
	}
	roll, err = r.SetDie(ctx, player, tb.campaign, roll.ID, 1, app.Fill{Value: 15})
	if err != nil || !roll.Dice[1].Kept || roll.Dice[0].Kept {
		t.Fatalf("advantage keeps the 15 = %+v %v", roll.Dice, err)
	}
	want := dice.Face(rng.New(1), 4)
	roll, err = r.RollRest(ctx, player, tb.campaign, roll.ID)
	if err != nil || roll.Status != domain.StatusResolved || roll.Dice[2].Value != want || roll.Dice[2].Mode != domain.ModeAuto ||
		roll.Total != 15+want+5 || roll.ResolvedAt.IsZero() {
		t.Fatalf("resolved = %+v %v", roll, err)
	}
	log, err := r.Log(ctx, dm, tb.campaign, 10)
	if err != nil || len(log) != 5 || log[0].Kind != domain.ActionRollResolved || log[0].Value != roll.Total ||
		log[1].Kind != domain.ActionDieRolled || *log[1].Seed != 1 || *log[1].DieNo != 2 || log[4].Kind != domain.ActionRollRequested ||
		log[4].Seq != 1 || log[0].Actor != "Ireena" || *log[0].RollID != roll.ID {
		t.Fatalf("log = %+v %v", log, err)
	}
	if _, err := r.SetDie(ctx, player, tb.campaign, roll.ID, 0, app.Fill{Auto: true}); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("resolved roll changed: %v", err)
	}
	list, err := r.List(ctx, dm, tb.campaign, 10)
	if err != nil || len(list) != 1 || !list[0].Dice[1].Kept {
		t.Fatalf("list = %+v %v", list, err)
	}
}

func TestSeededRollsReplay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	r := rolls(tb, pgstore.New(tb.pool))
	roll, _ := r.Create(ctx, dm, tb.campaign, app.RollInput{Purpose: "Fireball", Notation: "8d6"})
	roll, err := r.RollRest(ctx, dm, tb.campaign, roll.ID)
	if err != nil {
		t.Fatal(err)
	}
	log, _ := r.Log(ctx, dm, tb.campaign, 20)
	total := 0
	for _, a := range log {
		if a.Kind == domain.ActionDieRolled {
			if got := dice.Face(rng.New(*a.Seed), 6); got != a.Value || got != roll.Dice[*a.DieNo].Value {
				t.Fatalf("seed %d replays %d, log says %d", *a.Seed, got, a.Value)
			}
			total += a.Value
		}
	}
	if total != roll.Total {
		t.Fatalf("replayed total %d != %d", total, roll.Total)
	}
}

func TestRollRulesAndPermissions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	r := rolls(tb, pgstore.New(tb.pool))
	var rule *apperr.RuleError
	if _, err := r.Create(ctx, player, tb.campaign, app.RollInput{Purpose: "x", Notation: "1d7"}); !errors.As(err, &rule) || rule.Reason != "there is no d7" {
		t.Fatalf("bad notation: %v", err)
	}
	for _, in := range []app.RollInput{{Purpose: " ", Notation: "1d20"}, {Purpose: "x", Notation: "1d20", Labels: map[int]string{3: "no group"}}} {
		if _, err := r.Create(ctx, player, tb.campaign, in); !errors.Is(err, apperr.ErrInvalid) {
			t.Errorf("%+v: %v", in, err)
		}
	}
	if _, err := r.Create(ctx, stranger, tb.campaign, attack()); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("stranger: %v", err)
	}
}

func TestRollersAndFaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	r := rolls(tb, pgstore.New(tb.pool))
	var rule *apperr.RuleError
	someoneElse := attack()
	someoneElse.Roller = &tb.dmID
	if _, err := r.Create(ctx, player, tb.campaign, someoneElse); !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("player asks the DM to roll: %v", err)
	}
	ask := attack()
	ask.Roller = &tb.playerID
	roll, err := r.Create(ctx, dm, tb.campaign, ask)
	if err != nil || roll.Roller.Name != "Ireena" || roll.RequestedBy != "Joris" {
		t.Fatalf("DM asks the player = %+v %v", roll, err)
	}
	missing := uuid.New()
	ask.Roller = &missing
	if _, err := r.Create(ctx, dm, tb.campaign, ask); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("unknown roller: %v", err)
	}
	if _, err := r.SetDie(ctx, player, tb.campaign, roll.ID, 0, app.Fill{Value: 21}); !errors.As(err, &rule) || rule.Reason != "a d20 shows 1 to 20, not 21" {
		t.Fatalf("face 21 on a d20: %v", err)
	}
	if _, err := r.SetDie(ctx, player, tb.campaign, roll.ID, 9, app.Fill{Value: 1}); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("die 9: %v", err)
	}
	if _, err := r.SetDie(ctx, player, tb.campaign, roll.ID, 0, app.Fill{Value: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.SetDie(ctx, player, tb.campaign, roll.ID, 0, app.Fill{Value: 3}); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("die set twice: %v", err)
	}
	mine, _ := r.Create(ctx, dm, tb.campaign, attack())
	if _, err := r.SetDie(ctx, player, tb.campaign, mine.ID, 0, app.Fill{Value: 2}); !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("player rolls the DM's dice: %v", err)
	}
	if _, err := r.SetDie(ctx, dm, tb.campaign, roll.ID, 1, app.Fill{Value: 2}); err != nil {
		t.Fatalf("DM fills the player's die: %v", err)
	}
}

func TestRollsRefuseOutsiders(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	r := rolls(tb, pgstore.New(tb.pool))
	roll, _ := r.Create(ctx, player, tb.campaign, attack())
	checks := map[string]error{}
	_, checks["get"] = r.Get(ctx, stranger, tb.campaign, roll.ID)
	_, checks["list"] = r.List(ctx, stranger, tb.campaign, 5)
	_, checks["log"] = r.Log(ctx, stranger, tb.campaign, 5)
	_, checks["set"] = r.SetDie(ctx, stranger, tb.campaign, roll.ID, 0, app.Fill{Auto: true})
	_, checks["missing"] = r.Get(ctx, dm, tb.campaign, domain.RollID(uuid.New()))
	_, checks["fill missing"] = r.RollRest(ctx, dm, tb.campaign, domain.RollID(uuid.New()))
	for name, err := range checks {
		if !errors.Is(err, apperr.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := r.Log(ctx, player, tb.campaign, 5); !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("player reads the log: %v", err)
	}
}

func TestEveryPlayDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	base := rolls(tb, pgstore.New(tb.pool))
	resolved, _ := base.Create(ctx, player, tb.campaign, attack())
	_, _ = base.RollRest(ctx, player, tb.campaign, resolved.ID)
	n := 0
	ops := map[string]func(r *app.Rolls) error{
		"create": func(r *app.Rolls) error { _, err := r.Create(ctx, player, tb.campaign, attack()); return err },
		"get":    func(r *app.Rolls) error { _, err := r.Get(ctx, player, tb.campaign, resolved.ID); return err },
		"list":   func(r *app.Rolls) error { _, err := r.List(ctx, player, tb.campaign, 5); return err },
		"log":    func(r *app.Rolls) error { _, err := r.Log(ctx, dm, tb.campaign, 5); return err },
		"set": func(r *app.Rolls) error {
			n++
			fresh, err := base.Create(ctx, player, tb.campaign, app.RollInput{Purpose: "Check " + strconv.Itoa(n), Notation: "1d20+1d4"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := r.SetDie(ctx, player, tb.campaign, fresh.ID, 0, app.Fill{Value: 2}); err != nil {
				return err
			}
			_, err = r.SetDie(ctx, player, tb.campaign, fresh.ID, 1, app.Fill{Auto: true})
			return err
		},
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			r := rolls(tb, pgstore.NewFaulty(tb.pool, f))
			err := op(r)
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}

func (tb table) dmMember(t *testing.T) domain.Member {
	t.Helper()
	m, err := pgstore.CampaignMembers{Store: campaignpg.New(tb.pool)}.Membership(context.Background(), tb.campaign, dm.Subject)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
