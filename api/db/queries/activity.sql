-- name: Edits :many
SELECT r.id, r.entity_type, r.entity_id, r.revision_no, r.action, r.caller_name, r.origin, r.client, r.restored_from, r.created_at,
    coalesce(n.name, p.name, t.name, l.name, st.name, sh.name, ch.name, '')::text AS name,
    r.revision_no = (SELECT max(x.revision_no) FROM campaign.revisions x WHERE x.entity_type = r.entity_type AND x.entity_id = r.entity_id) AS latest
FROM campaign.revisions r
LEFT JOIN campaign.npc_revisions n ON n.revision_id = r.id
LEFT JOIN prep.pool_revisions p ON p.revision_id = r.id
LEFT JOIN prep.table_revisions t ON t.revision_id = r.id
LEFT JOIN prep.loot_table_revisions l ON l.revision_id = r.id
LEFT JOIN prep.settlement_revisions st ON st.revision_id = r.id
LEFT JOIN prep.shop_revisions sh ON sh.revision_id = r.id
LEFT JOIN campaign.characters ch ON r.entity_type = 'character' AND ch.id = r.entity_id
WHERE r.campaign_id = @campaign_id
    AND (sqlc.narg(origin)::text IS NULL OR r.origin = sqlc.narg(origin))
    AND (sqlc.narg(id)::uuid IS NULL OR r.id = sqlc.narg(id))
    AND (sqlc.narg(entity_type)::text IS NULL OR r.entity_type = sqlc.narg(entity_type))
    AND (sqlc.narg(entity_id)::uuid IS NULL OR r.entity_id = sqlc.narg(entity_id))
ORDER BY r.created_at DESC, r.revision_no DESC
LIMIT @lim;
