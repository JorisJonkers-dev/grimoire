package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	playpg "github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	prepapp "github.com/JorisJonkers-dev/grimoire/api/internal/prep/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	preppg "github.com/JorisJonkers-dev/grimoire/api/internal/prep/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// brokenPrep fails every operation, as an unreachable database would.
type brokenPrep struct{}

var errBroken = errors.New("db down")

func (brokenPrep) Pools(context.Context, caller.Caller, uuid.UUID) ([]domain.Pool, error) {
	return nil, errBroken
}

func (brokenPrep) SavePool(context.Context, caller.Caller, uuid.UUID, domain.Pool) (domain.Pool, error) {
	return domain.Pool{}, errBroken
}

func (brokenPrep) DeletePool(context.Context, caller.Caller, uuid.UUID, domain.PoolID) error {
	return errBroken
}

func (brokenPrep) PoolRevisions(context.Context, caller.Caller, uuid.UUID, domain.PoolID) ([]campaigndomain.Revision, error) {
	return nil, errBroken
}

func (brokenPrep) RestorePool(context.Context, caller.Caller, uuid.UUID, domain.PoolID, int) (domain.Pool, error) {
	return domain.Pool{}, errBroken
}

func (brokenPrep) Tables(context.Context, caller.Caller, uuid.UUID) ([]domain.Table, error) {
	return nil, errBroken
}

func (brokenPrep) SaveTable(context.Context, caller.Caller, uuid.UUID, domain.Table) (domain.Table, error) {
	return domain.Table{}, errBroken
}

func (brokenPrep) DeleteTable(context.Context, caller.Caller, uuid.UUID, domain.TableID) error {
	return errBroken
}

func (brokenPrep) TableRevisions(context.Context, caller.Caller, uuid.UUID, domain.TableID) ([]campaigndomain.Revision, error) {
	return nil, errBroken
}

func (brokenPrep) RestoreTable(context.Context, caller.Caller, uuid.UUID, domain.TableID, int) (domain.Table, error) {
	return domain.Table{}, errBroken
}

func (brokenPrep) Locations(context.Context, caller.Caller, uuid.UUID) ([]domain.Location, error) {
	return nil, errBroken
}

func (brokenPrep) Checks(context.Context, caller.Caller, uuid.UUID) ([]domain.Check, error) {
	return nil, errBroken
}

