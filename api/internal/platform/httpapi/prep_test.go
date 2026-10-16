package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
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
