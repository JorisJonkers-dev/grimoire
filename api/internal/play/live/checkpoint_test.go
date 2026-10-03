package live_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

var anID = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// shown is what the DM and the party see, as it goes over the wire. The Checkpoints themselves are left
// out: they are a list of the Session's past, not part of how it stands.
func shown(t *testing.T, w world, tb *table) (string, string) {
	t.Helper()
	party := look(t, w, tb.player)
	dm := look(t, w, tb.dm)
	dm.Checkpoints = nil
	// Tokens come in the order of their ids, which a second run makes anew: by name they compare.
	for _, v := range []*live.View{dm, party} {
		sort.Slice(v.Tokens, func(i, j int) bool { return v.Tokens[i].Label < v.Tokens[j].Label })
	}
	d, _ := json.Marshal(dm)
	p, _ := json.Marshal(party)
	return string(d), string(p)
}

// named replaces every id by the order it first shows in, so two runs of the same play compare equal
// although each makes ids of its own.
func named(s string) string {
	ids := map[string]string{}
	return anID.ReplaceAllStringFunc(s, func(id string) string {
		if _, ok := ids[id]; !ok {
			ids[id] = fmt.Sprintf("id-%d", len(ids))
		}
		return ids[id]
	})
}

func checkpointNamed(t *testing.T, v *live.View, name string) live.CheckpointView {
	t.Helper()
	for _, c := range v.Checkpoints {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no checkpoint %q in %+v", name, v.Checkpoints)
	return live.CheckpointView{}
}

// rewindTo rewinds, checks that every screen is shown the Session afresh, and returns what the DM is sent.
func rewindTo(t *testing.T, w world, tb *table, c live.CheckpointView) live.Update {
	t.Helper()
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdRewind, CheckpointID: c.ID, Nonce: "back"})
	d, p := next(t, tb.dm), next(t, tb.player)
	if d.Kind != live.UpdSnapshot || p.Kind != live.UpdSnapshot || d.Nonce != "back" || p.Nonce != "" || d.ActionSeq == 0 || p.ActionSeq != 0 {
		t.Fatalf("a rewind shows every screen the Session afresh: dm %+v, party %+v", d, p)
	}
	return d
}

