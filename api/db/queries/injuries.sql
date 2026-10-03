-- name: CharacterInjuries :many
SELECT character_id, slug, name, cure, level FROM campaign.character_injuries WHERE character_id = $1 ORDER BY created_at, slug;

-- name: UpsertCharacterInjury :exec
INSERT INTO campaign.character_injuries (character_id, slug, name, cure, level, created_at)
VALUES (@character_id, @slug, @name, @cure, @level, @now)
ON CONFLICT (character_id, slug) DO UPDATE SET name = excluded.name, cure = excluded.cure, level = excluded.level;

-- name: DeleteCharacterInjury :exec
DELETE FROM campaign.character_injuries WHERE character_id = @character_id AND slug = @slug;
