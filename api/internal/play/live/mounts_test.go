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

// stable puts Aria, the Player's, at the middle of the map with a Steed of the party's east of her, a
// goblin west of her and a banner north of her.
func stable(t *testing.T) (world, *table, map[string]string, []live.CombatantSetup) {
	t.Helper()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "wolf", TokenKind: domain.TokenParty, Label: "Steed", Q: 1})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: -1})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Banner", TokenKind: domain.TokenObject, R: -1})
	ids := map[string]string{}
	var fighters []live.CombatantSetup
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
		if tv.HP != nil {
			fighters = append(fighters, live.CombatantSetup{TokenID: tv.ID, SpeedFt: 30})
		}
	}
	return w, tb, ids, fighters
}

// refusal sends a command that must be refused, and for the reason given.
func (tb *table) refusal(sub *live.Subscriber, want string, cmd live.Command) {
	tb.t.Helper()
	tb.w.hub.Submit(sub, cmd)
	if u := next(tb.t, sub); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, want) {
		tb.t.Fatalf("%s = %+v", want, u)
	}
}

// seat is where a token stands, what it rides and who rides it.
type seat struct {
	q, r       int
	mount      string
	controlled bool
	rider      string
}

func seatOf(t *testing.T, v *live.View, id string) seat {
	t.Helper()
	tv := tokenByID(t, v, id)
	return seat{q: tv.Q, r: tv.R, mount: tv.MountID, controlled: tv.MountControlled, rider: tv.RiderID}
}

// rejoin ends the running Session's runtime and joins again, so what is seen was read back from the store.
func (tb *table) rejoin() {
	tb.t.Helper()
	tb.w.hub.Close(tb.w.session.ID)
	tb.dm, tb.player = join(tb.t, tb.w, tb.w.dm, dmCaller, live.AudienceDM), join(tb.t, tb.w, tb.w.player, playerCaller, live.AudienceParty)
}

