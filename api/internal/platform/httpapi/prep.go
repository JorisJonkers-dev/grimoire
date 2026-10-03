package httpapi

import (
	"context"
	"strconv"

	"github.com/google/uuid"

	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// PrepService is what the random-encounter prep operations need.
type PrepService interface {
	Pools(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Pool, error)
	SavePool(ctx context.Context, c caller.Caller, campaign uuid.UUID, p domain.Pool) (domain.Pool, error)
	DeletePool(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.PoolID) error
	PoolRevisions(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.PoolID) ([]campaigndomain.Revision, error)
	RestorePool(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.PoolID, no int) (domain.Pool, error)
	Tables(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Table, error)
	SaveTable(ctx context.Context, c caller.Caller, campaign uuid.UUID, t domain.Table) (domain.Table, error)
	DeleteTable(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.TableID) error
	TableRevisions(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.TableID) ([]campaigndomain.Revision, error)
	RestoreTable(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.TableID, no int) (domain.Table, error)
	Locations(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Location, error)
	Checks(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Check, error)
	LootTables(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.LootTable, error)
	SaveLootTable(ctx context.Context, c caller.Caller, campaign uuid.UUID, t domain.LootTable) (domain.LootTable, error)
	DeleteLootTable(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.LootTableID) error
	LootTableRevisions(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.LootTableID) ([]campaigndomain.Revision, error)
	RestoreLootTable(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.LootTableID, no int) (domain.LootTable, error)
	Settlements(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Settlement, error)
	SaveSettlement(ctx context.Context, c caller.Caller, campaign uuid.UUID, x domain.Settlement) (domain.Settlement, error)
	DeleteSettlement(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.SettlementID) error
	SettlementRevisions(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.SettlementID) ([]campaigndomain.Revision, error)
	RestoreSettlement(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.SettlementID, no int) (domain.Settlement, error)
	Shops(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Shop, error)
	SaveShop(ctx context.Context, c caller.Caller, campaign uuid.UUID, x domain.Shop) (domain.Shop, error)
	DeleteShop(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.ShopID) error
	ShopRevisions(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.ShopID) ([]campaigndomain.Revision, error)
	RestoreShop(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.ShopID, no int) (domain.Shop, error)
	RerollStock(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.ShopID) (domain.Shop, error)
}

func poolIn(id domain.PoolID, req *oas.EncounterPoolInput) domain.Pool {
	p := domain.Pool{ID: id, Name: req.Name, LevelMin: int(req.LevelMin), LevelMax: int(req.LevelMax), Difficulty: string(req.Difficulty), Members: []domain.PoolMember{}}
	for _, m := range req.Members {
		p.Members = append(p.Members, domain.PoolMember{Slug: string(m.MonsterSlug), Weight: int(m.Weight), Min: int(m.Min), Max: int(m.Max)})
	}
	return p
}

//nolint:gosec // levels, weights and counts are bounded by the rules
func poolOut(p domain.Pool) oas.EncounterPool {
	out := oas.EncounterPool{
		ID: oas.ID(p.ID), Name: p.Name, LevelMin: int32(p.LevelMin), LevelMax: int32(p.LevelMax), Difficulty: oas.EncounterDifficulty(p.Difficulty),
		Members: []oas.EncounterPoolMember{}, UpdatedAt: p.UpdatedAt.UTC(),
	}
	for _, m := range p.Members {
		out.Members = append(out.Members, oas.EncounterPoolMember{MonsterSlug: oas.Slug(m.Slug), Weight: int32(m.Weight), Min: int32(m.Min), Max: int32(m.Max)})
	}
	return out
}

func monstersIn(ms []oas.EncounterMonster) []domain.EntryMonster {
	var out []domain.EntryMonster
	for _, m := range ms {
		out = append(out, domain.EntryMonster{Slug: string(m.MonsterSlug), Count: int(m.Count)})
	}
	return out
}

//nolint:gosec // counts are bounded by the rules
func monstersOut(ms []domain.EntryMonster) []oas.EncounterMonster {
	out := []oas.EncounterMonster{}
	for _, m := range ms {
		out = append(out, oas.EncounterMonster{MonsterSlug: oas.Slug(m.Slug), Count: int32(m.Count)})
	}
	return out
}

func tableIn(id domain.TableID, req *oas.EncounterTableInput) domain.Table {
	t := domain.Table{ID: id, Name: req.Name, ChancePct: int(req.ChancePct), Visibility: string(req.Visibility), Entries: []domain.Entry{}}
	if r, ok := req.RegionId.Get(); ok {
		u := uuid.UUID(r)
		t.RegionID = &u
	}
	for _, e := range req.Entries {
		entry := domain.Entry{Weight: int(e.Weight), Kind: string(e.Kind), Label: e.Label, Monsters: monstersIn(e.Monsters)}
		if p, ok := e.PoolId.Get(); ok {
			pid := domain.PoolID(p)
			entry.PoolID = &pid
		}
		if f, ok := e.FactionId.Get(); ok {
			fid := uuid.UUID(f)
			entry.FactionID = &fid
		}
		t.Entries = append(t.Entries, entry)
	}
	return t
}

//nolint:gosec // chances and weights are bounded by the rules
func tableOut(t domain.Table) oas.EncounterTable {
	out := oas.EncounterTable{
		ID: oas.ID(t.ID), Name: t.Name, ChancePct: int32(t.ChancePct), Visibility: oas.EncounterVisibility(t.Visibility),
		Entries: []oas.EncounterEntry{}, UpdatedAt: t.UpdatedAt.UTC(),
	}
	if t.RegionID != nil {
		out.RegionId = oas.NewOptID(oas.ID(*t.RegionID))
	}
	for _, e := range t.Entries {
		entry := oas.EncounterEntry{Weight: int32(e.Weight), Kind: oas.EncounterEntryKind(e.Kind), Label: e.Label, Monsters: monstersOut(e.Monsters)}
		if e.PoolID != nil {
			entry.PoolId = oas.NewOptID(oas.ID(*e.PoolID))
		}
		if e.FactionID != nil {
			entry.FactionId = oas.NewOptID(oas.ID(*e.FactionID))
		}
		out.Entries = append(out.Entries, entry)
	}
	return out
}

func revisionsOut(revs []campaigndomain.Revision) []oas.Revision {
	out := make([]oas.Revision, 0, len(revs))
	for _, r := range revs {
		rev := oas.Revision{
			No: int32(r.No), Action: oas.RevisionAction(r.Action), Author: oas.DisplayName(r.Author), //nolint:gosec // bounded
			Origin: oas.RevisionOrigin(r.Origin), CreatedAt: r.CreatedAt.UTC(),
		}
		setOpt(&rev.Client, r.Client)
		if r.RestoredFrom > 0 {
			rev.RestoredFrom = oas.NewOptInt32(int32(r.RestoredFrom)) //nolint:gosec // bounded
		}
		out = append(out, rev)
	}
	return out
}

// ListEncounterPools lists the Campaign's Encounter Pools.
func (h *Handler) ListEncounterPools(ctx context.Context, p oas.ListEncounterPoolsParams) (oas.ListEncounterPoolsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.Prep.Pools(ctx, c, uuid.UUID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list pools", err), nil
	}
	out := make([]oas.EncounterPool, 0, len(list))
	for _, x := range list {
		out = append(out, poolOut(x))
	}
	return &oas.ListEncounterPoolsOKHeaders{Response: out}, nil
}

// CreateEncounterPool adds an Encounter Pool.
func (h *Handler) CreateEncounterPool(ctx context.Context, req *oas.EncounterPoolInput, p oas.CreateEncounterPoolParams) (oas.CreateEncounterPoolRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	saved, err := h.Prep.SavePool(ctx, c, uuid.UUID(p.CampaignId), poolIn(domain.PoolID{}, req))
	if err != nil {
		return h.campaignProblem(ctx, "create pool", err), nil
	}
	return &oas.EncounterPoolHeaders{Response: poolOut(saved)}, nil
}

// UpdateEncounterPool replaces an Encounter Pool.
func (h *Handler) UpdateEncounterPool(ctx context.Context, req *oas.EncounterPoolInput, p oas.UpdateEncounterPoolParams) (oas.UpdateEncounterPoolRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	saved, err := h.Prep.SavePool(ctx, c, uuid.UUID(p.CampaignId), poolIn(domain.PoolID(p.PoolId), req))
	if err != nil {
		return h.campaignProblem(ctx, "update pool", err), nil
	}
	return &oas.EncounterPoolHeaders{Response: poolOut(saved)}, nil
}

// DeleteEncounterPool removes an Encounter Pool.
func (h *Handler) DeleteEncounterPool(ctx context.Context, p oas.DeleteEncounterPoolParams) (oas.DeleteEncounterPoolRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Prep.DeletePool(ctx, c, uuid.UUID(p.CampaignId), domain.PoolID(p.PoolId)); err != nil {
		return h.campaignProblem(ctx, "delete pool", err), nil
	}
	return &oas.DeleteEncounterPoolNoContent{}, nil
}

// ListEncounterPoolRevisions lists an Encounter Pool's Revisions.
func (h *Handler) ListEncounterPoolRevisions(ctx context.Context, p oas.ListEncounterPoolRevisionsParams) (oas.ListEncounterPoolRevisionsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	revs, err := h.Prep.PoolRevisions(ctx, c, uuid.UUID(p.CampaignId), domain.PoolID(p.PoolId))
	if err != nil {
		return h.campaignProblem(ctx, "pool revisions", err), nil
	}
	return &oas.ListEncounterPoolRevisionsOKHeaders{Response: revisionsOut(revs)}, nil
}

// RestoreEncounterPoolRevision brings an Encounter Pool back to a Revision.
func (h *Handler) RestoreEncounterPoolRevision(ctx context.Context, p oas.RestoreEncounterPoolRevisionParams) (oas.RestoreEncounterPoolRevisionRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	saved, err := h.Prep.RestorePool(ctx, c, uuid.UUID(p.CampaignId), domain.PoolID(p.PoolId), int(p.RevisionNo))
	if err != nil {
		return h.campaignProblem(ctx, "restore pool", err), nil
	}
	return &oas.EncounterPoolHeaders{Response: poolOut(saved)}, nil
}

// ListEncounterTables lists the Campaign's Encounter Tables.
func (h *Handler) ListEncounterTables(ctx context.Context, p oas.ListEncounterTablesParams) (oas.ListEncounterTablesRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.Prep.Tables(ctx, c, uuid.UUID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list tables", err), nil
	}
	out := make([]oas.EncounterTable, 0, len(list))
	for _, x := range list {
		out = append(out, tableOut(x))
	}
	return &oas.ListEncounterTablesOKHeaders{Response: out}, nil
}

// CreateEncounterTable adds an Encounter Table.
func (h *Handler) CreateEncounterTable(ctx context.Context, req *oas.EncounterTableInput, p oas.CreateEncounterTableParams) (oas.CreateEncounterTableRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	saved, err := h.Prep.SaveTable(ctx, c, uuid.UUID(p.CampaignId), tableIn(domain.TableID{}, req))
	if err != nil {
		return h.campaignProblem(ctx, "create table", err), nil
	}
	return &oas.EncounterTableHeaders{Response: tableOut(saved)}, nil
}

// UpdateEncounterTable replaces an Encounter Table.
func (h *Handler) UpdateEncounterTable(ctx context.Context, req *oas.EncounterTableInput, p oas.UpdateEncounterTableParams) (oas.UpdateEncounterTableRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	saved, err := h.Prep.SaveTable(ctx, c, uuid.UUID(p.CampaignId), tableIn(domain.TableID(p.TableId), req))
	if err != nil {
		return h.campaignProblem(ctx, "update table", err), nil
	}
	return &oas.EncounterTableHeaders{Response: tableOut(saved)}, nil
}

// DeleteEncounterTable removes an Encounter Table.
func (h *Handler) DeleteEncounterTable(ctx context.Context, p oas.DeleteEncounterTableParams) (oas.DeleteEncounterTableRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Prep.DeleteTable(ctx, c, uuid.UUID(p.CampaignId), domain.TableID(p.TableId)); err != nil {
		return h.campaignProblem(ctx, "delete table", err), nil
	}
	return &oas.DeleteEncounterTableNoContent{}, nil
}

// ListEncounterTableRevisions lists an Encounter Table's Revisions.
func (h *Handler) ListEncounterTableRevisions(ctx context.Context, p oas.ListEncounterTableRevisionsParams) (oas.ListEncounterTableRevisionsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	revs, err := h.Prep.TableRevisions(ctx, c, uuid.UUID(p.CampaignId), domain.TableID(p.TableId))
	if err != nil {
		return h.campaignProblem(ctx, "table revisions", err), nil
	}
	return &oas.ListEncounterTableRevisionsOKHeaders{Response: revisionsOut(revs)}, nil
}

// RestoreEncounterTableRevision brings an Encounter Table back to a Revision.
func (h *Handler) RestoreEncounterTableRevision(ctx context.Context, p oas.RestoreEncounterTableRevisionParams) (oas.RestoreEncounterTableRevisionRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	saved, err := h.Prep.RestoreTable(ctx, c, uuid.UUID(p.CampaignId), domain.TableID(p.TableId), int(p.RevisionNo))
	if err != nil {
		return h.campaignProblem(ctx, "restore table", err), nil
	}
	return &oas.EncounterTableHeaders{Response: tableOut(saved)}, nil
}

// ListLocations lists the places a Table can belong to.
func (h *Handler) ListLocations(ctx context.Context, p oas.ListLocationsParams) (oas.ListLocationsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.Prep.Locations(ctx, c, uuid.UUID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list locations", err), nil
	}
	out := make([]oas.Location, 0, len(list))
	for _, l := range list {
		out = append(out, oas.Location{ID: oas.ID(l.ID), Name: l.Name, MapName: l.MapName})
	}
	return &oas.ListLocationsOKHeaders{Response: out}, nil
}

// ListEncounterChecks lists the Campaign's latest Encounter Checks.
//
//nolint:gosec // chances and rolls are bounded by the rules
func (h *Handler) ListEncounterChecks(ctx context.Context, p oas.ListEncounterChecksParams) (oas.ListEncounterChecksRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.Prep.Checks(ctx, c, uuid.UUID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list checks", err), nil
	}
	out := make([]oas.EncounterCheck, 0, len(list))
	for _, x := range list {
		ch := oas.EncounterCheck{
			ID: oas.ID(x.ID), TableName: x.TableName, Trigger: oas.EncounterTrigger(x.Trigger), Mode: oas.EncounterMode(x.Mode),
			Visibility: oas.EncounterVisibility(x.Visibility), Seed: strconv.FormatInt(x.Seed, 10), ChancePct: int32(x.ChancePct),
			Status: oas.EncounterCheckStatus(x.Status), EntryLabel: x.EntryLabel, Monsters: monstersOut(x.Monsters), CreatedAt: x.CreatedAt.UTC(),
		}
		if x.SessionID != nil {
			ch.SessionId = oas.NewOptID(oas.ID(*x.SessionID))
		}
		if x.ChanceRoll > 0 {
			ch.ChanceRoll = oas.NewOptInt32(int32(x.ChanceRoll))
		}
		if x.Outcome != "" {
			ch.Outcome = oas.NewOptEncounterCheckOutcome(oas.EncounterCheckOutcome(x.Outcome))
		}
		out = append(out, ch)
	}
	return &oas.ListEncounterChecksOKHeaders{Response: out}, nil
}