// A Checkpoint is a named point the DM can rewind the Session to. Rewinding puts the board, the fight
// and the fog back as they were, on every screen and after a restart; playing on from there goes as it
// went the first time.
func TestRewindingToACheckpointPutsTheSessionBackAsItWas(t *testing.T) {
	t.Parallel()
	w, tb, ids := magicTable(t)
	m := w.dungeon(t)
	tb.dmSays(live.Command{Kind: live.CmdSetMap, MapID: uuid.UUID(m.ID).String()})
	tb.fight(ids, "Aria", "Goblin")
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Goblin"], Effect: "poisoned"})
	barrier(t, w, tb)

	d, p := tb.dmSays(live.Command{Kind: live.CmdCheckpoint, Name: "  Before the ambush  ", Nonce: "keep"})
	kept := checkpointNamed(t, d.View, "Before the ambush")
	if kept.Kind != domain.CheckpointNamed || kept.Round != 1 || kept.ActionSeq != d.ActionSeq || d.ActionSeq == 0 || d.Nonce != "keep" || kept.At.IsZero() {
		t.Fatalf("the checkpoint = %+v in %+v", kept, d)
	}
	// The round's own Checkpoint is there too; a player is shown neither.
	if start := checkpointNamed(t, d.View, "Round 1"); start.Kind != domain.CheckpointRound || start.Round != 1 || start.ActionSeq >= kept.ActionSeq {
		t.Fatalf("the round's checkpoint = %+v", start)
	}
	if raw, _ := json.Marshal(p.View); len(p.View.Checkpoints) != 0 || strings.Contains(string(raw), "checkpoints") || strings.Contains(string(raw), "noUndo") {
		t.Fatalf("a player sees the checkpoints: %s", raw)
	}
	before, partyBefore := shown(t, w, tb)

	// Play goes on: a walk, damage, a condition, fog pushed back, a newcomer, and the turn passes.
	var hurt int64
	play := func() (string, string) {
		t.Helper()
		aria := combatant(look(t, w, tb.dm), "Aria")
		if u := tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 1, R: 0}); u.Kind != live.UpdView {
			t.Fatalf("walk = %+v", u)
		}
		d, _ := tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Goblin"], HPDelta: -3})
		hurt = d.ActionSeq
		tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "prone"})
		tb.dmSays(live.Command{Kind: live.CmdRevealHexes, Hexes: []live.Hex{{Q: 5, R: 0}}, On: true})
		tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Brom", TokenKind: domain.TokenParty, Q: 4})
		if u := tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: aria.ID}); u.Kind != live.UpdView {
			t.Fatalf("end turn = %+v", u)
		}
		return shown(t, w, tb)
	}
	after, partyAfter := play()
	if after == before || partyAfter == partyBefore {
		t.Fatal("nothing happened after the checkpoint")
	}
	later, _ := tb.dmSays(live.Command{Kind: live.CmdCheckpoint, Name: "After the ambush"})
	checkpointNamed(t, later.View, "After the ambush")

	d = rewindTo(t, w, tb, kept)
	// The Checkpoint it went to is still there to go back to again; the one made later is of a future
	// that is gone.
	if got := d.View.Checkpoints; len(got) != 2 || got[1].ID != kept.ID {
		t.Fatalf("checkpoints after the rewind = %+v", got)
	}
	if now, partyNow := shown(t, w, tb); now != before || partyNow != partyBefore {
		t.Fatalf("after the rewind the DM sees\n%s\nwant\n%s\nand the party\n%s\nwant\n%s", now, before, partyNow, partyBefore)
	}
	// What the rewind took back cannot be undone on top of it.
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdUndo, Seq: hurt})
	if u := next(t, tb.dm); u.Kind != live.UpdRejected || u.Reason != "A rewind already took that action back." {
		t.Fatalf("undoing an action a rewind took back = %+v", u)
	}

	// A runtime that starts afresh reads the same Session.
	w.hub.Close(w.session.ID)
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	if now, partyNow := shown(t, w, tb); now != before || partyNow != partyBefore {
		t.Fatalf("after a restart the DM sees\n%s\nwant\n%s", now, before)
	}

	// Played again from the Checkpoint, it goes as it went.
	again, partyAgain := play()
	if named(again) != named(after) || named(partyAgain) != named(partyAfter) {
		t.Fatalf("the replay shows the DM\n%s\nwant\n%s", named(again), named(after))
	}
	// And the Checkpoint is still good for another go.
	rewindTo(t, w, tb, kept)
	if now, _ := shown(t, w, tb); now != before {
		t.Fatalf("after a second rewind the DM sees\n%s\nwant\n%s", now, before)
	}

	// The Action Log keeps all of it: the Checkpoint, what was played, and each rewind.
	var kinds []string
	rows, err := w.pool.Query(context.Background(), "SELECT kind FROM play.actions WHERE session_id = $1 AND kind IN ('checkpoint_created', 'session_rewound', 'hp_adjusted') ORDER BY seq", uuid.UUID(w.session.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, k)
	}
	if got := strings.Join(kinds, " "); got != "checkpoint_created hp_adjusted checkpoint_created session_rewound hp_adjusted session_rewound" {
		t.Fatalf("the log = %s", got)
	}
}

