package live_test

import (
	"context"
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

func ambushTable(t *testing.T) (world, *table) {
	t.Helper()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	return w, &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
}

func zoneNamed(t *testing.T, v *live.View, name string) live.ZoneView {
	t.Helper()
	for _, z := range v.Zones {
		if z.Name == name {
			return z
		}
	}
	t.Fatalf("no zone %s in %+v", name, v.Zones)
	return live.ZoneView{}
}

func (tb *table) fill(rollID string, who domain.Member, faces ...int) {
	tb.t.Helper()
	cl := dmCaller
	if !who.DM {
		cl = playerCaller
	}
	for i, f := range faces {
		if _, err := tb.rolls.SetDie(context.Background(), cl, tb.w.session.CampaignID, domain.RollID(uuid.MustParse(rollID)), i, app.Fill{Value: f}); err != nil {
			tb.t.Fatal(err)
		}
	}
}

func TestAnAmbushSpringsOnThePartyAndSurprisesWhoeverMissedIt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb := ambushTable(t)
	refuse := func(want string, cmd live.Command) {
		t.Helper()
		w.hub.Submit(tb.dm, cmd)
		if u := next(t, tb.dm); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), want) {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Q: -4})
	tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Brom", TokenKind: domain.TokenParty, Q: -4, R: 1})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 3, Hidden: true})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "wolf", TokenKind: domain.TokenEnemy, Q: 3, R: 1, Hidden: true})
	refuse("name of up to 40", live.Command{Kind: live.CmdAddZone, Label: " ", RadiusHexes: 2})
	refuse("1 to 20 hexes", live.Command{Kind: live.CmdAddZone, Label: "Wide", RadiusHexes: 21})
	refuse("off the map", live.Command{Kind: live.CmdAddZone, Label: "Far", Q: 50, RadiusHexes: 2})
	refuse("no such zone", live.Command{Kind: live.CmdSpringZone, ZoneID: uuid.NewString()})
	tb.dmSays(live.Command{Kind: live.CmdAddZone, Label: "Empty", Q: -8, RadiusHexes: 1, DMOnly: true})
	d, p := tb.dmSays(live.Command{Kind: live.CmdAddZone, Label: "Ambush", Q: 3, RadiusHexes: 2})
	amb := zoneNamed(t, d.View, "Ambush")
	if amb.Status != domain.ZoneArmed || amb.Creatures != 2 || amb.DMOnly || len(p.View.Zones) != 0 {
		t.Fatalf("zone placed = %+v, player sees %+v", amb, p.View.Zones)
	}
	refuse("no hidden creatures", live.Command{Kind: live.CmdSpringZone, ZoneID: zoneNamed(t, d.View, "Empty").ID})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdHoldZone, ZoneID: amb.ID, On: true})
	if !zoneNamed(t, d.View, "Ambush").Held {
		t.Fatal("hold off")
	}
	aria := token(p.View, "Aria")
	if u := tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: aria.ID, Q: 1, R: 0}); u.Kind != live.UpdView {
		t.Fatalf("walk = %+v", u)
	}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdHoldZone, ZoneID: amb.ID})
	next(t, tb.dm)
	next(t, tb.player)
	d, p = next(t, tb.dm), next(t, tb.player)
	sprung := zoneNamed(t, d.View, "Ambush")
	if sprung.Status != domain.ZoneSpotting || sprung.DC != 16 || len(sprung.Checks) != 2 || len(p.View.Perception) != 2 || token(p.View, "Goblin") != nil {
		t.Fatalf("sprung = %+v, player perception %+v", sprung, p.View.Perception)
	}
	if raw := payloads(t, p); strings.Contains(raw, `"dc"`) || strings.Contains(raw, "Ambush") || strings.Contains(raw, "Goblin") {
		t.Fatalf("the ambush leaked to a player: %s", raw)
	}
	refuse("already sprung", live.Command{Kind: live.CmdSpringZone, ZoneID: amb.ID})
	refuse("already sprung", live.Command{Kind: live.CmdHoldZone, ZoneID: amb.ID, On: true})
	var ariaRoll, bromRoll string
	for _, pc := range p.View.Perception {
		if pc.TokenID == aria.ID {
			ariaRoll = pc.RollID
		} else {
			bromRoll = pc.RollID
		}
	}
	roll, _ := pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(ariaRoll)))
	if roll.Purpose != "Perception for Aria" || roll.Roller.ID != w.player.ID || roll.Modifiers[0].Value != 3 {
		t.Fatalf("perception roll = %+v", roll)
	}
	tb.fill(ariaRoll, w.player, 15)
	d, p = next(t, tb.dm), next(t, tb.player)
	if token(p.View, "Goblin") == nil || token(p.View, "Wolf") == nil || len(p.View.Perception) != 1 || p.View.Combat != nil {
		t.Fatalf("Aria noticed and everyone sees them = %+v", p.View)
	}

	w.hub.Close(w.session.ID)
	tb.fill(bromRoll, w.dm, 2)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	d = next(t, tb.dm)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	f := d.View.Combat
	if f == nil || len(f.Combatants) != 4 || zoneNamed(t, d.View, "Ambush").Status != domain.ZoneSprung {
		t.Fatalf("the fight starts = %+v %+v", f, d.View.Zones)
	}
	brom, ariaC, gob := combatant(d.View, "Brom"), combatant(d.View, "Aria"), combatant(d.View, "Goblin")
	if !brom.Surprised || ariaC.Surprised || gob.Surprised {
		t.Fatalf("surprise = Brom %v Aria %v Goblin %v", brom.Surprised, ariaC.Surprised, gob.Surprised)
	}
	low, _ := pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(brom.RollID)))
	if low.Notation != "2d20kl1" || low.Purpose != "Initiative for Brom (surprised)" || len(low.Dice) != 2 {
		t.Fatalf("surprised initiative = %+v", low)
	}
	tb.fill(brom.RollID, w.dm, 18, 4)
	d = next(t, tb.dm)
	next(t, tb.player)
	if got := combatant(d.View, "Brom").Initiative; got == nil || *got != 4 {
		t.Fatalf("Brom keeps the lower die = %v", got)
	}
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: -8, Hidden: true, Label: "Scout"})
	refuse("a fight is already on", live.Command{Kind: live.CmdSpringZone, ZoneID: zoneNamed(t, d.View, "Empty").ID})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdRemoveZone, ZoneID: amb.ID})
	if len(d.View.Zones) != 1 {
		t.Fatalf("zone removed = %+v", d.View.Zones)
	}
}

