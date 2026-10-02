-- name: ItemPrices :many
SELECT DISTINCT ON (i.slug) i.slug, i.name, (i.cost_gp * 100)::int AS cost_cp, i.magic, coalesce(i.rarity, '')::text AS rarity, i.weight_lb::float8 AS weight_lb
FROM compendium.items i
JOIN compendium.documents d ON d.id = i.document_id
WHERE i.slug = ANY(@slugs::text[])
ORDER BY i.slug, (d.key = (SELECT c.ruleset_pref FROM campaign.campaigns c WHERE c.id = @campaign_id)) DESC, d.precedence DESC;

-- name: CampaignNpc :one
SELECT name FROM campaign.npcs WHERE campaign_id = @campaign_id AND id = @id;

-- name: GameDay :one
SELECT game_day FROM campaign.campaigns WHERE id = $1;

-- name: SetGameDay :exec
UPDATE campaign.campaigns SET game_day = @game_day WHERE id = @id;

-- name: ListSettlements :many
SELECT id, name, size, wealth, location_id, updated_at FROM prep.settlements WHERE campaign_id = $1 ORDER BY name, id;

-- name: SaveSettlement :exec
INSERT INTO prep.settlements (id, campaign_id, name, size, wealth, location_id, updated_at)
VALUES (@id, @campaign_id, @name, @size, @wealth, sqlc.narg(location_id), @now)
ON CONFLICT (id) DO UPDATE SET name = excluded.name, size = excluded.size, wealth = excluded.wealth, location_id = excluded.location_id,
    updated_at = excluded.updated_at;

-- name: DeleteSettlement :execrows
DELETE FROM prep.settlements WHERE campaign_id = @campaign_id AND id = @id;

-- name: InsertSettlementRevision :exec
INSERT INTO prep.settlement_revisions (revision_id, name, size, wealth, location_id) VALUES (@revision_id, @name, @size, @wealth, sqlc.narg(location_id));

-- name: GetSettlementRevision :one
SELECT s.name, s.size, s.wealth, s.location_id
FROM campaign.revisions r JOIN prep.settlement_revisions s ON s.revision_id = r.id
WHERE r.campaign_id = @campaign_id AND r.entity_type = 'settlement' AND r.entity_id = @entity_id AND r.revision_no = @revision_no;

-- name: CampaignShops :many
SELECT s.id, s.settlement_id, s.name, s.kind, s.owner_npc_id, s.markup_pct, s.haggle_dc, s.haggle_pct, s.loot_table_id, s.restock, s.restock_days,
    s.stocked_day, s.updated_at
FROM prep.shops s JOIN prep.settlements t ON t.id = s.settlement_id
WHERE t.campaign_id = $1 ORDER BY s.name, s.id;

-- name: CampaignShopStock :many
SELECT k.shop_id, k.item_slug, k.quantity, k.price_cp
FROM prep.shop_stock k JOIN prep.shops s ON s.id = k.shop_id JOIN prep.settlements t ON t.id = s.settlement_id
WHERE t.campaign_id = $1 ORDER BY k.shop_id, k.item_slug;

-- name: SaveShop :exec
INSERT INTO prep.shops (id, settlement_id, name, kind, owner_npc_id, markup_pct, haggle_dc, haggle_pct, loot_table_id, restock, restock_days,
    stocked_day, updated_at)
VALUES (@id, @settlement_id, @name, @kind, sqlc.narg(owner_npc_id), @markup_pct, @haggle_dc, @haggle_pct, sqlc.narg(loot_table_id), @restock,
    sqlc.narg(restock_days), @stocked_day, @now)
ON CONFLICT (id) DO UPDATE SET settlement_id = excluded.settlement_id, name = excluded.name, kind = excluded.kind,
    owner_npc_id = excluded.owner_npc_id, markup_pct = excluded.markup_pct, haggle_dc = excluded.haggle_dc, haggle_pct = excluded.haggle_pct,
    loot_table_id = excluded.loot_table_id, restock = excluded.restock, restock_days = excluded.restock_days, stocked_day = excluded.stocked_day,
    updated_at = excluded.updated_at;

