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
)

// magicTable puts Aria at the centre and a goblin three hexes east.
func magicTable(t *testing.T) (world, *table, map[string]string) {
	t.Helper()
	return magicTableIn(t, setup(t))
}

// magicTableIn sets the table up in a world already made.
func magicTableIn(t *testing.T, w world) (world, *table, map[string]string) {
	t.Helper()
	w.hub.Stats = bestiary{owner: w.player.ID}
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

func (tb *table) fight(ids map[string]string, order ...string) {
	tb.t.Helper()
	var setup []live.CombatantSetup
	for _, label := range order {
		setup = append(setup, live.CombatantSetup{TokenID: ids[label], SpeedFt: 30})
	}
	d, _ := tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: setup})
	for i, label := range order {
		who := tb.w.dm
		if label == "Aria" {
			who = tb.w.player
		}
		tb.roll(combatant(d.View, label), who, 20-i)
	}
}

// Misty Step moves a creature without walking: anywhere free within 30 feet, for its Bonus Action.
func TestMistyStepTeleports(t *testing.T) {
	t.Parallel()
	w, tb, ids := magicTable(t)
	step := func(q, r int) live.Command {
		return live.Command{Kind: live.CmdTeleport, TokenID: ids["Aria"], Effect: "misty-step", Q: q, R: r}
	}
	goblin := step(1, 0)
	goblin.TokenID = ids["Goblin"]
	bless := step(1, 0)
	bless.Effect = "bless"
	for want, cmd := range map[string]live.Command{
		"out of range":      step(7, 0),
		"free hex":          step(3, 0),
		"a free hex on the": step(11, 0),
		"not a teleport":    bless,
		"not yours to play": goblin,
		"no such creature":  {Kind: live.CmdTeleport, TokenID: "nope", Effect: "misty-step"},
	} {
		if u := tb.playerSays(cmd); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), want) {
			t.Errorf("%s = %+v", want, u)
		}
	}
	if u := tb.playerSays(step(-6, 0)); token(u.View, "Aria").Q != -6 {
		t.Fatalf("out of combat Aria steps 30 feet west = %+v", token(u.View, "Aria"))
	}
	tb.fight(ids, "Aria", "Goblin")
	u := tb.playerSays(step(-1, 0))
	if aria := token(u.View, "Aria"); aria.Q != -1 || combatant(u.View, "Aria").BonusAction || !combatant(u.View, "Aria").Action || combatant(u.View, "Aria").MovementFt != 30 {
		t.Fatalf("Misty Step takes the Bonus Action and no movement = %+v %+v", aria, combatant(u.View, "Aria"))
	}
	if u := tb.playerSays(step(0, 0)); !strings.Contains(u.Reason, "Bonus Action") {
		t.Fatalf("a second Misty Step = %+v", u)
	}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdTeleport, TokenID: ids["Goblin"], Effect: "misty-step", Q: 2, R: 0})
	if u := next(t, tb.dm); !strings.Contains(u.Reason, "not Goblin's turn") {
		t.Fatalf("off turn = %+v", u)
	}
}

// Temporary hit points from False Life soak damage first; Thunderwave pushes those who fail its save
// straight away from the caster; Dispel Magic ends spells and leaves conditions.
func TestTempHPPushesAndDispelling(t *testing.T) {
	t.Parallel()
	w, tb, ids := magicTable(t)
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "false-life"})
	if d, p := tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "false-life"}); token(d.View, "Aria").TempHP != 9 || token(p.View, "Aria").TempHP != 9 {
		t.Fatalf("False Life gives 9 temporary hit points, not 18 = %+v", token(d.View, "Aria"))
	}
	tb.playerSays(live.Command{Kind: live.CmdTeleport, TokenID: ids["Aria"], Effect: "misty-step", Q: 2, R: 0})
	tb.fight(ids, "Goblin", "Aria")
	d, _ := tb.dmSays(live.Command{Kind: live.CmdCastArea, TokenID: ids["Goblin"], Effect: "thunderwave", Q: 2, R: 0})
	tb.fill(d.View.Area.DamageRollID, w.dm, 3, 3)
	tb.fill(d.View.Area.Saves[0].RollID, w.player, 1)
	var last live.Update
	for range 3 {
		next(t, tb.player)
		last = next(t, tb.dm)
	}
	aria := token(last.View, "Aria")
	if *aria.HP != 12 || aria.TempHP != 3 || aria.Q != 0 || aria.R != 0 {
		t.Fatalf("6 thunder damage soaked, then pushed 10 feet = %+v", aria)
	}
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "bless"})
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "prone"})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "dispel-magic"})
	if fx := token(d.View, "Aria").Effects; len(fx) != 1 || fx[0].Name != "Prone" {
		t.Fatalf("Dispel Magic ends Bless and False Life = %+v", fx)
	}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "dispel-magic"})
	if u := next(t, tb.dm); !strings.Contains(u.Reason, "No spell on Aria") {
		t.Fatalf("nothing to dispel = %+v", u)
	}
}