// Every round of a fight starts with a Checkpoint of its own, so the DM can take a round over.
func TestEveryRoundLeavesACheckpoint(t *testing.T) {
	t.Parallel()
	w, tb, ids := magicTable(t)
	if got := look(t, w, tb.dm).Checkpoints; len(got) != 0 {
		t.Fatalf("checkpoints before any fight = %+v", got)
	}
	tb.fight(ids, "Aria", "Goblin")
	endTurn := func(label string) *live.View {
		t.Helper()
		v := look(t, w, tb.dm)
		c := combatant(v, label)
		if label == "Aria" {
			tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: c.ID})
			return look(t, w, tb.dm)
		}
		d, _ := tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: c.ID})
		return d.View
	}
	names := func(v *live.View) string {
		var out []string
		for _, c := range v.Checkpoints {
			out = append(out, c.Name+"/"+c.Kind)
		}
		return strings.Join(out, " ")
	}
	if got := names(look(t, w, tb.dm)); got != "Round 1/round" {
		t.Fatalf("as the fight starts = %s", got)
	}
	endTurn("Aria")
	v := endTurn("Goblin")
	if got := names(v); got != "Round 1/round Round 2/round" || v.Combat.Round != 2 {
		t.Fatalf("as round 2 starts = %s, round %d", got, v.Combat.Round)
	}
	// Round 2 goes badly; the DM takes it over from its start.
	tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Aria"], HPDelta: -9})
	endTurn("Aria")
	endTurn("Goblin")
	if v := look(t, w, tb.dm); v.Combat.Round != 3 || tokenNamed(t, v, "Aria").HP == nil || *tokenNamed(t, v, "Aria").HP != 3 {
		t.Fatalf("round 3 = %+v", v.Combat)
	}
	d := rewindTo(t, w, tb, checkpointNamed(t, look(t, w, tb.dm), "Round 2"))
	if got := names(d.View); got != "Round 1/round Round 2/round" || d.View.Combat.Round != 2 || !combatant(d.View, "Aria").Acting || *tokenNamed(t, d.View, "Aria").HP != 12 {
		t.Fatalf("back at the start of round 2 = %s, %+v", got, d.View.Combat)
	}
	// A long fight keeps its last twenty rounds; the older ones go, here and in what is kept.
	for v := look(t, w, tb.dm); v.Combat.Round < 24; {
		endTurn("Aria")
		v = endTurn("Goblin")
	}
	last := look(t, w, tb.dm).Checkpoints
	if len(last) != 20 || last[0].Name != "Round 5" || last[19].Name != "Round 24" {
		t.Fatalf("after 24 rounds = %s", names(look(t, w, tb.dm)))
	}
	var keptRounds int
	if err := w.pool.QueryRow(context.Background(), "SELECT count(*) FROM play.checkpoints WHERE session_id = $1", uuid.UUID(w.session.ID)).Scan(&keptRounds); err != nil || keptRounds != 20 {
		t.Fatalf("rounds kept = %d, %v", keptRounds, err)
	}
	rewindTo(t, w, tb, checkpointNamed(t, look(t, w, tb.dm), "Round 2"+"3"))
	rewindTo(t, w, tb, last[0])
	for range 2 {
		endTurn("Aria")
		endTurn("Goblin")
	}
	// Ending the fight and starting another starts the count again.
	tb.dmSays(live.Command{Kind: live.CmdEndCombat})
	tb.fight(ids, "Aria", "Goblin")
	if got := names(look(t, w, tb.dm)); got != "Round 5/round Round 6/round Round 7/round Round 1/round" {
		t.Fatalf("a second fight = %s", got)
	}
}

// A rewind to before the dice were rolled puts the Roll Cards back out, and takes back the rolls that
// were asked for since.
func TestRewindingReopensRollsThatWereStillOut(t *testing.T) {
	t.Parallel()
	w, tb, ids := magicTable(t)
	ctx := context.Background()
	d, _ := tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: []live.CombatantSetup{{TokenID: ids["Aria"], SpeedFt: 30}, {TokenID: ids["Goblin"], SpeedFt: 30}}})
	aria, goblin := combatant(d.View, "Aria"), combatant(d.View, "Goblin")
	d, _ = tb.dmSays(live.Command{Kind: live.CmdCheckpoint, Name: "Roll for it"})
	kept := checkpointNamed(t, d.View, "Roll for it")
	before, partyBefore := shown(t, w, tb)

	tb.roll(aria, w.player, 4)
	tb.roll(goblin, w.dm, 18)
	if v := barrier(t, w, tb); v.Combat.Status != domain.CombatActive {
		t.Fatalf("the fight after both rolled = %+v", v.Combat)
	}
	rolls := pgstore.New(w.pool)
	if r, err := rolls.Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(aria.RollID))); err != nil || r.Status != domain.StatusResolved {
		t.Fatalf("Aria's roll = %+v, %v", r, err)
	}
	// The goblin attacks: a roll asked for after the Checkpoint, and nobody answers it.
	hit, _ := tb.dmSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Goblin"], TargetID: ids["Aria"], AttackNo: 1})
	asked := hit.View.Combat.Attack
	if asked == nil || asked.RollID == "" {
		t.Fatalf("the goblin's attack = %+v", hit.View.Combat)
	}

	rewindTo(t, w, tb, kept)
	if now, partyNow := shown(t, w, tb); now != before || partyNow != partyBefore {
		t.Fatalf("after the rewind the DM sees\n%s\nwant\n%s", now, before)
	}
	r, err := rolls.Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(aria.RollID)))
	if err != nil || r.Status != domain.StatusPending || r.Dice[0].Value != 0 || r.Total != 0 {
		t.Fatalf("Aria's roll after the rewind = %+v, %v", r, err)
	}
	if _, err := rolls.Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(asked.RollID))); err == nil {
		t.Fatal("the attack roll asked for after the Checkpoint is still out")
	}
	// The dice can be rolled again, and need not fall the same way.
	tb.roll(aria, w.player, 19)
	tb.roll(goblin, w.dm, 2)
	if v := barrier(t, w, tb); v.Combat.Status != domain.CombatActive || !combatant(v, "Aria").Acting {
		t.Fatalf("the fight after rolling again = %+v", v.Combat)
	}
}

