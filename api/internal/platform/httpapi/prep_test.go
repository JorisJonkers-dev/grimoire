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
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
	playpg "github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	prepapp "github.com/JorisJonkers-dev/grimoire/api/internal/prep/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	preppg "github.com/JorisJonkers-dev/grimoire/api/internal/prep/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
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

func (brokenPrep) LootTables(context.Context, caller.Caller, uuid.UUID) ([]domain.LootTable, error) {
	return nil, errBroken
}

func (brokenPrep) SaveLootTable(context.Context, caller.Caller, uuid.UUID, domain.LootTable) (domain.LootTable, error) {
	return domain.LootTable{}, errBroken
}

func (brokenPrep) DeleteLootTable(context.Context, caller.Caller, uuid.UUID, domain.LootTableID) error {
	return errBroken
}

func (brokenPrep) LootTableRevisions(context.Context, caller.Caller, uuid.UUID, domain.LootTableID) ([]campaigndomain.Revision, error) {
	return nil, errBroken
}

func (brokenPrep) RestoreLootTable(context.Context, caller.Caller, uuid.UUID, domain.LootTableID, int) (domain.LootTable, error) {
	return domain.LootTable{}, errBroken
}

func (brokenPrep) Settlements(context.Context, caller.Caller, uuid.UUID) ([]domain.Settlement, error) {
	return nil, errBroken
}

func (brokenPrep) SaveSettlement(context.Context, caller.Caller, uuid.UUID, domain.Settlement) (domain.Settlement, error) {
	return domain.Settlement{}, errBroken
}

func (brokenPrep) DeleteSettlement(context.Context, caller.Caller, uuid.UUID, domain.SettlementID) error {
	return errBroken
}

func (brokenPrep) SettlementRevisions(context.Context, caller.Caller, uuid.UUID, domain.SettlementID) ([]campaigndomain.Revision, error) {
	return nil, errBroken
}

func (brokenPrep) RestoreSettlement(context.Context, caller.Caller, uuid.UUID, domain.SettlementID, int) (domain.Settlement, error) {
	return domain.Settlement{}, errBroken
}

func (brokenPrep) Shops(context.Context, caller.Caller, uuid.UUID) ([]domain.Shop, error) {
	return nil, errBroken
}

func (brokenPrep) SaveShop(context.Context, caller.Caller, uuid.UUID, domain.Shop) (domain.Shop, error) {
	return domain.Shop{}, errBroken
}

func (brokenPrep) DeleteShop(context.Context, caller.Caller, uuid.UUID, domain.ShopID) error {
	return errBroken
}

func (brokenPrep) ShopRevisions(context.Context, caller.Caller, uuid.UUID, domain.ShopID) ([]campaigndomain.Revision, error) {
	return nil, errBroken
}

func (brokenPrep) RestoreShop(context.Context, caller.Caller, uuid.UUID, domain.ShopID, int) (domain.Shop, error) {
	return domain.Shop{}, errBroken
}

