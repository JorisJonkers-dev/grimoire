//go:build integration

package live_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/variants"
)

// switched sets a Rule Variant of the Campaign, as the DM does outside the Session.
func switched(t *testing.T, w world, slug, value string) {
	t.Helper()
	if _, err := w.pool.Exec(context.Background(), `INSERT INTO campaign.rule_variants (campaign_id, variant, value, updated_at) VALUES ($1, $2, $3, now())
		ON CONFLICT (campaign_id, variant) DO UPDATE SET value = excluded.value`, w.session.CampaignID, slug, value); err != nil {
		t.Fatal(err)
	}
}

type unreadableVariants struct{ live.Store }

func (unreadableVariants) RuleVariants(context.Context, uuid.UUID) (variants.Set, error) {
	return variants.Set{variants.Flanking: variants.On, variants.CriticalHits: variants.CritMaxDice, variants.ShortRestCap: "1"}, errors.New("no such table")
}

type uncountedRests struct{ live.Store }

func (uncountedRests) ShortRests(context.Context, uuid.UUID) (int, error) {
	return 0, errors.New("no such column")
}

// With flanking on, a melee attack has Advantage when an ally who can act stands straight across the
// target; with the critical hit variant, a Critical Hit rolls its dice once and adds their most. The
// Session under way follows the switches as the DM changes them.
func TestFlankingAndTheCriticalHitVariant(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids := magicTable(t)
	for _, c := range []live.Command{
		{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Raider", Q: 1},
		{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenParty, Label: "Brom", Q: 2},
		{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenParty, Label: "Cara", Q: 1, R: -1},
		{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Sneak", Q: -1},
		{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Lurker", Q: -2},
	} {
		d, _ := tb.dmSays(c)
		ids[c.Label] = token(d.View, c.Label).ID
	}
	tb.fight(ids, "Aria", "Raider", "Brom", "Cara", "Sneak")
	preview := func(sub *live.Subscriber, attacker string, no int, target string) *live.AttackPreview {
		t.Helper()
		w.hub.Submit(sub, live.Command{Kind: live.CmdPreviewAttack, TokenID: ids[attacker], AttackNo: no, TargetID: ids[target]})
		u := next(t, sub)
		if u.Preview == nil {
			t.Fatalf("%s at %s: %+v", attacker, target, u)
		}
		return u.Preview
	}
	flanked := func(p *live.AttackPreview) string {
		for _, r := range p.Reasons {
			if strings.Contains(r, "flanking") {
				return r
			}
		}
		return ""
	}
	// Without the variant, an ally across the target is only an ally.
	if p := preview(tb.player, "Aria", 0, "Raider"); p.Mode != "normal" || flanked(p) != "" {
		t.Fatalf("without the variant = %+v", p)
	}
	switched(t, w, variants.Flanking, variants.On)
	if p := preview(tb.player, "Aria", 0, "Raider"); p.Mode != "advantage" || flanked(p) != "Advantage: flanking with Brom" {
		t.Fatalf("with Brom across the Raider = %+v", p)
	}
	// A ranged attack flanks nobody, and an enemy across the target is no ally.
	if p := preview(tb.player, "Aria", 1, "Raider"); flanked(p) != "" {
		t.Fatalf("a ranged attack = %+v", p)
	}
	if p := preview(tb.player, "Aria", 0, "Sneak"); p.Mode != "normal" || flanked(p) != "" {
		t.Fatalf("at the Sneak, with the Lurker across it = %+v", p)
	}
	// An ally who is down flanks nobody, though another ally stands next to the target, off the line.
	tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Brom"], HPDelta: -999})
	if p := preview(tb.player, "Aria", 0, "Raider"); p.Mode != "normal" || flanked(p) != "" {
		t.Fatalf("with Brom down = %+v", p)
	}
	tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Brom"], HPDelta: 999})
	// When the variants cannot be read, the attack plays as the rules do without them.
	w.hub.Close(w.session.ID)
	kept := w.hub.Store
	w.hub.Store = unreadableVariants{Store: kept}
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	if p := preview(tb.player, "Aria", 0, "Raider"); p.Mode != "normal" || flanked(p) != "" {
		t.Fatalf("with the variants unreadable = %+v", p)
	}
	w.hub.Close(w.session.ID)
	w.hub.Store = kept
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)

	// The attack rolls with Advantage, and its Roll Card says why.
	fill := func(rollID string, who domain.Member, faces ...int) {
		t.Helper()
		cl := dmCaller
		if !who.DM {
			cl = playerCaller
		}
		for i, f := range faces {
			if _, err := tb.rolls.SetDie(ctx, cl, w.session.CampaignID, domain.RollID(uuid.MustParse(rollID)), i, app.Fill{Value: f}); err != nil {
				t.Fatal(err)
			}
		}
		next(t, tb.dm)
		tb.party = append(tb.party, next(t, tb.player))
	}
	rolled := func(id string) domain.Roll {
		t.Helper()
		roll, err := pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(id)))
		if err != nil {
			t.Fatal(err)
		}
		return roll
	}
	u := tb.playerSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Aria"], TargetID: ids["Raider"]})
	if u.View == nil || u.View.Combat.Attack == nil {
		t.Fatalf("the attack = %+v", u)
	}
	if roll := rolled(u.View.Combat.Attack.RollID); roll.Notation != "2d20kh1" {
		t.Fatalf("a flanking attack rolls %s", roll.Notation)
	}
	// A natural 20: with the variant the dice are rolled once, and their most is added.
	switched(t, w, variants.CriticalHits, variants.CritMaxDice)
	fill(u.View.Combat.Attack.RollID, w.player, 20, 3)
	a := tb.party[len(tb.party)-1].View.Combat.Attack
	if a == nil || !a.Critical || a.Stage != domain.StageDamage {
		t.Fatalf("a natural 20 = %+v", a)
	}
	dmg := rolled(a.RollID)
	most := slices.IndexFunc(dmg.Modifiers, func(m domain.Modifier) bool { return m.Label == "Critical Hit: the dice at their most" })
	if dmg.Notation != "1d8" || !strings.HasSuffix(dmg.Purpose, "(critical)") || most < 0 || dmg.Modifiers[most].Value != 8 || len(dmg.Modifiers) != 2 {
		t.Fatalf("critical damage with the variant = %s %+v", dmg.Notation, dmg.Modifiers)
	}
	before := *token(look(t, w, tb.dm), "Raider").HP
	fill(a.RollID, w.player, 2)
	if hp := *token(look(t, w, tb.dm), "Raider").HP; before-hp != min(before, 2+8+3) {
		t.Fatalf("2 on the die, 8 at its most and 3: %d to %d", before, hp)
	}

	// The enemy flanks too: the Raider and the Sneak stand either side of Aria.
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(tb.party[len(tb.party)-1].View, "Aria").ID})
	if p := preview(tb.dm, "Raider", 0, "Aria"); p.Mode != "advantage" || flanked(p) != "Advantage: flanking with Sneak" {
		t.Fatalf("the Raider with the Sneak across Aria = %+v", p)
	}
	// Switched off again, the Session under way follows.
	switched(t, w, variants.Flanking, variants.Off)
	if p := preview(tb.dm, "Raider", 0, "Aria"); p.Mode != "normal" || flanked(p) != "" {
		t.Fatalf("switched off = %+v", p)
	}
	// Without the critical variant every die is rolled twice.
	switched(t, w, variants.CriticalHits, variants.CritDoubleDice)
	d, _ := tb.dmSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Raider"], TargetID: ids["Aria"]})
	fill(d.View.Combat.Attack.RollID, w.dm, 20)
	a = tb.party[len(tb.party)-1].View.Combat.Attack
	if dmg := rolled(a.RollID); !a.Critical || dmg.Notation != "2d6" || slices.ContainsFunc(dmg.Modifiers, func(m domain.Modifier) bool { return strings.Contains(m.Label, "Critical") }) {
		t.Fatalf("critical damage without the variant = %s %+v", dmg.Notation, dmg.Modifiers)
	}
}

