package live_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// shapedTable stores Effects with modes, durations, branches and scaling before anyone joins, then puts
// Aria at the centre and a goblin three hexes east.
func shapedTable(t *testing.T) (world, *table, map[string]string) {
	t.Helper()
	ctx := context.Background()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	for _, d := range []effects.Definition{
		{Slug: "enlarge-reduce", Name: "Enlarge/Reduce", Owner: effects.OwnedBySpell, Concentration: true, Components: []effects.Component{
			effects.Choice{Modes: []effects.Mode{
				{Name: "Enlarge", Components: []effects.Component{effects.SaveEdge{Ability: "strength", Mode: effects.SaveAdvantage}}},
				{Name: "Reduce", Components: []effects.Component{effects.Manual{Instruction: "Weapon damage drops by 1d4."}}},
			}},
		}},
		{Slug: "hold-fast", Name: "Hold Fast", Owner: effects.OwnedBySpell, Duration: effects.Duration{Kind: effects.Rounds, Amount: 3, RepeatSave: "wisdom"}, Components: []effects.Component{effects.Immobile{}}},
		{Slug: "weary", Name: "Weary", Owner: effects.OwnedByFeature, Duration: effects.Duration{Kind: effects.UntilRest}, Components: []effects.Component{effects.SpeedPenalty{Ft: 5}}},
		{Slug: "frost-burst", Name: "Frost Burst", Owner: effects.OwnedBySpell, Scaling: &effects.Scaling{Axis: effects.SlotLevel, Base: 1, Dice: "1d6"}, Components: []effects.Component{
			effects.Area{Shape: hex.SphereArea, SizeFt: 5, RangeFt: 60},
			effects.SaveDamage{Ability: "constitution", Dice: "2d6", Type: "cold", Half: true},
			effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 5}, Then: []effects.Component{
				effects.SaveCondition{Ability: "constitution", Slug: "prone"}, effects.Manual{Instruction: "frost rimes its armour."},
			}},
		}},
	} {
		if err := comppg.New(w.pool).SaveEffect(ctx, effects.Owner{Kind: d.Owner, Slug: d.Slug}, d); err != nil {
			t.Fatal(err)
		}
	}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString()})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 3})
	ids := map[string]string{}
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
	}
	return w, tb, ids
}

// An Effect with modes must be applied in one; its Duration fills in the rounds and the repeat save
// against its source's spell DC; an Effect that lasts until a rest ends when one finishes.
func TestModesDurationsAndRests(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids := shapedTable(t)
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Goblin"], Effect: "enlarge-reduce", EffectMode: "Shrink"})
	if u := next(t, tb.dm); !strings.Contains(u.Reason, "Choose Enlarge or Reduce.") {
		t.Fatalf("a mode is required = %+v", u)
	}
	d, _ := tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Goblin"], Effect: "enlarge-reduce", EffectMode: "Reduce"})
	if e := effect(token(d.View, "Goblin"), "Enlarge/Reduce"); e == nil || e.Mode != "Reduce" || len(d.View.Manual) != 1 || d.View.Manual[0].Text != "Goblin: Weapon damage drops by 1d4." {
		t.Fatalf("reduced = %+v %+v", token(d.View, "Goblin").Effects, d.View.Manual)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Goblin"], SourceID: ids["Aria"], Effect: "hold-fast"})
	if e := effect(token(d.View, "Goblin"), "Hold Fast"); e == nil || e.RoundsLeft != 3 {
		t.Fatalf("hold fast lasts three rounds = %+v", token(d.View, "Goblin").Effects)
	}
	var ability string
	var dc int
	if err := w.pool.QueryRow(ctx, "SELECT save_ability, save_dc FROM play.active_effects WHERE slug = 'hold-fast'").Scan(&ability, &dc); err != nil || ability != "wisdom" || dc != 14 {
		t.Fatalf("repeat save = %s %d %v", ability, dc, err)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "bless"})
	if e := effect(token(d.View, "Aria"), "Bless"); e.RoundsLeft != 10 {
		t.Fatalf("bless lasts a minute = %+v", e)
	}
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "weary"})
	tb.dmSays(live.Command{Kind: live.CmdRest, Rest: live.RestShort})
	if v := look(t, w, tb.dm); effect(token(v, "Aria"), "Weary") != nil || effect(token(v, "Aria"), "Bless") == nil {
		t.Fatalf("a rest ends only what lasts until one = %+v", token(v, "Aria").Effects)
	}
}

// An area spell cast with a higher slot rolls more dice; a target that fails its save by 5 or more
// gets what the branch adds.
func TestSlotsAndBranches(t *testing.T) {
	t.Parallel()
	w, tb, ids := shapedTable(t)
	tb.fight(ids, "Aria", "Goblin")
	burst := live.Command{Kind: live.CmdCastArea, TokenID: ids["Aria"], Effect: "frost-burst", Q: 3, R: 0, Slot: 10}
	if u := tb.playerSays(burst); !strings.Contains(u.Reason, "slot of level 1 to 9") {
		t.Fatalf("slot 10 = %+v", u)
	}
	burst.Slot = 3
	u := tb.playerSays(burst)
	tb.fill(u.View.Area.DamageRollID, w.player, 1, 1, 1, 1)
	tb.fill(u.View.Area.Saves[0].RollID, w.dm, 1)
	for range 3 {
		next(t, tb.player)
		next(t, tb.dm)
	}
	v := look(t, w, tb.dm)
	if g := token(v, "Goblin"); *g.HP != 3 || effect(g, "Prone") == nil || len(v.Manual) != 1 || v.Manual[0].Text != "Goblin: frost rimes its armour." {
		t.Fatalf("four dice of cold, then the branch = %+v %+v", g, v.Manual)
	}
}
