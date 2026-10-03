package live_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

// A creature carries an attitude towards each Character. An Influence check moves it: a step towards
// Friendly when it meets the DC, a step towards Hostile when it misses by five or more. The attitude
// is itself a source of Advantage or Disadvantage on the next check, beside the Faction's Standing.
func TestInfluenceMovesACreaturesAttitude(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids := restingParty(t)
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenNPC, Label: "Innkeeper", Q: 1})
	keeper, brom := token(d.View, "Innkeeper"), ""
	for _, tv := range d.View.Tokens {
		if tv.Q == 2 {
			brom = tv.ID
		}
	}
	if len(keeper.Attitudes) != 0 {
		t.Fatalf("an attitude before anyone tried = %+v", keeper.Attitudes)
	}
	type line struct {
		Label string
		Value int
	}
	// last is the newest Roll Request: its id, what it is for, how its d20 is rolled, and the lines on its card.
	last := func() (string, string, string, []line) {
		t.Helper()
		var id uuid.UUID
		var purpose, notation string
		if err := w.pool.QueryRow(ctx, "SELECT id, purpose, notation FROM play.roll_requests WHERE campaign_id = $1 ORDER BY created_at DESC, id LIMIT 1", w.session.CampaignID).Scan(&id, &purpose, &notation); err != nil {
			t.Fatal(err)
		}
		rows, err := w.pool.Query(ctx, "SELECT label, value FROM play.roll_request_modifiers WHERE roll_id = $1 ORDER BY ordering", id)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		lines := []line{}
		for rows.Next() {
			var l line
			if err := rows.Scan(&l.Label, &l.Value); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(l.Label, " towards ") || strings.Contains(l.Label, " with ") {
				lines = append(lines, l)
			}
		}
		return id.String(), purpose, notation, lines
	}
	// sway has a Character try the Innkeeper and rolls the dice given; it says how the check was rolled.
	sway := func(who string, faces ...int) (string, string, []line) {
		t.Helper()
		if p := tb.playerSays(live.Command{Kind: live.CmdTakeAction, TokenID: who, Action: "influence", TargetID: keeper.ID}); p.View == nil {
			t.Fatalf("the try was refused: %+v", p)
		}
		id, purpose, notation, lines := last()
		tb.fill(id, w.player, faces...)
		// The roll settling is a change of its own: both screens catch up before anyone tries again.
		look(t, w, tb.player)
		look(t, w, tb.dm)
		return purpose, notation, lines
	}
	attitudes := func(sub *live.Subscriber) map[string]string {
		t.Helper()
		out := map[string]string{}
		for _, a := range token(look(t, w, sub), "Innkeeper").Attitudes {
			out[a.CharacterID] = a.Attitude
		}
		return out
	}

	// Indifferent to begin with: a plain d20 against 15, the DC kept from the table.
	purpose, notation, lines := sway(ids["aria-token"], 15)
	if purpose != "Influence: Charisma check" || notation != "1d20" || len(lines) != 0 {
		t.Fatalf("the first try = %q %s %+v", purpose, notation, lines)
	}
	if got := attitudes(tb.dm); !reflect.DeepEqual(got, map[string]string{ids["Aria"]: "friendly"}) {
		t.Fatalf("after meeting the DC = %v", got)
	}
	// The party's screens see how the creature takes to each of them.
	if got := attitudes(tb.player); !reflect.DeepEqual(got, map[string]string{ids["Aria"]: "friendly"}) {
		t.Fatalf("a Player's view of it = %v", got)
	}
	// Friendly towards Aria: her next try has Advantage, and the card says why. It stays Friendly.
	if _, notation, lines := sway(ids["aria-token"], 2, 3); notation != "2d20kh1" || !reflect.DeepEqual(lines, []line{{"Friendly towards Aria: Advantage", 0}}) {
		t.Fatalf("a try on a Friendly creature = %s %+v", notation, lines)
	}
	// Advantage kept the 3, which misses 15 by more than five: Friendly sours to Indifferent.
	if got := attitudes(tb.dm)[ids["Aria"]]; got != "indifferent" {
		t.Fatalf("after a bad miss = %q", got)
	}
	// A near miss changes nothing.
	sway(ids["aria-token"], 12)
	if got := attitudes(tb.dm)[ids["Aria"]]; got != "indifferent" {
		t.Fatalf("after a near miss = %q", got)
	}
	// Brom is his own case: a bad miss makes the Innkeeper Hostile towards him alone.
	sway(brom, 4)
	if got := attitudes(tb.dm); !reflect.DeepEqual(got, map[string]string{ids["Aria"]: "indifferent", ids["Brom"]: "hostile"}) {
		t.Fatalf("each Character apart = %v", got)
	}
	if _, notation, lines := sway(brom, 20, 15); notation != "2d20kl1" || !reflect.DeepEqual(lines, []line{{"Hostile towards Aria: Disadvantage", 0}}) {
		t.Fatalf("a try on a Hostile creature = %s %+v", notation, lines)
	}
	if got := attitudes(tb.dm)[ids["Brom"]]; got != "indifferent" {
		t.Fatalf("Disadvantage kept the 15, which meets the DC: %q", got)
	}

	// Standing is a source beside the attitude: Allied with a Hostile creature cancel out to a plain d20,
	// and the card carries both lines.
	watch := uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO campaign.factions (id, campaign_id, name, archetype, goals, territory, notes, score, created_at, updated_at)
			VALUES ($1, $2, 'The Lantern Watch', '', '', '', '', 80, now(), now())`, []any{watch, w.session.CampaignID}},
		{`UPDATE play.tokens SET faction_id = $1 WHERE id = $2`, []any{watch, keeper.ID}},
		{`UPDATE campaign.campaigns SET show_dcs = true WHERE id = $1`, []any{w.session.CampaignID}},
	} {
		if _, err := w.pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	// It was all kept; and a creature of a Faction nobody has tried starts from how the Faction regards them.
	if got := attitudes(tb.dm); !reflect.DeepEqual(got, map[string]string{ids["Aria"]: "indifferent", ids["Brom"]: "indifferent"}) {
		t.Fatalf("after a restart = %v", got)
	}
	// Allied gives Brom Advantage; both dice low, and the Innkeeper turns Hostile towards him.
	sway(brom, 3, 3)
	purpose, notation, lines = sway(brom, 15)
	if purpose != "Influence: Charisma check (DC 15)" || notation != "1d20" ||
		!reflect.DeepEqual(lines, []line{{"Hostile towards Aria: Disadvantage", 0}, {"Allied with The Lantern Watch: Advantage", 0}}) {
		t.Fatalf("a Hostile creature of an Allied Faction, with the DC shown = %q %s %+v", purpose, notation, lines)
	}
	// Both sources the same way are still one Advantage.
	sway(ids["aria-token"], 15, 15)
	if _, notation, lines := sway(ids["aria-token"], 12, 12); notation != "2d20kh1" || len(lines) != 2 {
		t.Fatalf("Friendly and Allied = %s %+v", notation, lines)
	}
	// A creature of a Faction nobody has tried yet starts from how the Faction regards whoever tries:
	// Allied, so Friendly, and a near miss leaves it there.
	d, _ = tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenNPC, Label: "Guard", Q: 1, R: 1, FactionID: watch.String()})
	guard := token(d.View, "Guard")
	if p := tb.playerSays(live.Command{Kind: live.CmdTakeAction, TokenID: ids["aria-token"], Action: "influence", TargetID: guard.ID}); p.View == nil {
		t.Fatalf("the try on the Guard was refused: %+v", p)
	}
	first, _, firstNotation, firstLines := last()
	if firstNotation != "2d20kh1" || !reflect.DeepEqual(firstLines, []line{{"Allied with The Lantern Watch: Advantage", 0}}) {
		t.Fatalf("a first try on a creature of an Allied Faction = %s %+v", firstNotation, firstLines)
	}
	tb.fill(first, w.player, 12, 12)
	if got := token(look(t, w, tb.dm), "Guard").Attitudes; len(got) != 1 || got[0].Attitude != "friendly" || got[0].CharacterID != ids["Aria"] {
		t.Fatalf("the Guard after a near miss = %+v", got)
	}
	look(t, w, tb.player)
	// Swaying nobody in particular moves nobody's attitude.
	tb.playerSays(live.Command{Kind: live.CmdTakeAction, TokenID: ids["aria-token"], Action: "influence"})
	id, purpose, _, _ := last()
	tb.fill(id, w.player, 1)
	look(t, w, tb.player)
	if got := attitudes(tb.dm)[ids["Aria"]]; got != "friendly" || purpose != "Influence: Charisma check" {
		t.Fatalf("after swaying nobody = %q, for %q", got, purpose)
	}
}
