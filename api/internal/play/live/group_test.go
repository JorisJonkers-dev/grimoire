package live_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func tokenLabels(v *live.View) string {
	var out []string
	for _, t := range v.Tokens {
		out = append(out, t.Label)
	}
	return strings.Join(out, " ")
}

// elsewhere is where a join that is refused sends the member, or the zero id when it is let in.
func elsewhere(t *testing.T, w world, id domain.SessionID, m domain.Member, c caller.Caller, a live.Audience) domain.SessionID {
	t.Helper()
	_, err := w.hub.Join(context.Background(), id, m, c, a)
	var away *live.ElsewhereError
	if !errors.As(err, &away) {
		t.Fatalf("joining %s as %s = %v", uuid.UUID(id), a, err)
	}
	return away.Session
}

func joinAt(t *testing.T, w world, id domain.SessionID, m domain.Member, c caller.Caller, a live.Audience) *live.Subscriber {
	t.Helper()
	sub, err := w.hub.Join(context.Background(), id, m, c, a)
	if err != nil {
		t.Fatal(err)
	}
	if u := next(t, sub); u.Kind != live.UpdSnapshot {
		t.Fatalf("first update = %+v", u)
	}
	return sub
}

// sentAfter reads a screen until it is let go and returns the Session it was sent to. A screen that is
// told the Session ended, or shown it once more, was not sent anywhere.
func sentAfter(t *testing.T, sub *live.Subscriber) domain.SessionID {
	t.Helper()
	var to domain.SessionID
	late := time.After(20 * time.Second)
	for {
		select {
		case u, ok := <-sub.Out:
			if !ok {
				return to
			}
			if u.Kind != live.UpdRegroup || u.Group == nil {
				t.Fatalf("a screen that belongs elsewhere was sent %+v", u)
			}
			to = domain.SessionID(uuid.MustParse(u.Group.SessionID))
		case <-late:
			t.Fatal("the screen was never let go")
		}
	}
}

func lookAt(t *testing.T, w world, sub *live.Subscriber) *live.View {
	t.Helper()
	return look(t, w, sub)
}

