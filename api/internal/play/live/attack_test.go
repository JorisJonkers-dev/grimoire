package live_test

import (
	"context"
	"errors"
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
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// bestiary hands out fixed statblocks: a goblin, and Aria for any Character, owned by owner.
type bestiary struct{ owner uuid.UUID }

func (bestiary) Monster(_ context.Context, _ uuid.UUID, slug string) (string, domain.Stats, error) {
	switch slug {
	case "hobgoblin":
		return "Hobgoblin", domain.Stats{Source: "monster:hobgoblin", AC: 15, HP: 11, HPMax: 11, Intelligence: 12, Attacks: []domain.Attack{
			{Name: "Longsword", ToHit: 3, ReachFt: 5, Damage: "1d8", DamageBonus: 1, DamageType: "slashing"},
			{Name: "Longbow", ToHit: 3, RangeFt: 150, LongRangeFt: 600, Damage: "1d8", DamageBonus: 1, DamageType: "piercing"},
		}}, nil
	case "wolf":
		return "Wolf", domain.Stats{Source: "monster:wolf", AC: 13, HP: 11, HPMax: 11, Intelligence: 3, Attacks: []domain.Attack{
			{Name: "Bite", ToHit: 4, ReachFt: 5, Damage: "2d4", DamageBonus: 2, DamageType: "piercing"},
		}}, nil
	case "goblin":
	default:
		return "", domain.Stats{}, errors.New("no such monster")
	}
	return "Goblin", domain.Stats{Source: "monster:goblin", AC: 15, HP: 7, HPMax: 7, Intelligence: 10, Stealth: 6, Perception: -1, Initiative: 2, SpeedFt: 30, UnarmedDC: 12, Attacks: []domain.Attack{
		{Name: "Scimitar", ToHit: 4, ReachFt: 5, Damage: "1d6", DamageBonus: 2, DamageType: "slashing"},
		{Name: "Shortbow", ToHit: 4, RangeFt: 80, LongRangeFt: 320, Damage: "1d6", DamageBonus: 2, DamageType: "piercing"},
		{Name: "Slam", ToHit: 4, ReachFt: 5, DamageBonus: 3, DamageType: "bludgeoning"},
		{Name: "Dart", ToHit: 4, RangeFt: 10, LongRangeFt: 30, Damage: "1d4", DamageType: "piercing"},
	}}, nil
}

func (b bestiary) Character(_ context.Context, _ caller.Caller, _, id uuid.UUID) (string, uuid.UUID, domain.Stats, error) {
	if id == uuid.Nil {
		return "", uuid.UUID{}, domain.Stats{}, errors.New("no such character")
	}
	return "Aria", b.owner, domain.Stats{Source: "character:" + id.String(), AC: 16, HP: 12, HPMax: 12, SpellDC: 14, Perception: 3, Initiative: 2, SpeedFt: 30, Saves: map[string]int{"constitution": 2}, Attacks: []domain.Attack{
		{Name: "Longsword", ToHit: 5, ReachFt: 5, Damage: "1d8", DamageBonus: 3, DamageType: "slashing"},
		{Name: "Longbow", ToHit: 4, RangeFt: 150, LongRangeFt: 600, Damage: "1d8", DamageBonus: 2, DamageType: "piercing"},
	}}, nil
}

func TestAttacksFromPreviewToDamageAndUndo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	for name, cmd := range map[string]live.Command{
		"No such character.": {Kind: live.CmdPlace, CharacterID: "nope"},
		"No such monster.":   {Kind: live.CmdPlace, MonsterSlug: "dragon", TokenKind: domain.TokenEnemy},
	} {
		w.hub.Submit(tb.dm, cmd)
		if u := next(t, tb.dm); u.Reason != name {
			t.Errorf("%s = %+v", name, u)
		}
	}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 1})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Archer", Q: 4})
	d, p := tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Crate", TokenKind: domain.TokenObject, Q: 0, R: 1})
	aria, gob := token(p.View, "Aria"), token(p.View, "Goblin")
	if aria.Kind != domain.TokenParty || aria.ControllerID != w.player.ID.String() || *aria.HP != 12 || len(aria.Attacks) != 2 {
		t.Fatalf("a character's token = %+v", aria)
	}
	if gob.HP != nil || gob.AC != nil || gob.Attacks != nil || gob.Health != "unhurt" || *token(d.View, "Goblin").AC != 15 {
		t.Fatalf("a monster to the party = %+v, to the DM = %+v", gob, token(d.View, "Goblin"))
	}
	ids := map[string]string{}
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
	}
	aim := func(who, attacker string, no int, target string) live.Command {
		return live.Command{Kind: who, TokenID: ids[attacker], AttackNo: no, TargetID: ids[target]}
	}
	if u := tb.playerSays(aim(live.CmdPreviewAttack, "Aria", 0, "Goblin")); !strings.Contains(u.Reason, "in combat") {
		t.Fatalf("attack out of combat = %+v", u)
	}

	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: []live.CombatantSetup{
		{TokenID: ids["Aria"], SpeedFt: 30}, {TokenID: ids["Goblin"], SpeedFt: 30}, {TokenID: ids["Archer"], SpeedFt: 30},
	}})
	for label, face := range map[string]int{"Aria": 20, "Goblin": 5, "Archer": 3} {
		who := w.dm
		if label == "Aria" {
			who = w.player
		}
		tb.roll(combatant(d.View, label), who, face)
	}

	u := tb.playerSays(aim(live.CmdPreviewAttack, "Aria", 0, "Goblin"))
	if pv := u.Preview; u.Kind != live.UpdAttackPreview || pv.HitChance != 55 || pv.DamageMin != 4 || pv.DamageMax != 11 || pv.CritMax != 19 || pv.Mode != "normal" || pv.Name != "Longsword" {
		t.Fatalf("melee preview = %+v", u.Preview)
	}
	u = tb.playerSays(aim(live.CmdPreviewAttack, "Aria", 1, "Archer"))
	if pv := u.Preview; pv.Mode != "disadvantage" || pv.HitChance != 16 || len(pv.Reasons) != 3 || !strings.Contains(pv.Reasons[1], "Cover: +2") {
		t.Fatalf("a shot past the goblin = %+v", u.Preview)
	}
	for want, cmd := range map[string]live.Command{
		"out of range":         aim(live.CmdPreviewAttack, "Aria", 0, "Archer"),
		"choose a creature":    aim(live.CmdPreviewAttack, "Aria", 0, "Crate"),
		"no such attack":       aim(live.CmdPreviewAttack, "Aria", 5, "Goblin"),
		"not yours to play":    aim(live.CmdAttack, "Goblin", 0, "Aria"),
		"not goblin's turn":    {Kind: live.CmdPreviewAttack, TokenID: ids["Goblin"], TargetID: ids["Aria"]},
		"choose a creature to": aim(live.CmdPreviewAttack, "Aria", 0, "Aria"),
	} {
		c := tb.playerSays
		if strings.HasPrefix(want, "not goblin") {
			c = func(cmd live.Command) live.Update { w.hub.Submit(tb.dm, cmd); return next(t, tb.dm) }
		}
		if u := c(cmd); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), want) {
			t.Errorf("%s = %+v", want, u)
		}
	}

	u = tb.playerSays(aim(live.CmdAttack, "Aria", 0, "Goblin"))
	pending := u.View.Combat.Attack
	if pending == nil || pending.Stage != domain.StageToHit || combatant(u.View, "Aria").Action || pending.Name != "Longsword" {
		t.Fatalf("declared = %+v", u.View.Combat)
	}
	if u := tb.playerSays(aim(live.CmdPreviewAttack, "Aria", 0, "Goblin")); !strings.Contains(u.Reason, "Finish the attack") {
		t.Fatalf("second attack while one is pending = %+v", u)
	}
	fill := func(rollID string, who domain.Member, faces ...int) {
		t.Helper()
		cl := dmCaller
		if !who.DM {
			cl = playerCaller
		}
		for i, f := range faces {
			if _, err := rolls.SetDie(ctx, cl, w.session.CampaignID, domain.RollID(uuid.MustParse(rollID)), i, app.Fill{Value: f}); err != nil {
				t.Fatal(err)
			}
		}
		next(t, tb.dm)
		tb.party = append(tb.party, next(t, tb.player))
	}
	fill(pending.RollID, w.player, 10)
	p = tb.party[len(tb.party)-1]
	if a := p.View.Combat.Attack; a == nil || a.Stage != domain.StageDamage || a.Critical || a.RollID == pending.RollID {
		t.Fatalf("a hit asks for damage = %+v", p.View.Combat)
	}
	roll, _ := pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(p.View.Combat.Attack.RollID)))
	if roll.Notation != "1d8" || roll.Modifiers[0].Value != 3 || roll.Roller.ID != w.player.ID || !strings.Contains(roll.Purpose, "damage to Goblin") {
		t.Fatalf("damage roll = %+v", roll)
	}
	fill(p.View.Combat.Attack.RollID, w.player, 6)
	p = tb.party[len(tb.party)-1]
	if token(p.View, "Goblin").Health != "down" || p.View.Combat.Attack != nil {
		t.Fatalf("9 damage fells the goblin = %+v", token(p.View, "Goblin"))
	}
	if u := tb.playerSays(aim(live.CmdAttack, "Aria", 0, "Goblin")); !strings.Contains(u.Reason, "no attacks left this turn") {
		t.Fatalf("a second action = %+v", u)
	}
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(p.View, "Aria").ID})

	d, _ = tb.dmSays(aim(live.CmdAttack, "Goblin", 2, "Aria"))
	fill(d.View.Combat.Attack.RollID, w.dm, 15)
	if hp := *token(tb.party[len(tb.party)-1].View, "Aria").HP; hp != 9 {
		t.Fatalf("a slam deals flat damage: hp %d", hp)
	}
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Goblin").ID})
	w.hub.Submit(tb.dm, aim(live.CmdPreviewAttack, "Archer", 3, "Aria"))
	if u := next(t, tb.dm); u.Preview.Mode != "disadvantage" || u.Preview.Reasons[1] != "Disadvantage: long range" {
		t.Fatalf("a dart at long range = %+v", u.Preview)
	}
	d, _ = tb.dmSays(aim(live.CmdAttack, "Archer", 1, "Aria"))
	fill(d.View.Combat.Attack.RollID, w.dm, 20)
	p = tb.party[len(tb.party)-1]
	if a := p.View.Combat.Attack; !a.Critical || a.Stage != domain.StageDamage {
		t.Fatalf("a natural 20 = %+v", a)
	}
	roll, _ = pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(p.View.Combat.Attack.RollID)))
	if roll.Notation != "2d6" || !strings.HasSuffix(roll.Purpose, "(critical)") || roll.Roller.ID != w.dm.ID {
		t.Fatalf("critical damage roll = %+v", roll)
	}
	fill(p.View.Combat.Attack.RollID, w.dm, 1, 1)
	if hp := *token(tb.party[len(tb.party)-1].View, "Aria").HP; hp != 5 {
		t.Fatalf("4 critical damage leaves Aria on 5: hp %d", hp)
	}
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Archer").ID})

	u = tb.playerSays(aim(live.CmdPreviewAttack, "Aria", 1, "Archer"))
	if pv := u.Preview; pv.Mode != "normal" || len(pv.Reasons) != 1 {
		t.Fatalf("a fallen goblin gives no cover = %+v", pv)
	}
	u = tb.playerSays(aim(live.CmdAttack, "Aria", 1, "Archer"))
	w.hub.Close(w.session.ID)
	if _, err := rolls.SetDie(ctx, playerCaller, w.session.CampaignID, domain.RollID(uuid.MustParse(u.View.Combat.Attack.RollID)), 0, app.Fill{Value: 1}); err != nil {
		t.Fatal(err)
	}
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdResync})
	if u := next(t, tb.dm); u.View.Combat.Attack != nil || *token(u.View, "Archer").HP != 7 {
		t.Fatalf("a natural 1 rolled while the runtime was down misses = %+v", u.View.Combat)
	}

	if u := tb.playerSays(live.Command{Kind: live.CmdUndoDamage}); u.Reason != "Only the DM can change the table." {
		t.Fatalf("player undo = %+v", u)
	}
	for _, want := range []struct {
		label string
		hp    int
	}{{"Aria", 9}, {"Aria", 12}, {"Goblin", 7}} {
		d, _ := tb.dmSays(live.Command{Kind: live.CmdUndoDamage})
		if hp := *token(d.View, want.label).HP; hp != want.hp {
			t.Fatalf("undo restores %s to %d, got %d", want.label, want.hp, hp)
		}
	}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdUndoDamage})
	if u := next(t, tb.dm); u.Reason != "There is no damage to undo." {
		t.Fatalf("nothing left to undo = %+v", u)
	}
	if body := payloads(t, tb.party...); strings.Contains(body, `"ac":15`) || strings.Contains(body, "Scimitar") {
		t.Fatalf("monster stats reached the party: %s", body)
	}
}

