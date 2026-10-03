package live_test

import (
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

// At 0 hit points a Character lies Unconscious and dying: it makes death saves at the start of its
// turns, damage costs it failures, an ally stabilises it with Medicine or a spell, healing wakes it,
// massive damage kills it, and revival magic works while its window lasts.
func TestDownedStabilisedAndRevived(t *testing.T) {
	t.Parallel()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Label: "Brom", Q: 0, R: 1})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 3})
	ids := map[string]string{}
	var setup []live.CombatantSetup
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
		setup = append(setup, live.CombatantSetup{TokenID: tv.ID, SpeedFt: 30})
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: setup})
	tb.roll(combatant(d.View, "Aria"), w.player, 20)
	tb.roll(combatant(d.View, "Brom"), w.player, 15)
	tb.roll(combatant(d.View, "Goblin"), w.dm, 10)
	settle := func() *live.View {
		t.Helper()
		barrier(t, w, tb)
		return look(t, w, tb.dm)
	}
	say := func(sub *live.Subscriber, cmd live.Command) *live.View {
		t.Helper()
		w.hub.Submit(sub, cmd)
		if u := next(t, sub); u.Kind == live.UpdRejected {
			t.Fatalf("%s = %s", cmd.Kind, u.Reason)
		}
		return settle()
	}
	refuse := func(want string, cmd live.Command) {
		t.Helper()
		settle()
		w.hub.Submit(tb.dm, cmd)
		if u := next(t, tb.dm); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, want) {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	endTurn := func(label string) *live.View {
		t.Helper()
		v := look(t, w, tb.dm)
		sub := tb.dm
		if label != "Goblin" {
			sub = tb.player
		}
		return say(sub, live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(v, label).ID})
	}
	dyingOf := func(v *live.View, label string) live.DyingView {
		t.Helper()
		d := token(v, label).Dying
		if d == nil {
			t.Fatalf("%s is not dying: %+v", label, token(v, label))
		}
		return *d
	}

	v := say(tb.dm, live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Aria"], HPDelta: -12})
	if effect(token(v, "Aria"), "Unconscious") == nil || dyingOf(v, "Aria") != (live.DyingView{}) {
		t.Fatalf("at 0 hit points Aria lies Unconscious and dying = %+v", token(v, "Aria"))
	}
	endTurn("Aria")
	refuse("Only a dying creature", live.Command{Kind: live.CmdStabilise, TokenID: ids["Brom"], TargetID: ids["Goblin"], Option: "spell"})
	endTurn("Brom")
	v = say(tb.dm, live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Aria"], HPDelta: -1})
	if dyingOf(v, "Aria").Failures != 1 {
		t.Fatalf("damage while down is a failure = %+v", dyingOf(v, "Aria"))
	}
	v = endTurn("Goblin")
	save := dyingOf(v, "Aria").RollID
	if save == "" || lastRoll(t, w, "Aria: death saving throw").Roller.ID != w.player.ID {
		t.Fatalf("Aria's turn opens her death save = %+v", dyingOf(v, "Aria"))
	}
	tb.fill(save, w.player, 15)
	v = settle()
	if got := dyingOf(v, "Aria"); got.Successes != 1 || got.Failures != 1 || got.RollID != "" {
		t.Fatalf("a 15 succeeds = %+v", got)
	}
	endTurn("Aria")
	refuse("Medicine check or a spell", live.Command{Kind: live.CmdStabilise, TokenID: ids["Brom"], TargetID: ids["Aria"], Option: "prayer"})
	v = say(tb.player, live.Command{Kind: live.CmdStabilise, TokenID: ids["Brom"], TargetID: ids["Aria"], Option: "spell"})
	if got := dyingOf(v, "Aria"); !got.Stable || got.Successes != 0 || combatant(v, "Brom").Action {
		t.Fatalf("Spare the Dying stabilises her for Brom's action = %+v", got)
	}
	endTurn("Brom")
	endTurn("Goblin")
	if dyingOf(look(t, w, tb.dm), "Aria").RollID != "" {
		t.Fatal("the stable roll no death saves")
	}
	v = say(tb.dm, live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Aria"], HPDelta: 3})
	if token(v, "Aria").Dying != nil || effect(token(v, "Aria"), "Unconscious") != nil || *token(v, "Aria").HP != 3 {
		t.Fatalf("healing wakes her = %+v", token(v, "Aria"))
	}

	say(tb.dm, live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Aria"], HPDelta: -3})
	endTurn("Aria")
	say(tb.player, live.Command{Kind: live.CmdStabilise, TokenID: ids["Brom"], TargetID: ids["Aria"], Option: "medicine"})
	tb.fill(uuid.UUID(lastRoll(t, w, "Wisdom (Medicine) check to stabilise Aria (DC 10)").ID).String(), w.player, 12)
	if !dyingOf(settle(), "Aria").Stable {
		t.Fatal("a Medicine check of 12 stabilises her")
	}

	v = say(tb.dm, live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Brom"], HPDelta: -30})
	if !dyingOf(v, "Brom").Dead || effect(token(v, "Brom"), "Unconscious") != nil {
		t.Fatalf("massive damage kills outright = %+v", token(v, "Brom"))
	}
	refuse("only revival magic", live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Brom"], HPDelta: 5})
	refuse("Only the dead", live.Command{Kind: live.CmdRevive, TargetID: ids["Aria"], Option: "revivify"})
	refuse("Revivify, Raise Dead or Resurrection", live.Command{Kind: live.CmdRevive, TargetID: ids["Brom"], Option: "wish"})
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	v = say(tb.dm, live.Command{Kind: live.CmdRevive, TargetID: ids["Brom"], Option: "revivify"})
	if token(v, "Brom").Dying != nil || *token(v, "Brom").HP != 1 {
		t.Fatalf("Revivify within the minute, after a restart = %+v", token(v, "Brom"))
	}
	say(tb.dm, live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Brom"], HPDelta: -30})
	say(tb.dm, live.Command{Kind: live.CmdEndCombat})
	refuse("too late for revivify", live.Command{Kind: live.CmdRevive, TargetID: ids["Brom"], Option: "revivify"})
	if v = say(tb.dm, live.Command{Kind: live.CmdRevive, TargetID: ids["Brom"], Option: "raise_dead"}); *token(v, "Brom").HP != 1 {
		t.Fatalf("Raise Dead after the fight = %+v", token(v, "Brom"))
	}
}
