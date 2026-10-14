package live_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// sunbeam is the SRD's Sunbeam as it lands: a 60-foot Line from the caster, a Constitution save for
// 6d8 Radiant damage or half, and the Blinded condition on a failed save, written the way the Homebrew
// builder writes a condition on a failed save.
func sunbeam() effects.Definition {
	return effects.Definition{
		Slug: "sunbeam", Name: "Sunbeam", Owner: effects.OwnedBySpell, Concentration: true, Duration: effects.Duration{Kind: effects.Minutes, Amount: 1},
		Components: []effects.Component{
			effects.Area{Shape: hex.LineArea, SizeFt: 60, RangeFt: 0},
			effects.SaveDamage{Ability: "constitution", Dice: "6d8", Type: "radiant", Half: true},
			effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 1}, Then: []effects.Component{effects.SaveCondition{Ability: "constitution", Slug: "blinded"}}},
		},
	}
}

// Bless, Fireball and Sunbeam are read from the database and played in a Session, each through a kind
// defined in its own file: Bless folds its die into a save and an attack, Fireball's damage lands as
// acts the runtime applies, and Sunbeam's branch blinds whoever fails. An invisible creature in the
// beam takes it all, and the party never learns it is there.
func TestKindsInTheirOwnFilesPlayInASession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	if err := comppg.New(w.pool).SaveEffect(ctx, effects.Owner{Kind: effects.OwnedBySpell, Slug: "sunbeam"}, sunbeam()); err != nil {
		t.Fatal(err)
	}
	cat, err := comppg.New(w.pool).Effects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cat["bless"].Components[0].(effects.SaveFolder); !ok {
		t.Fatalf("bless folds into saves = %+v", cat["bless"])
	}
	if _, ok := cat["fireball"].Components[1].(effects.Lander); !ok {
		t.Fatalf("fireball lands = %+v", cat["fireball"])
	}
	if _, ok := cat["sunbeam"].Components[2].(effects.Holder); !ok {
		t.Fatalf("sunbeam's branch holds the blinding = %+v", cat["sunbeam"])
	}

	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Label: "Brom", Q: 0, R: 1})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Cultist", Q: 4})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Shade", Q: 5, Hidden: true})
	ids := map[string]string{}
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
	}
	tb.dmSays(live.Command{Kind: live.CmdSetVisibility, TokenID: ids["Shade"], Qualities: []string{"invisible"}})
	tb.dmSays(live.Command{Kind: live.CmdSetHidden, TokenID: ids["Shade"], Hidden: false})
	tb.dmSays(live.Command{Kind: live.CmdApplyEffect, TargetID: ids["Aria"], SourceID: ids["Aria"], Effect: "bless"})

	var setupOrder []live.CombatantSetup
	for _, label := range []string{"Cultist", "Aria", "Brom", "Shade"} {
		setupOrder = append(setupOrder, live.CombatantSetup{TokenID: ids[label], SpeedFt: 30})
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: setupOrder})
	for label, face := range map[string]int{"Cultist": 20, "Aria": 15, "Brom": 10, "Shade": 5} {
		who := w.dm
		if label == "Aria" || label == "Brom" {
			who = w.player
		}
		tb.roll(combatant(d.View, label), who, face)
	}
	partyLook := func() *live.View {
		t.Helper()
		look(t, w, tb.dm)
		w.hub.Submit(tb.player, live.Command{Kind: live.CmdResync})
		for {
			u := frame(t, tb.player)
			tb.party = append(tb.party, u)
			if u.Kind == live.UpdSnapshot {
				return u.View
			}
		}
	}

	d, _ = tb.dmSays(live.Command{Kind: live.CmdCastArea, TokenID: ids["Cultist"], Effect: "fireball", Q: -1, R: 0})
	saves := map[string]string{}
	for _, s := range d.View.Area.Saves {
		saves[s.TokenID] = s.RollID
	}
	if len(saves) != 2 || saves[ids["Aria"]] == "" || saves[ids["Brom"]] == "" {
		t.Fatalf("fireball catches Aria and Brom = %+v", d.View.Area)
	}
	notation := func(rollID string) string {
		t.Helper()
		roll, err := pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(rollID)))
		if err != nil {
			t.Fatal(err)
		}
		return roll.Notation
	}
	if notation(saves[ids["Aria"]]) != "1d20+1d4" || notation(saves[ids["Brom"]]) != "1d20" || notation(d.View.Area.DamageRollID) != "8d6" {
		t.Fatalf("Bless joins Aria's save only = %s, %s", notation(saves[ids["Aria"]]), notation(saves[ids["Brom"]]))
	}
	tb.fill(d.View.Area.DamageRollID, w.dm, 1, 1, 1, 1, 1, 1, 1, 1)
	tb.fill(saves[ids["Aria"]], w.player, 10, 4)
	tb.fill(saves[ids["Brom"]], w.player, 12)
	partyLook()
	v := partyLook()
	if v.Area != nil || *token(v, "Aria").HP != 8 || *token(v, "Brom").HP != 4 {
		t.Fatalf("8 fire: Aria saves with Bless's die and halves it, Brom fails = %+v %+v", token(v, "Aria"), token(v, "Brom"))
	}

	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(v, "Cultist").ID})
	u := tb.playerSays(live.Command{Kind: live.CmdPreviewAttack, TokenID: ids["Aria"], AttackNo: 1, TargetID: ids["Cultist"]})
	if u.Preview == nil || !slices.Contains(u.Preview.Reasons, "Bless: +1d4 to hit") {
		t.Fatalf("Bless joins Aria's attack = %+v", u.Preview)
	}

	beam := live.Command{Kind: live.CmdPreviewArea, TokenID: ids["Aria"], Effect: "sunbeam", Q: 1, R: 0}
	u = tb.playerSays(beam)
	if u.Area == nil || len(u.Area.Targets) != 1 || u.Area.Targets[0].TokenID != ids["Cultist"] || len(u.Area.Ends) != 1 || u.Area.Ends[0] != "Bless" {
		t.Fatalf("the party's preview shows the cultist only, and ends Bless = %+v", u.Area)
	}
	w.hub.Submit(tb.dm, beam)
	if u := next(t, tb.dm); u.Area == nil || len(u.Area.Targets) != 2 {
		t.Fatalf("the DM's preview shows the shade too = %+v", u.Area)
	}
	beam.Kind = live.CmdCastArea
	u = tb.playerSays(beam)
	if u.View.Area == nil || len(u.View.Area.Saves) != 1 {
		t.Fatalf("the party sees the cultist's save only = %+v", u.View.Area)
	}
	dmArea := look(t, w, tb.dm).Area
	if dmArea == nil || len(dmArea.Saves) != 2 {
		t.Fatalf("the DM sees both saves = %+v", dmArea)
	}
	tb.fill(dmArea.DamageRollID, w.player, 1, 1, 1, 1, 1, 1)
	for _, s := range dmArea.Saves {
		tb.fill(s.RollID, w.dm, 1)
	}
	partyLook()
	v = partyLook()
	if v.Area != nil || effect(token(v, "Cultist"), "Blinded") == nil || token(v, "Shade") != nil || effect(token(v, "Aria"), "Bless") != nil {
		t.Fatalf("the branch blinds the cultist, and Sunbeam's concentration ends Bless = %+v", v.Tokens)
	}
	dm := look(t, w, tb.dm)
	for _, label := range []string{"Cultist", "Shade"} {
		if tv := token(dm, label); tv == nil || *tv.HP != 1 || effect(tv, "Blinded") == nil {
			t.Fatalf("6 radiant and blinded: %s = %+v", label, tv)
		}
	}
	if body := payloads(t, tb.party...); strings.Contains(body, ids["Shade"]) || strings.Contains(body, "Shade") {
		t.Fatalf("the invisible shade reached the party: %s", body)
	}
}