// The party splits: a group goes to a map of its own, in a Session of its own, with its own fog and
// its own fight. Whoever went is sent after it, the DM sees every group, the Table Display follows one,
// and neither group sees the other. Brought back, the group stands with the party again.
func TestThePartySplitsAcrossMaps(t *testing.T) {
	t.Parallel()
	w, tb, ids := magicTable(t)
	ctx := context.Background()
	crypt, tower := w.dungeon(t), w.picture(t, "Tower", domain.MapLocal)
	tb.dmSays(live.Command{Kind: live.CmdSetMap, MapID: uuid.UUID(crypt.ID).String()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Brom", TokenKind: domain.TokenParty, Q: 1})
	// What is on a token goes where the token goes.
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], Effect: "poisoned"})
	tv := join(t, w, w.player, playerCaller, live.AudienceTable)
	if v := look(t, w, tb.dm); len(v.Groups) != 0 {
		t.Fatalf("a party that is together has groups: %+v", v.Groups)
	}
	home := w.session.ID

	// Aria goes up the tower; Brom stays in the crypt with the goblin.
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdSplitParty, Nonce: "go", Name: "  The tower  ", TokenIDs: []string{ids["Aria"]}, MapID: uuid.UUID(tower.ID).String()})
	d := next(t, tb.dm)
	if d.Kind != live.UpdSnapshot || d.Nonce != "go" || d.ActionSeq == 0 || tokenLabels(d.View) != "Brom Goblin" && tokenLabels(d.View) != "Goblin Brom" {
		t.Fatalf("the DM after the split = %+v, tokens %s", d, tokenLabels(d.View))
	}
	if g := d.View.Groups; len(g) != 2 || !g[0].Home || !g[0].Here || !g[0].Table || strings.Join(g[0].Tokens, ",") != "Brom" ||
		g[1].Home || g[1].Here || g[1].Table || g[1].Name != "The tower" || strings.Join(g[1].Tokens, ",") != "Aria" || g[1].Number != g[0].Number+1 {
		t.Fatalf("groups = %+v", d.View.Groups)
	}
	away := domain.SessionID(uuid.MustParse(d.View.Groups[1].SessionID))
	// The Player who went is sent after the group and sees nothing more of the crypt.
	if to := sentAfter(t, tb.player); to != away {
		t.Fatalf("Aria's player was sent to %s, want the tower", uuid.UUID(to))
	}
	// The Table Display stays with the party it was following, and shows the crypt.
	if u := next(t, tv); u.Kind != live.UpdSnapshot || u.View.Map.Name != "Crypt" || strings.Contains(tokenLabels(u.View), "Aria") || len(u.View.Groups) != 0 {
		t.Fatalf("the table after the split = %+v", u)
	}

	// Each group's Party Vision is its own: Aria's player may not watch the crypt, only the tower.
	if to := elsewhere(t, w, home, w.player, playerCaller, live.AudienceParty); to != away {
		t.Fatalf("watching the crypt sends Aria's player to %s", uuid.UUID(to))
	}
	if to := elsewhere(t, w, away, w.player, playerCaller, live.AudienceTable); to != home {
		t.Fatalf("a table opened on the tower is sent to %s", uuid.UUID(to))
	}
	up := joinAt(t, w, away, w.player, playerCaller, live.AudienceParty)
	upDM := joinAt(t, w, away, w.dm, dmCaller, live.AudienceDM)
	v := lookAt(t, w, upDM)
	if tokenLabels(v) != "Aria" || v.Map == nil || v.Map.Name != "Tower" || tokenNamed(t, v, "Aria").Q != 0 || len(v.Groups) != 2 || !v.Groups[1].Here || v.Groups[0].Here {
		t.Fatalf("the DM in the tower sees %s on %+v, groups %+v", tokenLabels(v), v.Map, v.Groups)
	}
	if fx := tokenNamed(t, v, "Aria").Effects; len(fx) != 1 || fx[0].Slug != "poisoned" {
		t.Fatalf("Aria's effects in the tower = %+v", fx)
	}
	if p := lookAt(t, w, up); tokenLabels(p) != "Aria" || p.Map.Name != "Tower" || len(p.Groups) != 0 {
		t.Fatalf("Aria's player in the tower sees %s on %+v", tokenLabels(p), p.Map)
	}

	// Each group plays on by itself: a walk in the tower, a blow in the crypt.
	w.hub.Submit(up, live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 1, R: 0})
	if u := next(t, up); u.Kind != live.UpdView || tokenNamed(t, u.View, "Aria").Q != 1 {
		t.Fatalf("Aria walks in the tower = %+v", u)
	}
	next(t, upDM)
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Goblin"], HPDelta: -2})
	if u := next(t, tb.dm); u.Kind != live.UpdView || strings.Contains(tokenLabels(u.View), "Aria") || *tokenNamed(t, u.View, "Goblin").HP != 5 {
		t.Fatalf("the crypt goes on = %+v", u)
	}
	next(t, tv)
	// A split party keeps no Checkpoints: one Session's rows are no longer the whole of it.
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdCheckpoint, Name: "Here"})
	if u := next(t, tb.dm); u.Kind != live.UpdRejected || u.Reason != "Bring the party back together before keeping a checkpoint." {
		t.Fatalf("a checkpoint while split = %+v", u)
	}

	// The Table Display follows the tower: the screen is sent there, and the tower hears of it.
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdTableFollow, SessionID: uuid.UUID(away).String()})
	if u := next(t, tb.dm); u.Kind != live.UpdSnapshot || u.View.Groups[0].Table || !u.View.Groups[1].Table {
		t.Fatalf("the table follows the tower = %+v", u.View.Groups)
	}
	if to := sentAfter(t, tv); to != away {
		t.Fatalf("the table was sent to %s", uuid.UUID(to))
	}
	if u := next(t, upDM); u.Kind != live.UpdView || !u.View.Groups[1].Table {
		t.Fatalf("the tower hears the table follows it = %+v", u)
	}
	next(t, up)
	if to := elsewhere(t, w, home, w.player, playerCaller, live.AudienceTable); to != away {
		t.Fatalf("a table opened on the crypt is sent to %s", uuid.UUID(to))
	}
	tv = joinAt(t, w, away, w.player, playerCaller, live.AudienceTable)
	// And back to the party it split from.
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdTableFollow})
	if u := next(t, tb.dm); u.Kind != live.UpdSnapshot || !u.View.Groups[0].Table {
		t.Fatalf("the table follows the crypt again = %+v", u.View.Groups)
	}
	if to := sentAfter(t, tv); to != home {
		t.Fatalf("the table was sent to %s", uuid.UUID(to))
	}
	next(t, upDM)
	next(t, up)

	// The group comes back: Aria stands in the crypt again, her player is sent after her, and the
	// tower's Session is over.
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdRejoinParty, SessionID: uuid.UUID(away).String(), Q: 2, R: 0})
	d = next(t, tb.dm)
	if fx := tokenNamed(t, d.View, "Aria").Effects; len(fx) != 1 || fx[0].Slug != "poisoned" {
		t.Fatalf("Aria's effects back in the crypt = %+v", fx)
	}
	if d.Kind != live.UpdSnapshot || len(d.View.Groups) != 0 || tokenNamed(t, d.View, "Aria").Q != 2 || len(d.View.Tokens) != 3 {
		t.Fatalf("the DM after the group came back = %+v, tokens %s", d, tokenLabels(d.View))
	}
	for name, sub := range map[string]*live.Subscriber{"the player": up, "the DM": upDM} {
		if to := sentAfter(t, sub); to != home {
			t.Fatalf("%s in the tower was sent to %s", name, uuid.UUID(to))
		}
	}
	if to := elsewhere(t, w, away, w.player, playerCaller, live.AudienceParty); to != home {
		t.Fatalf("the tower, once over, sends a player to %s", uuid.UUID(to))
	}
	var status string
	if err := w.pool.QueryRow(ctx, "SELECT status FROM play.sessions WHERE id = $1", uuid.UUID(away)).Scan(&status); err != nil || status != domain.SessionEnded {
		t.Fatalf("the tower's Session is %s, %v", status, err)
	}
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	if p := look(t, w, tb.player); !strings.Contains(tokenLabels(p), "Aria") || p.Map.Name != "Crypt" {
		t.Fatalf("Aria's player back in the crypt sees %s", tokenLabels(p))
	}
	// Together again, the party keeps Checkpoints as before.
	if d, _ := tb.dmSays(live.Command{Kind: live.CmdCheckpoint, Name: "Together"}); len(d.View.Checkpoints) != 1 {
		t.Fatalf("checkpoints after the group came back = %+v", d.View.Checkpoints)
	}
}

