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

// weaponMaster has mastered one weapon of every mastery and attacks four times an action.
type weaponMaster struct{ bestiary }

func (f weaponMaster) Character(ctx context.Context, c caller.Caller, campaign, id uuid.UUID) (string, uuid.UUID, domain.Stats, error) {
	name, owner, stats, err := f.bestiary.Character(ctx, c, campaign, id)
	stats.AttacksPerAction = 4
	weapon := func(name, die, mastery string, light bool) domain.Attack {
		return domain.Attack{Name: name, ToHit: 6, ReachFt: 5, Damage: die, DamageBonus: 3, DamageMod: 3, DamageType: "slashing", Light: light, Mastery: mastery}
	}
	stats.Attacks = []domain.Attack{
		weapon("Mace", "1d6", "sap", false), weapon("Greataxe", "1d12", "cleave", false), weapon("Greatsword", "2d6", "graze", false),
		weapon("Club", "1d4", "slow", false), weapon("Battleaxe", "1d8", "topple", false), weapon("Warhammer", "1d8", "push", false),
		weapon("Shortsword", "1d6", "vex", true), weapon("Dagger", "1d4", "nick", true),
	}
	return name, owner, stats, err
}

func TestEveryWeaponMastery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	w.hub.Stats = weaponMaster{bestiary{owner: w.player.ID}}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "hobgoblin", TokenKind: domain.TokenEnemy, Label: "First", Q: 1, R: 0})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "hobgoblin", TokenKind: domain.TokenEnemy, Label: "Second", Q: 1, R: -1})
	ids := map[string]string{}
	var setup []live.CombatantSetup
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
		setup = append(setup, live.CombatantSetup{TokenID: tv.ID, SpeedFt: 30})
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: setup})
	tb.roll(combatant(d.View, "Aria"), w.player, 20)
	tb.roll(combatant(d.View, "First"), w.dm, 3)
	tb.roll(combatant(d.View, "Second"), w.dm, 2)
	// settle fills every die of a roll with one face and waits for the view it makes.
	settle := func(rollID string, who domain.Member, face int) {
		t.Helper()
		roll, err := pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(rollID)))
		if err != nil {
			t.Fatal(err)
		}
		faces := make([]int, len(roll.Dice))
		for i := range faces {
			faces[i] = face
		}
		tb.fill(rollID, who, faces...)
		drain(tb.dm)
		drain(tb.player)
	}
	strike := func(no int, target string, hit bool, extra live.Command) {
		t.Helper()
		extra.Kind, extra.TokenID, extra.AttackNo, extra.TargetID = live.CmdAttack, ids["Aria"], no, ids[target]
		u := tb.playerSays(extra)
		if u.View == nil {
			t.Fatalf("attack %d = %+v", no, u)
		}
		face := 1
		if hit {
			face = 15
		}
		settle(u.View.Combat.Attack.RollID, w.player, face)
		if a := look(t, w, tb.player).Combat.Attack; a != nil {
			settle(a.RollID, w.player, 1)
		}
	}
	effectsOn := func(label string) string {
		t.Helper()
		var names []string
		for _, e := range token(look(t, w, tb.dm), label).Effects {
			names = append(names, e.Name)
		}
		return strings.Join(names, ",")
	}
	hp := func(label string) int {
		t.Helper()
		return *token(look(t, w, tb.dm), label).HP
	}

	if a := token(look(t, w, tb.player), "Aria").Attacks; a[0].Mastery != "sap" || !a[6].Light {
		t.Fatalf("the hotbar shows each mastery = %+v", a)
	}
	strike(0, "First", true, live.Command{})
	if effectsOn("First") != "Sapped" {
		t.Fatalf("Sap = %s", effectsOn("First"))
	}
	strike(1, "First", true, live.Command{})
	if c := combatant(look(t, w, tb.player), "Aria"); !c.Cleave || c.AttacksLeft != 2 {
		t.Fatalf("Cleave opens a second attack = %+v", c)
	}
	w.hub.Submit(tb.player, live.Command{Kind: live.CmdAttack, TokenID: ids["Aria"], AttackNo: 1, TargetID: ids["First"], Cleave: true})
	if u := next(t, tb.player); !strings.Contains(u.Reason, "a second creature") {
		t.Fatalf("Cleave needs another creature = %+v", u)
	}
	before := hp("Second")
	u := tb.playerSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Aria"], AttackNo: 1, TargetID: ids["Second"], Cleave: true})
	settle(u.View.Combat.Attack.RollID, w.player, 15)
	cleave := lastRoll(t, w, "Greataxe damage to Second")
	if len(cleave.Modifiers) != 0 || combatant(look(t, w, tb.player), "Aria").AttacksLeft != 2 {
		t.Fatalf("Cleave's attack costs no attack and leaves out the modifier = %+v", cleave)
	}
	settle(uuid.UUID(cleave.ID).String(), w.player, 1)
	if hp("Second") != before-1 {
		t.Fatalf("the cleave lands = %d", hp("Second"))
	}
	w.hub.Submit(tb.player, live.Command{Kind: live.CmdAttack, TokenID: ids["Aria"], AttackNo: 1, TargetID: ids["Second"], Cleave: true})
	if u := next(t, tb.player); !strings.Contains(u.Reason, "no Cleave attack open") {
		t.Fatalf("one Cleave a turn = %+v", u)
	}
	before = hp("Second")
	strike(2, "Second", false, live.Command{})
	if hp("Second") != before-3 {
		t.Fatalf("Graze deals the modifier on a miss = %d", hp("Second"))
	}
	strike(3, "Second", true, live.Command{})
	if effectsOn("Second") != "Slowed" {
		t.Fatalf("Slow = %s", effectsOn("Second"))
	}
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(look(t, w, tb.player), "Aria").ID})
	v := look(t, w, tb.dm)
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(v, "First").ID})
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(v, "Second").ID})
	if effectsOn("First") != "" || effectsOn("Second") != "" {
		t.Fatalf("mastery marks last until the attacker's next turn = %s %s", effectsOn("First"), effectsOn("Second"))
	}

	strike(6, "First", true, live.Command{})
	if effectsOn("First") != "Vexed" || !combatant(look(t, w, tb.player), "Aria").OffHand {
		t.Fatalf("Vex = %s", effectsOn("First"))
	}
	strike(7, "First", true, live.Command{OffHand: true})
	if c := combatant(look(t, w, tb.player), "Aria"); !c.BonusAction {
		t.Fatalf("Nick makes the off-hand attack without the bonus action = %+v", c)
	}
	strike(5, "Second", true, live.Command{})
	if q := qOf(t, look(t, w, tb.dm), "Second"); q != 3 {
		t.Fatalf("Push drives the hobgoblin 10 ft away = %d", q)
	}
	u = tb.playerSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Aria"], AttackNo: 4, TargetID: ids["First"]})
	settle(u.View.Combat.Attack.RollID, w.player, 15)
	settle(look(t, w, tb.player).Combat.Attack.RollID, w.player, 1)
	topple := lastRoll(t, w, "Constitution save against Aria's Topple (DC 14)")
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	settle(uuid.UUID(topple.ID).String(), w.dm, 2)
	if !strings.Contains(effectsOn("First"), "Prone") {
		t.Fatalf("a failed Topple save knocks the hobgoblin Prone, after a restart = %s", effectsOn("First"))
	}
}
