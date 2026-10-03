package live_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
)

func sightOf(p *live.PathView, id string) *live.PathSight {
	for i := range p.Sight {
		if p.Sight[i].TokenID == id {
			return &p.Sight[i]
		}
	}
	return nil
}

// A planned walk comes back with what it would cost beyond movement: the opportunity attacks it would
// draw, exactly those the reactions engine then offers, and how each enemy would see the mover where
// the walk ends. A creature the party cannot see is in the DM's plan only.
func TestAPlannedWalkWarnsOfOpportunityAttacksAndShowsSightAndCover(t *testing.T) {
	t.Parallel()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Q: 0})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 1})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", Label: "Archer", TokenKind: domain.TokenEnemy, Q: 2})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", Label: "Lurker", TokenKind: domain.TokenEnemy, Q: -1, R: 1})
	tb.dmSays(live.Command{Kind: live.CmdSetVisibility, TokenID: token(d.View, "Lurker").ID, Qualities: []string{"invisible"}})
	ids := map[string]string{}
	var fighters []live.CombatantSetup
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
		fighters = append(fighters, live.CombatantSetup{TokenID: tv.ID, SpeedFt: 30})
	}
	plan := func(sub *live.Subscriber) *live.PathView {
		t.Helper()
		barrier(t, w, tb)
		w.hub.Submit(sub, live.Command{Kind: live.CmdPlanWalk, TokenID: ids["Aria"], Q: -2, R: 0})
		u := next(t, sub)
		if u.Kind != live.UpdPath || u.Path == nil {
			t.Fatalf("a planned walk = %+v", u)
		}
		return u.Path
	}

	if p := plan(tb.player); len(p.Threats) != 0 || len(p.Sight) != 2 {
		t.Fatalf("out of a fight nobody takes an opportunity attack: %+v", p)
	}

	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: fighters})
	for label, face := range map[string]int{"Aria": 20, "Goblin": 10, "Archer": 8, "Lurker": 5} {
		who := w.dm
		if label == "Aria" {
			who = w.player
		}
		tb.roll(combatant(d.View, label), who, face)
	}

	p := plan(tb.player)
	if p.CostFt != 10 || len(p.Threats) != 1 || p.Threats[0].TokenID != ids["Goblin"] || p.Threats[0].Label != "Goblin" || p.Threats[0].Q != 0 || p.Threats[0].R != 0 {
		t.Fatalf("the player's warnings = %+v", p.Threats)
	}
	if g, a := sightOf(p, ids["Goblin"]), sightOf(p, ids["Archer"]); len(p.Sight) != 2 || g == nil || !g.Visible || g.Cover != "none" || a == nil || !a.Visible || a.Cover != "half" {
		t.Fatalf("how the enemies see Aria at the end = %+v", p.Sight)
	}
	dm := plan(tb.dm)
	if len(dm.Threats) != 2 || dm.Threats[0].TokenID != ids["Goblin"] || dm.Threats[1].TokenID != ids["Lurker"] || dm.Threats[1].Q != -1 || sightOf(dm, ids["Lurker"]) == nil || len(dm.Sight) != 3 {
		t.Fatalf("the DM's plan names the unseen creature too = %+v %+v", dm.Threats, dm.Sight)
	}

	// The walk itself draws the opportunity attack the plan warned of first.
	barrier(t, w, tb)
	u := tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: -2, R: 0})
	if pr := u.View.Combat.Prompt; pr == nil || pr.Kind != domain.PromptOpportunity || pr.ReactorID != p.Threats[0].TokenID {
		t.Fatalf("the reactions engine offers = %+v, the plan warned of %+v", u.View.Combat.Prompt, p.Threats)
	}
}

// A plan tells its asker nothing they could not already see. A wall the party never saw does not show
// in a line of sight, and a DM who connects as the party gets the party's plan.
func TestAPlannedWalkKeepsWhatThePartyCannotSee(t *testing.T) {
	t.Parallel()
	w := setup(t)
	m := w.dungeon(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdSetMap, MapID: uuid.UUID(m.ID).String()})
	tb.dmSays(live.Command{Kind: live.CmdSetAmbient, Ambient: domain.AmbientDark})
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Q: 1, R: 2, DarkvisionFt: 5})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 5, R: 0})
	tb.dmSays(live.Command{Kind: live.CmdPlaceLight, Q: 5, R: 0, BrightFt: 5, DimFt: 5})
	tb.dmSays(live.Command{Kind: live.CmdSetWalls, Hexes: []live.Hex{{Q: 3, R: 0}}, On: true})
	d, p := tb.dmSays(live.Command{Kind: live.CmdRevealHexes, Hexes: []live.Hex{{Q: 1, R: 0}}, On: true})
	if token(p.View, "Goblin") == nil || has(p.View.Visible, 3, 0) {
		t.Fatalf("the party should see the lit goblin and not the wall in the dark: %+v", p.View.Visible)
	}
	aria, goblin := token(d.View, "Aria").ID, token(d.View, "Goblin").ID
	plan := func(sub *live.Subscriber) *live.PathView {
		t.Helper()
		barrier(t, w, tb)
		w.hub.Submit(sub, live.Command{Kind: live.CmdPlanWalk, TokenID: aria, Q: 1, R: 0})
		u := next(t, sub)
		if u.Kind != live.UpdPath || u.Path == nil {
			t.Fatalf("a planned walk = %+v", u)
		}
		return u.Path
	}
	if s := sightOf(plan(tb.dm), goblin); s == nil || s.Visible {
		t.Fatalf("the DM knows the wall stands between them: %+v", s)
	}
	if s := sightOf(plan(tb.player), goblin); s == nil || !s.Visible {
		t.Fatalf("the party's plan gives away a wall it never saw: %+v", s)
	}

	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", Label: "Lurker", TokenKind: domain.TokenEnemy, Q: 2, R: 1})
	lurker := token(look(t, w, tb.dm), "Lurker").ID
	tb.dmSays(live.Command{Kind: live.CmdSetVisibility, TokenID: lurker, Qualities: []string{"invisible"}})
	asParty := join(t, w, w.dm, dmCaller, live.AudienceParty)
	if got := plan(asParty); sightOf(got, lurker) != nil || sightOf(plan(tb.dm), lurker) == nil {
		t.Fatalf("a DM connected as the party is told of a creature the party cannot see: %+v", got.Sight)
	}
}
