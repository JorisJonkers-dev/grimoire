package live_test

import (
	"context"
	"strings"
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

// The SRD conditions do what the rules say at the table: a paralysed goblin is struck critically from
// close by, can neither act nor move, and fails its Strength save unrolled; a restrained archer saves
// with disadvantage; exhaustion stacks, slows and penalises, and kills at six; and an incapacitated
// caster loses concentration.
func TestConditionsPlayOutAtTheTable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 1})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Archer", Q: 4})
	ids := map[string]string{}
	var setup []live.CombatantSetup
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
		setup = append(setup, live.CombatantSetup{TokenID: tv.ID, SpeedFt: 30})
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: setup})
	for label, face := range map[string]int{"Aria": 20, "Goblin": 10, "Archer": 5} {
		who := w.dm
		if label == "Aria" {
			who = w.player
		}
		tb.roll(combatant(d.View, label), who, face)
	}
	put := func(target, slug, source string, extra live.Command) (live.Update, live.Update) {
		t.Helper()
		extra.Kind, extra.TargetID, extra.Effect = live.CmdApplyEffect, ids[target], slug
		if source != "" {
			extra.SourceID = ids[source]
		}
		return tb.dmSays(extra)
	}
	refuse := func(sub *live.Subscriber, want string, cmd live.Command) {
		t.Helper()
		w.hub.Submit(sub, cmd)
		if u := next(t, sub); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, want) {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	fill := func(rollID string, who domain.Member, faces ...int) {
		t.Helper()
		tb.fill(rollID, who, faces...)
		next(t, tb.dm)
		tb.party = append(tb.party, next(t, tb.player))
	}

	put("Goblin", "paralyzed", "", live.Command{SaveAbility: "strength", SaveDC: 12})
	put("Archer", "restrained", "", live.Command{SaveAbility: "dexterity", SaveDC: 10})
	pv := tb.playerSays(live.Command{Kind: live.CmdPreviewAttack, TokenID: ids["Aria"], AttackNo: 0, TargetID: ids["Goblin"]}).Preview
	if pv.Mode != "advantage" || !strings.Contains(strings.Join(pv.Reasons, "|"), "Paralyzed: a hit from this close is a Critical Hit") {
		t.Fatalf("striking the paralysed goblin = %+v", pv)
	}
	u := tb.playerSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Aria"], TargetID: ids["Goblin"]})
	fill(u.View.Combat.Attack.RollID, w.player, 12, 3)
	dmg, _ := pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(tb.party[len(tb.party)-1].View.Combat.Attack.RollID)))
	if dmg.Notation != "2d8" || !strings.HasSuffix(dmg.Purpose, "(critical)") {
		t.Fatalf("a plain hit from 5 ft is a Critical Hit = %+v", dmg)
	}
	fill(uuid.UUID(dmg.ID).String(), w.player, 1, 1)

	put("Aria", "exhaustion", "", live.Command{})
	_, p := put("Aria", "exhaustion", "", live.Command{})
	if e := effect(token(p.View, "Aria"), "Exhaustion"); e == nil || e.Level != 2 || len(token(p.View, "Aria").Effects) != 1 {
		t.Fatalf("exhaustion stacks into one effect = %+v", token(p.View, "Aria").Effects)
	}

	d, _ = tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "bless", SourceID: ids["Aria"]})
	_, p = put("Aria", "stunned", "", live.Command{})
	if effect(token(p.View, "Aria"), "Bless") != nil {
		t.Fatalf("a stunned caster drops concentration = %+v", token(p.View, "Aria").Effects)
	}
	tb.dmSays(live.Command{Kind: live.CmdEndEffect, EffectID: effect(token(p.View, "Aria"), "Stunned").ID})
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Aria").ID})

	refuse(tb.dm, "Goblin can't act while Incapacitated.", live.Command{Kind: live.CmdSpend, CombatantID: combatant(d.View, "Goblin").ID, Resource: live.ResourceAction})
	refuse(tb.dm, "Goblin can't move.", live.Command{Kind: live.CmdWalk, TokenID: ids["Goblin"], Q: 2, R: -1})
	refuse(tb.dm, "Goblin can't act while Incapacitated.", live.Command{Kind: live.CmdAttack, TokenID: ids["Goblin"], TargetID: ids["Aria"]})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Goblin").ID})
	if len(d.View.Saves) != 0 {
		t.Fatalf("a paralysed creature fails its Strength save without a roll = %+v", d.View.Saves)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Archer").ID})
	if len(d.View.Saves) != 1 {
		t.Fatalf("the archer saves = %+v", d.View.Saves)
	}
	save, _ := pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(d.View.Saves[0].RollID)))
	if save.Notation != "2d20kl1" {
		t.Fatalf("a restrained Dexterity save has disadvantage = %+v", save)
	}
	if aria := combatant(d.View, "Aria"); aria.MovementFt != 20 {
		t.Fatalf("two levels of exhaustion take 10 ft = %+v", aria)
	}
	pv = tb.playerSays(live.Command{Kind: live.CmdPreviewAttack, TokenID: ids["Aria"], AttackNo: 1, TargetID: ids["Archer"]}).Preview
	if !strings.Contains(strings.Join(pv.Reasons, "|"), "Exhaustion 2: -4 to hit") || pv.Mode != "advantage" {
		t.Fatalf("an exhausted archer = %+v", pv)
	}
	for range 3 {
		put("Aria", "exhaustion", "", live.Command{})
	}
	d, _ = put("Aria", "exhaustion", "", live.Command{})
	if hp := token(d.View, "Aria").HP; hp == nil || *hp != 0 || d.View.Manual[len(d.View.Manual)-1].Text != "Aria dies of Exhaustion." {
		t.Fatalf("six levels kill = %v %+v", hp, d.View.Manual)
	}
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	if e := effect(token(look(t, w, tb.dm), "Aria"), "Exhaustion"); e == nil || e.Level != 6 || *token(look(t, w, tb.dm), "Aria").HP != 0 {
		t.Fatalf("levels and death survive a restart = %+v", e)
	}
}