// A rider gets onto a willing creature beside it and from then on goes where the mount goes: along
// each step of a walk, and wherever the DM puts it. The rider's Player steers a controlled mount and
// not an independent one. Getting off, being moved off, or the mount leaving the map ends the ride.
func TestARiderGoesWhereItsMountGoes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids, _ := stable(t)
	aria, steed, goblin := ids["Aria"], ids["Steed"], ids["Goblin"]
	riding := func() int {
		t.Helper()
		var n int
		if err := w.pool.QueryRow(ctx, "SELECT count(*) FROM play.tokens WHERE session_id = $1 AND mount_token_id IS NOT NULL", uuid.UUID(w.session.ID)).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	tb.refusal(tb.player, "not yours", live.Command{Kind: live.CmdMount, TokenID: goblin, TargetID: steed})
	tb.refusal(tb.dm, "No such creature.", live.Command{Kind: live.CmdMount, TokenID: ids["Banner"], TargetID: steed})
	tb.refusal(tb.dm, "No such creature.", live.Command{Kind: live.CmdMount, TokenID: uuid.NewString(), TargetID: steed})
	for _, target := range []string{uuid.NewString(), ids["Banner"], aria, "the horse"} {
		tb.refusal(tb.player, "Choose a creature to ride.", live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: target})
	}
	tb.refusal(tb.dm, "Goblin will not carry Aria.", live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: goblin})
	tb.refusal(tb.dm, "Aria will not carry Goblin.", live.Command{Kind: live.CmdMount, TokenID: goblin, TargetID: aria})
	// A Player gets a creature only onto one of their own: whether another will carry it is the DM's to say.
	tb.refusal(tb.player, "Steed is not yours to ride: ask the DM.", live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: steed, Controlled: true})
	tb.refusal(tb.player, "Goblin is not yours to ride: ask the DM.", live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: goblin})
	// One the party cannot see is not there to be ridden, and is not named.
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "wolf", TokenKind: domain.TokenParty, Label: "Lurker", Hidden: true, R: 1})
	lurker := token(d.View, "Lurker").ID
	tb.refusal(tb.player, "Choose a creature to ride.", live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: lurker})
	tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: lurker})
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: steed, Q: 2})
	tb.refusal(tb.dm, "Steed is out of reach.", live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: steed})
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: steed, Q: 1})
	tb.refusal(tb.player, "Aria is not riding.", live.Command{Kind: live.CmdDismount, TokenID: aria, Q: 0, R: 1})
	tb.refusal(tb.player, "not yours", live.Command{Kind: live.CmdDismount, TokenID: goblin, Q: 0, R: 1})
	tb.refusal(tb.player, "No such creature.", live.Command{Kind: live.CmdDismount, TokenID: "her", Q: 0, R: 1})

	// Held fast, she cannot swing herself up.
	d, _ = tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: aria, Effect: "restrained"})
	tb.refusal(tb.dm, "Aria can't move.", live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: steed})
	tb.dmSays(live.Command{Kind: live.CmdEndEffect, EffectID: effect(token(d.View, "Aria"), "Restrained").ID})

	// The DM puts Aria on, holding the reins: she is where the Steed is, and both screens see who rides what.
	_, p := tb.dmSays(live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: steed, Controlled: true})
	for who, v := range map[string]*live.View{"the party": p.View, "the DM": look(t, w, tb.dm)} {
		if got, want := seatOf(t, v, aria), (seat{q: 1, mount: steed, controlled: true}); got != want {
			t.Fatalf("Aria mounted, as %s sees it = %+v", who, got)
		}
		if got, want := seatOf(t, v, steed), (seat{q: 1, rider: aria}); got != want {
			t.Fatalf("the Steed ridden, as %s sees it = %+v", who, got)
		}
	}
	// A mount the party cannot see is not named to it; the DM still sees who rides what.
	_, p = tb.dmSays(live.Command{Kind: live.CmdSetHidden, TokenID: steed, Hidden: true})
	if a := seatOf(t, p.View, aria); a != (seat{q: 1}) || token(p.View, "Steed") != nil || seatOf(t, look(t, w, tb.dm), aria).mount != steed {
		t.Fatalf("a hidden mount, as the party sees its rider = %+v", a)
	}
	tb.dmSays(live.Command{Kind: live.CmdSetHidden, TokenID: steed})
	tb.refusal(tb.player, "Aria is riding Steed: dismount first.", live.Command{Kind: live.CmdWalk, TokenID: aria, Q: 0, R: 0})
	tb.refusal(tb.dm, "Aria is riding already.", live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: steed})
	tb.refusal(tb.dm, "Steed already carries a rider.", live.Command{Kind: live.CmdMount, TokenID: goblin, TargetID: steed})
	tb.refusal(tb.dm, "Steed is carrying a rider.", live.Command{Kind: live.CmdMount, TokenID: steed, TargetID: goblin})
	tb.refusal(tb.dm, "Aria is riding, and carries no one.", live.Command{Kind: live.CmdMount, TokenID: goblin, TargetID: aria})

	// She steers the Steed: it walks at her word and takes her along every step.
	p = tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: steed, Q: 3, R: 0})
	if len(p.Steps) != 1 || seatOf(t, &p.Steps[0], aria).q != 2 || seatOf(t, &p.Steps[0], steed).q != 2 {
		t.Fatalf("half way, the rider is with its mount = %+v", p.Steps)
	}
	if a, s := seatOf(t, p.View, aria), seatOf(t, p.View, steed); a != (seat{q: 3, mount: steed, controlled: true}) || s != (seat{q: 3, rider: aria}) {
		t.Fatalf("the Steed carries Aria = %+v on %+v", a, s)
	}
	tb.rejoin()
	if a := seatOf(t, look(t, w, tb.player), aria); a != (seat{q: 3, mount: steed, controlled: true}) || riding() != 1 {
		t.Fatalf("the ride is kept = %+v, %d riding", a, riding())
	}

	// Moved by the DM's hand, the mount still takes its rider, and nobody rolls to stay on.
	_, p = tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: steed, Q: 0, R: 2})
	if a := seatOf(t, p.View, aria); a != (seat{r: 2, mount: steed, controlled: true}) {
		t.Fatalf("the Steed put elsewhere takes Aria = %+v", a)
	}
	open := func() int {
		t.Helper()
		var n int
		if err := w.pool.QueryRow(ctx, "SELECT count(*) FROM play.roll_requests WHERE campaign_id = $1 AND status = 'pending'", w.session.CampaignID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if open() != 0 {
		t.Fatalf("no save for the DM's own hand: %d open", open())
	}

	// Getting off: onto a free hex next to the mount.
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: goblin, Q: 1, R: 2})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "wolf", TokenKind: domain.TokenParty, Label: "Pony", ControllerID: w.player.ID.String(), Q: -1, R: 2})
	pony := token(d.View, "Pony").ID
	// Off the map, too far, the mount's own hex, a hex a foe or a friend stands on: one answer for each,
	// so it tells nothing more.
	for _, to := range [][2]int{{99, 99}, {0, 0}, {0, 2}, {1, 2}, {-1, 2}} {
		tb.refusal(tb.player, "Dismount onto a free hex next to Steed.", live.Command{Kind: live.CmdDismount, TokenID: aria, Q: to[0], R: to[1]})
	}
	p = tb.playerSays(live.Command{Kind: live.CmdDismount, TokenID: aria, Q: 0, R: 1})
	if a, s := seatOf(t, p.View, aria), seatOf(t, p.View, steed); a != (seat{r: 1}) || s != (seat{r: 2}) || riding() != 0 {
		t.Fatalf("Aria dismounted = %+v beside %+v, %d riding", a, s, riding())
	}
	// A creature of her own she gets onto, and off, without asking.
	p = tb.playerSays(live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: pony, Controlled: true})
	if a := seatOf(t, p.View, aria); a != (seat{q: -1, r: 2, mount: pony, controlled: true}) {
		t.Fatalf("Aria on her own Pony = %+v", a)
	}
	tb.playerSays(live.Command{Kind: live.CmdDismount, TokenID: aria, Q: 0, R: 1})
	tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: pony})

	// On foot again, she walks by herself, and the Steed is the DM's to move.
	tb.refusal(tb.player, "not yours to move", live.Command{Kind: live.CmdWalk, TokenID: steed, Q: 0, R: 3})
	tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: aria, Q: -1, R: 2})

	// An independent mount carries her, and goes where it likes: only the DM moves it.
	_, p = tb.dmSays(live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: steed})
	if a := seatOf(t, p.View, aria); a != (seat{r: 2, mount: steed}) {
		t.Fatalf("Aria on an independent mount = %+v", a)
	}
	tb.refusal(tb.player, "not yours to move", live.Command{Kind: live.CmdWalk, TokenID: steed, Q: 0, R: 0})
	_, p = tb.dmSays(live.Command{Kind: live.CmdWalk, TokenID: steed, Q: 0, R: 0})
	if a := seatOf(t, p.View, aria); a != (seat{mount: steed}) {
		t.Fatalf("the independent Steed carries Aria = %+v", a)
	}

	// A leap of its own takes her along too, and is nothing she must hold on through.
	_, p = tb.dmSays(live.Command{Kind: live.CmdJump, TokenID: steed, Q: 1, R: 0})
	if a := seatOf(t, p.View, aria); a != (seat{q: 1, mount: steed}) || open() != 0 {
		t.Fatalf("the Steed leaps with Aria on it = %+v, %d open", a, open())
	}

	// Taken off her mount by the DM's hand, she rides no more.
	_, p = tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: aria, Q: 1, R: -1})
	if a, s := seatOf(t, p.View, aria), seatOf(t, p.View, steed); a != (seat{q: 1, r: -1}) || s != (seat{q: 1}) || riding() != 0 {
		t.Fatalf("Aria moved off her mount = %+v, %+v, %d riding", a, s, riding())
	}
	// A mount somebody else's creature controls is not hers to move.
	d, _ = tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "wolf", TokenKind: domain.TokenParty, Label: "Brom", ControllerID: w.dm.ID.String(), Q: 2})
	brom := token(d.View, "Brom").ID
	tb.dmSays(live.Command{Kind: live.CmdMount, TokenID: brom, TargetID: steed, Controlled: true})
	tb.refusal(tb.player, "not yours to move", live.Command{Kind: live.CmdWalk, TokenID: steed, Q: 0, R: 0})
	_, p = tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: brom})
	if s := seatOf(t, p.View, steed); s != (seat{q: 1}) || riding() != 0 {
		t.Fatalf("its rider gone from the map, the Steed carries nobody = %+v, %d riding", s, riding())
	}

	// Her mount leaving the map leaves her standing where it stood.
	tb.dmSays(live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: steed, Controlled: true})
	_, p = tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: steed})
	if a := seatOf(t, p.View, aria); a != (seat{q: 1}) || riding() != 0 {
		t.Fatalf("her mount gone, Aria stands where it stood = %+v, %d riding", a, riding())
	}
	tb.rejoin()
	if a := seatOf(t, look(t, w, tb.player), aria); a != (seat{q: 1}) {
		t.Fatalf("and still does after a restart = %+v", a)
	}
}

