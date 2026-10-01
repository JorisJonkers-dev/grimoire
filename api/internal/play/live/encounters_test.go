package live_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	prepapp "github.com/JorisJonkers-dev/grimoire/api/internal/prep/app"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	preppg "github.com/JorisJonkers-dev/grimoire/api/internal/prep/pgstore"
)

type failingPrep struct{ live.Store }

func (failingPrep) LoadPrep(context.Context, uuid.UUID) (prep.Prep, error) {
	return prep.Prep{}, errors.New("gone")
}

type failingChecks struct{ live.Store }

func (failingChecks) LoadChecks(context.Context, uuid.UUID, domain.SessionID) ([]prep.Check, error) {
	return nil, errors.New("gone")
}

func beast(slug, name string, xp int) snapshot.Monster {
	return snapshot.Monster{
		Entry: snapshot.Entry{Document: "srd-2024", Slug: slug, Name: name}, Size: "small", Type: "humanoid", Alignment: "neutral", ArmorClass: 12,
		HitPoints: 7, HitDice: "2d6", ChallengeRating: 0.25, XP: xp,
		Abilities: map[string]int{"strength": 8, "dexterity": 14, "constitution": 10, "intelligence": 10, "wisdom": 8, "charisma": 8},
		Saves:     map[string]int{}, Skills: map[string]int{}, Speeds: map[string]int{"walk": 30}, Senses: map[string]int{},
		Resistances: []string{}, Immunities: []string{}, Vulnerabilities: []string{}, ConditionImmunities: []string{}, Traits: []snapshot.Named{},
		Actions: []snapshot.Action{},
	}
}

