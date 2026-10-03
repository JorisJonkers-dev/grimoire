-- name: MonsterXP :one
SELECT m.xp, m.name
FROM compendium.monsters m
JOIN compendium.documents d ON d.id = m.document_id
WHERE m.slug = @slug
ORDER BY (d.key = (SELECT c.ruleset_pref FROM campaign.campaigns c WHERE c.id = @campaign_id)) DESC, d.precedence DESC
LIMIT 1;

-- name: CampaignNode :one
SELECT n.id, n.name
FROM campaign.map_nodes n JOIN campaign.maps m ON m.id = n.map_id
WHERE m.campaign_id = @campaign_id AND n.id = @id;

-- name: CampaignLocations :many
SELECT n.id, n.name, m.name AS map_name
FROM campaign.map_nodes n JOIN campaign.maps m ON m.id = n.map_id
WHERE m.campaign_id = $1 AND m.kind = 'world'
ORDER BY m.name, n.name, n.id;

-- name: PartyLevels :many
SELECT level FROM campaign.characters WHERE campaign_id = $1 ORDER BY id;

-- name: ListPools :many
SELECT id, name, level_min, level_max, difficulty, updated_at FROM prep.encounter_pools WHERE campaign_id = $1 ORDER BY name, id;

-- name: CampaignPoolMembers :many
SELECT m.pool_id, m.ordering, m.monster_slug, m.weight, m.min_count, m.max_count
FROM prep.pool_members m JOIN prep.encounter_pools p ON p.id = m.pool_id
WHERE p.campaign_id = $1 ORDER BY m.pool_id, m.ordering;

-- name: SavePool :exec
INSERT INTO prep.encounter_pools (id, campaign_id, name, level_min, level_max, difficulty, updated_at)
VALUES (@id, @campaign_id, @name, @level_min, @level_max, @difficulty, @now)
ON CONFLICT (id) DO UPDATE SET name = excluded.name, level_min = excluded.level_min, level_max = excluded.level_max,
    difficulty = excluded.difficulty, updated_at = excluded.updated_at;

-- name: ClearPoolMembers :exec
DELETE FROM prep.pool_members WHERE pool_id = $1;

-- name: InsertPoolMember :exec
INSERT INTO prep.pool_members (pool_id, ordering, monster_slug, weight, min_count, max_count)
VALUES (@pool_id, @ordering, @monster_slug, @weight, @min_count, @max_count);

-- name: DeletePool :execrows
DELETE FROM prep.encounter_pools WHERE campaign_id = @campaign_id AND id = @id;

-- name: PoolInUse :one
SELECT count(*)::int FROM prep.table_entries WHERE pool_id = $1;

-- name: ListTables :many
SELECT id, name, region_node_id, chance_pct, visibility, updated_at FROM prep.encounter_tables WHERE campaign_id = $1 ORDER BY name, id;

-- name: CampaignTableEntries :many
SELECT e.table_id, e.ordering, e.weight, e.kind, e.label, e.pool_id, e.faction_id
FROM prep.table_entries e JOIN prep.encounter_tables t ON t.id = e.table_id
WHERE t.campaign_id = $1 ORDER BY e.table_id, e.ordering;

-- name: CampaignEntryMonsters :many
SELECT m.table_id, m.ordering, m.position, m.monster_slug, m.count
FROM prep.entry_monsters m JOIN prep.encounter_tables t ON t.id = m.table_id
WHERE t.campaign_id = $1 ORDER BY m.table_id, m.ordering, m.position;

-- name: SaveEncounterTable :exec
INSERT INTO prep.encounter_tables (id, campaign_id, name, region_node_id, chance_pct, visibility, updated_at)
VALUES (@id, @campaign_id, @name, sqlc.narg(region_node_id), @chance_pct, @visibility, @now)
ON CONFLICT (id) DO UPDATE SET name = excluded.name, region_node_id = excluded.region_node_id, chance_pct = excluded.chance_pct,
    visibility = excluded.visibility, updated_at = excluded.updated_at;

-- name: ClearTableEntries :exec
DELETE FROM prep.table_entries WHERE table_id = $1;

-- name: InsertTableEntry :exec
INSERT INTO prep.table_entries (table_id, ordering, weight, kind, label, pool_id, faction_id)
VALUES (@table_id, @ordering, @weight, @kind, @label, sqlc.narg(pool_id), sqlc.narg(faction_id));

-- name: InsertEntryMonster :exec
INSERT INTO prep.entry_monsters (table_id, ordering, position, monster_slug, count)
VALUES (@table_id, @ordering, @position, @monster_slug, @count);

-- name: DeleteEncounterTable :execrows
DELETE FROM prep.encounter_tables WHERE campaign_id = @campaign_id AND id = @id;