// A rider sent off with another group of the party leaves its mount behind.
func TestARiderSentOffLeavesItsMount(t *testing.T) {
	t.Parallel()
	w, tb, ids, _ := stable(t)
	tower := w.picture(t, "Tower", domain.MapLocal)
	tb.dmSays(live.Command{Kind: live.CmdMount, TokenID: ids["Aria"], TargetID: ids["Steed"], Controlled: true})
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdSplitParty, Name: "The tower", TokenIDs: []string{ids["Aria"]}, MapID: uuid.UUID(tower.ID).String()})
	d := next(t, tb.dm)
	if d.Kind != live.UpdSnapshot || seatOf(t, d.View, ids["Steed"]) != (seat{q: 1}) {
		t.Fatalf("the Steed left behind = %+v", d)
	}
	var riding int
	if err := w.pool.QueryRow(context.Background(), "SELECT count(*) FROM play.tokens WHERE id = $1 AND (mount_token_id IS NOT NULL OR mount_controlled)", uuid.MustParse(ids["Aria"])).Scan(&riding); err != nil || riding != 0 {
		t.Fatalf("Aria rides nothing where she went: %d, %v", riding, err)
	}
}

// In a fight getting on or off costs half the rider's Speed. An independent mount keeps its own turn.
// A controlled mount takes its rider's initiative, moves at the rider's word on the rider's turn, and
// only Dashes, Disengages or Dodges; its turn ends with its rider's.
func TestAMountInAFight(t *testing.T) {
	t.Parallel()
	w, tb, ids, fighters := stable(t)
	aria, steed, goblin := ids["Aria"], ids["Steed"], ids["Goblin"]
	d, _ := tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: fighters})
	tb.refusal(tb.player, "It is not Aria's turn.", live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: steed})
	for label, face := range map[string]int{"Steed": 20, "Aria": 15, "Goblin": 10} {
		who := w.dm
		if label == "Aria" {
			who = w.player
		}
		tb.roll(combatant(d.View, label), who, face)
	}
	end := func(label string) *live.View {
		t.Helper()
		cmd := live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(look(t, w, tb.dm), label).ID}
		if label == "Aria" {
			tb.playerSays(cmd)
		} else {
			tb.dmSays(cmd)
		}
		return look(t, w, tb.dm)
	}
	initiative := func(v *live.View, label string) int {
		t.Helper()
		return *combatant(v, label).Initiative
	}

	// Round 1: the Steed has had its turn when Aria gets on it for half her Speed. Independent, it keeps
	// its own initiative.
	tb.refusal(tb.dm, "It is not Goblin's turn.", live.Command{Kind: live.CmdMount, TokenID: goblin, TargetID: aria})
	end("Steed")
	_, p := tb.dmSays(live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: steed})
	if a, s := combatant(p.View, "Aria"), combatant(p.View, "Steed"); a.MovementFt != 15 || s.Acting || initiative(p.View, "Steed") == initiative(p.View, "Aria") {
		t.Fatalf("mounting an independent Steed = %+v, %+v", a, s)
	}
	tb.rejoin()
	if a := combatant(look(t, w, tb.dm), "Aria"); a.MovementFt != 15 || seatOf(t, look(t, w, tb.dm), aria) != (seat{q: 1, mount: steed}) {
		t.Fatalf("what mounting cost her is kept = %+v", a)
	}
	tb.refusal(tb.player, "not yours to move", live.Command{Kind: live.CmdWalk, TokenID: steed, Q: 2, R: 0})
	tb.refusal(tb.player, "not yours to play", live.Command{Kind: live.CmdTakeAction, TokenID: steed, Action: "dash"})
	tb.refusal(tb.player, "not yours to play", live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(p.View, "Steed").ID})
	end("Aria")
	end("Goblin")
	// Round 2: the independent Steed does what it likes on its own turn.
	tb.dmSays(live.Command{Kind: live.CmdTakeAction, TokenID: steed, Action: "help"})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdWalk, TokenID: steed, Q: 3, R: 0})
	if a := seatOf(t, d.View, aria); a != (seat{q: 3, mount: steed}) || combatant(d.View, "Steed").MovementFt != 20 || combatant(d.View, "Aria").MovementFt != 15 {
		t.Fatalf("the Steed walks on its own turn and movement = %+v, %+v", a, combatant(d.View, "Steed"))
	}
	end("Steed")

	// Still round 2: off for half her Speed, on again for the other half, and nothing is left to get
	// off with. The Steed, its own turn over, gets another on hers.
	p = tb.playerSays(live.Command{Kind: live.CmdDismount, TokenID: aria, Q: 2, R: 0})
	if a := combatant(p.View, "Aria"); a.MovementFt != 15 || seatOf(t, p.View, aria) != (seat{q: 2}) {
		t.Fatalf("dismounting costs half her Speed = %+v", a)
	}
	_, p = tb.dmSays(live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: steed, Controlled: true})
	a, s := combatant(p.View, "Aria"), combatant(p.View, "Steed")
	if a.MovementFt != 0 || !s.Acting || s.Done || initiative(p.View, "Steed") != initiative(p.View, "Aria") || s.MovementFt != 30 || !s.Action {
		t.Fatalf("a controlled Steed takes Aria's initiative and can move at once = %+v, %+v", a, s)
	}
	tb.rejoin()
	v := look(t, w, tb.dm)
	if a, s := combatant(v, "Aria"), combatant(v, "Steed"); a.MovementFt != 0 || !s.Acting || initiative(v, "Steed") != initiative(v, "Aria") || s.MovementFt != 30 || !s.Action {
		t.Fatalf("and all of it is kept = %+v, %+v", a, s)
	}
	tb.refusal(tb.player, "Aria has 0 ft of movement left.", live.Command{Kind: live.CmdDismount, TokenID: aria, Q: 2, R: 0})
	// The reins move it and nothing more: its action, its Bonus Action and whatever else it could do
	// stay with whoever runs it.
	tb.refusal(tb.player, "not yours to play", live.Command{Kind: live.CmdSpend, CombatantID: combatant(v, "Steed").ID, Resource: live.ResourceBonusAction})
	tb.refusal(tb.player, "not yours to play", live.Command{Kind: live.CmdThrow, TokenID: steed})
	tb.refusal(tb.player, "not yours to play", live.Command{Kind: live.CmdStabilise, TokenID: steed, TargetID: aria})
	tb.refusal(tb.player, "not yours to play", live.Command{Kind: live.CmdUnarmed, TokenID: steed, TargetID: goblin, Option: "grapple"})
	// It only Dashes, Disengages or Dodges, whoever asks.
	for _, sub := range []*live.Subscriber{tb.player, tb.dm} {
		tb.refusal(sub, "Steed is ridden: it only Dashes, Disengages or Dodges.", live.Command{Kind: live.CmdTakeAction, TokenID: steed, Action: "help"})
	}
	tb.refusal(tb.dm, "Steed is ridden: it only Dashes, Disengages or Dodges.", live.Command{Kind: live.CmdAttack, TokenID: steed, TargetID: goblin})
	tb.refusal(tb.dm, "Steed is ridden: it only Dashes, Disengages or Dodges.", live.Command{Kind: live.CmdUnarmed, TokenID: steed, TargetID: goblin, Option: "grapple"})
	p = tb.playerSays(live.Command{Kind: live.CmdTakeAction, TokenID: steed, Action: "dash"})
	if s := combatant(p.View, "Steed"); s.MovementFt != 60 || s.Action {
		t.Fatalf("Aria has the Steed Dash = %+v", s)
	}
	p = tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: steed, Q: 1, R: 0})
	if a := seatOf(t, p.View, aria); a != (seat{q: 1, mount: steed, controlled: true}) || combatant(p.View, "Steed").MovementFt != 50 || combatant(p.View, "Aria").MovementFt != 0 {
		t.Fatalf("the Steed spends its own movement = %+v, %+v", a, combatant(p.View, "Steed"))
	}
	// Her turn over, so is its.
	v = end("Aria")
	if s := combatant(v, "Steed"); !s.Done || s.Acting || !combatant(v, "Goblin").Acting {
		t.Fatalf("the Steed's turn ends with Aria's = %+v", s)
	}
	tb.refusal(tb.player, "It is not Steed's turn.", live.Command{Kind: live.CmdWalk, TokenID: steed, Q: 2, R: 0})
	tb.refusal(tb.player, "It is not Aria's turn.", live.Command{Kind: live.CmdDismount, TokenID: aria, Q: 2, R: 0})
	tb.rejoin()

	// Round 3: both act on her count, and she may end its turn herself.
	v = end("Goblin")
	if a, s := combatant(v, "Aria"), combatant(v, "Steed"); !a.Acting || !s.Acting || s.MovementFt != 30 || v.Combat.Round != 3 {
		t.Fatalf("round 3 opens on Aria and her Steed = %+v, %+v", a, s)
	}
	p = tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(v, "Steed").ID})
	if a, s := combatant(p.View, "Aria"), combatant(p.View, "Steed"); !a.Acting || !s.Done {
		t.Fatalf("Aria ends the Steed's turn and keeps her own = %+v, %+v", a, s)
	}
	// Off its back, the Steed is no longer hers to play, and it fights as it likes again.
	tb.playerSays(live.Command{Kind: live.CmdDismount, TokenID: aria, Q: 2, R: 0})
	end("Aria")
	end("Goblin")
	tb.refusal(tb.player, "not yours to play", live.Command{Kind: live.CmdTakeAction, TokenID: steed, Action: "dash"})
	// On it again without the reins, her turn ending leaves the Steed its own, though they share a count.
	tb.dmSays(live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: steed})
	if s := combatant(end("Aria"), "Steed"); s.Done || !s.Acting {
		t.Fatalf("an independent Steed's turn outlasts its rider's = %+v", s)
	}
	tb.dmSays(live.Command{Kind: live.CmdTakeAction, TokenID: steed, Action: "help"})
}

