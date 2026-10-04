package pgstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// scripted hands the dice the faces a test wrote down, in order, whatever the seed.
type scripted struct{ faces []int }

func (s *scripted) IntN(int) int {
	face := s.faces[0]
	s.faces = s.faces[1:]
	return face - 1
}

func (tb table) karmic(t *testing.T, on bool) {
	t.Helper()
	if _, err := tb.pool.Exec(context.Background(), "UPDATE campaign.campaigns SET karmic_dice = $2 WHERE id = $1", tb.campaign, on); err != nil {
		t.Fatal(err)
	}
}

// asked opens a roll the way live play asks one of a roller: it is theirs to roll, and not of their own making.
func asked(t *testing.T, tb table, c caller.Caller, purpose, notation string) domain.RollID {
	t.Helper()
	ctx := context.Background()
	roller, err := pgstore.CampaignMembers{Store: campaignpg.New(tb.pool)}.Membership(ctx, tb.campaign, c.Subject)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := dice.Parse(notation)
	if err != nil {
		t.Fatal(err)
	}
	roll := domain.Roll{CampaignID: tb.campaign, Purpose: purpose, Notation: notation, RequestedBy: "Joris", Roller: roller, Status: domain.StatusPending, Asked: true}
	for g, group := range spec.Groups {
		for range group.Count {
			roll.Dice = append(roll.Dice, domain.Die{No: len(roll.Dice), Group: g, Faces: group.Faces})
		}
	}
	id, err := pgstore.New(tb.pool).InsertRoll(ctx, roll, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// d20 rolls one d20 asked of a caller: by the server, or thrown by hand when a face is given.
func d20(t *testing.T, r *app.Rolls, tb table, c caller.Caller, thrown int) domain.Die {
	t.Helper()
	roll, err := r.SetDie(context.Background(), c, tb.campaign, asked(t, tb, c, "Check", "1d20"), 0, app.Fill{Auto: thrown == 0, Value: thrown})
	if err != nil || roll.Status != domain.StatusResolved {
		t.Fatalf("roll = %+v %v", roll, err)
	}
	return roll.Dice[0]
}

func dropped(d domain.Die) int {
	if d.KarmicDropped == nil {
		return 0
	}
	return *d.KarmicDropped
}

func TestKarmicDiceLeanAfterARunAndNeverTouchADieAPlayerThrew(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	r := rolls(tb, pgstore.New(tb.pool))
	script := &scripted{faces: nil}
	r.Source = func(uint64) dice.Source { return script }
	auto := func(c caller.Caller, faces ...int) domain.Die {
		t.Helper()
		script.faces = faces
		d := d20(t, r, tb, c, 0)
		if len(script.faces) != 0 {
			t.Fatalf("faces left undrawn: %v", script.faces)
		}
		return d
	}

	// Off, as a Campaign starts: two low d20s lean nothing.
	auto(player, 3)
	auto(player, 4)
	if d := auto(player, 2); d.Value != 2 || d.KarmicDropped != nil {
		t.Fatalf("karmic dice off = %+v", d)
	}

	// On: the run of low d20s rolled while it was off counts, so the next one leans up.
	tb.karmic(t, true)
	if d := auto(player, 2, 17); d.Value != 17 || dropped(d) != 2 || d.Mode != domain.ModeAuto {
		t.Fatalf("after three lows = %+v", d)
	}
	// The run is broken: one roll again.
	if d := auto(player, 4); d.Value != 4 || d.KarmicDropped != nil {
		t.Fatalf("after a high = %+v", d)
	}
	if d := auto(player, 5); d.Value != 5 || d.KarmicDropped != nil {
		t.Fatalf("one low = %+v", d)
	}
	// Two lows: a lean is waiting. A die the player throws is taken as thrown, is no part of the run,
	// and leaves the lean for the next die the server rolls.
	if d := d20(t, r, tb, player, 1); d.Value != 1 || d.KarmicDropped != nil || d.Mode != domain.ModeManual {
		t.Fatalf("thrown die = %+v", d)
	}
	if d := d20(t, r, tb, player, 20); d.Value != 20 || d.KarmicDropped != nil {
		t.Fatalf("thrown die = %+v", d)
	}
	// Another roller has a run of their own.
	if d := auto(dm, 1); d.Value != 1 || d.KarmicDropped != nil {
		t.Fatalf("the DM's first = %+v", d)
	}
	if d := auto(player, 19, 6); d.Value != 19 || dropped(d) != 6 {
		t.Fatalf("after two lows and two thrown dice = %+v", d)
	}
	// A run of highs leans down.
	auto(player, 16)
	if d := auto(player, 18, 3); d.Value != 3 || dropped(d) != 18 {
		t.Fatalf("after two highs = %+v", d)
	}

	// Dice that are not d20s are no part of it, whatever they show.
	auto(dm, 2)
	script.faces = []int{1, 1}
	other, err := r.RollRest(ctx, dm, tb.campaign, asked(t, tb, dm, "Damage", "2d6"))
	if err != nil || other.Dice[0].KarmicDropped != nil || other.Dice[1].KarmicDropped != nil || other.Total != 2 || len(script.faces) != 0 {
		t.Fatalf("d6s = %+v %v", other.Dice, err)
	}
	// Both d20s of one roll lean the way the run before it says.
	script.faces = []int{4, 12, 9, 2, 3}
	both, err := r.RollRest(ctx, dm, tb.campaign, asked(t, tb, dm, "Attack", "2d20kh1+1d4"))
	if err != nil || both.Dice[0].Value != 12 || dropped(both.Dice[0]) != 4 || both.Dice[1].Value != 9 || dropped(both.Dice[1]) != 2 ||
		both.Dice[2].Value != 3 || both.Dice[2].KarmicDropped != nil || len(script.faces) != 0 {
		t.Fatalf("two d20s = %+v %v", both.Dice, err)
	}
	// What was let go is kept with the die.
	again, err := r.Get(ctx, dm, tb.campaign, both.ID)
	if err != nil || dropped(again.Dice[0]) != 4 || dropped(again.Dice[1]) != 2 {
		t.Fatalf("read back = %+v %v", again.Dice, err)
	}

	// Off again: nothing leans, though the run is there.
	tb.karmic(t, false)
	auto(player, 2)
	auto(player, 2)
	if d := auto(player, 1); d.Value != 1 || d.KarmicDropped != nil {
		t.Fatalf("turned off = %+v", d)
	}
}

// Every karmic die replays from its logged seed and the run before it.
func TestKarmicDiceReplayFromTheirSeeds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	tb.karmic(t, true)
	r := rolls(tb, pgstore.New(tb.pool))
	var recent []int
	leaned := 0
	for seed := uint64(1); seed <= 60; seed++ {
		d := d20(t, r, tb, player, 0)
		lean := dice.Karma(recent)
		kept, let := dice.KarmicFace(rng.New(seed), 20, lean)
		if d.Value != kept || dropped(d) != let {
			t.Fatalf("seed %d after %v: rolled %d (let go %d), replays %d (let go %d)", seed, recent, d.Value, dropped(d), kept, let)
		}
		if lean != 0 {
			leaned++
		}
		recent = append([]int{d.Value}, recent...)
	}
	if leaned == 0 {
		t.Fatal("no die leaned in 60 rolls")
	}
	log, err := r.Log(ctx, dm, tb.campaign, 200)
	if err != nil {
		t.Fatal(err)
	}
	seeds := map[uint64]bool{}
	for _, a := range log {
		if a.Kind == domain.ActionDieRolled {
			seeds[*a.Seed] = true
		}
	}
	if len(seeds) != 60 {
		t.Fatalf("seeds logged = %d", len(seeds))
	}
}

// A die rerolled with Heroic Inspiration is a plain die: what a karmic die let go does not stay on it.
func TestARerolledDieIsNoLongerKarmic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	tb.karmic(t, true)
	r := rolls(tb, pgstore.New(tb.pool))
	script := &scripted{faces: []int{2, 3}}
	r.Source = func(uint64) dice.Source { return script }
	d20(t, r, tb, player, 0)
	d20(t, r, tb, player, 0)
	char := uuid.New()
	if _, err := tb.pool.Exec(ctx, `WITH hero AS (INSERT INTO campaign.account_characters (id, owner_subject, name, ruleset, species_slug, class_slug, background_slug, created_at, updated_at)
			SELECT $1, m.auth_subject, 'Hero', 'srd-2024', 'human', 'fighter', 'soldier', now(), now() FROM campaign.members m WHERE m.id = $3)
		INSERT INTO campaign.characters (character_id, id, campaign_id, owner_member_id, name, ruleset, species_slug, class_slug, background_slug,
		ability_method, hp_max, hp_current, level, heroic_inspiration) VALUES ($1, $1, $2, $3, 'Aria', 'srd-2024', 'human', 'fighter', 'soldier', 'standard-array', 20, 5, 4, true)`, char, tb.campaign, tb.playerID); err != nil {
		t.Fatal(err)
	}
	script.faces = []int{6, 14}
	roll, err := r.RollRest(ctx, player, tb.campaign, asked(t, tb, player, "Save", "1d20"))
	if err != nil || !roll.Choosing || roll.Dice[0].Value != 14 || dropped(roll.Dice[0]) != 6 {
		t.Fatalf("held = %+v %v", roll, err)
	}
	script.faces = []int{8}
	roll, err = r.Reroll(ctx, player, tb.campaign, roll.ID, 0)
	if err != nil || roll.Dice[0].Value != 8 || roll.Dice[0].KarmicDropped != nil || len(script.faces) != 0 {
		t.Fatalf("rerolled = %+v %v", roll.Dice, err)
	}
}