// A Campaign can be played without undo: nothing is taken back, no Checkpoint is made, and no round
// leaves one.
func TestACampaignPlayedWithoutUndo(t *testing.T) {
	t.Parallel()
	w := setup(t)
	if _, err := w.pool.Exec(context.Background(), "UPDATE campaign.campaigns SET no_undo = true WHERE id = $1", w.session.CampaignID); err != nil {
		t.Fatal(err)
	}
	w, tb, ids := magicTableIn(t, w)
	tb.fight(ids, "Aria", "Goblin")
	d, p := tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Goblin"], HPDelta: -3})
	if !d.View.NoUndo || len(d.View.Checkpoints) != 0 || p.View.NoUndo {
		t.Fatalf("without undo the DM sees noUndo %v and %+v, the party noUndo %v", d.View.NoUndo, d.View.Checkpoints, p.View.NoUndo)
	}
	for name, cmd := range map[string]live.Command{
		"undo":        {Kind: live.CmdUndo, Seq: d.ActionSeq},
		"undo damage": {Kind: live.CmdUndoDamage},
		"checkpoint":  {Kind: live.CmdCheckpoint, Name: "Here"},
		"rewind":      {Kind: live.CmdRewind, CheckpointID: uuid.NewString()},
	} {
		w.hub.Submit(tb.dm, cmd)
		if u := next(t, tb.dm); u.Kind != live.UpdRejected || u.Reason != "This Campaign is played without undo." {
			t.Fatalf("%s without undo = %+v", name, u)
		}
	}
	var n int
	if err := w.pool.QueryRow(context.Background(), "SELECT count(*) FROM play.checkpoints WHERE session_id = $1", uuid.UUID(w.session.ID)).Scan(&n); err != nil || n != 0 {
		t.Fatalf("checkpoints kept without undo: %d, %v", n, err)
	}
}

type noRewind struct{ live.Store }

func (noRewind) SaveCheckpoint(context.Context, domain.Session, domain.Checkpoint, domain.Member, caller.Caller) (live.Committed, error) {
	return live.Committed{}, errors.New("disk full")
}

func (noRewind) MarkRound(context.Context, domain.Session, domain.Checkpoint, int) error {
	return errors.New("disk full")
}

func (noRewind) Rewind(context.Context, domain.Session, domain.Checkpoint, domain.Member, caller.Caller, time.Time, func(live.Store) error) (live.Committed, error) {
	return live.Committed{}, errors.New("disk full")
}

// unreadable puts the Session back and then cannot read it.
type unreadable struct{ live.Store }

func (u unreadable) Rewind(ctx context.Context, s domain.Session, c domain.Checkpoint, m domain.Member, cl caller.Caller, now time.Time, _ func(live.Store) error) (live.Committed, error) {
	return u.Store.Rewind(ctx, s, c, m, cl, now, func(live.Store) error { return errors.New("gone") })
}