// Rest variants: how long a rest takes, how many Short Rests fit between two Long Rests, and whether
// a Long Rest heals. Each is read when it matters, so a Session under way follows the DM's switches.
// restTable is a resting party and the time its rests take on the Game Clock.
type restTable struct {
	t     *testing.T
	w     world
	tb    *table
	ids   map[string]string
	start int
}

func newRestTable(t *testing.T) *restTable {
	t.Helper()
	w, tb, ids := restingParty(t)
	r := &restTable{t: t, w: w, tb: tb, ids: ids}
	r.start, _ = r.clock()
	return r
}

// clock is the minutes since the start on the Game Clock, and the Short Rests taken since the last
// Long Rest.
func (r *restTable) clock() (int, int) {
	r.t.Helper()
	var day, minute, short int
	if err := r.w.pool.QueryRow(context.Background(), "SELECT game_day, game_minute, short_rests FROM campaign.campaigns WHERE id = $1", r.w.session.CampaignID).Scan(&day, &minute, &short); err != nil {
		r.t.Fatal(err)
	}
	return day*1440 + minute - r.start, short
}

func (r *restTable) rest(kind string) {
	r.t.Helper()
	if p := r.tb.playerSays(live.Command{Kind: live.CmdProposeRest, Rest: kind}); p.View == nil {
		r.t.Fatalf("a %s rest was refused: %+v", kind, p)
	}
	r.tb.dmSays(live.Command{Kind: live.CmdAgreeRest})
}