// Nobody makes their own luck: a roll a Member makes for themself is plain, whatever the run, and is no
// part of one. Otherwise free rolls from the dice tray could be thrown until the next real roll leans.
func TestARollAMemberMakesForThemselfIsNeverKarmic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	tb.karmic(t, true)
	r := rolls(tb, pgstore.New(tb.pool))
	script := &scripted{faces: nil}
	r.Source = func(uint64) dice.Source { return script }
	free := func(c caller.Caller, in app.RollInput, faces ...int) domain.Die {
		t.Helper()
		script.faces = faces
		roll, err := r.Create(ctx, c, tb.campaign, in)
		if err != nil {
			t.Fatal(err)
		}
		roll, err = r.RollRest(ctx, c, tb.campaign, roll.ID)
		if err != nil || len(script.faces) != 0 {
			t.Fatalf("free roll = %+v %v, undrawn %v", roll, err, script.faces)
		}
		return roll.Dice[0]
	}
	own := app.RollInput{Purpose: "Luck", Notation: "1d20"}
	// Two low rolls of the player's own making are no run.
	free(player, own, 2)
	free(player, own, 3)
	script.faces = []int{4}
	if d := d20(t, r, tb, player, 0); d.Value != 4 || d.KarmicDropped != nil || len(script.faces) != 0 {
		t.Fatalf("after two free lows = %+v", d)
	}
	// With a lean waiting, a roll of their own is still rolled once, and leaves the lean where it was.
	script.faces = []int{5}
	d20(t, r, tb, player, 0)
	if d := free(player, own, 1); d.Value != 1 || d.KarmicDropped != nil {
		t.Fatalf("a free roll under a lean = %+v", d)
	}
	if d := free(player, own, 20); d.Value != 20 || d.KarmicDropped != nil {
		t.Fatalf("a free roll under a lean = %+v", d)
	}
	script.faces = []int{6, 15}
	if d := d20(t, r, tb, player, 0); d.Value != 15 || dropped(d) != 6 || len(script.faces) != 0 {
		t.Fatalf("the lean was still waiting = %+v", d)
	}
	// The DM's own rolls from the dice tray are no different.
	free(dm, own, 1)
	free(dm, own, 1)
	if d := free(dm, own, 2); d.KarmicDropped != nil {
		t.Fatalf("the DM's own roll = %+v", d)
	}
	// A roll the DM asks of a Player is asked: it is part of the Player's run.
	theirs := app.RollInput{Purpose: "Stealth", Notation: "1d20", Roller: &tb.playerID}
	script.faces = []int{3}
	sent, err := r.Create(ctx, dm, tb.campaign, theirs)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := r.RollRest(ctx, player, tb.campaign, sent.ID); err != nil || got.Dice[0].Value != 3 || got.Dice[0].KarmicDropped != nil {
		t.Fatalf("asked by the DM = %+v %v", got, err)
	}
	script.faces = []int{2}
	d20(t, r, tb, player, 0)
	script.faces = []int{8, 11}
	sent, err = r.Create(ctx, dm, tb.campaign, theirs)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := r.RollRest(ctx, player, tb.campaign, sent.ID); err != nil || got.Dice[0].Value != 11 || dropped(got.Dice[0]) != 8 {
		t.Fatalf("asked by the DM after two lows = %+v %v", got, err)
	}
}

// A roll asked of a Member who has since left the Campaign still rolls: the DM finishes it.
func TestARollOfAMemberWhoLeftStillRolls(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	tb.karmic(t, true)
	r := rolls(tb, pgstore.New(tb.pool))
	first, pending := asked(t, tb, player, "Save", "1d20"), asked(t, tb, player, "Check", "1d20")
	if _, err := r.RollRest(ctx, player, tb.campaign, first); err != nil {
		t.Fatal(err)
	}
	if _, err := tb.pool.Exec(ctx, "DELETE FROM campaign.members WHERE id = $1", tb.playerID); err != nil {
		t.Fatal(err)
	}
	got, err := r.RollRest(ctx, dm, tb.campaign, pending)
	if err != nil || got.Status != domain.StatusResolved || got.Dice[0].Mode != domain.ModeAuto {
		t.Fatalf("rolled for a Member who left = %+v %v", got, err)
	}
}
