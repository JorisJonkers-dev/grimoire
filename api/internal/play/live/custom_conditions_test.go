package live_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	libraryapp "github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	librarydomain "github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	librarypg "github.com/JorisJonkers-dev/grimoire/api/internal/library/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/conditionbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
)

// Frostbite, a homebrew condition linked into the Campaign, shows on a token with its icon and stacks
// to its highest level and no further; and the Campaign's grim exhaustion kills at its fourth level.
func TestCustomConditionsAndExhaustionVariants(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	lib := &libraryapp.Service{
		Repo: librarypg.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Now: time.Now,
		Surfaces: pgstore.New(w.pool).SurfaceKinds,
	}
	entry, err := lib.Create(ctx, dmCaller, librarydomain.Draft{Kind: "condition", Name: "Frostbite"})
	if err != nil {
		t.Fatal(err)
	}
	design := conditionbuild.Design{
		Icon: "snow", Color: "#7fa8dd", Ends: "rest", Stacks: true, MaxLevel: 2, PerLevel: conditionbuild.Penalty{D20: 1, SpeedFt: 5},
		Parts: []conditionbuild.Part{{Type: "attacked_advantage"}},
	}
	if _, err := lib.SaveCondition(ctx, dmCaller, entry.ID, design); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Link(ctx, dmCaller, w.session.CampaignID, entry.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx, "UPDATE campaign.campaigns SET exhaustion_variant = 'grim' WHERE id = $1", w.session.CampaignID); err != nil {
		t.Fatal(err)
	}
	frost := spellbuild.Slug(entry.ID.String())

	w.hub.Stats = bestiary{owner: w.player.ID}
	tb := &table{t: t, w: w, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Q: 0})
	aria := d.View.Tokens[0].ID
	if c := d.View.Conditions; len(c) != 1 || c[0] != (live.ConditionKindView{Slug: frost, Name: "Frostbite", Icon: "snow", Color: "#7fa8dd"}) {
		t.Fatalf("the DM's homebrew conditions = %+v", c)
	}
	var p live.Update
	for range 2 {
		_, p = tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: aria, Effect: frost})
	}
	fx := token(p.View, "Aria").Effects
	if len(fx) != 1 || fx[0].Level != 2 || fx[0].Icon != "snow" || fx[0].Color != "#7fa8dd" || fx[0].Name != "Frostbite" {
		t.Fatalf("Aria's effects as the party sees them = %+v", fx)
	}
	if p.View.Conditions != nil {
		t.Fatal("the party never sees the DM's list of homebrew conditions")
	}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdApplyEffect, TargetID: aria, Effect: frost})
	if u := next(t, tb.dm); u.Kind != live.UpdRejected || u.Reason != "Frostbite is at its highest level." {
		t.Fatalf("a third level of Frostbite = %+v", u)
	}

	for range 3 {
		tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: aria, Effect: "exhaustion"})
	}
	v := look(t, w, tb.dm)
	if tok := token(v, "Aria"); tok.HP == nil || *tok.HP == 0 {
		t.Fatalf("three levels of grim exhaustion leave Aria alive: %+v", tok.HP)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: aria, Effect: "exhaustion"})
	if tok := token(d.View, "Aria"); tok.HP == nil || *tok.HP != 0 {
		t.Fatalf("the fourth level of grim exhaustion = %+v", tok.HP)
	}
	barrier(t, w, tb)
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdApplyEffect, TargetID: aria, Effect: "exhaustion"})
	if u := next(t, tb.dm); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, "highest level") {
		t.Fatalf("a fifth level of grim exhaustion = %+v", u)
	}
}
