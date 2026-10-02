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

// summonTable stores summoning Effects before anyone joins, then puts Aria at the centre and a goblin
// three hexes east.
func summonTable(t *testing.T) (world, *table, map[string]string) {
	t.Helper()
	ctx := context.Background()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	for _, d := range []effects.Definition{
		{Slug: "summon-wolf", Name: "Summon Wolf", Owner: effects.OwnedBySpell, Concentration: true, Components: []effects.Component{
			effects.Summon{Monster: "wolf", Count: 1, Shares: true, NeedsCommand: true},
		}},
		{Slug: "wolf-pack", Name: "Wolf Pack", Owner: effects.OwnedBySpell, Components: []effects.Component{
			effects.Summon{Monster: "wolf", Count: 2, Shares: false, NeedsCommand: false},
		}},
		{Slug: "outlining-burst", Name: "Outlining Burst", Owner: effects.OwnedBySpell, Components: []effects.Component{
			effects.Area{Shape: hex.SphereArea, SizeFt: 5, RangeFt: 60},
			effects.SaveDamage{Ability: "dexterity", Dice: "1d4", Type: "radiant", Half: true},
			effects.Reveal{Qualities: []string{"invisible", "disguised"}},
		}},
		{Slug: "summon-owl", Name: "Summon Owl", Owner: effects.OwnedBySpell, Components: []effects.Component{
			effects.Summon{Monster: "owl", Count: 1, Shares: true, NeedsCommand: false},
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

// A summon that shares its caster's turn acts alongside it, Dodges until commanded with the caster's
// Bonus Action, ends its turn with the caster, and leaves when the caster's concentration ends.
func TestSummonsShareTheTurnAndWaitForCommands(t *testing.T) {
	t.Parallel()
	w, tb, ids := summonTable(t)
	tb.fight(ids, "Aria", "Goblin")
	summon := func(slug string) live.Command {
		return live.Command{Kind: live.CmdSummon, TokenID: ids["Aria"], Effect: slug, Q: 1, R: 1}
	}
	bad := summon("summon-wolf")
	bad.Q = 99
	for want, cmd := range map[string]live.Command{
		"not a summoning":    summon("bless"),
		"off the map":        bad,
		"no such creature":   summon("summon-owl"),
		"not yours to play":  {Kind: live.CmdSummon, TokenID: ids["Goblin"], Effect: "summon-wolf"},
		"command a creature": {Kind: live.CmdCommand, TokenID: ids["Aria"], TargetID: ids["Goblin"]},
	} {
		if u := tb.playerSays(cmd); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), want) {
			t.Errorf("%s = %+v", want, u)
		}
	}
	u := tb.playerSays(summon("summon-wolf"))
	wolf, aria := combatant(u.View, "Wolf"), combatant(u.View, "Aria")
	if wolf == nil || wolf.OwnerID != aria.ID || !wolf.Acting || !wolf.AwaitingCommand || *wolf.Initiative != *aria.Initiative || aria.Action {
		t.Fatalf("the wolf joins on Aria's turn = %+v %+v", wolf, aria)
	}
	if tv := token(u.View, "Wolf"); tv.Kind != domain.TokenParty || tv.ControllerID != w.player.ID.String() || effect(token(u.View, "Aria"), "Summon Wolf") == nil {
		t.Fatalf("the wolf is the party's and Aria concentrates = %+v", tv)
	}
	ids["Wolf"] = token(u.View, "Wolf").ID
	if u := tb.playerSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Wolf"], TargetID: ids["Goblin"]}); !strings.Contains(u.Reason, "Dodge action until it is commanded") {
		t.Fatalf("an uncommanded wolf attacks = %+v", u)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdTakeAction, TokenID: ids["Wolf"], Action: "dash"}); !strings.Contains(u.Reason, "until it is commanded") {
		t.Fatalf("an uncommanded wolf dashes = %+v", u)
	}
	u = tb.playerSays(live.Command{Kind: live.CmdCommand, TokenID: ids["Aria"], TargetID: ids["Wolf"]})
	if combatant(u.View, "Wolf").AwaitingCommand || combatant(u.View, "Aria").BonusAction {
		t.Fatalf("commanded = %+v %+v", combatant(u.View, "Wolf"), combatant(u.View, "Aria"))
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdCommand, TokenID: ids["Aria"], TargetID: ids["Wolf"]}); !strings.Contains(u.Reason, "Bonus Action") {
		t.Fatalf("a second command = %+v", u)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdTakeAction, TokenID: ids["Wolf"], Action: "dash"}); u.Kind != live.UpdView {
		t.Fatalf("a commanded wolf dashes = %+v", u)
	}
	u = tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: aria.ID})
	if !combatant(u.View, "Wolf").Done || !combatant(u.View, "Goblin").Acting {
		t.Fatalf("the wolf's turn ends with Aria's = %+v", u.View.Combat)
	}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdCommand, TokenID: ids["Aria"], TargetID: ids["Wolf"]})
	if u := next(t, tb.dm); !strings.Contains(u.Reason, "not Aria's turn") {
		t.Fatalf("off turn = %+v", u)
	}
	d, _ := tb.dmSays(live.Command{Kind: live.CmdEndEffect, EffectID: effect(token(look(t, w, tb.dm), "Aria"), "Summon Wolf").ID})
	if token(d.View, "Wolf") != nil || combatant(d.View, "Wolf") != nil {
		t.Fatalf("the wolf leaves with the concentration = %+v", d.View.Tokens)
	}
}

// A summon that rolls its own Initiative opens a Roll Card per creature; removing the caster sends its
// summons away.
func TestSummonsWithTheirOwnInitiative(t *testing.T) {
	t.Parallel()
	w, tb, ids := summonTable(t)
	w.hub.Submit(tb.player, live.Command{Kind: live.CmdCommand, TokenID: ids["Aria"], TargetID: ids["Goblin"]})
	if u := next(t, tb.player); !strings.Contains(u.Reason, "only in combat") {
		t.Fatalf("a command out of combat = %+v", u)
	}
	tb.fight(ids, "Aria", "Goblin")
	u := tb.playerSays(live.Command{Kind: live.CmdSummon, TokenID: ids["Aria"], Effect: "wolf-pack", Q: 0, R: 2})
	one, two := combatant(u.View, "Wolf 1"), combatant(u.View, "Wolf 2")
	if one == nil || two == nil || one.Initiative != nil || one.RollID == combatant(u.View, "Aria").RollID || one.AwaitingCommand {
		t.Fatalf("each wolf rolls = %+v %+v", one, two)
	}
	tb.roll(one, w.player, 5)
	tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: ids["Aria"]})
	v := look(t, w, tb.dm)
	if token(v, "Wolf 1") != nil || token(v, "Wolf 2") != nil || combatant(v, "Wolf 1") != nil {
		t.Fatalf("the pack leaves with Aria = %+v", v.Tokens)
	}
}

// Out of combat a summon simply appears.
func TestSummoningOutOfCombat(t *testing.T) {
	t.Parallel()
	_, tb, ids := summonTable(t)
	u := tb.playerSays(live.Command{Kind: live.CmdSummon, TokenID: ids["Aria"], Effect: "wolf-pack", Q: 0, R: 0})
	if token(u.View, "Wolf 1") == nil || token(u.View, "Wolf 2") == nil || u.View.Combat != nil {
		t.Fatalf("the pack appears = %+v", u.View.Tokens)
	}
}