func (brokenPrep) RerollStock(context.Context, caller.Caller, uuid.UUID, domain.ShopID) (domain.Shop, error) {
	return domain.Shop{}, errBroken
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
		Items:     []snapshot.Item{{Entry: snapshot.Entry{Document: "srd-2024", Slug: "rope", Name: "Rope", Description: "Rope."}, Category: "gear", WeightLB: 5}},
	}, "http"); err != nil {
		t.Fatal(err)
	}
	repo := campaignpg.New(store.Pool())
	svc := &prepapp.Service{
		Repo: preppg.New(store.Pool()), Members: playpg.CampaignMembers{Store: repo}, Now: time.Now,
		Seed: func() uint64 { return 5 }, Source: func(seed uint64) dice.Source { return rng.New(seed) },
	}
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
	// An entry can be a Faction's own, and a Shop can belong to one.
	watch := uuid.NewString()
	if _, err := store.Pool().Exec(ctx, `INSERT INTO campaign.factions (id, campaign_id, name, archetype, goals, territory, notes, score, created_at, updated_at)
		VALUES ($1, $2, 'The Lantern Watch', '', '', '', '', 0, now(), now())`, watch, id); err != nil {
		t.Fatal(err)
	}
	table := `{"name":"Road","chancePct":30,"visibility":"open","entries":[` +
		`{"weight":1,"kind":"encounter","label":"Ambush","factionId":"` + watch + `","monsters":[{"monsterSlug":"goblin","count":3}]},` +
		`{"weight":2,"kind":"pool","label":"","poolId":"` + poolID + `"},{"weight":3,"kind":"nothing","label":"Birdsong"}]}`
	rec = call(h, http.MethodPost, base+"/encounter-tables", "dm", table)
	tableID, _ := decode(t, rec)["id"].(string)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"poolId":"`+poolID+`"`) || !strings.Contains(rec.Body.String(), `"count":3`) ||
		!strings.Contains(rec.Body.String(), `"factionId":"`+watch+`"`) {
		t.Fatalf("create table: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, base+"/encounter-tables", "dm", strings.Replace(table, watch, uuid.NewString(), 1)); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("an entry of an unknown Faction: %d %s", rec.Code, rec.Body.String())
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
	rec = call(h, http.MethodPost, base+"/loot-tables", "dm", `{"name":"Purse","rolls":1,"entries":[{"weight":1,"kind":"currency","coin":"gp","amount":"2d6x10"}]}`)
	purse, _ := decode(t, rec)["id"].(string)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"amount":"2d6x10"`) {
		t.Fatalf("create loot: %d %s", rec.Code, rec.Body.String())
	}
	loot := `{"name":"Hoard","rolls":2,"entries":[{"weight":1,"kind":"table","tableId":"` + purse + `"},{"weight":2,"kind":"item","itemSlug":"rope","amount":"1"},{"weight":1,"kind":"nothing"}]}`
	rec = call(h, http.MethodPost, base+"/loot-tables", "dm", loot)
	hoard, _ := decode(t, rec)["id"].(string)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"tableId":"`+purse) || !strings.Contains(rec.Body.String(), `"itemSlug":"rope"`) {
		t.Fatalf("create hoard: %d %s", rec.Code, rec.Body.String())
	}
	lootOne := base + "/loot-tables/" + hoard
	if rec := call(h, http.MethodPut, lootOne, "dm", strings.Replace(loot, `"rolls":2`, `"rolls":4`, 1)); rec.Code != 200 || decode(t, rec)["rolls"] != 4.0 {
		t.Fatalf("update loot: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, base+"/loot-tables", "dm", ""); !strings.Contains(rec.Body.String(), `"name":"Purse"`) {
		t.Fatalf("list loot: %s", rec.Body.String())
	}
	if rec := call(h, http.MethodDelete, lootOne, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete loot: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, lootOne+"/revisions/1/restore", "dm", ""); rec.Code != 200 || decode(t, rec)["rolls"] != 2.0 {
		t.Fatalf("restore loot: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, lootOne+"/revisions", "dm", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"action":"restore"`) {
		t.Fatalf("loot revisions: %d %s", rec.Code, rec.Body.String())
	}
	var place, owner uuid.UUID
	if err := store.Pool().QueryRow(ctx, `INSERT INTO campaign.maps (campaign_id, name, kind, image_key, image_type, width_px, height_px, hex_size_px, origin_x, origin_y, created_at, updated_at)
		VALUES ($1, 'Realm', 'world', 'k', 'image/png', 400, 300, 40, 35, 40, now(), now()) RETURNING id`, cid).Scan(&place); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool().QueryRow(ctx, "INSERT INTO campaign.map_nodes (id, map_id, name, q, r) VALUES (gen_random_uuid(), $1, 'Oakford', 0, 0) RETURNING id", place).Scan(&place); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool().QueryRow(ctx, "INSERT INTO campaign.npcs (campaign_id, name) VALUES ($1, 'Tamsin') RETURNING id", cid).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	rec = call(h, http.MethodPost, base+"/settlements", "dm", `{"name":"Oakford","size":"village","wealth":"modest","locationId":"`+place.String()+`"}`)
	townID, _ := decode(t, rec)["id"].(string)
	if rec.Code != http.StatusCreated || townID == "" {
		t.Fatalf("create settlement: %d %s", rec.Code, rec.Body.String())
	}
	town := base + "/settlements/" + townID
	if rec := call(h, http.MethodPut, town, "dm", `{"name":"Oakford","size":"town","wealth":"wealthy"}`); rec.Code != 200 || decode(t, rec)["size"] != "town" || strings.Contains(rec.Body.String(), "locationId") {
		t.Fatalf("update settlement: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, base+"/settlements", "dm", ""); !strings.Contains(rec.Body.String(), `"wealth":"wealthy"`) {
		t.Fatalf("list settlements: %s", rec.Body.String())
	}
	// The Shop belongs to a Faction of the Campaign.
	shopBody := `{"settlementId":"` + townID + `","ownerId":"` + owner.String() + `","factionId":"` + watch + `","name":"Store","kind":"general","markupPct":50,"haggleDc":15,"hagglePct":20,"lootTableId":"` + hoard + `","restock":"days","restockDays":3}`
	rec = call(h, http.MethodPost, base+"/shops", "dm", shopBody)
	shopID, _ := decode(t, rec)["id"].(string)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"restockDays":3`) || !strings.Contains(rec.Body.String(), `"ownerId":"`+owner.String()) || !strings.Contains(rec.Body.String(), `"factionId":"`+watch) {
		t.Fatalf("create shop: %d %s", rec.Code, rec.Body.String())
	}
	shop := base + "/shops/" + shopID
	rec = call(h, http.MethodPost, shop+"/stock", "dm", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"itemSlug":"rope"`) {
		t.Fatalf("reroll stock: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPut, shop, "dm", strings.Replace(shopBody, `"restock":"days","restockDays":3`, `"restock":"never"`, 1)); rec.Code != 200 || strings.Contains(rec.Body.String(), "restockDays") {
		t.Fatalf("update shop: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, base+"/shops", "dm", ""); !strings.Contains(rec.Body.String(), `"restock":"never"`) {
		t.Fatalf("list shops: %s", rec.Body.String())
	}
	if rec := call(h, http.MethodDelete, shop, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete shop: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, shop+"/revisions/2/restore", "dm", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"itemSlug":"rope"`) {
		t.Fatalf("restore shop: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, shop+"/revisions", "dm", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"restoredFrom":2`) {
		t.Fatalf("shop revisions: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodDelete, town, "dm", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("delete a town with shops: %d", rec.Code)
	}
	call(h, http.MethodDelete, shop, "dm", "")
	if rec := call(h, http.MethodDelete, town, "dm", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete settlement: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, town+"/revisions/1/restore", "dm", ""); rec.Code != 200 || decode(t, rec)["locationId"] != place.String() {
		t.Fatalf("restore settlement: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, town+"/revisions", "dm", ""); rec.Code != 200 {
		t.Fatalf("settlement revisions: %d", rec.Code)
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
	loot, lootBody := base+"/loot-tables/"+uuid.NewString(), `{"name":"A","rolls":1,"entries":[{"weight":1,"kind":"nothing"}]}`
	town, townBody := base+"/settlements/"+uuid.NewString(), `{"name":"A","size":"town","wealth":"poor"}`
	shop, shopBody := base+"/shops/"+uuid.NewString(), `{"settlementId":"`+uuid.NewString()+`","name":"A","kind":"b","markupPct":0,"haggleDc":10,"hagglePct":0,"restock":"never"}`
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
		{http.MethodGet, base + "/loot-tables", ""},
		{http.MethodPost, base + "/loot-tables", lootBody},
		{http.MethodPut, loot, lootBody},
		{http.MethodDelete, loot, ""},
		{http.MethodGet, loot + "/revisions", ""},
		{http.MethodPost, loot + "/revisions/1/restore", ""},
		{http.MethodGet, base + "/settlements", ""},
		{http.MethodPost, base + "/settlements", townBody},
		{http.MethodPut, town, townBody},
		{http.MethodDelete, town, ""},
		{http.MethodGet, town + "/revisions", ""},
		{http.MethodPost, town + "/revisions/1/restore", ""},
		{http.MethodGet, base + "/shops", ""},
		{http.MethodPost, base + "/shops", shopBody},
		{http.MethodPut, shop, shopBody},
		{http.MethodDelete, shop, ""},
		{http.MethodGet, shop + "/revisions", ""},
		{http.MethodPost, shop + "/revisions/1/restore", ""},
		{http.MethodPost, shop + "/stock", ""},
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
	add(hh.ListLootTables(ctx, oas.ListLootTablesParams{}))
	add(hh.CreateLootTable(ctx, &oas.LootTableInput{}, oas.CreateLootTableParams{}))
	add(hh.UpdateLootTable(ctx, &oas.LootTableInput{}, oas.UpdateLootTableParams{}))
	add(hh.DeleteLootTable(ctx, oas.DeleteLootTableParams{}))
	add(hh.ListLootTableRevisions(ctx, oas.ListLootTableRevisionsParams{}))
	add(hh.RestoreLootTableRevision(ctx, oas.RestoreLootTableRevisionParams{}))
	add(hh.ListSettlements(ctx, oas.ListSettlementsParams{}))
	add(hh.CreateSettlement(ctx, &oas.SettlementInput{}, oas.CreateSettlementParams{}))
	add(hh.UpdateSettlement(ctx, &oas.SettlementInput{}, oas.UpdateSettlementParams{}))
	add(hh.DeleteSettlement(ctx, oas.DeleteSettlementParams{}))
	add(hh.ListSettlementRevisions(ctx, oas.ListSettlementRevisionsParams{}))
	add(hh.RestoreSettlementRevision(ctx, oas.RestoreSettlementRevisionParams{}))
	add(hh.ListShops(ctx, oas.ListShopsParams{}))
	add(hh.CreateShop(ctx, &oas.ShopInput{}, oas.CreateShopParams{}))
	add(hh.UpdateShop(ctx, &oas.ShopInput{}, oas.UpdateShopParams{}))
	add(hh.DeleteShop(ctx, oas.DeleteShopParams{}))
	add(hh.ListShopRevisions(ctx, oas.ListShopRevisionsParams{}))
	add(hh.RestoreShopRevision(ctx, oas.RestoreShopRevisionParams{}))
	add(hh.RerollStock(ctx, oas.RerollStockParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