// A rider keeps its seat on a Dexterity save of 10 when it is knocked Prone or its mount is moved
// against its will; failing, it lands Prone beside the mount. A mount knocked Prone throws its rider.
func TestARiderFallsOff(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids, fighters := stable(t)
	aria, steed, goblin := ids["Aria"], ids["Steed"], ids["Goblin"]
	mount := func() {
		t.Helper()
		tb.dmSays(live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: steed, Controlled: true})
	}
	prone := func(label string) *live.EffectView {
		t.Helper()
		return effect(token(look(t, w, tb.dm), label), "Prone")
	}
	stand := func(label string) {
		t.Helper()
		tb.dmSays(live.Command{Kind: live.CmdEndEffect, EffectID: prone(label).ID})
	}
	fill := func(roll domain.Roll, who domain.Member, face int) {
		t.Helper()
		tb.fill(uuid.UUID(roll.ID).String(), who, face)
		look(t, w, tb.dm)
		look(t, w, tb.player)
	}
	open := func() int {
		t.Helper()
		var n int
		if err := w.pool.QueryRow(ctx, "SELECT count(*) FROM play.roll_requests WHERE campaign_id = $1 AND status = 'pending'", w.session.CampaignID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	// fallen checks Aria lies Prone, once, that far from her mount, riding nothing.
	fallen := func(why string, dq, dr int) {
		t.Helper()
		v := look(t, w, tb.player)
		a, s := seatOf(t, v, aria), seatOf(t, v, steed)
		lying := 0
		for _, e := range token(v, "Aria").Effects {
			if e.Name == "Prone" {
				lying++
			}
		}
		if a != (seat{q: s.q + dq, r: s.r + dr}) || s.rider != "" || lying != 1 {
			t.Fatalf("%s: Aria = %+v beside %+v, %+v", why, a, s, token(v, "Aria").Effects)
		}
	}

	// Knocked Prone in the saddle, Aria saves to stay on: a 10 keeps her seat.
	mount()
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: aria, Effect: "prone"})
	look(t, w, tb.dm)
	save := lastRoll(t, w, "Dexterity save to stay on")
	if save.Purpose != "Dexterity save to stay on Steed (DC 10)" || save.Roller.ID != w.player.ID || save.Notation != "1d20" || open() != 1 {
		t.Fatalf("the save to keep her seat = %+v, %d open", save, open())
	}
	fill(save, w.player, 10)
	if a := seatOf(t, look(t, w, tb.player), aria); a != (seat{q: 1, mount: steed, controlled: true}) || open() != 0 {
		t.Fatalf("a save of 10 keeps Aria in the saddle = %+v, %d open", a, open())
	}
	// Already Prone, she is not knocked Prone again, and saves no more.
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: aria, Effect: "poisoned"})
	look(t, w, tb.dm)
	if open() != 0 {
		t.Fatalf("an Effect that knocks nobody down asks for no save: %d open", open())
	}

	// A 9 does not: she lands beside the Steed, Prone.
	stand("Aria")
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: aria, Effect: "prone"})
	look(t, w, tb.dm)
	fill(lastRoll(t, w, "Dexterity save to stay on"), w.player, 9)
	fallen("a save of 9", 1, 0)
	var logged int
	if err := w.pool.QueryRow(ctx, "SELECT count(*) FROM play.actions WHERE session_id = $1 AND kind = 'unseated'", uuid.UUID(w.session.ID)).Scan(&logged); err != nil || logged != 1 {
		t.Fatalf("the fall is in the Action Log: %d, %v", logged, err)
	}
	tb.rejoin()
	fallen("after a restart", 1, 0)

	// A save still open when she gets off by herself comes to nothing.
	stand("Aria")
	mount()
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: aria, Effect: "prone"})
	look(t, w, tb.dm)
	tb.playerSays(live.Command{Kind: live.CmdDismount, TokenID: aria, Q: 0, R: 0})
	fill(lastRoll(t, w, "Dexterity save to stay on"), w.player, 1)
	if a := seatOf(t, look(t, w, tb.player), aria); a != (seat{}) || open() != 0 {
		t.Fatalf("already off her mount, a failed save moves Aria nowhere = %+v, %d open", a, open())
	}

	// Her mount knocked Prone throws her with no save, onto the first free hex round it.
	stand("Aria")
	mount()
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: goblin, Q: 2})
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: steed, Effect: "prone"})
	look(t, w, tb.dm)
	if open() != 0 {
		t.Fatalf("a thrown rider makes no save: %d open", open())
	}
	fallen("the Steed knocked Prone", 1, -1)
	// Back on a mount that still lies there, she is not thrown again.
	stand("Aria")
	mount()
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: steed, Effect: "poisoned"})
	look(t, w, tb.dm)
	if a := seatOf(t, look(t, w, tb.player), aria); a != (seat{q: 1, mount: steed, controlled: true}) {
		t.Fatalf("a mount already Prone throws nobody = %+v", a)
	}
	// A creature that fails Dexterity saves outright falls without rolling, and never off the map.
	stand("Steed")
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: steed, Q: 10})
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: aria, Effect: "paralyzed"})
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: aria, Effect: "prone"})
	look(t, w, tb.dm)
	if open() != 0 {
		t.Fatalf("a paralysed rider rolls nothing: %d open", open())
	}
	fallen("paralysed and knocked Prone", 0, -1)
	for _, e := range token(look(t, w, tb.dm), "Aria").Effects {
		tb.dmSays(live.Command{Kind: live.CmdEndEffect, EffectID: e.ID})
	}
	for _, e := range token(look(t, w, tb.dm), "Steed").Effects {
		tb.dmSays(live.Command{Kind: live.CmdEndEffect, EffectID: e.ID})
	}
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: steed, Q: 1})
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: aria})
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: goblin, Q: -1})

	// Shoved away with her on it, the Steed takes her along, and she saves to stay on.
	mount()
	d, _ := tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: fighters})
	for label, face := range map[string]int{"Goblin": 20, "Aria": 10, "Steed": 5} {
		who := w.dm
		if label == "Aria" {
			who = w.player
		}
		tb.roll(combatant(d.View, label), who, face)
	}
	at := seatOf(t, look(t, w, tb.dm), steed)
	if v := look(t, w, tb.dm); *combatant(v, "Steed").Initiative != 10 || combatant(v, "Steed").Acting {
		t.Fatalf("a Steed ridden into the fight takes Aria's initiative = %+v", combatant(v, "Steed"))
	}
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: goblin, Q: at.q - 1, R: at.r})
	tb.dmSays(live.Command{Kind: live.CmdUnarmed, TokenID: goblin, TargetID: steed, Option: "shove_push"})
	fill(lastRoll(t, w, "Strength save against Goblin's shove push"), w.dm, 1)
	v := look(t, w, tb.player)
	if a, s := seatOf(t, v, aria), seatOf(t, v, steed); s.q != at.q+1 || a != (seat{q: s.q, r: s.r, mount: steed, controlled: true}) || open() != 1 {
		t.Fatalf("the shoved Steed takes Aria along while she saves = %+v on %+v, %d open", a, s, open())
	}
	fill(lastRoll(t, w, "Dexterity save to stay on"), w.player, 3)
	fallen("the Steed shoved from under her", 1, 0)
}