// The groups of a split party share the Campaign's Containers: loot that drops for one shows for the
// other without anyone asking.
func TestGroupsShareWhatTheCampaignKeeps(t *testing.T) {
	t.Parallel()
	w, tb := ambushTable(t)
	loot := stocked(t, w)
	tower := w.picture(t, "Tower", domain.MapLocal)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	d := tbPlace(t, w, tb.dm, live.Command{Kind: live.CmdPlace, Label: "Brom", TokenKind: domain.TokenParty, Q: 1})
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdSplitParty, Name: "The tower", TokenIDs: []string{tokenNamed(t, d, "Brom").ID}, MapID: uuid.UUID(tower.ID).String()})
	split := next(t, tb.dm)
	if split.Kind != live.UpdSnapshot || len(split.View.Groups) != 2 {
		t.Fatalf("split = %+v", split)
	}
	away := domain.SessionID(uuid.MustParse(split.View.Groups[1].SessionID))
	up := joinAt(t, w, away, w.dm, dmCaller, live.AudienceDM)
	if v := look(t, w, up); len(v.Inventory) == 0 || slicesContainLabel(v, "Loot: Hoard") {
		t.Fatalf("the tower's Containers before the drop = %+v", v.Inventory)
	}
	// Loot drops in the Session the party split from: the tower is shown it too.
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdRollLoot, LootTableID: loot["Hoard"]})
	if u := next(t, tb.dm); u.Kind != live.UpdView || !slicesContainLabel(u.View, "Loot: Hoard") {
		t.Fatalf("the drop = %+v", u)
	}
	if u := next(t, up); u.Kind != live.UpdView || !slicesContainLabel(u.View, "Loot: Hoard") {
		t.Fatalf("the tower after the drop = %+v", u)
	}
	// A change that is one group's own, a token placed, is not passed on.
	tbPlace(t, w, tb.dm, live.Command{Kind: live.CmdPlace, Label: "Rat", TokenKind: domain.TokenEnemy, Q: 2})
	w.hub.Submit(up, live.Command{Kind: live.CmdPlace, Label: "Owl", TokenKind: domain.TokenNPC, Q: 2})
	if u := next(t, up); u.Kind != live.UpdView || tokenLabels(u.View) != "Brom Owl" && tokenLabels(u.View) != "Owl Brom" {
		t.Fatalf("the tower was shown something of the other group: %+v, tokens %s", u, tokenLabels(u.View))
	}
}