func (r *restTable) restAndFinish(kind string) {
	r.t.Helper()
	r.rest(kind)
	r.tb.dmSays(live.Command{Kind: live.CmdFinishRest})
}

func (r *restTable) refused(want string, sub *live.Subscriber, cmd live.Command) {
	r.t.Helper()
	r.w.hub.Submit(sub, cmd)
	if u := next(r.t, sub); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, want) {
		r.t.Fatalf("%q = %+v", want, u)
	}
}

func (r *restTable) hp() int {
	r.t.Helper()
	return *tokenByID(r.t, look(r.t, r.w, r.tb.dm), r.ids["aria-token"]).HP
}

var proposeShort = live.Command{Kind: live.CmdProposeRest, Rest: live.RestShort}

func TestAStandardShortRestTakesAnHourAndCounts(t *testing.T) {
	t.Parallel()
	r := newRestTable(t)
	r.restAndFinish(live.RestShort)
	if now, short := r.clock(); now != 60 || short != 1 {
		t.Fatalf("%d minutes on, %d taken", now, short)
	}
}

func TestAShortRestPastTheCapWaitsForALongRest(t *testing.T) {
	t.Parallel()
	r := newRestTable(t)
	r.restAndFinish(live.RestShort)
	switched(t, r.w, variants.ShortRestCap, "1")
	r.refused("Long Rest first", r.tb.player, proposeShort)
}

// grittyTwice is two Short Rests taken under a cap of two, the second one gritty.
func (r *restTable) grittyTwice() {
	r.t.Helper()
	r.restAndFinish(live.RestShort)
	switched(r.t, r.w, variants.ShortRestCap, "2")
	switched(r.t, r.w, variants.Rests, variants.RestsGritty)
	r.restAndFinish(live.RestShort)
}

func TestAGrittyShortRestTakesEightHours(t *testing.T) {
	t.Parallel()
	r := newRestTable(t)
	r.grittyTwice()
	if now, short := r.clock(); now != 60+480 || short != 2 {
		t.Fatalf("%d minutes on, %d taken", now, short)
	}
}

func TestPastTheCapNotEvenTheDMProposesAShortRest(t *testing.T) {
	t.Parallel()
	r := newRestTable(t)
	r.grittyTwice()
	r.refused("Long Rest first", r.tb.dm, proposeShort)
}

// slowLongRest is a gritty Long Rest with slow natural healing, in which Aria spends a Hit Die and
// rolls a 1; it returns her hit points before it.
func (r *restTable) slowLongRest() int {
	r.t.Helper()
	switched(r.t, r.w, variants.Rests, variants.RestsGritty)
	switched(r.t, r.w, variants.SlowNaturalHealing, variants.On)
	hurt := r.hp()
	r.rest(live.RestLong)
	p := r.tb.playerSays(live.Command{Kind: live.CmdSpendHitDie, TokenID: r.ids["aria-token"]})
	if p.View == nil {
		r.t.Fatalf("a Hit Die in the Long Rest: %+v", p)
	}
	r.tb.fill(resterNamed(r.t, p.View.Rest, r.ids["Aria"]).RollID, r.w.player, 1)
	next(r.t, r.tb.dm)
	next(r.t, r.tb.player)
	r.tb.dmSays(live.Command{Kind: live.CmdFinishRest})
	return hurt
}

