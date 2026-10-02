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

func effect(t *live.TokenView, name string) *live.EffectView {
	for i := range t.Effects {
		if t.Effects[i].Name == name {
			return &t.Effects[i]
		}
	}
	return nil
}

func TestEffectsFoldIntoRollsAndMovement(t *testing.T) {
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
	preview := func(no int, target string) *live.AttackPreview {
		t.Helper()
		return tb.playerSays(live.Command{Kind: live.CmdPreviewAttack, TokenID: ids["Aria"], AttackNo: no, TargetID: ids[target]}).Preview
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

	_, p := put("Aria", "bless", "Aria", live.Command{Rounds: 10})
	if e := effect(token(p.View, "Aria"), "Bless"); e == nil || !e.Concentration || e.RoundsLeft != 10 || e.SourceID != ids["Aria"] {
		t.Fatalf("bless = %+v", token(p.View, "Aria").Effects)
	}
	if pv := preview(0, "Goblin"); pv.HitChance != 68 || !strings.Contains(strings.Join(pv.Reasons, "|"), "Bless: +1d4 to hit") {
		t.Fatalf("blessed preview = %+v", pv)
	}
	_, p = put("Goblin", "faerie-fire", "Aria", live.Command{})
	if effect(token(p.View, "Aria"), "Bless") != nil || effect(token(p.View, "Goblin"), "Faerie Fire") == nil {
		t.Fatalf("a new concentration effect ends the old one = %+v", p.View.Tokens)
	}
	if pv := preview(0, "Goblin"); pv.HitChance != 80 || pv.Mode != "advantage" {
		t.Fatalf("faerie fire = %+v", pv)
	}
	put("Goblin", "hunters-mark", "Aria", live.Command{Rounds: 1})
	if pv := preview(0, "Goblin"); pv.DamageMin != 5 || pv.DamageMax != 17 || pv.Mode != "normal" {
		t.Fatalf("hunter's mark = %+v", pv)
	}
	put("Archer", "prone", "", live.Command{})
	if pv := preview(1, "Archer"); pv.Mode != "disadvantage" || !strings.Contains(strings.Join(pv.Reasons, "|"), "Prone: disadvantage") {
		t.Fatalf("shooting a prone archer = %+v", pv)
	}
	u := tb.playerSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Aria"], TargetID: ids["Goblin"]})
	fill(u.View.Combat.Attack.RollID, w.player, 15)
	dmg := tb.party[len(tb.party)-1].View.Combat.Attack.RollID
	roll, _ := pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(dmg)))
	if roll.Notation != "1d8+1d6" {
		t.Fatalf("the mark joins the damage = %+v", roll)
	}
	fill(dmg, w.player, 3, 3)

	d, p = put("Aria", "poisoned", "", live.Command{})
	if len(d.View.Manual) != 1 || d.View.Manual[0].Text != "Aria: Poisoned: ability checks are made with disadvantage." || !p.View.Resolving || p.View.Manual != nil {
		t.Fatalf("the unmodelled part of poisoned = dm %+v party %v", d.View.Manual, p.View.Resolving)
	}
	d, p = tb.dmSays(live.Command{Kind: live.CmdResolveManual, ManualID: d.View.Manual[0].ID})
	if d.View.Manual != nil || p.View.Resolving {
		t.Fatalf("resolved = %+v", d.View)
	}
	d, _ = put("Archer", "hold-person", "Aria", live.Command{EffectName: "Hold Person", SaveAbility: "wisdom", SaveDC: 13})
	if e := effect(token(d.View, "Archer"), "Hold Person"); e == nil || e.Concentration || len(d.View.Manual) != 1 || d.View.Manual[0].Text != "Archer: Resolve Hold Person by hand." {
		t.Fatalf("an unknown effect is tracked and handed to the DM = %+v %+v", token(d.View, "Archer").Effects, d.View.Manual)
	}
	put("Archer", "frightened", "", live.Command{SaveAbility: "wisdom", SaveDC: 10})

	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Aria").ID})
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Goblin").ID})
	d, p = tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Archer").ID})
	if len(d.View.Saves) != 2 || len(p.View.Saves) != 2 {
		t.Fatalf("two ending saves = %+v", d.View.Saves)
	}
	if effect(token(p.View, "Goblin"), "Hunter's Mark") != nil {
		t.Fatalf("a one-round mark expires as its source starts a new turn = %+v", token(p.View, "Goblin").Effects)
	}
	for _, s := range d.View.Saves {
		face := 5
		if s.Effect == "frightened" {
			face = 12
		}
		fill(s.RollID, w.dm, face)
	}
	look := func() *live.View {
		t.Helper()
		w.hub.Submit(tb.dm, live.Command{Kind: live.CmdResync})
		return next(t, tb.dm).View
	}
	v := look()
	arch := token(v, "Archer")
	if effect(arch, "Hold Person") == nil || effect(arch, "frightened") != nil || v.Saves != nil {
		t.Fatalf("hold person holds (5 < 13), fright passes (12 ≥ 10) = %+v %+v", arch.Effects, v.Saves)
	}

	put("Aria", "prone", "", live.Command{})
	if u := tb.playerSays(live.Command{Kind: live.CmdPlanWalk, TokenID: ids["Aria"], Q: -1, R: 0}); u.Path == nil || u.Path.CostFt != 10 {
		t.Fatalf("crawling costs double = %+v", u)
	}
	put("Aria", "bless", "Archer", live.Command{})
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	d, _ = tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: ids["Archer"]})
	aria := token(d.View, "Aria")
	if b := effect(aria, "Bless"); b == nil || b.SourceID != "" || effect(aria, "Prone") == nil || d.View.Manual[0].Text != "Archer: Resolve Hold Person by hand." {
		t.Fatalf("effects reload, and a removed source leaves its effects sourceless = %+v", aria.Effects)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdEndEffect, EffectID: effect(aria, "Prone").ID})
	if effect(token(d.View, "Aria"), "Prone") != nil {
		t.Fatal("prone ended")
	}
	for want, cmd := range map[string]live.Command{
		"no such effect":             {Kind: live.CmdEndEffect, EffectID: "nope"},
		"no such prompt":             {Kind: live.CmdResolveManual, ManualID: "nope"},
		"choose a token":             {Kind: live.CmdApplyEffect, TargetID: "nope", Effect: "bless"},
		"name the effect":            {Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: " "},
		"0 to 100 rounds":            {Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "bless", Rounds: 101},
		"an ability and a dc":        {Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "x", SaveAbility: "luck", SaveDC: 5},
		"an ability and a dc from 1": {Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "x", SaveDC: 5},
		"no such source":             {Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "bless", SourceID: uuid.NewString()},
	} {
		w.hub.Submit(tb.dm, cmd)
		if u := next(t, tb.dm); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), strings.TrimSpace(want)) {
			t.Errorf("%s = %+v", want, u)
		}
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "bless"}); u.Reason != "Only the DM can change the table." {
		t.Fatalf("players do not apply effects = %+v", u)
	}
}

