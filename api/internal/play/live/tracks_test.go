package live_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
)

// When a Track's score crosses a threshold, the Session under way lands what it triggers: an Effect
// on the Character's token, or a roll on a Roll Table that is the Character's to make. A threshold of
// the party's Track lands on every Character on the board, and on nobody else.
func TestTrackThresholdsLandOnTheBoard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids := restingParty(t)
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Raider", Q: 1})
	raider := token(d.View, "Raider").ID
	var brom string
	for _, tv := range d.View.Tokens {
		if tv.Q == 2 {
			brom = tv.ID
		}
	}
	madness := libraryTable(t, w, strings.Repeat("z", 80), `{"dice":"1d4","results":[{"from":1,"to":4,"text":"The walls whisper.","effect":"frightened"}]}`)
	events := pgstore.TrackEvents{Hub: w.hub}
	by := campaigndomain.Member{ID: campaigndomain.MemberID(w.dm.ID), Subject: w.dm.Subject, DisplayName: w.dm.Name, Role: campaigndomain.RoleDM}
	aria := campaigndomain.CharacterID(uuid.MustParse(ids["Aria"]))
	cross := func(character *campaigndomain.CharacterID, track string, th campaigndomain.TrackThreshold) {
		t.Helper()
		events.TrackCrossed(campaigndomain.CampaignID(w.session.CampaignID), by, dmCaller, []campaignapp.TrackCrossing{{Track: track, Character: character, Threshold: th}})
		look(t, w, tb.dm)
		look(t, w, tb.player)
	}
	has := func(id, slug string) bool {
		return slices.ContainsFunc(tokenByID(t, look(t, w, tb.dm), id).Effects, func(e live.EffectView) bool { return e.Slug == slug })
	}
	actions := func() int {
		t.Helper()
		var n int
		if err := w.pool.QueryRow(ctx, "SELECT count(*) FROM play.actions WHERE session_id = $1", w.session.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// A threshold that triggers nothing, and one for a Character who is not on the board, change nothing.
	before := actions()
	cross(&aria, "Stress", campaigndomain.TrackThreshold{At: 2, Rising: false, Label: "Calm again"})
	stranger := campaigndomain.CharacterID(uuid.New())
	cross(&stranger, "Stress", campaigndomain.TrackThreshold{At: 4, Rising: true, Label: "Shaken", Effect: "frightened"})
	if after := actions(); after != before || has(ids["aria-token"], "frightened") {
		t.Fatalf("%d Actions became %d, and Aria Frightened is %v", before, after, has(ids["aria-token"], "frightened"))
	}
	// Aria's own threshold lands on her alone.
	cross(&aria, "Stress", campaigndomain.TrackThreshold{At: 4, Rising: true, Label: "Shaken", Effect: "frightened"})
	if !has(ids["aria-token"], "frightened") || has(brom, "frightened") || has(raider, "frightened") {
		t.Fatalf("Shaken: Aria %v, Brom %v, the Raider %v", has(ids["aria-token"], "frightened"), has(brom, "frightened"), has(raider, "frightened"))
	}
	var label string
	if err := w.pool.QueryRow(ctx, `SELECT e.label FROM play.actions a JOIN play.action_token_events e ON e.action_id = a.id
		WHERE a.session_id = $1 AND a.kind = 'rule_hook_fired' ORDER BY a.seq DESC LIMIT 1`, w.session.ID).Scan(&label); err != nil || label != "Aria: Stress (Shaken)" {
		t.Fatalf("the Action Log says %q %v", label, err)
	}
	// The party's threshold lands on every Character on the board, and not on the Raider.
	cross(nil, "Notoriety", campaigndomain.TrackThreshold{At: 5, Rising: true, Label: "Hunted", Effect: "poisoned"})
	if !has(ids["aria-token"], "poisoned") || !has(brom, "poisoned") || has(raider, "poisoned") {
		t.Fatalf("Hunted: Aria %v, Brom %v, the Raider %v", has(ids["aria-token"], "poisoned"), has(brom, "poisoned"), has(raider, "poisoned"))
	}

	// A threshold that rolls on a table asks the Character's Player, though the DM moved the score. However
	// long the names, the roll is asked for.
	track, mark := strings.Repeat("x", 80), strings.Repeat("y", 80)
	cross(&aria, track, campaigndomain.TrackThreshold{At: 8, Rising: true, Label: mark, RollTable: &madness})
	var id uuid.UUID
	var purpose string
	var roller uuid.UUID
	if err := w.pool.QueryRow(ctx, "SELECT id, purpose, roller_member_id FROM play.roll_requests WHERE campaign_id = $1 AND status = 'pending' AND notation = '1d4'", w.session.CampaignID).Scan(&id, &purpose, &roller); err != nil {
		t.Fatalf("the roll on the table: %v", err)
	}
	if want := (track + ": " + strings.Repeat("z", 80))[:120]; purpose != want || len(purpose) != 120 || roller != w.player.ID {
		t.Fatalf("the roll = %q (%d long) for %s", purpose, len(purpose), roller)
	}
	if _, err := tb.rolls.SetDie(ctx, playerCaller, w.session.CampaignID, domain.RollID(id), 0, app.Fill{Value: 3}); err != nil {
		t.Fatal(err)
	}
	v := look(t, w, tb.player)
	if r := v.TableResult; r == nil || r.Label != "Aria" || r.Text != "The walls whisper." || r.Hook != track || r.Total != 3 {
		t.Fatalf("the result = %+v", v.TableResult)
	}

	// With the tables unreadable an Effect still lands, and no roll is asked for.
	w.hub.Close(w.session.ID)
	w.hub.Store = unreadableTables{Store: w.hub.Store}
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	before = actions()
	cross(&aria, "Stress", campaigndomain.TrackThreshold{At: 8, Rising: true, Label: "Breaking point", RollTable: &madness})
	if after := actions(); after != before {
		t.Fatalf("with the tables unreadable a roll was asked for: %d Actions became %d", before, after)
	}
	cross(nil, "Dread", campaigndomain.TrackThreshold{At: 3, Rising: true, Label: "Uneasy", Effect: "prone"})
	if !has(brom, "prone") {
		t.Fatal("with the tables unreadable an Effect did not land")
	}
}
