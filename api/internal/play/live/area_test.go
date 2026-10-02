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

func surfaceAt(v *live.View, q, r int) string {
	for _, s := range v.Surfaces {
		if s.Q == q && s.R == r {
			return s.Kind
		}
	}
	return ""
}

func TestAreaSpellsSavesAndSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	storm := effects.Definition{Slug: "storm-sphere", Name: "Storm Sphere", Concentration: true, Components: []effects.Component{
		effects.Area{Shape: hex.SphereArea, SizeFt: 20, RangeFt: 150}, effects.SaveDamage{Ability: "strength", Dice: "2d6", Type: "bludgeoning", Half: false},
	}}
	if err := comppg.New(w.pool).SaveEffect(ctx, effects.Owner{Kind: effects.OwnedBySpell, Slug: storm.Slug}, storm); err != nil {
		t.Fatal(err)
	}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Label: "Brom", Q: 3, R: 1})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 3})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Archer", Q: 4})
	ids := map[string]string{}
	var setup []live.CombatantSetup
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
		setup = append(setup, live.CombatantSetup{TokenID: tv.ID, SpeedFt: 30})
	}
	cast := func(kind, spell string, q, r int) live.Command {
		return live.Command{Kind: kind, TokenID: ids["Aria"], Effect: spell, Q: q, R: r}
	}
	if u := tb.playerSays(cast(live.CmdPreviewArea, "shatter", 4, 0)); !strings.Contains(u.Reason, "in combat") {
		t.Fatalf("out of combat = %+v", u)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: setup})
	for label, face := range map[string]int{"Aria": 20, "Brom": 15, "Goblin": 10, "Archer": 5} {
		who := w.dm
		if label == "Aria" || label == "Brom" {
			who = w.player
		}
		tb.roll(combatant(d.View, label), who, face)
	}

	u := tb.playerSays(cast(live.CmdPreviewArea, "shatter", 4, 0))
	pv := u.Area
	if pv == nil || pv.Name != "Shatter" || pv.DC != 14 || len(pv.Hexes) != 19 || len(pv.Targets) != 3 || pv.Allies != 1 {
		t.Fatalf("shatter preview = %+v", pv)
	}
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Brom"], SourceID: ids["Aria"], Effect: "bless"})
	if u := tb.playerSays(cast(live.CmdPreviewArea, "storm-sphere", 4, 0)); u.Area == nil || len(u.Area.Ends) != 1 || u.Area.Ends[0] != "Bless" {
		t.Fatalf("a concentration area warns it ends Bless = %+v", u.Area)
	}
	if u := tb.playerSays(cast(live.CmdPreviewArea, "shatter", 4, 0)); u.Area.Ends != nil {
		t.Fatalf("shatter needs no concentration = %+v", u.Area)
	}
	tb.dmSays(live.Command{Kind: live.CmdEndEffect, EffectID: effect(token(look(t, w, tb.dm), "Brom"), "Bless").ID})
	if u := tb.playerSays(cast(live.CmdPreviewArea, "burning-hands", 1, 0)); len(u.Area.Targets) != 1 || u.Area.Targets[0].TokenID != ids["Goblin"] || u.Area.Allies != 0 {
		t.Fatalf("burning hands east = %+v", u.Area)
	}
	for want, cmd := range map[string]live.Command{
		"not an area spell": cast(live.CmdPreviewArea, "bless", 4, 0),
		"aim away from":     cast(live.CmdPreviewArea, "burning-hands", 0, 0),
		"off the map":       cast(live.CmdPreviewArea, "shatter", 99, 0),
		"no such caster":    {Kind: live.CmdPreviewArea, TokenID: "nope", Effect: "shatter"},
		"not yours to play": {Kind: live.CmdCastArea, TokenID: ids["Goblin"], Effect: "shatter", Q: 1},
	} {
		if u := tb.playerSays(cmd); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), want) {
			t.Errorf("%s = %+v", want, u)
		}
	}

	u = tb.playerSays(cast(live.CmdCastArea, "shatter", 4, 0))
	area := u.View.Area
	if area == nil || area.Name != "Shatter" || area.DamageRollID == "" || len(area.Saves) != 3 || combatant(u.View, "Aria").Action {
		t.Fatalf("cast = %+v", u.View)
	}
	if u := tb.playerSays(cast(live.CmdPreviewArea, "shatter", 4, 0)); !strings.Contains(u.Reason, "still waiting on its rolls") {
		t.Fatalf("a second spell = %+v", u)
	}
	saves := map[string]string{}
	for _, s := range area.Saves {
		saves[s.TokenID] = s.RollID
	}
	roll, _ := pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(saves[ids["Brom"]])))
	if roll.Purpose != "Constitution save against Shatter (DC 14)" || roll.Modifiers[0].Value != 2 || roll.Roller.ID != w.player.ID {
		t.Fatalf("brom's save = %+v", roll)
	}
	fill := func(rollID string, faces ...int) {
		t.Helper()
		for i, f := range faces {
			if _, err := rolls.SetDie(ctx, dmCaller, w.session.CampaignID, domain.RollID(uuid.MustParse(rollID)), i, app.Fill{Value: f}); err != nil {
				t.Fatal(err)
			}
		}
	}
	fill(area.DamageRollID, 4, 4, 4)
	fill(saves[ids["Goblin"]], 5)
	w.hub.Close(w.session.ID)
	fill(saves[ids["Archer"]], 18)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	v := func() *live.View {
		t.Helper()
		w.hub.Submit(tb.dm, live.Command{Kind: live.CmdResync})
		return next(t, tb.dm).View
	}
	if a := v().Area; a == nil || len(a.Saves) != 3 {
		t.Fatalf("the cast survives a restart = %+v", a)
	}
	fill(saves[ids["Brom"]], 13)
	drain(tb.dm)
	drain(tb.player)
	now := look(t, w, tb.dm)
	if now.Area != nil || *token(now, "Goblin").HP != 0 || *token(now, "Archer").HP != 1 || *token(now, "Brom").HP != 6 {
		t.Fatalf("12 thunder: the goblin fails, the archer and brom halve it to 6 = %+v %+v %+v", token(now, "Goblin"), token(now, "Archer"), token(now, "Brom"))
	}

	tb.dmSays(live.Command{Kind: live.CmdPaintSurface, Hexes: []live.Hex{{Q: 3, R: 0}}, Surface: "grease"})
	tb.dmSays(live.Command{Kind: live.CmdPaintSurface, Hexes: []live.Hex{{Q: 2, R: 0}}, Surface: "web"})
	tb.dmSays(live.Command{Kind: live.CmdPaintSurface, Hexes: []live.Hex{{Q: 0, R: 1}}, Surface: "water", Rounds: 1})
	d, p := tb.dmSays(live.Command{Kind: live.CmdPaintSurface, Hexes: []live.Hex{{Q: 1, R: -1}}, Surface: "fire"})
	if surfaceAt(p.View, 3, 0) != "grease" || surfaceAt(p.View, 2, 0) != "web" || surfaceAt(d.View, 1, -1) != "fire" {
		t.Fatalf("painted surfaces = %+v", p.View.Surfaces)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdPlanWalk, TokenID: ids["Aria"], Q: 1, R: 0}); u.Path.CostFt != 5 {
		t.Fatalf("bare ground = %+v", u.Path)
	}
	for want, cmd := range map[string]live.Command{
		"no such surface":    {Kind: live.CmdPaintSurface, Hexes: []live.Hex{{Q: 1, R: 1}}, Surface: "quicksand"},
		"surfaces last":      {Kind: live.CmdPaintSurface, Hexes: []live.Hex{{Q: 1, R: 1}}, Surface: "fire", Rounds: 101},
		"choose a map":       {Kind: live.CmdSetElevation, Hexes: []live.Hex{{Q: 1, R: 1}}, ElevationFt: 10},
		"between 1 and 2000": {Kind: live.CmdPaintSurface, Surface: "fire"},
	} {
		w.hub.Submit(tb.dm, cmd)
		if u := next(t, tb.dm); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), want) {
			t.Errorf("%s = %+v", want, u)
		}
	}
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(now, "Aria").ID})
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(now, "Brom").ID})
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(now, "Goblin").ID})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdCastArea, TokenID: ids["Archer"], Effect: "burning-hands", Q: 3, R: 0})
	if len(d.View.Area.Saves) != 0 {
		t.Fatalf("nobody standing in the cone = %+v", d.View.Area)
	}
	fill(d.View.Area.DamageRollID, 1, 1, 1)
	next(t, tb.player)
	d = next(t, tb.dm)
	if surfaceAt(d.View, 3, 0) != "fire" || surfaceAt(d.View, 2, 0) != "" || surfaceAt(d.View, 0, 1) != "water" {
		t.Fatalf("fire lights the grease and burns the web away = %+v", d.View.Surfaces)
	}
	tb.dmSays(live.Command{Kind: live.CmdPaintSurface, Hexes: []live.Hex{{Q: 0, R: 0}}, Surface: "fire"})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(now, "Archer").ID})
	if surfaceAt(d.View, 0, 1) != "" || !strings.Contains(d.View.Manual[len(d.View.Manual)-1].Text, "Aria starts its turn in fire: 1d4 fire damage.") {
		t.Fatalf("a new round: the one-round water dries up and the fire under aria burns = %+v %+v", d.View.Surfaces, d.View.Manual)
	}
	u = tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 1, R: -1})
	if !strings.Contains(v().Manual[len(v().Manual)-1].Text, "Aria walks into fire") {
		t.Fatal("walking into fire hands the DM its damage")
	}
	tb.dmSays(live.Command{Kind: live.CmdPaintSurface, Hexes: []live.Hex{{Q: 2, R: -1}, {Q: 2, R: 0}}, Surface: "grease"})
	if u := tb.playerSays(live.Command{Kind: live.CmdPlanWalk, TokenID: ids["Aria"], Q: 2, R: -1}); u.Path == nil || u.Path.CostFt != 10 {
		t.Fatalf("grease is difficult ground = %+v", u)
	}
	u = tb.playerSays(cast(live.CmdCastArea, "grease", 4, -1))
	fill(u.View.Area.Saves[0].RollID, 2)
	next(t, tb.player)
	d = next(t, tb.dm)
	next(t, tb.player)
	d = next(t, tb.dm)
	if surfaceAt(d.View, 4, -1) != "grease" || effect(token(d.View, "Archer"), "Prone") == nil {
		t.Fatalf("grease covers the ground and trips the archer = %+v %+v", d.View.Surfaces, token(d.View, "Archer").Effects)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdPaintSurface, Hexes: []live.Hex{{Q: 3, R: -1}}})
	if surfaceAt(d.View, 3, -1) != "" {
		t.Fatal("cleared")
	}
}

