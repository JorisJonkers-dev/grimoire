-- name: ItemsBySlug :many
SELECT DISTINCT ON (i.slug) i.slug, i.name, i.weight_lb::float8 AS weight_lb, i.category, i.requires_attunement,
    coalesce(i.attunement_detail, '')::text AS attunement_detail,
    coalesce(ch.max_charges, 0)::int AS max_charges, coalesce(ch.regain_dice, 0)::int AS regain_dice,
    coalesce(ch.regain_faces, 0)::int AS regain_faces, coalesce(ch.regain_bonus, 0)::int AS regain_bonus,
    coalesce(ch.recharge_on, '')::text AS recharge_on
FROM compendium.items i
LEFT JOIN compendium.item_charges ch ON ch.item_slug = i.slug
JOIN compendium.documents d ON d.id = i.document_id
WHERE i.slug = ANY(@slugs::text[])
ORDER BY i.slug, (d.key = (SELECT c.ruleset_pref FROM campaign.campaigns c WHERE c.id = @campaign_id)) DESC, d.precedence DESC;

-- name: ListLootTables :many
SELECT id, name, rolls, updated_at FROM prep.loot_tables WHERE campaign_id = $1 ORDER BY name, id;

-- name: CampaignLootEntries :many
SELECT e.table_id, e.ordering, e.weight, e.kind, e.item_slug, e.coin, e.amount, e.nested_table_id
FROM prep.loot_entries e JOIN prep.loot_tables t ON t.id = e.table_id
WHERE t.campaign_id = $1 ORDER BY e.table_id, e.ordering;

-- name: SaveLootTable :exec
INSERT INTO prep.loot_tables (id, campaign_id, name, rolls, updated_at) VALUES (@id, @campaign_id, @name, @rolls, @now)
ON CONFLICT (id) DO UPDATE SET name = excluded.name, rolls = excluded.rolls, updated_at = excluded.updated_at;

-- name: ClearLootEntries :exec
DELETE FROM prep.loot_entries WHERE table_id = $1;

-- name: InsertLootEntry :exec
INSERT INTO prep.loot_entries (table_id, ordering, weight, kind, item_slug, coin, amount, nested_table_id)
VALUES (@table_id, @ordering, @weight, @kind, sqlc.narg(item_slug), sqlc.narg(coin), @amount, sqlc.narg(nested_table_id));

-- name: DeleteLootTable :execrows
DELETE FROM prep.loot_tables WHERE campaign_id = @campaign_id AND id = @id;

-- name: LootTableInUse :one
SELECT count(*)::int FROM prep.loot_entries WHERE nested_table_id = $1;

-- name: InsertLootTableRevision :exec
INSERT INTO prep.loot_table_revisions (revision_id, name, rolls) VALUES (@revision_id, @name, @rolls);

-- name: InsertLootRevisionEntry :exec
INSERT INTO prep.loot_revision_entries (revision_id, ordering, weight, kind, item_slug, coin, amount, nested_table_id)
VALUES (@revision_id, @ordering, @weight, @kind, sqlc.narg(item_slug), sqlc.narg(coin), @amount, sqlc.narg(nested_table_id));

-- name: GetLootTableRevision :one
SELECT r.id, t.name, t.rolls
FROM campaign.revisions r JOIN prep.loot_table_revisions t ON t.revision_id = r.id
WHERE r.campaign_id = @campaign_id AND r.entity_type = 'loot_table' AND r.entity_id = @entity_id AND r.revision_no = @revision_no;

-- name: LootRevisionEntries :many
SELECT ordering, weight, kind, item_slug, coin, amount, nested_table_id FROM prep.loot_revision_entries WHERE revision_id = $1 ORDER BY ordering;

-- name: InventoryCharacters :many
SELECT c.id, c.name, c.owner_member_id, coalesce((SELECT a.base + a.bonus + a.increase FROM campaign.character_abilities a WHERE a.character_id = c.id AND a.ability = 'strength'), 10)::int AS strength,
    coalesce(nullif(ARRAY(SELECT x.class_slug FROM campaign.character_classes x WHERE x.character_id = c.id ORDER BY x.position), '{}'), ARRAY[c.class_slug])::text[] AS classes