func TestDamageTestsConcentration(t *testing.T) {
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
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 1})
	ids := map[string]string{}
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
	}
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], SourceID: ids["Aria"], Effect: "bless"})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: []live.CombatantSetup{{TokenID: ids["Aria"], SpeedFt: 30}, {TokenID: ids["Goblin"], SpeedFt: 30}}})
	tb.roll(combatant(d.View, "Goblin"), w.dm, 20)
	tb.roll(combatant(d.View, "Aria"), w.player, 2)
	fill := func(rollID string, faces ...int) live.Update {
		t.Helper()
		for i, f := range faces {
			if _, err := rolls.SetDie(ctx, dmCaller, w.session.CampaignID, domain.RollID(uuid.MustParse(rollID)), i, app.Fill{Value: f}); err != nil {
				t.Fatal(err)
			}
		}
		next(t, tb.player)
		return next(t, tb.dm)
	}
	strike := func(faces ...int) live.Update {
		t.Helper()
		d, _ := tb.dmSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Goblin"], TargetID: ids["Aria"]})
		u := fill(d.View.Combat.Attack.RollID, faces[0])
		return fill(u.View.Combat.Attack.RollID, faces[1:]...)
	}
	strike(15, 1)
	next(t, tb.player)
	next(t, tb.dm)
	save := lastRoll(t, w, "Constitution save to keep concentrating on Bless (DC 10)")
	if save.Roller.ID != w.player.ID || save.Notation != "1d20+1d4" || effect(token(look(t, w, tb.dm), "Aria"), "Bless") == nil {
		t.Fatalf("damage opens Aria's concentration save = %+v", save)
	}
	tb.fill(uuid.UUID(save.ID).String(), w.player, 12, 1)
	drain(tb.dm)
	drain(tb.player)
	u := live.Update{View: look(t, w, tb.dm)}
	if effect(token(u.View, "Aria"), "Bless") == nil || lastRoll(t, w, "Constitution save").Status != domain.StatusResolved {
		t.Fatalf("a 12 keeps Bless = %+v", token(u.View, "Aria").Effects)
	}
	tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Aria"], HPDelta: -2})
	drain(tb.dm)
	drain(tb.player)
	second := lastRoll(t, w, "Constitution save to keep concentrating on")
	if second.ID == save.ID || second.Status == domain.StatusResolved {
		t.Fatalf("lost hit points open a second save = %+v", second)
	}
	tb.fill(uuid.UUID(second.ID).String(), w.player, 2, 1)
	drain(tb.dm)
	drain(tb.player)
	u = live.Update{View: look(t, w, tb.dm)}
	if effect(token(u.View, "Aria"), "Bless") != nil {
		t.Fatalf("a failed save ends her concentration = %+v", token(u.View, "Aria").Effects)
	}
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], SourceID: ids["Aria"], Effect: "bless"})
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Goblin").ID})
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Aria").ID})
	u = strike(20, 6, 6)
	if *token(u.View, "Aria").HP != 0 || effect(token(u.View, "Aria"), "Bless") != nil {
		t.Fatalf("dropping to 0 hit points ends concentration = %+v", token(u.View, "Aria"))
	}
}
