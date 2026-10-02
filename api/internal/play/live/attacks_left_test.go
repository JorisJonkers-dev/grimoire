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
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// fighter5 is a level 5 Fighter with Extra Attack, a longsword, a shortsword and a dagger.
type fighter5 struct{ bestiary }

func (f fighter5) Character(ctx context.Context, c caller.Caller, campaign, id uuid.UUID) (string, uuid.UUID, domain.Stats, error) {
	name, owner, stats, err := f.bestiary.Character(ctx, c, campaign, id)
	stats.AttacksPerAction = 2
	stats.Attacks = []domain.Attack{
		{Name: "Longsword", ToHit: 6, ReachFt: 5, Damage: "1d8", DamageBonus: 3, DamageMod: 3, DamageType: "slashing"},
		{Name: "Shortsword", ToHit: 6, ReachFt: 5, Damage: "1d6", DamageBonus: 3, DamageMod: 3, DamageType: "piercing", Light: true},
		{Name: "Dagger", ToHit: 6, ReachFt: 5, Damage: "1d4", DamageBonus: 3, DamageMod: 3, DamageType: "piercing", Light: true},
	}
	return name, owner, stats, err
}

// A level 5 Fighter's turn: two attacks for one action with a step between them, the off-hand attack
// after a Light weapon for the bonus action without the ability modifier, and one free object interaction.
func TestALevelFiveFightersTurn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	w.hub.Stats = fighter5{bestiary{owner: w.player.ID}}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString()})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "hobgoblin", TokenKind: domain.TokenEnemy, Q: 1})
	ids := map[string]string{}
	var setup []live.CombatantSetup
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
		setup = append(setup, live.CombatantSetup{TokenID: tv.ID, SpeedFt: 30})
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: setup})
	tb.roll(combatant(d.View, "Aria"), w.player, 20)
	tb.roll(combatant(d.View, "Hobgoblin"), w.dm, 2)
	refuse := func(want string, cmd live.Command) {
		t.Helper()
		w.hub.Submit(tb.player, cmd)
		if u := next(t, tb.player); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, want) {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	strike := func(no int, offHand bool, face int) live.Update {
		t.Helper()
		u := tb.playerSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Aria"], AttackNo: no, TargetID: ids["Hobgoblin"], OffHand: offHand})
		if u.View == nil {
			t.Fatalf("attack %d = %+v", no, u)
		}
		tb.fill(u.View.Combat.Attack.RollID, w.player, face)
		next(t, tb.dm)
		return next(t, tb.player)
	}

	refuse("Only a Light weapon", live.Command{Kind: live.CmdAttack, TokenID: ids["Aria"], AttackNo: 0, TargetID: ids["Hobgoblin"], OffHand: true})
	refuse("no off-hand attack left", live.Command{Kind: live.CmdAttack, TokenID: ids["Aria"], AttackNo: 2, TargetID: ids["Hobgoblin"], OffHand: true})
	p := strike(1, false, 1)
	if aria := combatant(p.View, "Aria"); aria.Action || aria.AttacksLeft != 1 || !aria.OffHand {
		t.Fatalf("the first of two attacks, with a Light shortsword = %+v", aria)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 0, R: 1}); u.View == nil || combatant(u.View, "Aria").MovementFt != 25 {
		t.Fatalf("a step between attacks = %+v", u)
	}
	p = strike(0, false, 1)
	if aria := combatant(p.View, "Aria"); aria.AttacksLeft != 0 {
		t.Fatalf("the second attack = %+v", aria)
	}
	refuse("no attacks left", live.Command{Kind: live.CmdAttack, TokenID: ids["Aria"], AttackNo: 0, TargetID: ids["Hobgoblin"]})
	p = strike(2, true, 19)
	dmg, _ := pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(p.View.Combat.Attack.RollID)))
	if dmg.Notation != "1d4" || len(dmg.Modifiers) != 0 {
		t.Fatalf("the off-hand dagger leaves out the ability modifier = %+v", dmg)
	}
	tb.fill(p.View.Combat.Attack.RollID, w.player, 2)
	next(t, tb.dm)
	p = next(t, tb.player)
	if aria := combatant(p.View, "Aria"); aria.BonusAction || aria.OffHand {
		t.Fatalf("the off-hand attack costs the bonus action = %+v", aria)
	}
	refuse("no off-hand attack left", live.Command{Kind: live.CmdAttack, TokenID: ids["Aria"], AttackNo: 2, TargetID: ids["Hobgoblin"], OffHand: true})

	refuse("Say what", live.Command{Kind: live.CmdInteract, TokenID: ids["Aria"]})
	p = tb.playerSays(live.Command{Kind: live.CmdInteract, TokenID: ids["Aria"], Detail: "sheathes her dagger"})
	if aria := combatant(p.View, "Aria"); aria.Interaction || !p.View.Resolving {
		t.Fatalf("the free object interaction = %+v", aria)
	}
	refuse("another takes the Utilize action", live.Command{Kind: live.CmdInteract, TokenID: ids["Aria"], Detail: "draws a dagger"})
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(p.View, "Aria").ID})
	refuse("not Aria's turn", live.Command{Kind: live.CmdInteract, TokenID: ids["Aria"], Detail: "draws a dagger"})
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	if h := combatant(look(t, w, tb.dm), "Hobgoblin"); !h.Interaction || h.AttacksLeft != 0 {
		t.Fatalf("the hobgoblin's turn starts fresh, and survives a restart = %+v", h)
	}
}