// prepared stocks the compendium and the Campaign's prep: a goblin pool for levels 1–4 and a giant
// pool for 5–10, a secret table for everywhere, an open road table and a den that is nearly always quiet.
func prepared(t *testing.T, w world) (map[string]prep.Table, *prepapp.Service) {
	t.Helper()
	ctx := context.Background()
	snap := snapshot.Snapshot{
		Documents: []snapshot.Document{{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0", Attribution: "a", URL: "https://a"}},
		Monsters:  []snapshot.Monster{beast("goblin", "Goblin", 50), beast("ogre", "Ogre", 450)},
	}
	if _, err := comppg.New(w.pool).Import(ctx, snap, "encounters"); err != nil {
		t.Fatal(err)
	}
	s := &prepapp.Service{Repo: preppg.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Now: time.Now}
	band, err := s.SavePool(ctx, dmCaller, w.session.CampaignID, prep.Pool{Name: "Goblin band", LevelMin: 1, LevelMax: 4, Difficulty: "low", Members: []prep.PoolMember{
		{Slug: "goblin", Weight: 1, Min: 2, Max: 6}, {Slug: "ogre", Weight: 1, Min: 0, Max: 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	giants, _ := s.SavePool(ctx, dmCaller, w.session.CampaignID, prep.Pool{Name: "Giants", LevelMin: 5, LevelMax: 10, Difficulty: "high", Members: []prep.PoolMember{{Slug: "ogre", Weight: 1, Min: 1, Max: 2}}})
	out := map[string]prep.Table{}
	for _, in := range []prep.Table{
		{Name: "Anywhere", ChancePct: 100, Visibility: prep.Secret, Entries: []prep.Entry{{Weight: 1, Kind: prep.EntryPool, PoolID: &band.ID}}},
		{Name: "Road", ChancePct: 50, Visibility: prep.Open, Entries: []prep.Entry{
			{Weight: 1, Kind: prep.EntryEncounter, Label: "Ambush", Monsters: []prep.EntryMonster{{Slug: "goblin", Count: 3}}},
			{Weight: 1, Kind: prep.EntryNothing, Label: "Birdsong"},
			{Weight: 1, Kind: prep.EntryPool, PoolID: &giants.ID},
		}},
		{Name: "Den", ChancePct: 0, Visibility: prep.Secret, Entries: []prep.Entry{
			{Weight: 100, Kind: prep.EntryNothing}, {Weight: 1, Kind: prep.EntryEncounter, Label: "Ogre", Monsters: []prep.EntryMonster{{Slug: "ogre", Count: 1}}},
		}},
	} {
		tb, err := s.SaveTable(ctx, dmCaller, w.session.CampaignID, in)
		if err != nil {
			t.Fatal(err)
		}
		out[tb.Name] = tb
	}
	return out, s
}

func TestEncounterChecksRollOnRestsAndAtTheDMsWord(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb := ambushTable(t)
	tables, svc := prepared(t, w)
	id := func(name string) string { return uuid.UUID(tables[name].ID).String() }
	table := join(t, w, w.player, playerCaller, live.AudienceTable)
	says := func(cmd live.Command) (live.Update, live.Update) {
		t.Helper()
		d, p := tb.dmSays(cmd)
		next(t, table)
		return d, p
	}
	refuse := func(want string, cmd live.Command) {
		t.Helper()
		w.hub.Submit(tb.dm, cmd)
		if u := next(t, tb.dm); u.Kind != live.UpdRejected || !strings.Contains(strings.ToLower(u.Reason), want) {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	refuse("short or long", live.Command{Kind: live.CmdRest, Rest: "nap"})
	refuse("no such encounter table", live.Command{Kind: live.CmdEncounterCheck, TableID: uuid.NewString(), Mode: prep.ModeNormal})
	refuse("rolls normally", live.Command{Kind: live.CmdEncounterCheck, TableID: id("Road"), Mode: "maybe"})
	refuse("pick one of", live.Command{Kind: live.CmdEncounterCheck, TableID: id("Road"), Mode: prep.ModePick, Entry: 3})
	refuse("next rest or the next", live.Command{Kind: live.CmdScheduleCheck, TableID: id("Den"), Due: "tomorrow"})

	says(live.Command{Kind: live.CmdRest, Rest: live.RestLong})
	d, p := next(t, tb.dm), next(t, tb.player)
	next(t, table)
	dc, pc := d.View.Checks[0], p.View.Checks[0]
	if dc.Trigger != prep.TriggerLongRest || dc.TableName != "Anywhere" || dc.Outcome != prep.OutcomeFight || dc.EntryLabel != "Goblin band" ||
		len(dc.Monsters) != 1 || dc.Monsters[0] != (live.CheckMonsterView{Slug: "goblin", Count: 2}) || dc.ChanceRoll < 1 || dc.Seed == "" {
		t.Fatalf("a long rest's check = %+v", dc)
	}
	if pc.Outcome != prep.OutcomeFight || pc.TableName != "" || pc.ChanceRoll != 0 || pc.Monsters != nil {
		t.Fatalf("a secret check shows players only the outcome = %+v", pc)
	}
	if raw := payloads(t, p); strings.Contains(raw, "Anywhere") || strings.Contains(raw, "goblin") || strings.Contains(raw, "seed") {
		t.Fatalf("a secret check leaked: %s", raw)
	}

	says(live.Command{Kind: live.CmdScheduleCheck, TableID: id("Den"), Due: prep.DueNextRest})
	says(live.Command{Kind: live.CmdRest, Rest: live.RestShort})
	next(t, tb.dm)
	next(t, tb.player)
	next(t, table)
	d, _ = next(t, tb.dm), next(t, tb.player)
	next(t, table)
	if got := d.View.Checks; len(got) != 3 || got[1].TableName != "Den" || got[1].Trigger != prep.TriggerShortRest || got[1].Outcome != prep.OutcomeNothing || got[2].TableName != "Anywhere" {
		t.Fatalf("a short rest runs the scheduled check first = %+v", got)
	}
	if p, _ := pgstore.New(w.pool).LoadPrep(ctx, w.session.CampaignID); len(p.Scheduled) != 0 {
		t.Fatalf("the scheduled check is used up = %+v", p.Scheduled)
	}

	d, _ = says(live.Command{Kind: live.CmdEncounterCheck, TableID: id("Road"), Mode: prep.ModePick, Entry: 1})
	if c := d.View.Checks[3]; c.Outcome != prep.OutcomeNothing || c.EntryLabel != "Birdsong" || c.ChanceRoll != 0 || c.Mode != prep.ModePick {
		t.Fatalf("picked Nothing = %+v", c)
	}
	d, _ = says(live.Command{Kind: live.CmdEncounterCheck, TableID: id("Road"), Mode: prep.ModePick, Entry: 2})
	if c := d.View.Checks[4]; c.Outcome != prep.OutcomeNothing || !strings.Contains(c.EntryLabel, "Giants (the party is outside its levels)") {
		t.Fatalf("a pool outside the party's levels = %+v", c)
	}
	d, _ = says(live.Command{Kind: live.CmdEncounterCheck, TableID: id("Den"), Mode: prep.ModeForce})
	if c := d.View.Checks[5]; c.Outcome != prep.OutcomeFight || c.EntryLabel != "Ogre" || c.ChanceRoll != 0 {
		t.Fatalf("a forced encounter never draws Nothing = %+v", c)
	}

	d, p = says(live.Command{Kind: live.CmdEncounterCheck, TableID: id("Road"), Mode: prep.ModeNormal})
	open := p.View.Checks[6]
	if open.Status != prep.CheckPending || open.RollID == "" || open.ChancePct != 50 || open.TableName != "" {
		t.Fatalf("an open check waits on its roll, which players see = %+v", open)
	}
	roll, _ := pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(open.RollID)))
	if roll.Notation != "1d100" || roll.Purpose != "Encounter check" || roll.Roller.ID != w.dm.ID {
		t.Fatalf("the open roll = %+v", roll)
	}
	tb.fill(open.RollID, w.dm, 99)
	d = next(t, tb.dm)
	next(t, tb.player)
	next(t, table)
	if c := d.View.Checks[6]; c.Status != prep.CheckResolved || c.ChanceRoll != 99 || c.Outcome != prep.OutcomeNothing {
		t.Fatalf("a 99 against 50%% = %+v", c)
	}

	_, p = says(live.Command{Kind: live.CmdEncounterCheck, TableID: id("Road"), Mode: prep.ModeNormal})
	pending := p.View.Checks[7].RollID
	w.hub.Close(w.session.ID)
	tb.fill(pending, w.dm, 10)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdResync})
	if c := next(t, tb.dm).View.Checks[7]; c.Status != prep.CheckResolved || c.ChanceRoll != 10 || c.Outcome == "" || c.EntryLabel == "" {
		t.Fatalf("a roll resolved while the Session was down = %+v", c)
	}
	log, err := svc.Checks(ctx, dmCaller, w.session.CampaignID)
	if err != nil || len(log) != 8 || log[0].Seed == 0 {
		t.Fatalf("the check log = %d %v", len(log), err)
	}
	var revisions int
	if err := w.pool.QueryRow(ctx, "SELECT count(*) FROM campaign.revisions WHERE entity_type = 'encounter_check'").Scan(&revisions); err != nil || revisions != 10 {
		t.Fatalf("revisions = %d %v", revisions, err)
	}

	realm := w.realm(t)
	for _, cmd := range []live.Command{
		{Kind: live.CmdSetWorld, MapID: uuid.UUID(realm.ID).String()},
		{Kind: live.CmdAddNode, Label: "Oakford", Q: 0, R: 0},
		{Kind: live.CmdAddNode, Label: "Mill", Q: 5, R: 0},
	} {
		w.hub.Submit(tb.dm, cmd)
		next(t, tb.dm)
	}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdResync})
	wv := next(t, tb.dm).View.World
	oak, mill := nodeNamed(t, wv, "Oakford").ID, nodeNamed(t, wv, "Mill").ID
	road := tables["Road"]
	millID := uuid.MustParse(mill)
	road.RegionID = &millID
	if _, err := svc.SaveTable(ctx, dmCaller, w.session.CampaignID, road); err != nil {
		t.Fatal(err)
	}
	for _, cmd := range []live.Command{{Kind: live.CmdAddRoute, NodeID: oak, ToNodeID: mill, DistanceMi: 6}, {Kind: live.CmdPlaceParty, NodeID: oak}} {
		w.hub.Submit(tb.dm, cmd)
		next(t, tb.dm)
	}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdResync})
	route := next(t, tb.dm).View.World.Routes[0].ID
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdTravel, RouteID: route, Pace: "normal"})
	next(t, tb.dm)
	if c := next(t, tb.dm).View.Checks; len(c) != 9 || c[8].TableName != "Road" || c[8].Trigger != prep.TriggerTravelLeg || c[8].Status != prep.CheckPending {
		t.Fatalf("arriving at the Mill rolls on its road = %+v", c[len(c)-1])
	}

	w.hub.Close(w.session.ID)
	w.hub.Store = failingPrep{Store: pgstore.New(w.pool)}
	dm := join(t, w, w.dm, dmCaller, live.AudienceDM)
	w.hub.Submit(dm, live.Command{Kind: live.CmdEncounterCheck, TableID: id("Den"), Mode: prep.ModeForce})
	if u := next(t, dm); u.Reason != "The encounter tables could not be read." {
		t.Fatalf("prep load failure = %+v", u)
	}
	w.hub.Submit(dm, live.Command{Kind: live.CmdRest, Rest: live.RestLong})
	if u := next(t, dm); u.Kind != live.UpdView || len(u.View.Checks) != 9 {
		t.Fatalf("a rest without prep still rests = %+v", u.View.Checks)
	}
	w.hub.Close(w.session.ID)
	w.hub.Store = failingChecks{Store: pgstore.New(w.pool)}
	if _, err := w.hub.Join(ctx, w.session.ID, w.dm, dmCaller, live.AudienceDM); err == nil {
		t.Fatal("check load failure ignored")
	}
}