func TestEncounterPrepOverHTTP(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	goblin := snapshot.Monster{
		Entry: snapshot.Entry{Document: "srd-2024", Slug: "goblin", Name: "Goblin"}, Size: "small", Type: "humanoid", Alignment: "neutral", ArmorClass: 12,
		HitPoints: 7, HitDice: "2d6", XP: 50, Abilities: map[string]int{"strength": 8, "dexterity": 14, "constitution": 10, "intelligence": 10, "wisdom": 8, "charisma": 8},
		Saves: map[string]int{}, Skills: map[string]int{}, Speeds: map[string]int{}, Senses: map[string]int{}, Resistances: []string{}, Immunities: []string{},
		Vulnerabilities: []string{}, ConditionImmunities: []string{}, Traits: []snapshot.Named{}, Actions: []snapshot.Action{},
	}
	if _, err := comppg.New(store.Pool()).Import(ctx, snapshot.Snapshot{
		Documents: []snapshot.Document{{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0", Attribution: "a", URL: "https://a"}},
		Monsters:  []snapshot.Monster{goblin},
	}, "http"); err != nil {
		t.Fatal(err)
	}
	repo := campaignpg.New(store.Pool())
	svc := &prepapp.Service{Repo: preppg.New(store.Pool()), Members: playpg.CampaignMembers{Store: repo}, Now: time.Now}
	h := campaignServer(t, campaignapp.NewService(repo), httpapi.PrepService(svc))
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id
	rec := call(h, http.MethodPost, base+"/encounter-pools", "dm", `{"name":"Band","levelMin":1,"levelMax":4,"difficulty":"low","members":[{"monsterSlug":"goblin","weight":2,"min":1,"max":4}]}`)
	poolID, _ := decode(t, rec)["id"].(string)
	if rec.Code != http.StatusCreated || poolID == "" {
		t.Fatalf("create pool: %d %s", rec.Code, rec.Body.String())
	}
	pool := base + "/encounter-pools/" + poolID
	if rec := call(h, http.MethodPut, pool, "dm", `{"name":"Band","levelMin":1,"levelMax":6,"difficulty":"high","members":[{"monsterSlug":"goblin","weight":2,"min":1,"max":4}]}`); rec.Code != 200 || decode(t, rec)["levelMax"] != 6.0 {
		t.Fatalf("update pool: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, base+"/encounter-pools", "dm", ""); !strings.Contains(rec.Body.String(), `"difficulty":"high"`) {
		t.Fatalf("list pools: %s", rec.Body.String())
	}
	if rec := call(h, http.MethodPost, base+"/encounter-pools", "dm", `{"name":"Band","levelMin":1,"levelMax":4,"difficulty":"low","members":[{"monsterSlug":"dragon","weight":1,"min":0,"max":1}]}`); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "dragon") {
		t.Fatalf("unknown monster: %d %s", rec.Code, rec.Body.String())
	}
	table := `{"name":"Road","chancePct":30,"visibility":"open","entries":[` +
		`{"weight":1,"kind":"encounter","label":"Ambush","monsters":[{"monsterSlug":"goblin","count":3}]},` +
		`{"weight":2,"kind":"pool","label":"","poolId":"` + poolID + `"},{"weight":3,"kind":"nothing","label":"Birdsong"}]}`
	rec = call(h, http.MethodPost, base+"/encounter-tables", "dm", table)
	tableID, _ := decode(t, rec)["id"].(string)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"poolId":"`+poolID+`"`) || !strings.Contains(rec.Body.String(), `"count":3`) {
		t.Fatalf("create table: %d %s", rec.Code, rec.Body.String())
	}
	one := base + "/encounter-tables/" + tableID
	if rec := call(h, http.MethodPut, one, "dm", strings.Replace(table, `"chancePct":30`, `"chancePct":60`, 1)); rec.Code != 200 {
		t.Fatalf("update table: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, base+"/encounter-tables", "dm", ""); !strings.Contains(rec.Body.String(), `"chancePct":60`) {
		t.Fatalf("list tables: %s", rec.Body.String())
	}
	if rec := call(h, http.MethodDelete, pool, "dm", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("delete a pool in use: %d", rec.Code)
	}
	if rec := call(h, http.MethodDelete, one, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete table: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, one+"/revisions/1/restore", "dm", ""); rec.Code != 200 || decode(t, rec)["chancePct"] != 30.0 {
		t.Fatalf("restore table: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, one+"/revisions", "dm", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"restoredFrom":1`) {
		t.Fatalf("table revisions: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodDelete, one, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete table again: %d", rec.Code)
	}
	if rec := call(h, http.MethodDelete, pool, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete pool: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, pool+"/revisions/1/restore", "dm", ""); rec.Code != 200 || decode(t, rec)["levelMax"] != 4.0 {
		t.Fatalf("restore pool: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, pool+"/revisions", "dm", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"action":"delete"`) {
		t.Fatalf("pool revisions: %d %s", rec.Code, rec.Body.String())
	}
	cid := uuid.MustParse(id)
	sid := uuid.New()
	if _, err := store.Pool().Exec(ctx, "INSERT INTO play.sessions (id, campaign_id, number, status, started_at) VALUES ($1, $2, 1, 'live', now())", sid, cid); err != nil {
		t.Fatal(err)
	}
	if err := preppg.WriteCheck(ctx, queries.New(store.Pool()), cid, domain.Check{
		ID: domain.CheckID(uuid.New()), SessionID: &sid, TableName: "Road", Trigger: domain.TriggerDM, Mode: domain.ModeNormal, Visibility: domain.Secret,
		Seed: -7, ChancePct: 30, ChanceRoll: 12, Status: domain.CheckResolved, Outcome: domain.OutcomeFight, EntryLabel: "Ambush",
		Monsters: []domain.EntryMonster{{Slug: "goblin", Count: 3}},
	}, time.Now()); err != nil {
		t.Fatal(err)
	}
	rec = call(h, http.MethodGet, base+"/encounter-checks", "dm", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"seed":"-7"`) || !strings.Contains(rec.Body.String(), `"chanceRoll":12`) || !strings.Contains(rec.Body.String(), `"sessionId":"`+sid.String()) {
		t.Fatalf("checks: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, base+"/locations", "dm", ""); rec.Code != 200 || rec.Body.String() != "[]" {
		t.Fatalf("locations: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, base+"/encounter-pools", "player", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("player: %d", rec.Code)
	}
}

func TestEncounterPrepFailuresOverHTTP(t *testing.T) {
	t.Parallel()
	h := campaignServer(t, brokenCampaigns{}, httpapi.PrepService(brokenPrep{}))
	base := "/api/v1/campaigns/" + uuid.NewString()
	pool, table := base+"/encounter-pools/"+uuid.NewString(), base+"/encounter-tables/"+uuid.NewString()
	poolBody := `{"name":"A","levelMin":1,"levelMax":2,"difficulty":"low","members":[{"monsterSlug":"goblin","weight":1,"min":0,"max":1}]}`
	tableBody := `{"name":"A","chancePct":1,"visibility":"secret","entries":[{"weight":1,"kind":"nothing","label":""}]}`
	for _, o := range []struct{ method, path, body string }{
		{http.MethodGet, base + "/encounter-pools", ""},
		{http.MethodPost, base + "/encounter-pools", poolBody},
		{http.MethodPut, pool, poolBody},
		{http.MethodDelete, pool, ""},
		{http.MethodGet, pool + "/revisions", ""},
		{http.MethodPost, pool + "/revisions/1/restore", ""},
		{http.MethodGet, base + "/encounter-tables", ""},
		{http.MethodPost, base + "/encounter-tables", tableBody},
		{http.MethodPut, table, tableBody},
		{http.MethodDelete, table, ""},
		{http.MethodGet, table + "/revisions", ""},
		{http.MethodPost, table + "/revisions/1/restore", ""},
		{http.MethodGet, base + "/locations", ""},
		{http.MethodGet, base + "/encounter-checks", ""},
	} {
		if rec := call(h, o.method, o.path, "u", o.body); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s: %d %s", o.method, o.path, rec.Code, rec.Body.String())
		}
	}
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.ListEncounterPools(ctx, oas.ListEncounterPoolsParams{}))
	add(hh.CreateEncounterPool(ctx, &oas.EncounterPoolInput{}, oas.CreateEncounterPoolParams{}))
	add(hh.UpdateEncounterPool(ctx, &oas.EncounterPoolInput{}, oas.UpdateEncounterPoolParams{}))
	add(hh.DeleteEncounterPool(ctx, oas.DeleteEncounterPoolParams{}))
	add(hh.ListEncounterPoolRevisions(ctx, oas.ListEncounterPoolRevisionsParams{}))
	add(hh.RestoreEncounterPoolRevision(ctx, oas.RestoreEncounterPoolRevisionParams{}))
	add(hh.ListEncounterTables(ctx, oas.ListEncounterTablesParams{}))
	add(hh.CreateEncounterTable(ctx, &oas.EncounterTableInput{}, oas.CreateEncounterTableParams{}))
	add(hh.UpdateEncounterTable(ctx, &oas.EncounterTableInput{}, oas.UpdateEncounterTableParams{}))
	add(hh.DeleteEncounterTable(ctx, oas.DeleteEncounterTableParams{}))
	add(hh.ListEncounterTableRevisions(ctx, oas.ListEncounterTableRevisionsParams{}))
	add(hh.RestoreEncounterTableRevision(ctx, oas.RestoreEncounterTableRevisionParams{}))
	add(hh.ListLocations(ctx, oas.ListLocationsParams{}))
	add(hh.ListEncounterChecks(ctx, oas.ListEncounterChecksParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