-- name: InsertPoolRevision :exec
INSERT INTO prep.pool_revisions (revision_id, name, level_min, level_max, difficulty)
VALUES (@revision_id, @name, @level_min, @level_max, @difficulty);

-- name: InsertPoolRevisionMember :exec
INSERT INTO prep.pool_revision_members (revision_id, ordering, monster_slug, weight, min_count, max_count)
VALUES (@revision_id, @ordering, @monster_slug, @weight, @min_count, @max_count);

-- name: GetPoolRevision :one
SELECT r.id, p.name, p.level_min, p.level_max, p.difficulty
FROM campaign.revisions r JOIN prep.pool_revisions p ON p.revision_id = r.id
WHERE r.campaign_id = @campaign_id AND r.entity_type = 'encounter_pool' AND r.entity_id = @entity_id AND r.revision_no = @revision_no;

-- name: PoolRevisionMembers :many
SELECT ordering, monster_slug, weight, min_count, max_count FROM prep.pool_revision_members WHERE revision_id = $1 ORDER BY ordering;

-- name: InsertTableRevision :exec
INSERT INTO prep.table_revisions (revision_id, name, region_node_id, chance_pct, visibility)
VALUES (@revision_id, @name, sqlc.narg(region_node_id), @chance_pct, @visibility);

-- name: InsertTableRevisionEntry :exec
INSERT INTO prep.table_revision_entries (revision_id, ordering, weight, kind, label, pool_id, faction_id)
VALUES (@revision_id, @ordering, @weight, @kind, @label, sqlc.narg(pool_id), sqlc.narg(faction_id));

-- name: InsertTableRevisionMonster :exec
INSERT INTO prep.table_revision_monsters (revision_id, ordering, position, monster_slug, count)
VALUES (@revision_id, @ordering, @position, @monster_slug, @count);

-- name: GetTableRevision :one
SELECT r.id, t.name, t.region_node_id, t.chance_pct, t.visibility
FROM campaign.revisions r JOIN prep.table_revisions t ON t.revision_id = r.id
WHERE r.campaign_id = @campaign_id AND r.entity_type = 'encounter_table' AND r.entity_id = @entity_id AND r.revision_no = @revision_no;

-- name: TableRevisionEntries :many
SELECT ordering, weight, kind, label, pool_id, faction_id FROM prep.table_revision_entries WHERE revision_id = $1 ORDER BY ordering;

-- name: TableRevisionMonsters :many
SELECT ordering, position, monster_slug, count FROM prep.table_revision_monsters WHERE revision_id = $1 ORDER BY ordering, position;

-- name: ScheduledChecks :many
SELECT s.id, s.table_id, s.due FROM prep.scheduled_checks s WHERE s.campaign_id = $1 ORDER BY s.created_at, s.id;

-- name: InsertScheduledCheck :exec
INSERT INTO prep.scheduled_checks (id, campaign_id, table_id, due, created_at) VALUES (@id, @campaign_id, @table_id, @due, @now);

-- name: DeleteScheduledCheck :exec
DELETE FROM prep.scheduled_checks WHERE id = $1;

-- name: SaveEncounterCheck :exec
INSERT INTO prep.encounter_checks (id, campaign_id, session_id, table_id, table_name, trigger, mode, visibility, seed, chance_pct,
    chance_roll, roll_id, status, outcome, entry_label, created_at)
VALUES (@id, @campaign_id, sqlc.narg(session_id), sqlc.narg(table_id), @table_name, @trigger, @mode, @visibility, @seed, @chance_pct,
    sqlc.narg(chance_roll), sqlc.narg(roll_id), @status, sqlc.narg(outcome), @entry_label, @now)
ON CONFLICT (id) DO UPDATE SET chance_roll = excluded.chance_roll, status = excluded.status, outcome = excluded.outcome,
    entry_label = excluded.entry_label;

-- name: InsertCheckMonster :exec
INSERT INTO prep.check_monsters (check_id, position, monster_slug, count) VALUES (@check_id, @position, @monster_slug, @count);

-- name: CampaignChecks :many
SELECT id, session_id, table_id, table_name, trigger, mode, visibility, seed, chance_pct, chance_roll, roll_id, status, outcome,
    entry_label, created_at
FROM prep.encounter_checks WHERE campaign_id = $1 ORDER BY created_at DESC, id LIMIT 100;

-- name: SessionChecks :many
SELECT id, session_id, table_id, table_name, trigger, mode, visibility, seed, chance_pct, chance_roll, roll_id, status, outcome,
    entry_label, created_at
FROM prep.encounter_checks WHERE session_id = $1 ORDER BY created_at, id;

-- name: CheckMonsters :many
SELECT c.check_id, c.position, c.monster_slug, c.count
FROM prep.check_monsters c JOIN prep.encounter_checks e ON e.id = c.check_id
WHERE e.campaign_id = $1 ORDER BY c.check_id, c.position;