func slicesContainLabel(v *live.View, label string) bool {
	for _, c := range v.Inventory {
		if c.Label == label {
			return true
		}
	}
	return false
}

// tbPlace sends a DM command and returns the view it answers with.
func tbPlace(t *testing.T, w world, dm *live.Subscriber, cmd live.Command) *live.View {
	t.Helper()
	w.hub.Submit(dm, cmd)
	u := next(t, dm)
	if u.Kind != live.UpdView {
		t.Fatalf("%s rejected: %+v", cmd.Kind, u)
	}
	return u.View
}

type noGroups struct{ live.Store }

func (noGroups) Groups(context.Context, domain.Session) ([]domain.PartyGroup, error) {
	return nil, errors.New("gone")
}

type noPlace struct{ live.Store }

func (noPlace) Place(context.Context, domain.SessionID, uuid.UUID, bool, bool) (domain.SessionID, error) {
	return domain.SessionID{}, errors.New("gone")
}

type noSplit struct{ live.Store }

func (noSplit) SplitParty(context.Context, domain.Session, string, domain.MapID, []domain.TokenPlace, domain.Member, caller.Caller, time.Time, func(live.Store) error) (domain.Session, live.Committed, error) {
	return domain.Session{}, live.Committed{}, errors.New("disk full")
}

func (noSplit) RejoinParty(context.Context, domain.Session, domain.SessionID, []domain.TokenPlace, domain.Member, caller.Caller, time.Time, func(live.Store) error) (live.Committed, error) {
	return live.Committed{}, errors.New("disk full")
}

func (noSplit) FollowTable(context.Context, domain.Session, *domain.SessionID, domain.Member, caller.Caller, time.Time) (live.Committed, error) {
	return live.Committed{}, errors.New("disk full")
}

// unreadableSplit splits and then cannot read the Session back, so the split is not made.
type unreadableSplit struct{ live.Store }

func (u unreadableSplit) SplitParty(ctx context.Context, s domain.Session, name string, to domain.MapID, places []domain.TokenPlace, m domain.Member, cl caller.Caller, now time.Time, _ func(live.Store) error) (domain.Session, live.Committed, error) {
	return u.Store.SplitParty(ctx, s, name, to, places, m, cl, now, func(live.Store) error { return errors.New("gone") })
}