// Checkpoints are the DM's, named, and of this Session; what cannot be kept or put back says so and
// changes nothing.
func TestWhatACheckpointRefuses(t *testing.T) {
	t.Parallel()
	w, tb, ids := magicTable(t)
	refuse := func(sub *live.Subscriber, want string, cmd live.Command) {
		t.Helper()
		w.hub.Submit(sub, cmd)
		if u := next(t, sub); u.Kind != live.UpdRejected || u.Reason != want {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	for _, kind := range []string{live.CmdCheckpoint, live.CmdRewind} {
		refuse(tb.player, "Only the DM can change the table.", live.Command{Kind: kind, Name: "Mine", CheckpointID: uuid.NewString()})
	}
	for _, name := range []string{"", "   ", strings.Repeat("x", 61), strings.Repeat("🎲", 31)} {
		refuse(tb.dm, "Name the checkpoint, in up to 60 characters.", live.Command{Kind: live.CmdCheckpoint, Name: name})
	}
	d, _ := tb.dmSays(live.Command{Kind: live.CmdCheckpoint, Name: strings.Repeat("🎲", 30)})
	kept := d.View.Checkpoints[0]
	for _, id := range []string{"", "nope", uuid.NewString()} {
		refuse(tb.dm, "No such checkpoint.", live.Command{Kind: live.CmdRewind, CheckpointID: id})
	}

	// A store that cannot keep or put back: the Session stays as it is, with no round's Checkpoint either.
	w.hub.Close(w.session.ID)
	good := w.hub.Store
	w.hub.Store = noRewind{good}
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	refuse(tb.dm, "That checkpoint could not be saved.", live.Command{Kind: live.CmdCheckpoint, Name: "Here"})
	refuse(tb.dm, "The rewind could not be made.", live.Command{Kind: live.CmdRewind, CheckpointID: kept.ID})
	tb.fight(ids, "Aria", "Goblin")
	if v := look(t, w, tb.dm); len(v.Checkpoints) != 1 || v.Combat.Round != 1 {
		t.Fatalf("with a store that keeps nothing = %+v, round %d", v.Checkpoints, v.Combat.Round)
	}

	// A rewind that cannot be read back is not made at all: the fight goes on where it was, here and
	// in what is kept.
	w.hub.Close(w.session.ID)
	w.hub.Store = unreadable{good}
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	before, _ := shown(t, w, tb)
	refuse(tb.dm, "The rewind could not be made.", live.Command{Kind: live.CmdRewind, CheckpointID: kept.ID})
	if now, _ := shown(t, w, tb); now != before {
		t.Fatalf("after a rewind that was not made the DM sees\n%s\nwant\n%s", now, before)
	}
	w.hub.Close(w.session.ID)
	w.hub.Store = good
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	if now, _ := shown(t, w, tb); now != before {
		t.Fatalf("what is kept after a rewind that was not made shows the DM\n%s\nwant\n%s", now, before)
	}

	// A Session keeps fifty named Checkpoints and no more.
	for i := 1; i < 50; i++ {
		d, _ = tb.dmSays(live.Command{Kind: live.CmdCheckpoint, Name: fmt.Sprintf("Point %d", i)})
	}
	if len(d.View.Checkpoints) != 50 {
		t.Fatalf("checkpoints = %d", len(d.View.Checkpoints))
	}
	refuse(tb.dm, "A Session keeps up to 50 named checkpoints.", live.Command{Kind: live.CmdCheckpoint, Name: "One more"})
}

type noSetting struct{ live.Store }

func (noSetting) NoUndo(context.Context, uuid.UUID) (bool, error) { return false, errors.New("gone") }

type noCheckpoints struct{ live.Store }

func (noCheckpoints) Checkpoints(context.Context, domain.SessionID) ([]domain.Checkpoint, error) {
	return nil, errors.New("gone")
}

// A Session whose Checkpoints or undo setting cannot be read does not start half-known.
func TestASessionNeedsItsCheckpointsToStart(t *testing.T) {
	t.Parallel()
	w := setup(t)
	good := w.hub.Store
	for name, store := range map[string]live.Store{"the undo setting": noSetting{good}, "the checkpoints": noCheckpoints{good}} {
		w.hub.Store = store
		if _, err := w.hub.Join(context.Background(), w.session.ID, w.dm, dmCaller, live.AudienceDM); err == nil {
			t.Fatalf("a Session started without %s", name)
		}
	}
	w.hub.Store = good
	join(t, w, w.dm, dmCaller, live.AudienceDM)
}