FROM campaign.characters c WHERE c.campaign_id = $1 ORDER BY c.name, c.id;

-- name: CampaignContainers :many
SELECT id, kind, character_id, parent_id, label, created_at FROM campaign.containers WHERE campaign_id = $1 ORDER BY created_at, id;

-- name: CampaignItemInstances :many
SELECT i.id, i.container_id, i.item_slug, i.custom_name, i.quantity, i.charges, i.identified, i.attuned, i.equipped_slot
FROM campaign.item_instances i JOIN campaign.containers c ON c.id = i.container_id
WHERE c.campaign_id = $1 ORDER BY i.container_id, i.created_at, i.id;

-- name: CampaignContainerCoins :many
SELECT k.container_id, k.coin, k.amount
FROM campaign.container_coins k JOIN campaign.containers c ON c.id = k.container_id
WHERE c.campaign_id = $1 ORDER BY k.container_id, k.coin;

-- name: InsertContainer :exec
INSERT INTO campaign.containers (id, campaign_id, kind, character_id, label, created_at)
VALUES (@id, @campaign_id, @kind, sqlc.narg(character_id), @label, @now) ON CONFLICT DO NOTHING;

-- name: DeleteContainer :exec
DELETE FROM campaign.containers WHERE id = $1;

-- name: SetContainerCoins :exec
INSERT INTO campaign.container_coins (container_id, coin, amount) VALUES (@container_id, @coin, @amount)
ON CONFLICT (container_id, coin) DO UPDATE SET amount = excluded.amount;

-- name: DeleteContainerCoins :exec
DELETE FROM campaign.container_coins WHERE container_id = @container_id AND coin = @coin;

-- name: InsertItemEvent :exec
INSERT INTO play.action_item_events (action_id, position, from_label, to_label, item_slug, coin, count)
VALUES (@action_id, @position, @from_label, @to_label, sqlc.narg(item_slug), sqlc.narg(coin), @count);


-- name: SetStack :exec
INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, created_at)
VALUES (@id, @container_id, @item_slug, @quantity, true, false, @now)
ON CONFLICT (container_id, item_slug) WHERE custom_name IS NULL AND charges IS NULL AND equipped_slot IS NULL AND NOT attuned AND identified
DO UPDATE SET quantity = excluded.quantity;

-- name: DeleteStack :exec
DELETE FROM campaign.item_instances
WHERE container_id = @container_id AND item_slug = @item_slug
    AND custom_name IS NULL AND charges IS NULL AND equipped_slot IS NULL AND NOT attuned AND identified;

-- name: MoveInstance :exec
UPDATE campaign.item_instances SET container_id = @container_id, equipped_slot = NULL WHERE id = @id;

-- name: CampaignHasLiveSession :one
SELECT EXISTS (SELECT 1 FROM play.sessions WHERE campaign_id = $1 AND status = 'live')::boolean;

-- name: InsertInstance :exec
INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, equipped_slot, created_at)
VALUES (@id, @container_id, @item_slug, 1, true, @attuned, sqlc.narg(equipped_slot), @now);

-- name: DeleteInstance :exec
DELETE FROM campaign.item_instances WHERE id = $1;

-- name: SetInstanceSlot :exec
UPDATE campaign.item_instances SET equipped_slot = sqlc.narg(equipped_slot) WHERE id = @id;

-- name: HealCharacter :exec
UPDATE campaign.characters SET hp_current = LEAST(hp_max, hp_current + @amount) WHERE id = @id;

-- name: SetCharacterArmor :exec
UPDATE campaign.characters SET armor_slug = sqlc.narg(armor_slug), shield = @shield WHERE id = @id;

-- name: SetInstanceAttuned :exec
UPDATE campaign.item_instances SET attuned = @attuned WHERE id = @id;

-- name: IdentifyInstance :exec
UPDATE campaign.item_instances SET identified = true WHERE id = $1;

-- name: SetInstanceCharges :exec
UPDATE campaign.item_instances SET charges = @charges WHERE id = @id;