// A group forms and comes back between scenes, by the DM's hand, out of party tokens, to a map that is
// there; what cannot be done says so and changes nothing.
func TestWhatSplittingThePartyRefuses(t *testing.T) {
	t.Parallel()
	w, tb, ids := magicTable(t)
	crypt, tower := w.dungeon(t), w.picture(t, "Tower", domain.MapLocal)
	tb.dmSays(live.Command{Kind: live.CmdSetMap, MapID: uuid.UUID(crypt.ID).String()})
	for q, name := range []string{"Brom", "Cade", "Dara"} {
		d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, Label: name, TokenKind: domain.TokenParty, Q: q, R: 1})
		ids[name] = tokenNamed(t, d.View, name).ID
	}
	refuse := func(sub *live.Subscriber, want string, cmd live.Command) {
		t.Helper()
		w.hub.Submit(sub, cmd)
		if u := next(t, sub); u.Kind != live.UpdRejected || u.Reason != want {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	to := uuid.UUID(tower.ID).String()
	split := func(name string, tokens ...string) live.Command {
		return live.Command{Kind: live.CmdSplitParty, Name: name, TokenIDs: tokens, MapID: to}
	}
	for _, kind := range []string{live.CmdSplitParty, live.CmdRejoinParty, live.CmdTableFollow} {
		refuse(tb.player, "Only the DM can change the table.", live.Command{Kind: kind})
	}
	refuse(tb.dm, "Choose who goes.", split("Up"))
	refuse(tb.dm, "No such token.", split("Up", uuid.NewString()))
	refuse(tb.dm, "No such token.", split("Up", "nope"))
	refuse(tb.dm, "Only party tokens split off as a group.", split("Up", ids["Goblin"]))
	refuse(tb.dm, "Choose each token once.", split("Up", ids["Aria"], ids["Aria"]))
	refuse(tb.dm, "Name the group, in up to 40 characters.", split("  ", ids["Aria"]))
	refuse(tb.dm, "Name the group, in up to 40 characters.", split(strings.Repeat("🎲", 21), ids["Aria"]))
	bad := split("Up", ids["Aria"])
	bad.MapID = "nowhere"
	refuse(tb.dm, "Choose the map the group goes to.", bad)
	bad.MapID = uuid.NewString()
	refuse(tb.dm, "No such map.", bad)
	off := split("Up", ids["Aria"])
	off.Q = 90
	refuse(tb.dm, "That hex is off the map.", off)
	// A closet of a map has no room for four.
	closet, err := pgstore.New(w.pool).InsertMap(context.Background(), domain.Map{
		CampaignID: w.session.CampaignID, Name: "Closet", Kind: domain.MapLocal, ImageKey: "sha256/x.png", ImageType: "image/png", Width: 90, Height: 80,
		HexSize: 40, OriginX: 34.64, OriginY: 40,
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	crowd := split("Up", ids["Aria"], ids["Brom"], ids["Cade"], ids["Dara"])
	crowd.MapID = uuid.UUID(closet.ID).String()
	refuse(tb.dm, "There is no room for the group there.", crowd)
	refuse(tb.dm, "No such group.", live.Command{Kind: live.CmdRejoinParty, SessionID: uuid.NewString()})
	refuse(tb.dm, "No such group.", live.Command{Kind: live.CmdRejoinParty, SessionID: "nope"})
	refuse(tb.dm, "No such group.", live.Command{Kind: live.CmdTableFollow, SessionID: uuid.NewString()})
	refuse(tb.dm, "No such group.", live.Command{Kind: live.CmdTableFollow, SessionID: uuid.UUID(w.session.ID).String()})

	// Not in the middle of a fight, nor of anything else that is under way.
	tb.fight(ids, "Aria", "Goblin")
	refuse(tb.dm, "Finish the fight first.", split("Up", ids["Aria"]))
	tb.dmSays(live.Command{Kind: live.CmdEndCombat})
	tb.dmSays(live.Command{Kind: live.CmdExplore, On: true})
	refuse(tb.dm, "Finish what is under way first: a rest, rolls that are out, sneaking or exploring in turns.", split("Up", ids["Aria"]))
	tb.dmSays(live.Command{Kind: live.CmdExplore, On: false})

	// A store that cannot split, or that splits and cannot be read back: the party stays together.
	w.hub.Close(w.session.ID)
	good := w.hub.Store
	for name, store := range map[string]live.Store{"cannot split": noSplit{good}, "cannot be read back": unreadableSplit{good}} {
		w.hub.Store = store
		tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
		refuse(tb.dm, "The party could not be split.", split("Up", ids["Aria"]))
		if v := look(t, w, tb.dm); len(v.Groups) != 0 || len(v.Tokens) != 5 {
			t.Fatalf("with a store that %s = %+v", name, v.Groups)
		}
		look(t, w, tb.player)
		w.hub.Close(w.session.ID)
	}
	var live1 int
	if err := w.pool.QueryRow(context.Background(), "SELECT count(*) FROM play.sessions WHERE campaign_id = $1 AND status = 'live'", w.session.CampaignID).Scan(&live1); err != nil || live1 != 1 {
		t.Fatalf("live Sessions after splits that were not made: %d, %v", live1, err)
	}
	w.hub.Store = good
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)

	// Three groups leave; a fourth cannot, and a group that left does not split again.
	var groups []live.GroupView
	for i, name := range []string{"Brom", "Cade", "Dara"} {
		w.hub.Submit(tb.dm, split("Group "+name, ids[name]))
		u := next(t, tb.dm)
		if u.Kind != live.UpdSnapshot || len(u.View.Groups) != i+2 {
			t.Fatalf("group %s = %+v", name, u)
		}
		groups = u.View.Groups
		look(t, w, tb.player)
	}
	refuse(tb.dm, "The party is split into as many groups as it can be.", split("Up", ids["Aria"]))
	broms := domain.SessionID(uuid.MustParse(groups[1].SessionID))
	up := joinAt(t, w, broms, w.dm, dmCaller, live.AudienceDM)
	refuse(up, "A group that left the party does not split again.", split("Further", ids["Brom"]))
	refuse(up, "Bring a group back from the Session the party split from.", live.Command{Kind: live.CmdRejoinParty, SessionID: groups[2].SessionID})
	refuse(up, "Choose what the Table Display follows from the Session the party split from.", live.Command{Kind: live.CmdTableFollow, SessionID: groups[2].SessionID})

	// A group in a fight is not ready to come back; nor is there room for it on a hex off the map.
	w.hub.Submit(up, live.Command{Kind: live.CmdPlace, Label: "Rat", TokenKind: domain.TokenEnemy, Q: 2})
	rat := tokenNamed(t, next(t, up).View, "Rat").ID
	w.hub.Submit(up, live.Command{Kind: live.CmdStartCombat, Combatants: []live.CombatantSetup{{TokenID: rat, SpeedFt: 30}}})
	if u := next(t, up); u.Kind != live.UpdView {
		t.Fatalf("the group's fight = %+v", u)
	}
	refuse(tb.dm, "The group is not ready: finish the fight first.", live.Command{Kind: live.CmdRejoinParty, SessionID: groups[1].SessionID})
	refuse(tb.dm, "That hex is off the map.", live.Command{Kind: live.CmdRejoinParty, SessionID: groups[2].SessionID, Q: 90})
	// And the party it comes back to has to be between scenes as well.
	tb.dmSays(live.Command{Kind: live.CmdExplore, On: true})
	refuse(tb.dm, "Finish what is under way first: a rest, rolls that are out, sneaking or exploring in turns.", live.Command{Kind: live.CmdRejoinParty, SessionID: groups[2].SessionID})
	tb.dmSays(live.Command{Kind: live.CmdExplore, On: false})

	// A store that fails while the party is split: nothing moves, and nobody is let in half-placed.
	w.hub.Close(w.session.ID)
	w.hub.Store = noSplit{good}
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	refuse(tb.dm, "The group could not be brought back.", live.Command{Kind: live.CmdRejoinParty, SessionID: groups[2].SessionID})
	refuse(tb.dm, "That change could not be saved.", live.Command{Kind: live.CmdTableFollow, SessionID: groups[2].SessionID})
	w.hub.Close(w.session.ID)
	for name, store := range map[string]live.Store{"its groups": noGroups{good}, "who belongs where": noPlace{good}} {
		w.hub.Store = store
		if _, err := w.hub.Join(context.Background(), w.session.ID, w.dm, dmCaller, live.AudienceDM); err == nil {
			t.Fatalf("a Session started without %s", name)
		}
		w.hub.Close(w.session.ID)
	}
	w.hub.Store = good
}

// flaky fails what a group reads when another tells it something, for as long as it is told to.
type flaky struct {
	live.Store
	failing *atomic.Bool
}

func (f flaky) Groups(ctx context.Context, s domain.Session) ([]domain.PartyGroup, error) {
	if f.failing.Load() {
		return nil, errors.New("gone")
	}
	return f.Store.Groups(ctx, s)
}

func (f flaky) LoadInventory(ctx context.Context, campaign uuid.UUID) (domain.Inventory, error) {
	if f.failing.Load() {
		return domain.Inventory{}, errors.New("gone")
	}
	return f.Store.LoadInventory(ctx, campaign)
}

func (f flaky) Load(ctx context.Context, id domain.SessionID) (domain.Session, []domain.Token, *domain.MapState, error) {
	if f.failing.Load() {
		return domain.Session{}, nil, nil, errors.New("gone")
	}
	return f.Store.Load(ctx, id)
}

// still reports that a screen is sent nothing for a moment.
func still(t *testing.T, sub *live.Subscriber) {
	t.Helper()
	select {
	case u := <-sub.Out:
		t.Fatalf("a screen that should hear nothing was sent %+v", u)
	case <-time.After(300 * time.Millisecond):
	}
}

// A group that cannot read what another group tells it keeps what it had: it shows nothing half-read,
// and catches up when it can read again.
func TestAGroupThatCannotReadWhatItIsTold(t *testing.T) {
	t.Parallel()
	w, tb := ambushTable(t)
	loot := stocked(t, w)
	tower := w.picture(t, "Tower", domain.MapLocal)
	var failing atomic.Bool
	w.hub.Store = flaky{Store: w.hub.Store, failing: &failing}
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	d := tbPlace(t, w, tb.dm, live.Command{Kind: live.CmdPlace, Label: "Brom", TokenKind: domain.TokenParty, Q: 1})
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdSplitParty, Name: "The tower", TokenIDs: []string{tokenNamed(t, d, "Brom").ID}, MapID: uuid.UUID(tower.ID).String()})
	split := next(t, tb.dm)
	away := split.View.Groups[1].SessionID
	up := joinAt(t, w, domain.SessionID(uuid.MustParse(away)), w.dm, dmCaller, live.AudienceDM)

	failing.Store(true)
	// Loot drops for the party; the tower cannot read the Containers and is shown nothing.
	tbPlace(t, w, tb.dm, live.Command{Kind: live.CmdRollLoot, LootTableID: loot["Hoard"]})
	still(t, up)
	// The Table Display is sent to follow the tower: the DM is shown so, and the tower, which cannot
	// read the groups, hears nothing yet.
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdTableFollow, SessionID: away})
	if u := next(t, tb.dm); u.Kind != live.UpdSnapshot || len(u.View.Groups) != 2 || !u.View.Groups[1].Table || u.View.Groups[0].Table {
		t.Fatalf("the table follows the tower = %+v", u)
	}
	still(t, up)
	// A group that cannot be read is not brought back.
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdRejoinParty, SessionID: away})
	if u := next(t, tb.dm); u.Kind != live.UpdRejected || u.Reason != "The group could not be read." {
		t.Fatalf("bringing back a group that cannot be read = %+v", u)
	}

	failing.Store(false)
	// Once it can read again, the next word it is told brings it up to date.
	tbPlace(t, w, tb.dm, live.Command{Kind: live.CmdRollLoot, LootTableID: loot["Purse"]})
	if u := next(t, up); u.Kind != live.UpdView || !slicesContainLabel(u.View, "Loot: Hoard") || !slicesContainLabel(u.View, "Loot: Purse") {
		t.Fatalf("the tower once it can read again = %+v", u)
	}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdTableFollow})
	next(t, tb.dm)
	if u := next(t, up); u.Kind != live.UpdView || u.View.Groups[1].Table || !u.View.Groups[0].Table {
		t.Fatalf("the tower's groups once they can be read = %+v", u.View.Groups)
	}
}