func TestAWatchfulPartyNoticesAtOnceAndTheDMSpringsZonesByHand(t *testing.T) {
	t.Parallel()
	w, tb := ambushTable(t)
	tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Brom", TokenKind: domain.TokenParty, Q: 0})
	tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Mule", TokenKind: domain.TokenParty, Q: 0, R: 1})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "wolf", TokenKind: domain.TokenEnemy, Q: 1, Hidden: true})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdAddZone, Label: "Den", Q: 1, RadiusHexes: 3, DMOnly: true})
	if zoneNamed(t, d.View, "Den").Status != domain.ZoneArmed {
		t.Fatal("a DM-only zone waits even with the party inside")
	}
	d, p := tb.dmSays(live.Command{Kind: live.CmdSpringZone, ZoneID: zoneNamed(t, d.View, "Den").ID})
	den := zoneNamed(t, d.View, "Den")
	if den.DC != 10 || len(den.Checks) != 2 || den.Checks[0].Noticed == nil || !*den.Checks[0].Noticed || token(p.View, "Wolf") == nil {
		t.Fatalf("passive Perception notices = %+v", den)
	}
	d, p = next(t, tb.dm), next(t, tb.player)
	if d.View.Combat == nil || len(d.View.Combat.Combatants) != 3 || combatant(p.View, "Brom").Surprised {
		t.Fatalf("nobody surprised = %+v", d.View.Combat)
	}
	tb.dmSays(live.Command{Kind: live.CmdEndCombat})
	tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: -3, Hidden: true})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdAddZone, Label: "Hollow", Q: -3, RadiusHexes: 1, DMOnly: true})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdSpringZone, ZoneID: zoneNamed(t, d.View, "Hollow").ID})
	if h := zoneNamed(t, d.View, "Hollow"); h.Status != domain.ZoneSpotting || len(d.View.Perception) != 2 {
		t.Fatalf("hollow = %+v", h)
	}
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: token(d.View, "Brom").ID})
	d, _ = tb.dmSays(live.Command{Kind: live.CmdRemove, TokenID: token(d.View, "Mule").ID})
	d, _ = next(t, tb.dm), next(t, tb.player)
	if d.View.Combat == nil || len(d.View.Combat.Combatants) != 1 || zoneNamed(t, d.View, "Hollow").Status != domain.ZoneSprung {
		t.Fatalf("with the party gone the goblin fights alone = %+v", d.View.Combat)
	}
	w.hub.Close(w.session.ID)
	w.hub.Store = failingZones{Store: pgstore.New(w.pool)}
	if _, err := w.hub.Join(context.Background(), w.session.ID, w.dm, dmCaller, live.AudienceDM); err == nil {
		t.Fatal("zone load failure ignored")
	}
}

type failingZones struct{ live.Store }

func (failingZones) LoadZones(context.Context, domain.SessionID) ([]domain.Zone, error) {
	return nil, context.Canceled
}
