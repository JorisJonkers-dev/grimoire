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
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
)

// kin gives the goblin's stat block to an undead zombie and a fey pixie too.
type kin struct{ bestiary }

func (k kin) Monster(ctx context.Context, campaign uuid.UUID, slug string) (string, domain.Stats, error) {
	types := map[string]string{"zombie": "undead", "pixie": "fey", "goblin": "humanoid"}
	_, stats, err := k.bestiary.Monster(ctx, campaign, "goblin")
	stats.CreatureType = types[slug]
	return strings.ToUpper(slug[:1]) + slug[1:], stats, err
}

// The Marsh Lantern, built in the Effect builder and linked into the Campaign, runs in play: it lights
// the caster's hex, reveals the invisible goblin, charms only the Undead and the Fey that fail their
// Wisdom save, and burns with radiant light whoever starts a turn near it.
func TestAHomebrewSpellRunsInPlay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := setup(t)
	lib := &libraryapp.Service{
		Repo: librarypg.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Now: time.Now,
		Surfaces: pgstore.New(w.pool).SurfaceKinds,
	}
	entry, err := lib.Create(ctx, dmCaller, librarydomain.Draft{Kind: "spell", Name: "Marsh Lantern"})
	if err != nil {
		t.Fatal(err)
	}
	design := spellbuild.Design{
		Targeting: spellbuild.Targeting{Shape: "emanation", SizeFt: 10}, Save: "wisdom", Concentration: true,
		Duration: spellbuild.Duration{Unit: "minutes", Amount: 1}, CastingTime: spellbuild.CastingTime{Kind: "action"},
		Components: spellbuild.Components{Verbal: true, Material: &spellbuild.Material{Text: "a lantern of bog glass"}},
		Parts: []spellbuild.Part{
			{Type: "light", BrightFt: 20, DimFt: 20},
			{Type: "reveal", Qualities: []string{"invisible"}},
			{Type: "condition", Condition: "charmed", OnlyTypes: []string{"undead", "fey"}},
			{Type: "damage", When: "start_of_turn", Dice: "1d6", DamageType: "radiant"},
		},
	}
	if _, err := lib.SaveSpell(ctx, dmCaller, entry.ID, design); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Link(ctx, dmCaller, w.session.CampaignID, entry.ID); err != nil {
		t.Fatal(err)
	}
	slug := spellbuild.Slug(entry.ID.String())

	w.hub.Stats = kin{bestiary{owner: w.player.ID}}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	m := w.dungeon(t)
	tb.dmSays(live.Command{Kind: live.CmdSetMap, MapID: uuid.UUID(m.ID).String()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Q: 2})
	for slugOf, q := range map[string]int{"zombie": 3, "pixie": 1, "goblin": 4} {
		tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: slugOf, TokenKind: domain.TokenEnemy, Q: q})
	}
	ids := map[string]string{}
	for _, tv := range look(t, w, tb.dm).Tokens {
		ids[tv.Label] = tv.ID
	}
	tb.dmSays(live.Command{Kind: live.CmdSetVisibility, TokenID: ids["Goblin"], Qualities: []string{"invisible"}})
	tb.fight(ids, "Aria", "Zombie", "Pixie", "Goblin")

	if u := tb.playerSays(live.Command{Kind: live.CmdCastArea, TokenID: ids["Aria"], Effect: slug, Q: 2}); u.View == nil {
		t.Fatalf("cast = %+v", u)
	}
	area := look(t, w, tb.dm).Area
	if area == nil || area.Name != "Marsh Lantern" || area.DamageRollID != "" || len(area.Saves) != 3 {
		t.Fatalf("the lantern is cast, each creature near saving with Wisdom = %+v", area)
	}
	for _, s := range area.Saves {
		if _, err := rolls.SetDie(ctx, dmCaller, w.session.CampaignID, domain.RollID(uuid.MustParse(s.RollID)), 0, app.Fill{Value: 2}); err != nil {
			t.Fatal(err)
		}
	}
	var v *live.View
	for range 50 {
		if v = look(t, w, tb.dm); v.Area == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if v.Area != nil {
		t.Fatalf("the lantern resolves = %+v", v.Area)
	}
	look(t, w, tb.player)
	charmed := func(label string) bool { return effect(token(v, label), "Charmed") != nil }
	if !charmed("Zombie") || !charmed("Pixie") || charmed("Goblin") {
		t.Fatalf("only the Undead and the Fey are charmed: zombie %v, pixie %v, goblin %v", charmed("Zombie"), charmed("Pixie"), charmed("Goblin"))
	}
	if q := token(v, "Goblin").Qualities; len(q) != 0 {
		t.Fatalf("the goblin is revealed = %v", q)
	}
	if lights := v.Lights; len(lights) != 1 || lights[0].Q != 2 || lights[0].R != 0 || lights[0].BrightFt != 20 || lights[0].DimFt != 20 {
		t.Fatalf("the lantern lights the caster's hex = %+v", v.Lights)
	}
	if surfaceAt(v, 3, 0) != slug+"-ground" || surfaceAt(v, 2, 0) != "" {
		t.Fatalf("the glow lies around the caster = %+v", v.Surfaces)
	}
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(v, "Aria").ID})
	if got := manuals(look(t, w, tb.dm)); !strings.Contains(got, "Zombie starts its turn in marsh lantern: 1d6 radiant damage.") {
		t.Fatalf("the zombie burns at the start of its turn = %s", got)
	}
}