func TestElevationHighGroundAndAreaEdges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	m := w.dungeon(t)
	if _, err := w.pool.Exec(ctx, "UPDATE campaign.campaigns SET high_ground = true WHERE id = $1", w.session.CampaignID); err != nil {
		t.Fatal(err)
	}
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdSetMap, MapID: uuid.UUID(m.ID).String()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Q: 1})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 2})
	ids := map[string]string{}
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
	}
	d, p := tb.dmSays(live.Command{Kind: live.CmdSetElevation, Hexes: []live.Hex{{Q: 1, R: 0}, {Q: 2, R: 1}}, ElevationFt: 10})
	if len(d.View.Elevation) != 2 || len(p.View.Elevation) == 0 {
		t.Fatalf("heights = dm %+v party %+v", d.View.Elevation, p.View.Elevation)
	}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdSetElevation, Hexes: []live.Hex{{Q: 1, R: 0}}, ElevationFt: 101})
	if u := next(t, tb.dm); !strings.Contains(u.Reason, "-100 to 100") {
		t.Fatalf("too high = %+v", u)
	}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdPlanWalk, TokenID: ids["Goblin"], Q: 2, R: 1})
	if u := next(t, tb.dm); u.Path == nil || u.Path.CostFt != 15 {
		t.Fatalf("climbing 10 ft costs 10 more = %+v", u)
	}
	tb.dmSays(live.Command{Kind: live.CmdSetElevation, Hexes: []live.Hex{{Q: 2, R: 1}}, ElevationFt: 0})
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdPlanWalk, TokenID: ids["Goblin"], Q: 2, R: 1})
	if u := next(t, tb.dm); u.Path == nil || u.Path.CostFt != 5 {
		t.Fatalf("flattened = %+v", u)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: []live.CombatantSetup{{TokenID: ids["Aria"], SpeedFt: 30}, {TokenID: ids["Goblin"], SpeedFt: 30}}})
	tb.roll(combatant(d.View, "Aria"), w.player, 20)
	tb.roll(combatant(d.View, "Goblin"), w.dm, 5)
	u := tb.playerSays(live.Command{Kind: live.CmdPreviewAttack, TokenID: ids["Aria"], TargetID: ids["Goblin"]})
	if u.Preview == nil || u.Preview.HitChance != 65 || !strings.Contains(strings.Join(u.Preview.Reasons, "|"), "High ground: +2 to hit") {
		t.Fatalf("attacking from higher ground = %+v", u.Preview)
	}
	tb.dmSays(live.Command{Kind: live.CmdPaintSurface, Hexes: []live.Hex{{Q: 0, R: 1}}, Surface: "ice", Rounds: 2})
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Aria").ID})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdCastArea, TokenID: ids["Goblin"], Effect: "thunderwave", Q: 1, R: 0})
	fill := func(rollID string, faces ...int) {
		t.Helper()
		for i, f := range faces {
			if _, err := rolls.SetDie(ctx, dmCaller, w.session.CampaignID, domain.RollID(uuid.MustParse(rollID)), i, app.Fill{Value: f}); err != nil {
				t.Fatal(err)
			}
		}
	}
	fill(d.View.Area.DamageRollID, 1, 1)
	fill(d.View.Area.Saves[0].RollID, 1)
	var last live.Update
	for range 4 {
		next(t, tb.player)
		last = next(t, tb.dm)
	}
	aria := token(last.View, "Aria")
	if len(last.View.Manual) != 1 || last.View.Manual[0].Text != "Aria falls 10 feet: 1d6 bludgeoning damage." || *aria.HP != 10 || aria.Q != 0 || aria.R != 0 || effect(aria, "Prone") == nil {
		t.Fatalf("thunderwave pushes Aria off the 10-foot rise, and she falls = %+v %+v", last.View.Manual, aria)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Goblin").ID})
	if surfaceAt(d.View, 0, 1) != "ice" {
		t.Fatalf("two-round ice lasts into round two = %+v", d.View.Surfaces)
	}
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Aria"], Q: 1, R: 0})
	u = tb.playerSays(live.Command{Kind: live.CmdCastArea, TokenID: ids["Aria"], Effect: "burning-hands", Q: 2, R: 0})
	if len(u.View.Area.Saves) != 1 {
		t.Fatalf("burning hands on the goblin = %+v", u.View.Area)
	}
	if _, p := tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: ids["Goblin"]}); len(p.View.Area.Saves) != 0 {
		t.Fatalf("a removed target leaves the area = %+v", p.View.Area)
	}
	if d, _ := tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: ids["Aria"]}); d.View.Area != nil {
		t.Fatalf("a removed caster drops the spell = %+v", d.View.Area)
	}
}
