-- name: InsertSnapshot :exec
INSERT INTO campaign.character_snapshots (id, character_id, species_slug, background_slug, ability_method, created_at)
VALUES (@id, @character_id, @species_slug, @background_slug, @ability_method, @now);

-- name: InsertSnapshotAbility :exec
INSERT INTO campaign.character_snapshot_abilities (snapshot_id, ability, base, bonus, increase) VALUES (@snapshot_id, @ability, @base, @bonus, @increase);

-- name: InsertSnapshotSkill :exec
INSERT INTO campaign.character_snapshot_skills (snapshot_id, skill) VALUES (@snapshot_id, @skill);

-- name: InsertSnapshotPick :exec
INSERT INTO campaign.character_snapshot_picks (snapshot_id, level, choice, value) VALUES (@snapshot_id, @level, @choice, @value);

-- name: GetSnapshot :one
SELECT id, species_slug, background_slug, ability_method, created_at FROM campaign.character_snapshots WHERE id = $1;

-- name: SnapshotAbilities :many
SELECT ability, base, bonus, increase FROM campaign.character_snapshot_abilities WHERE snapshot_id = $1;

-- name: SnapshotSkills :many
SELECT skill FROM campaign.character_snapshot_skills WHERE snapshot_id = $1 ORDER BY skill;

-- name: SnapshotPicks :many
SELECT level, choice, value FROM campaign.character_snapshot_picks WHERE snapshot_id = $1 ORDER BY level, choice, value;

-- name: InsertRetrain :exec
INSERT INTO campaign.retrains (id, campaign_id, character_id, proposed_id, status, reason, requested_by, created_at)
VALUES (@id, @campaign_id, @character_id, @proposed_id, 'pending', @reason, @requested_by, @now);

-- name: ListRetrains :many
SELECT id, character_id, proposed_id, status, reason, requested_by, decided_by, created_at, decided_at
FROM campaign.retrains WHERE campaign_id = @campaign_id AND character_id = @character_id ORDER BY created_at DESC, id;

-- name: GetRetrain :one
SELECT id, character_id, proposed_id, status, reason, requested_by, decided_by, created_at, decided_at
FROM campaign.retrains WHERE campaign_id = @campaign_id AND id = @id;

-- name: DecideRetrain :execrows
UPDATE campaign.retrains SET status = @status, decided_by = @decided_by, decided_at = @now WHERE id = @id AND status = 'pending';

-- name: InsertCharacterRevision :exec
INSERT INTO campaign.character_revisions (revision_id, snapshot_id, retrain_id) VALUES (@revision_id, @snapshot_id, sqlc.narg(retrain_id));

-- name: CharacterRevisions :many
SELECT r.revision_no, r.caller_name, r.created_at, cr.snapshot_id, cr.retrain_id
FROM campaign.revisions r JOIN campaign.character_revisions cr ON cr.revision_id = r.id
WHERE r.campaign_id = @campaign_id AND r.entity_type = 'character' AND r.entity_id = @entity_id
ORDER BY r.revision_no DESC;

-- name: RetrainCharacter :exec
UPDATE campaign.characters
SET species_slug = @species_slug, background_slug = @background_slug, ability_method = @ability_method,
    hp_max = @hp_max, hp_current = LEAST(hp_current, @hp_max), updated_at = @now
WHERE campaign_id = @campaign_id AND id = @id;

-- name: ClearCharacterAbilities :exec
DELETE FROM campaign.character_abilities WHERE character_id = $1;

-- name: ClearCharacterSkills :exec
DELETE FROM campaign.character_skills WHERE character_id = $1;

-- name: ClearCharacterPicks :exec
DELETE FROM campaign.character_picks WHERE character_id = $1;
