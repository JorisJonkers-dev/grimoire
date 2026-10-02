package live_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

func TestStoredEffectsReachTheTable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	slug := "war-chant-" + uuid.NewString()[:8]
	chant := effects.Definition{Slug: slug, Name: "War Chant", Components: []effects.Component{effects.Manual{Instruction: "War Chant: add 1d4 to the next check."}}}
	if err := comppg.New(w.pool).SaveEffect(ctx, effects.Owner{Kind: effects.OwnedBySpell, Slug: slug}, chant); err != nil {
		t.Fatal(err)
	}
	tb := &table{t: t, w: w, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString()})
	aria := token(d.View, "Aria")
	d, _ = tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: aria.ID, Effect: slug})
	if effect(token(d.View, "Aria"), "War Chant") == nil || len(d.View.Manual) != 1 || d.View.Manual[0].Text != "Aria: War Chant: add 1d4 to the next check." {
		t.Fatalf("a stored effect resolves like a built-in = %+v %+v", token(d.View, "Aria").Effects, d.View.Manual)
	}

	w.hub.Close(w.session.ID)
	w.hub.Store = failingCatalog{Store: pgstore.New(w.pool)}
	if _, err := w.hub.Join(ctx, w.session.ID, w.dm, dmCaller, live.AudienceDM); err == nil {
		t.Fatal("catalogue load failure ignored")
	}
}

type failingCatalog struct{ live.Store }

func (failingCatalog) Effects(context.Context) (effects.Catalog, error) {
	return nil, context.Canceled
}
