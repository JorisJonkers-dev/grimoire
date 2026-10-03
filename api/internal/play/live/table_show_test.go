package live_test

import (
	"context"
	"encoding/json"
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

// The Table Display shows the DM's caption and the last roll a player made, dice and all. It is the
// party's view and no more: a DM's roll never reaches it, nor anything else that is the DM's alone.
func TestTheTableDisplayShowsCaptionsAndPlayersRollsAndNothingOfTheDMs(t *testing.T) {
	t.Parallel()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	ember := &live.DiceLook{
		Dice:     map[string]live.DieLook{"d20": {Pattern: "marble", Body: "#7a1f1a", Numbers: "#f3d27a", Image: &live.DiePlacement{X: 0.5, Y: 0.5, Scale: 1, Rotation: 90}}},
		ImageURL: "/api/v1/dice-sets/0190c7a8-0000-7000-8000-000000000041/image?v=abc",
	}
	sets := diceSets{playerCaller.Subject: ember, dmCaller.Subject: ember}
	w.hub.Dice = sets
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tv := join(t, w, w.player, playerCaller, live.AudienceTable)
	var shown []live.Update
	tvNext := func() live.Update {
		t.Helper()
		u := next(t, tv)
		shown = append(shown, u)
		return u
	}
	say := func(cmd live.Command) live.Update {
		t.Helper()
		tb.dmSays(cmd)
		return tvNext()
	}

	say(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Q: 0})
	say(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 1})
	say(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", Label: "Lurker", TokenKind: domain.TokenEnemy, Q: 2, Hidden: true})
	if u := say(live.Command{Kind: live.CmdTableCaption, Caption: "  The gate creaks open.  "}); u.View.Table.Caption != "The gate creaks open." {
		t.Fatalf("the caption on the table = %+v", u.View.Table)
	}
	if p := tb.party[len(tb.party)-1]; p.View.Table.Caption != "The gate creaks open." {
		t.Fatalf("the caption for the party = %+v", p.View.Table)
	}
	for want, sub := range map[string]*live.Subscriber{"Only the DM can change the table.": tb.player, "Captions run to 200 characters.": tb.dm} {
		barrier(t, w, tb)
		w.hub.Submit(sub, live.Command{Kind: live.CmdTableCaption, Caption: strings.Repeat("x", 201)})
		if u := next(t, sub); u.Kind != live.UpdRejected || u.Reason != want {
			t.Fatalf("a caption that should be refused = %+v, want %q", u, want)
		}
	}
	// The limit is in what the screens count, so 150 dice emoji are 300 and too many; the same goes for a title.
	barrier(t, w, tb)
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdTableScene, Scene: domain.SceneTitle, Title: strings.Repeat("🎲", 50)})
	if u := next(t, tb.dm); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, "Titles run to 80") {
		t.Fatalf("a title too long on the wire = %+v", u)
	}
	barrier(t, w, tb)
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdTableCaption, Caption: strings.Repeat("🎲", 150)})
	if u := next(t, tb.dm); u.Kind != live.UpdRejected || u.Reason != "Captions run to 200 characters." {
		t.Fatalf("a caption too long on the wire = %+v", u)
	}

	// A player's roll shows on the table with its dice; the DM's does not.
	roll := func(c *app.Rolls, who domain.Member, purpose string, face int) {
		t.Helper()
		cl := dmCaller
		if !who.DM {
			cl = playerCaller
		}
		r, err := c.Create(t.Context(), cl, w.session.CampaignID, app.RollInput{Purpose: purpose, Notation: "1d20", Modifiers: []domain.Modifier{{Label: "Skill", Value: 3}}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.SetDie(t.Context(), cl, w.session.CampaignID, r.ID, 0, app.Fill{Value: face}); err != nil {
			t.Fatal(err)
		}
	}
	roll(rolls, w.player, "Athletics", 14)
	u := frame(t, tv)
	shown = append(shown, u)
	if _, err := uuid.Parse(u.Roll.ID); err != nil {
		t.Fatalf("a shared roll names itself, so a screen shows it once: %+v", u.Roll)
	}
	if r := u.Roll; u.Kind != live.UpdRoll || r == nil || r.Roller != w.player.Name || r.Purpose != "Athletics" || r.Total != 17 || len(r.Dice) != 1 ||
		r.Dice[0] != (live.RollDie{Faces: 20, Value: 14, Kept: true}) || r.Modifier != 3 {
		t.Fatalf("the player's roll on the table = %+v %+v", u, u.Roll)
	}
	// It rolls in the Dice Set its roller chose.
	if got, _ := json.Marshal(u.Roll.Look); string(got) != `{"dice":{"d20":{"pattern":"marble","body":"#7a1f1a","numbers":"#f3d27a","image":{"x":0.5,"y":0.5,"scale":1,"rotation":90}}},"imageUrl":"/api/v1/dice-sets/0190c7a8-0000-7000-8000-000000000041/image?v=abc"}` {
		t.Fatalf("the look of the player's roll = %s", got)
	}
	roll(rolls, w.dm, "Lurker's Stealth", 19)
	// What the screens are sent has to fit what they accept, or a roll kept for later screens would
	// spoil every snapshot. A purpose within its limit in characters can still be too long on the wire.
	roll(rolls, w.player, strings.Repeat("🎲", 100), 9)
	barrier(t, w, tb)
	w.hub.Submit(tv, live.Command{Kind: live.CmdResync})
	snap := tvNext()
	if snap.Kind != live.UpdSnapshot || snap.Roll == nil || snap.Roll.Purpose != "Athletics" {
		t.Fatalf("after the DM rolled, the table still shows the player's roll: %+v %+v", snap, snap.Roll)
	}

	// A restart keeps the caption; the last roll was only ever on screen.
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	tv = join(t, w, w.player, playerCaller, live.AudienceTable)
	w.hub.Submit(tv, live.Command{Kind: live.CmdResync})
	if again := tvNext(); again.View.Table.Caption != "The gate creaks open." || again.Roll != nil {
		t.Fatalf("after a restart = %+v %+v", again.View.Table, again.Roll)
	}
	// A roller on the plain dice sends no look.
	delete(sets, playerCaller.Subject)
	roll(rolls, w.player, "Acrobatics", 2)
	if u := frame(t, tv); u.Kind != live.UpdRoll || u.Roll.Purpose != "Acrobatics" || u.Roll.Look != nil {
		t.Fatalf("a roll on the plain dice = %+v %+v", u, u.Roll)
	}
	if u := say(live.Command{Kind: live.CmdTableCaption, Caption: ""}); u.View.Table.Caption != "" {
		t.Fatalf("a cleared caption = %+v", u.View.Table)
	}

	for i, u := range shown {
		raw, err := json.Marshal(u)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"Lurker", "Stealth", `"suggestion"`, `"tactics"`, `"lights"`, `"qualities"`, `"legend"`} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("update %d to the Table Display carries %s: %s", i, secret, raw)
			}
		}
	}
}

// diceSets is the Dice Set each subject rolls with.
type diceSets map[string]*live.DiceLook

func (d diceSets) DiceLook(_ context.Context, subject string) *live.DiceLook { return d[subject] }