func TestAttacksNeedAClearLineAndEndWithTheirTarget(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	m := w.dungeon(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdSetMap, MapID: uuid.UUID(m.ID).String()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 2})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdSetWalls, Hexes: []live.Hex{{Q: 1, R: 0}}, On: true})
	ids := map[string]string{}
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: []live.CombatantSetup{{TokenID: ids["Aria"], SpeedFt: 30}, {TokenID: ids["Goblin"], SpeedFt: 30}}})
	tb.roll(combatant(d.View, "Goblin"), w.dm, 20)
	tb.roll(combatant(d.View, "Aria"), w.player, 2)
	shot := live.Command{Kind: live.CmdAttack, TokenID: ids["Goblin"], AttackNo: 1, TargetID: ids["Aria"]}
	w.hub.Submit(tb.dm, shot)
	if u := next(t, tb.dm); u.Reason != "There is no clear line to the target." {
		t.Fatalf("through a wall = %+v", u)
	}
	tb.dmSays(live.Command{Kind: live.CmdSetWalls, Hexes: []live.Hex{{Q: 1, R: 0}}, On: false})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Late", Q: 3})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Aria"], Q: 1})
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdAttack, TokenID: token(d.View, "Late").ID, TargetID: ids["Aria"]})
	if u := next(t, tb.dm); u.Reason != "It is not Late's turn." {
		t.Fatalf("a creature outside the fight = %+v", u)
	}
	d, _ = tb.dmSays(shot)
	id := domain.RollID(uuid.MustParse(d.View.Combat.Attack.RollID))
	roll, _ := pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, id)
	if roll.Notation != "2d20kl1" {
		t.Fatalf("a shot next to an enemy = %+v", roll)
	}
	for i, f := range []int{18, 2} {
		if _, err := rolls.SetDie(ctx, dmCaller, w.session.CampaignID, id, i, app.Fill{Value: f}); err != nil {
			t.Fatal(err)
		}
	}
	next(t, tb.dm)
	if p := next(t, tb.player); p.View.Combat.Attack != nil || *token(p.View, "Aria").HP != 12 {
		t.Fatalf("the lower die misses = %+v", p.View.Combat)
	}
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Goblin").ID})
	u := tb.playerSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Aria"], TargetID: ids["Goblin"]})
	if u.View.Combat.Attack == nil {
		t.Fatalf("declared = %+v", u)
	}
	if _, p := tb.dmSays(live.Command{Kind: live.CmdSetHidden, TokenID: ids["Goblin"], Hidden: true}); p.View.Combat.Attack != nil {
		t.Fatalf("an attack on a creature the party cannot see stays with the DM = %+v", p.View.Combat)
	}
	if _, p := tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: ids["Goblin"]}); p.View.Combat.Attack != nil {
		t.Fatalf("removing the target ends the attack = %+v", p.View.Combat)
	}
}