func TestWithSlowNaturalHealingALongRestHealsOnlyByTheHitDiceSpent(t *testing.T) {
	t.Parallel()
	r := newRestTable(t)
	hurt := r.slowLongRest()
	if got := r.hp(); got != hurt+1+2 {
		t.Fatalf("Aria is on %d, from %d and a Hit Die of 1 + 2", got, hurt)
	}
}

func TestAGrittyLongRestTakesAWeekGivesTheHitDiceBackAndStartsTheCountAgain(t *testing.T) {
	t.Parallel()
	r := newRestTable(t)
	r.restAndFinish(live.RestShort)
	hurt := r.slowLongRest()
	var spent, kept int
	if err := r.w.pool.QueryRow(context.Background(), "SELECT hit_dice_spent, hp_current FROM campaign.characters WHERE id = $1", r.ids["Aria"]).Scan(&spent, &kept); err != nil {
		t.Fatal(err)
	}
	if now, short := r.clock(); now != 60+7*1440 || short != 0 || spent != 0 || kept != hurt+3 {
		t.Fatalf("%d minutes on, %d Short Rests, %d Hit Dice spent, %d hit points kept", now, short, spent, kept)
	}
}

func TestWithoutSlowNaturalHealingNoHitDieIsSpentInALongRest(t *testing.T) {
	t.Parallel()
	r := newRestTable(t)
	r.rest(live.RestLong)
	r.refused("in a Short Rest", r.tb.player, live.Command{Kind: live.CmdSpendHitDie, TokenID: r.ids["aria-token"]})
}

func TestAnEpicLongRestTakesAnHourAndHeals(t *testing.T) {
	t.Parallel()
	r := newRestTable(t)
	switched(t, r.w, variants.Rests, variants.RestsEpic)
	hurt := r.hp()
	r.restAndFinish(live.RestLong)
	if now, _ := r.clock(); now != 60 || r.hp() <= hurt {
		t.Fatalf("%d minutes on, Aria on %d from %d", now, r.hp(), hurt)
	}
}

func TestAnEpicShortRestTakesFiveMinutes(t *testing.T) {
	t.Parallel()
	r := newRestTable(t)
	switched(t, r.w, variants.Rests, variants.RestsEpic)
	r.restAndFinish(live.RestShort)
	if now, short := r.clock(); now != 5 || short != 1 {
		t.Fatalf("%d minutes on, %d taken", now, short)
	}
}

// rejoinOver restarts the Session over store.
func (r *restTable) rejoinOver(store func(whole live.Store) live.Store) {
	r.t.Helper()
	r.w.hub.Close(r.w.session.ID)
	r.w.hub.Store = store(r.w.hub.Store)
	r.tb.dm, r.tb.player = join(r.t, r.w, r.w.dm, dmCaller, live.AudienceDM), join(r.t, r.w, r.w.player, playerCaller, live.AudienceParty)
}

func TestWhenTheVariantsCannotBeReadNoRestIsProposedOrFinished(t *testing.T) {
	t.Parallel()
	r := newRestTable(t)
	r.restAndFinish(live.RestShort)
	r.rest(live.RestLong)
	r.rejoinOver(func(whole live.Store) live.Store { return unreadableVariants{Store: whole} })
	r.refused("could not be read", r.tb.dm, live.Command{Kind: live.CmdFinishRest})
	r.refused("could not be read", r.tb.player, live.Command{Kind: live.CmdSpendHitDie, TokenID: r.ids["aria-token"]})
	r.tb.dmSays(live.Command{Kind: live.CmdInterruptRest})
	r.refused("could not be read", r.tb.player, proposeShort)
	if now, short := r.clock(); now != 60 || short != 1 {
		t.Fatalf("%d minutes on, %d taken", now, short)
	}
}

func TestWhenTheCountOfShortRestsCannotBeReadNoRestIsProposedOrFinished(t *testing.T) {
	t.Parallel()
	r := newRestTable(t)
	r.rejoinOver(func(whole live.Store) live.Store { return uncountedRests{Store: whole} })
	r.refused("could not be read", r.tb.player, proposeShort)
	r.rest(live.RestLong)
	r.refused("could not be read", r.tb.dm, live.Command{Kind: live.CmdFinishRest})
}