// Spirit Guardians stays on its caster and its area moves with them; Wall of Fire rasterises to a line
// of hexes beyond the point, away from the caster.
func TestEmanationsFollowAndWallsStand(t *testing.T) {
	t.Parallel()
	w, tb, ids := magicTable(t)
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Goblin"], SourceID: ids["Aria"], Effect: "bless"})
	tb.fight(ids, "Aria", "Goblin")
	u := tb.playerSays(live.Command{Kind: live.CmdPreviewArea, TokenID: ids["Aria"], Effect: "wall-of-fire", Q: 2, R: 0})
	if u.Area == nil || len(u.Area.Hexes) != 9 || u.Area.Hexes[0] != (live.Hex{Q: 2, R: 0}) || len(u.Area.Targets) != 1 {
		t.Fatalf("wall of fire = %+v", u.Area)
	}
	u = tb.playerSays(live.Command{Kind: live.CmdCastArea, TokenID: ids["Aria"], Effect: "spirit-guardians", Q: 0, R: 0})
	tb.fill(u.View.Area.DamageRollID, w.player, 2, 2, 2)
	tb.fill(u.View.Area.Saves[0].RollID, w.dm, 1, 1)
	for range 3 {
		next(t, tb.player)
		next(t, tb.dm)
	}
	v := look(t, w, tb.dm)
	sg := effect(token(v, "Aria"), "Spirit Guardians")
	if sg == nil || !sg.Concentration || len(sg.Hexes) != 36 || effect(token(v, "Goblin"), "Bless") != nil {
		t.Fatalf("Spirit Guardians sits on Aria and ends Bless = %+v %+v", token(v, "Aria").Effects, token(v, "Goblin").Effects)
	}
	tb.playerSays(live.Command{Kind: live.CmdTeleport, TokenID: ids["Aria"], Effect: "misty-step", Q: -4, R: 0})
	sg = effect(token(look(t, w, tb.dm), "Aria"), "Spirit Guardians")
	if sg.Hexes[0].Q > -7 || !containsHex(sg.Hexes, live.Hex{Q: -3, R: 0}) || containsHex(sg.Hexes, live.Hex{Q: 1, R: 0}) {
		t.Fatalf("the emanation follows Aria = %+v", sg.Hexes)
	}
}

func containsHex(hs []live.Hex, h live.Hex) bool {
	for _, x := range hs {
		if x == h {
			return true
		}
	}
	return false
}

// A creature with Counterspell ready is asked when an enemy casts within 60 feet; countering spends its
// Reaction and the spell never lands, while declining lets it resolve.
func TestCounterspell(t *testing.T) {
	t.Parallel()
	w, tb, ids := magicTable(t)
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Goblin"], Effect: "counterspell"})
	tb.fight(ids, "Aria", "Goblin")
	u := tb.playerSays(live.Command{Kind: live.CmdCastArea, TokenID: ids["Aria"], Effect: "shatter", Q: 3, R: 0})
	cast := u.View.Area
	d := next(t, tb.dm)
	next(t, tb.player)
	if p := d.View.Combat.Prompt; p == nil || p.Kind != domain.PromptEffect || !strings.Contains(p.Effect, "counter Aria's Shatter") {
		t.Fatalf("the goblin is asked to counter = %+v", d.View.Combat)
	}
	tb.fill(cast.DamageRollID, w.player, 4, 4, 4)
	tb.fill(cast.Saves[0].RollID, w.dm, 1)
	if v := look(t, w, tb.dm); v.Area == nil || *token(v, "Goblin").HP != 7 {
		t.Fatalf("the spell waits on the answer = %+v", v.Area)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdReact, Use: true})
	if d.View.Area != nil || combatant(d.View, "Goblin").Reaction || *token(d.View, "Goblin").HP != 7 {
		t.Fatalf("countered = %+v %+v", d.View.Area, combatant(d.View, "Goblin"))
	}
}

// Declining the counter lets the spell resolve with the rolls already in.
func TestDecliningACounter(t *testing.T) {
	t.Parallel()
	w, tb, ids := magicTable(t)
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Goblin"], Effect: "counterspell"})
	tb.fight(ids, "Aria", "Goblin")
	u := tb.playerSays(live.Command{Kind: live.CmdCastArea, TokenID: ids["Aria"], Effect: "shatter", Q: 3, R: 0})
	next(t, tb.dm)
	next(t, tb.player)
	tb.fill(u.View.Area.DamageRollID, w.player, 1, 1, 1)
	tb.fill(u.View.Area.Saves[0].RollID, w.dm, 1)
	tb.dmSays(live.Command{Kind: live.CmdReact, Use: false})
	for range 2 {
		next(t, tb.player)
		next(t, tb.dm)
	}
	if v := look(t, w, tb.dm); v.Area != nil || *token(v, "Goblin").HP != 4 {
		t.Fatalf("declined, the spell lands = %+v %+v", v.Area, token(v, "Goblin"))
	}
}

// An Effect can give a feature, noted for the DM, and give back a Character's spent Resources.
func TestGrantsAndResourceChanges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids := restingParty(t)
	boost := effects.Definition{Slug: "battle-hymn", Name: "Battle Hymn", Owner: effects.OwnedBySpell, Components: []effects.Component{
		effects.GrantFeature{Name: "Darkvision"}, effects.ResourceChange{Resource: "rage", Delta: 2},
	}}
	if err := comppg.New(w.pool).SaveEffect(ctx, effects.Owner{Kind: effects.OwnedBySpell, Slug: boost.Slug}, boost); err != nil {
		t.Fatal(err)
	}
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	brom := ""
	for _, tv := range look(t, w, tb.dm).Tokens {
		if tv.Q == 2 {
			brom = tv.ID
		}
	}
	d, _ := tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: brom, Effect: "battle-hymn"})
	if len(d.View.Manual) != 1 || !strings.HasSuffix(d.View.Manual[0].Text, " gains Darkvision.") {
		t.Fatalf("grant = %+v", d.View.Manual)
	}
	var used int
	if err := w.pool.QueryRow(ctx, "SELECT used FROM campaign.character_resources WHERE character_id = $1 AND resource_slug = 'rage'", ids["Brom"]).Scan(&used); err != nil || used != 1 {
		t.Fatalf("rage used = %d %v", used, err)
	}
}
