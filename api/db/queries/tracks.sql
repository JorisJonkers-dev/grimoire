-- name: ListTracks :many
SELECT id, campaign_id, name, scope, lowest, highest, start, created_at FROM campaign.tracks WHERE campaign_id = $1 ORDER BY created_at, id;

-- name: ListTrackThresholds :many
SELECT th.track_id, th.at, th.rising, th.label, th.effect, th.roll_table
FROM campaign.track_thresholds th JOIN campaign.tracks t ON t.id = th.track_id
WHERE t.campaign_id = $1 ORDER BY th.track_id, th.position;

-- name: ListTrackValues :many
SELECT v.track_id, v.character_id, v.value
FROM campaign.track_values v JOIN campaign.tracks t ON t.id = v.track_id
WHERE t.campaign_id = $1;

-- name: TrackCharacters :many
-- The Characters of a Campaign a Track can be kept for, with who owns each.
SELECT id, name, owner_member_id FROM campaign.characters WHERE campaign_id = $1 ORDER BY name, id;

-- name: InsertTrack :exec
INSERT INTO campaign.tracks (id, campaign_id, name, scope, lowest, highest, start, created_at)
VALUES (@id, @campaign_id, @name, @scope, @lowest, @highest, @start, @now);

-- name: InsertTrackThreshold :exec
INSERT INTO campaign.track_thresholds (track_id, position, at, rising, label, effect, roll_table)
VALUES (@track_id, @position, @at, @rising, @label, sqlc.narg(effect), sqlc.narg(roll_table));

-- name: DeleteTrack :execrows
DELETE FROM campaign.tracks WHERE campaign_id = @campaign_id AND id = @id;

-- name: SetCharacterTrackValue :exec
INSERT INTO campaign.track_values (track_id, character_id, value) VALUES (@track_id, @character_id, @value)
ON CONFLICT (track_id, character_id) WHERE character_id IS NOT NULL DO UPDATE SET value = excluded.value;

-- name: SetPartyTrackValue :exec
INSERT INTO campaign.track_values (track_id, character_id, value) VALUES (@track_id, NULL, @value)
ON CONFLICT (track_id) WHERE character_id IS NULL DO UPDATE SET value = excluded.value;
