package live_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	libraryapp "github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	librarydomain "github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	librarypg "github.com/JorisJonkers-dev/grimoire/api/internal/library/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/conditionbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
)

type unreadableInjuries struct {
	live.Store
	list []domain.Injury
}

func (u unreadableInjuries) Injuries(context.Context, uuid.UUID) ([]domain.Injury, error) {
	return u.list, errors.New("no such table")
}

// A lingering injury comes off a Roll Table, and stays: no rest ends it, the DM cannot simply take it
// off, and it goes with the Character when its token leaves the map and comes back. Only its cure ends it.
func TestALingeringInjuryStaysUntilItsCure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids := restingParty(t)
	lib := &libraryapp.Service{
		Repo: librarypg.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Now: time.Now,
		Surfaces: pgstore.New(w.pool).SurfaceKinds,
	}
	entry, err := lib.Create(ctx, dmCaller, librarydomain.Draft{Kind: "condition", Name: "Limp"})
	if err != nil {
		t.Fatal(err)
	}
	design := conditionbuild.Design{
		Icon: "skull", Color: "#aa3344", Text: "The leg never set right.", Ends: "cure", Cure: "Regenerate", Stacks: true, MaxLevel: 3,
		PerLevel: conditionbuild.Penalty{SpeedFt: 5}, Parts: []conditionbuild.Part{{Type: "speed_penalty", Feet: 10}},
	}
	if _, err := lib.SaveCondition(ctx, dmCaller, entry.ID, design); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Link(ctx, dmCaller, w.session.CampaignID, entry.ID); err != nil {
		t.Fatal(err)
	}
	limp := spellbuild.Slug(entry.ID.String())
	injuries := libraryTable(t, w, "Injuries", `{"dice":"1d4","results":[{"from":1,"to":4,"text":"Your leg is broken.","effect":"`+limp+`"}]}`)
	hookOn(t, w, "Lingering injuries", "drop-to-0", &injuries, "")
	rejoin := func() {
		t.Helper()
		w.hub.Close(w.session.ID)
		tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	}
	rejoin()
	// aria is her token now: it changes when she leaves the map and comes back.
	aria := ids["aria-token"]
	limping := func(sub *live.Subscriber, tokenID string) *live.EffectView {
		t.Helper()
		for _, e := range tokenByID(t, look(t, w, sub), tokenID).Effects {
			if e.Slug == limp {
				return &e
			}
		}
		return nil
	}
	kept := func() []string {
		t.Helper()
		rows, err := w.pool.Query(ctx, "SELECT character_id::text || ' ' || name || ' ' || cure || ' ' || level FROM campaign.character_injuries ORDER BY 1")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := []string{}
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				t.Fatal(err)
			}
			out = append(out, s)
		}
		return out
	}

	// Aria drops to 0, rolls on the Injuries table, and limps.
	tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: aria, HPDelta: -*tokenByID(t, look(t, w, tb.dm), aria).HP})
	look(t, w, tb.dm)
	var roll uuid.UUID
	if err := w.pool.QueryRow(ctx, "SELECT id FROM play.roll_requests WHERE campaign_id = $1 AND status = 'pending' AND purpose = 'Lingering injuries: Injuries'", w.session.CampaignID).Scan(&roll); err != nil {
		t.Fatalf("the roll on the Injuries table: %v", err)
	}
	if _, err := tb.rolls.SetDie(ctx, playerCaller, w.session.CampaignID, domain.RollID(roll), 0, app.Fill{Value: 2}); err != nil {
		t.Fatal(err)
	}
	if e := limping(tb.player, aria); e == nil || e.Name != "Limp" || e.Cure != "Regenerate" || e.Icon != "skull" {
		t.Fatalf("Aria's injury as the party sees it = %+v", e)
	}
	if got := kept(); !slices.Equal(got, []string{ids["Aria"] + " Limp Regenerate 1"}) {
		t.Fatalf("kept on the Character = %v", got)
	}
	tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: aria, HPDelta: 6})

	// No rest ends it.
	for _, kind := range []string{live.RestShort, live.RestLong} {
		tb.playerSays(live.Command{Kind: live.CmdProposeRest, Rest: kind})
		tb.dmSays(live.Command{Kind: live.CmdAgreeRest})
		tb.dmSays(live.Command{Kind: live.CmdFinishRest})
		if limping(tb.dm, aria) == nil {
			t.Fatalf("a %s rest ended the injury", kind)
		}
	}
	// The DM cannot simply take it off: it lingers until its cure, which the refusal names.
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdEndEffect, EffectID: limping(tb.dm, aria).ID})
	if u := next(t, tb.dm); u.Kind != live.UpdRejected || u.Reason != "Limp lingers until cured: Regenerate." {
		t.Fatalf("ending it without its cure = %+v", u)
	}
	// It outlasts a restart of the Session, and goes with Aria when her token leaves the map and returns.
	rejoin()
	if limping(tb.dm, aria) == nil {
		t.Fatal("the injury did not outlast a restart")
	}
	leave := func(tokenID string) {
		t.Helper()
		tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: tokenID})
	}
	arrive := func() string {
		t.Helper()
		tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: ids["Aria"], Q: 0, R: 2})
		// What she carries lands as changes of its own: both screens catch up first.
		v := look(t, w, tb.dm)
		look(t, w, tb.player)
		for _, tv := range v.Tokens {
			if tv.Q == 0 && tv.R == 2 {
				return tv.ID
			}
		}
		t.Fatal("Aria is not on the map")
		return ""
	}
	leave(aria)
	if got := kept(); len(got) != 1 {
		t.Fatalf("leaving the map took the injury off the Character: %v", got)
	}
	aria = arrive()
	if e := limping(tb.player, aria); e == nil || e.Cure != "Regenerate" {
		t.Fatalf("back on the map Aria's injury = %+v", e)
	}
	// Brom was never hurt, and a creature that is no Character keeps no record.
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Raider", Q: 1})
	raider := token(d.View, "Raider").ID
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: raider, Effect: limp})
	if got := kept(); len(got) != 1 || limping(tb.dm, raider) == nil {
		t.Fatalf("after a creature is injured = %v", got)
	}
	// What does not linger is no injury: Aria knocked Prone carries nothing more, and the DM ends it as ever.
	d, _ = tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: aria, Effect: "prone"})
	if got := kept(); len(got) != 1 {
		t.Fatalf("after Aria is knocked Prone = %v", got)
	}
	for _, e := range tokenByID(t, d.View, aria).Effects {
		if e.Slug == "prone" {
			tb.dmSays(live.Command{Kind: live.CmdEndEffect, EffectID: e.ID})
		}
	}
	// A record of something that does not linger, left by an older design, does not come back either.
	if _, err := w.pool.Exec(ctx, "INSERT INTO campaign.character_injuries (character_id, slug, name, cure, level, created_at) VALUES ($1, 'prone', 'Prone', '', 1, now())", ids["Aria"]); err != nil {
		t.Fatal(err)
	}

	// When the injuries cannot be read, a Character comes back without them and the log says why;
	// the record is still there.
	problems := &faults{Handler: slog.NewTextHandler(io.Discard, nil)}
	whole := w.hub.Store
	leave(aria)
	w.hub.Close(w.session.ID)
	w.hub.Store, w.hub.Log = unreadableInjuries{Store: whole, list: []domain.Injury{{Character: uuid.MustParse(ids["Aria"]), Slug: limp, Name: "Limp", Cure: "Regenerate", Level: 1}}}, slog.New(problems)
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	aria = arrive()
	if fx := tokenByID(t, look(t, w, tb.dm), aria).Effects; len(fx) != 0 || problems.seen.Load() != 1 || len(kept()) != 2 {
		t.Fatalf("with the injuries unreadable Aria has %+v, %d errors logged, kept %v", fx, problems.seen.Load(), kept())
	}
	leave(aria)
	w.hub.Close(w.session.ID)
	w.hub.Store = whole
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	aria = arrive()
	if fx := tokenByID(t, look(t, w, tb.dm), aria).Effects; len(fx) != 1 || fx[0].Slug != limp {
		t.Fatalf("back on the map Aria has %+v", fx)
	}
	if _, err := w.pool.Exec(ctx, "DELETE FROM campaign.character_injuries WHERE slug = 'prone'"); err != nil {
		t.Fatal(err)
	}

	// Cured, it ends, and the Character keeps no record: she comes back whole.
	tb.dmSays(live.Command{Kind: live.CmdEndEffect, EffectID: limping(tb.dm, aria).ID, Cured: true})
	if limping(tb.dm, aria) != nil || len(kept()) != 0 {
		t.Fatalf("cured, Aria still limps or the record stays: %v", kept())
	}
	leave(aria)
	aria = arrive()
	if limping(tb.dm, aria) != nil {
		t.Fatal("a cured injury came back")
	}
	// An injury that has worsened comes back as bad as it was.
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: aria, Effect: limp})
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: aria, Effect: limp})
	if got := kept(); len(got) != 1 || !strings.HasSuffix(got[0], "Limp Regenerate 2") {
		t.Fatalf("injured twice by the DM's hand = %v", got)
	}
	leave(aria)
	aria = arrive()
	if e := limping(tb.dm, aria); e == nil || e.Level != 2 {
		t.Fatalf("back on the map the worsened injury = %+v", e)
	}
	// An injury whose condition the Campaign no longer sees does not come back, stays on record, and is no fault.
	leave(aria)
	if _, err := w.pool.Exec(ctx, "DELETE FROM library.campaign_links WHERE entry_id = $1", entry.ID); err != nil {
		t.Fatal(err)
	}
	problems = &faults{Handler: slog.NewTextHandler(io.Discard, nil)}
	w.hub.Close(w.session.ID)
	w.hub.Log = slog.New(problems)
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	aria = arrive()
	if fx := tokenByID(t, look(t, w, tb.dm), aria).Effects; len(fx) != 0 || len(kept()) != 1 || problems.seen.Load() != 0 {
		t.Fatalf("with the condition gone Aria has %+v, kept %v, %d errors logged", fx, kept(), problems.seen.Load())
	}
}