-- name: DeleteShop :execrows
DELETE FROM prep.shops s USING prep.settlements t WHERE s.settlement_id = t.id AND t.campaign_id = @campaign_id AND s.id = @id;

-- name: ClearShopStock :exec
DELETE FROM prep.shop_stock WHERE shop_id = $1;

-- name: SetShopStock :exec
INSERT INTO prep.shop_stock (shop_id, item_slug, quantity, price_cp) VALUES (@shop_id, @item_slug, @quantity, @price_cp)
ON CONFLICT (shop_id, item_slug) DO UPDATE SET quantity = excluded.quantity, price_cp = excluded.price_cp;

-- name: DeleteShopStock :exec
DELETE FROM prep.shop_stock WHERE shop_id = @shop_id AND item_slug = @item_slug;

-- name: InsertShopRevision :exec
INSERT INTO prep.shop_revisions (revision_id, settlement_id, name, kind, owner_npc_id, markup_pct, haggle_dc, haggle_pct, loot_table_id, restock,
    restock_days, stocked_day)
VALUES (@revision_id, @settlement_id, @name, @kind, sqlc.narg(owner_npc_id), @markup_pct, @haggle_dc, @haggle_pct, sqlc.narg(loot_table_id),
    @restock, sqlc.narg(restock_days), @stocked_day);

-- name: InsertShopRevisionStock :exec
INSERT INTO prep.shop_revision_stock (revision_id, item_slug, quantity, price_cp) VALUES (@revision_id, @item_slug, @quantity, @price_cp);

-- name: GetShopRevision :one
SELECT r.id, s.settlement_id, s.name, s.kind, s.owner_npc_id, s.markup_pct, s.haggle_dc, s.haggle_pct, s.loot_table_id, s.restock, s.restock_days,
    s.stocked_day
FROM campaign.revisions r JOIN prep.shop_revisions s ON s.revision_id = r.id
WHERE r.campaign_id = @campaign_id AND r.entity_type = 'shop' AND r.entity_id = @entity_id AND r.revision_no = @revision_no;

-- name: ShopRevisionStock :many
SELECT item_slug, quantity, price_cp FROM prep.shop_revision_stock WHERE revision_id = $1 ORDER BY item_slug;

-- name: SessionShop :many
SELECT shop_id FROM play.session_shops WHERE session_id = $1;

-- name: SetSessionShop :exec
INSERT INTO play.session_shops (session_id, shop_id) VALUES (@session_id, @shop_id)
ON CONFLICT (session_id) DO UPDATE SET shop_id = excluded.shop_id;

-- name: ClearSessionShop :exec
DELETE FROM play.session_shops WHERE session_id = $1;

-- name: SessionHaggles :many
SELECT shop_id, character_id, roll_id, adjust_pct FROM play.haggles WHERE session_id = $1 ORDER BY character_id;

-- name: SaveHaggle :exec
INSERT INTO play.haggles (session_id, shop_id, character_id, roll_id, adjust_pct) VALUES (@session_id, @shop_id, @character_id, sqlc.narg(roll_id), sqlc.narg(adjust_pct))
ON CONFLICT (session_id, shop_id, character_id) DO UPDATE SET roll_id = excluded.roll_id, adjust_pct = excluded.adjust_pct;

-- name: CharacterTrade :many
SELECT c.id, c.level,
    coalesce((SELECT a.base + a.bonus + a.increase FROM campaign.character_abilities a WHERE a.character_id = c.id AND a.ability = 'charisma'), 10)::int AS charisma,
    EXISTS (SELECT 1 FROM campaign.character_skills k WHERE k.character_id = c.id AND k.skill = 'persuasion') AS persuasive
FROM campaign.characters c WHERE c.campaign_id = $1 ORDER BY c.id;

-- name: SetShopStockedDay :exec
UPDATE prep.shops SET stocked_day = @stocked_day WHERE id = @id;

-- name: ClearSessionHaggles :exec
DELETE FROM play.haggles WHERE session_id = $1;

-- name: ClearContainerCoins :exec
DELETE FROM campaign.container_coins WHERE container_id = $1;
