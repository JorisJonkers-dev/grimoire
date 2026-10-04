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

// lastRoll is the newest Roll Request of the Campaign whose purpose starts so.
func lastRoll(t *testing.T, w world, purpose string) domain.Roll {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	if err := w.pool.QueryRow(ctx, "SELECT id FROM play.roll_requests WHERE campaign_id = $1 AND purpose LIKE $2 ORDER BY created_at DESC, id LIMIT 1",
		w.session.CampaignID, purpose+"%").Scan(&id); err != nil {
		t.Fatalf("no roll for %q: %v", purpose, err)
	}
	roll, err := pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, domain.RollID(id))
	if err != nil {
		t.Fatal(err)
	}
	return roll
}

func qOf(t *testing.T, v *live.View, label string) int {
	t.Helper()
	return token(v, label).Q
}

// The 2024 actions at the table: Aria readies an attack that fires as the goblin comes within reach;
// the goblin grapples her and drags her along at double cost; Dash, Disengage, Dodge and Hide each do
// what the rules say; Help, Magic and Utilize hand the DM what was done.
func TestTheActionsOfATurn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	showDCs(t, w)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 3})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Archer", Q: -3, R: 0})
	ids := map[string]string{}
	var setup []live.CombatantSetup
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
		setup = append(setup, live.CombatantSetup{TokenID: tv.ID, SpeedFt: 30})
	}
	refuse := func(sub *live.Subscriber, want string, cmd live.Command) {
		t.Helper()
		w.hub.Submit(sub, cmd)
		if u := next(t, sub); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, want) {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	fill := func(rollID string, who domain.Member, faces ...int) live.Update {
		t.Helper()
		tb.fill(rollID, who, faces...)
		d := next(t, tb.dm)
		tb.party = append(tb.party, next(t, tb.player))
		return d
	}
	act := func(sub *live.Subscriber, token, action string, extra live.Command) live.Update {
		t.Helper()
		extra.Kind, extra.TokenID, extra.Action = live.CmdTakeAction, ids[token], action
		if sub == tb.player {
			return tb.playerSays(extra)
		}
		u, _ := tb.dmSays(extra)
		return u
	}

	refuse(tb.player, "only matters in combat", live.Command{Kind: live.CmdTakeAction, TokenID: ids["Aria"], Action: "dash"})
	tb.playerSays(live.Command{Kind: live.CmdTakeAction, TokenID: ids["Aria"], Action: "search"})
	search := lastRoll(t, w, "Search")
	if search.Purpose != "Search: Wisdom (Perception) check" || search.Notation != "1d20" || search.Modifiers[0].Value != 3 {
		t.Fatalf("searching out of combat = %+v", search)
	}

	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: setup})
	for label, face := range map[string]int{"Aria": 20, "Goblin": 10, "Archer": 5} {
		who := w.dm
		if label == "Aria" {
			who = w.player
		}
		tb.roll(combatant(d.View, label), who, face)
	}
	refuse(tb.player, "Choose one of the actions.", live.Command{Kind: live.CmdTakeAction, TokenID: ids["Aria"], Action: "juggle"})
	refuse(tb.player, "Choose what sets the readied attack off.", live.Command{Kind: live.CmdTakeAction, TokenID: ids["Aria"], Action: "ready", Trigger: "sneeze"})
	refuse(tb.player, "Ready an attack the token can make in reach.", live.Command{Kind: live.CmdTakeAction, TokenID: ids["Aria"], Action: "ready", Trigger: "enters_reach", AttackNo: 1})
	refuse(tb.player, "No such creature to watch for.", live.Command{Kind: live.CmdTakeAction, TokenID: ids["Aria"], Action: "ready", Trigger: "enters_reach", TargetID: uuid.NewString()})
	refuse(tb.player, "not yours", live.Command{Kind: live.CmdTakeAction, TokenID: ids["Goblin"], Action: "dodge"})
	p := act(tb.player, "Aria", "ready", live.Command{Trigger: "enters_reach", TargetID: ids["Goblin"]})
	if aria := combatant(p.View, "Aria"); !aria.Readied || aria.Action {
		t.Fatalf("a readied attack waits, and costs the action = %+v", aria)
	}
	refuse(tb.player, "already used their action", live.Command{Kind: live.CmdTakeAction, TokenID: ids["Aria"], Action: "dash"})
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(p.View, "Aria").ID})

	d, p = tb.dmSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Goblin"], Q: 1, R: 0})
	if q := qOf(t, d.View, "Goblin"); q != 1 || d.View.Combat.Prompt == nil || d.View.Combat.Prompt.Kind != "readied" {
		t.Fatalf("the goblin stops where it comes within reach = %+v", d.View.Combat.Prompt)
	}
	p = tb.playerSays(live.Command{Kind: live.CmdReact, Use: true})
	readied, _ := pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(p.View.Combat.Attack.RollID)))
	if readied.Purpose != "Readied attack against Goblin with Longsword" {
		t.Fatalf("the readied attack = %+v", readied)
	}
	fill(p.View.Combat.Attack.RollID, w.player, 1)
	next(t, tb.dm)
	next(t, tb.player)
	if aria := combatant(look(t, w, tb.dm), "Aria"); aria.Readied || aria.Reaction {
		t.Fatalf("the readied attack is spent with the reaction = %+v", aria)
	}

	refuse(tb.dm, "Grapple, or shove away or down.", live.Command{Kind: live.CmdUnarmed, TokenID: ids["Goblin"], TargetID: ids["Aria"], Option: "tickle"})
	refuse(tb.dm, "out of reach", live.Command{Kind: live.CmdUnarmed, TokenID: ids["Goblin"], TargetID: ids["Archer"], Option: "grapple"})
	refuse(tb.dm, "Choose a creature", live.Command{Kind: live.CmdUnarmed, TokenID: ids["Goblin"], TargetID: ids["Goblin"], Option: "grapple"})
	tb.dmSays(live.Command{Kind: live.CmdUnarmed, TokenID: ids["Goblin"], TargetID: ids["Aria"], Option: "grapple"})
	save := lastRoll(t, w, "Strength save against")
	if save.Purpose != "Strength save against Goblin's grapple (DC 12)" || save.Roller.ID != w.player.ID {
		t.Fatalf("Aria resists the grapple = %+v", save)
	}
	d = fill(uuid.UUID(save.ID).String(), w.player, 4)
	if g := effect(token(d.View, "Aria"), "Grappled"); g == nil || g.SourceID != ids["Goblin"] {
		t.Fatalf("a failed save leaves Aria grappled = %+v", token(d.View, "Aria").Effects)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Goblin"], Q: 2, R: 0})
	if q := qOf(t, d.View, "Aria"); q != 1 || combatant(d.View, "Goblin").MovementFt != 30-10-10 {
		t.Fatalf("the goblin drags Aria at double cost = Aria at %d, %+v", q, combatant(d.View, "Goblin"))
	}
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Goblin").ID})

	act(tb.dm, "Archer", "disengage", live.Command{})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Archer"], Q: 0, R: 1})
	if d.View.Combat.Prompt != nil || !combatant(d.View, "Archer").Disengaged {
		t.Fatalf("a disengaged archer leaves without an opportunity attack = %+v", d.View.Combat.Prompt)
	}
	tb.dmSays(live.Command{Kind: live.CmdEndEffect, EffectID: effect(token(d.View, "Aria"), "Grappled").ID})
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Archer").ID})

	p = act(tb.player, "Aria", "dash", live.Command{})
	if aria := combatant(p.View, "Aria"); aria.MovementFt != 60 {
		t.Fatalf("Dash doubles movement = %+v", aria)
	}
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(p.View, "Aria").ID})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(p.View, "Goblin").ID})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Archer").ID})

	p = act(tb.player, "Aria", "dodge", live.Command{})
	if effect(token(p.View, "Aria"), "Dodging") == nil {
		t.Fatal("Dodge puts on Dodging")
	}
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(p.View, "Aria").ID})
	d = act(tb.dm, "Goblin", "utilize", live.Command{Detail: "pulls the lever"})
	if d.View.Manual[len(d.View.Manual)-1].Text != "Goblin takes the Utilize action: pulls the lever." {
		t.Fatalf("utilize = %+v", d.View.Manual)
	}
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Goblin").ID})
	act(tb.dm, "Archer", "hide", live.Command{})
	hide := uuid.UUID(lastRoll(t, w, "Hide").ID).String()
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	d = fill(hide, w.dm, 10)
	if effect(token(d.View, "Archer"), "Invisible") == nil {
		t.Fatalf("a Hide of 16 makes the archer Invisible, and survives a restart = %+v", token(d.View, "Archer").Effects)
	}

	d, _ = tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Archer").ID})
	tb.playerSays(live.Command{Kind: live.CmdUnarmed, TokenID: ids["Aria"], TargetID: ids["Goblin"], Option: "shove_push"})
	d = fill(uuid.UUID(lastRoll(t, w, "Strength save against Aria's shove push").ID).String(), w.dm, 1)
	if q := qOf(t, d.View, "Goblin"); q != 3 {
		t.Fatalf("a failed save pushes the goblin 5 ft away = %d", q)
	}
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Aria").ID})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Goblin").ID})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(d.View, "Archer").ID})
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Goblin"], Effect: "paralyzed"})
	w.hub.Submit(tb.player, live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 2, R: 0})
	p = next(t, tb.player)
	if d := next(t, tb.dm); d.View.Combat.Prompt == nil || p.View.Combat.Prompt != nil || token(p.View, "Archer") != nil {
		t.Fatalf("leaving the invisible archer's reach offers it an opportunity attack the party cannot see = %+v %+v", d.View.Combat, p.View.Combat)
	}
	tb.dmSays(live.Command{Kind: live.CmdReact, Use: false})
	barrier(t, w, tb)
	p = tb.playerSays(live.Command{Kind: live.CmdUnarmed, TokenID: ids["Aria"], TargetID: ids["Goblin"], Option: "shove_prone"})
	if effect(token(p.View, "Goblin"), "Prone") == nil {
		t.Fatalf("a paralysed goblin fails its save unrolled and falls = %+v", token(p.View, "Goblin").Effects)
	}
}