// A spell that hurls the mount away takes its rider with it, and the rider saves to stay on.
func TestASpellMovesAMountFromUnderItsRider(t *testing.T) {
	t.Parallel()
	w, tb, ids, _ := stable(t)
	aria, steed := ids["Aria"], ids["Steed"]
	tb.dmSays(live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: steed, Controlled: true})
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Goblin"]})
	tb.fight(ids, "Goblin", "Aria", "Steed")
	d, _ := tb.dmSays(live.Command{Kind: live.CmdCastArea, TokenID: ids["Goblin"], Effect: "thunderwave", Q: 1, R: 0})
	if len(d.View.Area.Saves) != 2 {
		t.Fatalf("the rider and the mount both save = %+v", d.View.Area.Saves)
	}
	tb.fill(d.View.Area.DamageRollID, w.dm, 1, 1)
	for _, save := range d.View.Area.Saves {
		if save.TokenID == aria {
			tb.fill(save.RollID, w.player, 20)
		} else {
			tb.fill(save.RollID, w.dm, 1)
		}
	}
	v := look(t, w, tb.dm)
	if a, s := seatOf(t, v, aria), seatOf(t, v, steed); s.q != 3 || a != (seat{q: 3, mount: steed, controlled: true}) {
		t.Fatalf("the Steed hurled 10 feet takes Aria along = %+v on %+v", a, s)
	}
	save := lastRoll(t, w, "Dexterity save to stay on")
	tb.fill(uuid.UUID(save.ID).String(), w.player, 2)
	v = look(t, w, tb.player)
	if a, s := seatOf(t, v, aria), seatOf(t, v, steed); a.mount != "" || s.rider != "" || a.q == s.q && a.r == s.r || effect(token(v, "Aria"), "Prone") == nil {
		t.Fatalf("failing the save, Aria falls = %+v beside %+v", a, s)
	}
}

// Getting off and falling off put a rider only where it could step: never into a wall.
func TestARiderNeverLandsInAWall(t *testing.T) {
	t.Parallel()
	w, tb, ids, _ := stable(t)
	aria, steed := ids["Aria"], ids["Steed"]
	tb.dmSays(live.Command{Kind: live.CmdSetMap, MapID: uuid.UUID(w.dungeon(t).ID).String()})
	tb.dmSays(live.Command{Kind: live.CmdSetAmbient, Ambient: domain.AmbientBright})
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Goblin"], Q: 5})
	tb.dmSays(live.Command{Kind: live.CmdMount, TokenID: aria, TargetID: steed, Controlled: true})
	tb.dmSays(live.Command{Kind: live.CmdSetWalls, Hexes: []live.Hex{{Q: 2, R: 0}}, On: true})
	for _, sub := range []*live.Subscriber{tb.player, tb.dm} {
		tb.refusal(sub, "Dismount onto a free hex next to Steed.", live.Command{Kind: live.CmdDismount, TokenID: aria, Q: 2, R: 0})
	}
	// Thrown, she lands on the first hex round the Steed she could step onto: the wall east of it is not one.
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: steed, Effect: "prone"})
	v := look(t, w, tb.dm)
	if a := seatOf(t, v, aria); a.mount != "" || a == (seat{q: 2}) || a == (seat{q: 1}) {
		t.Fatalf("thrown beside a wall, Aria lands = %+v", a)
	}
}
