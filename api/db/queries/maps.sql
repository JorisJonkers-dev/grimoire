-- name: InsertMap :one
INSERT INTO campaign.maps (campaign_id, name, image_key, image_type, width_px, height_px, hex_size_px, origin_x, origin_y, created_at, updated_at)
VALUES (@campaign_id, @name, @image_key, @image_type, @width_px, @height_px, @hex_size_px, @origin_x, @origin_y, @now, @now)
RETURNING id, campaign_id, name, image_key, image_type, width_px, height_px, hex_size_px, origin_x, origin_y, ambient, created_at, updated_at;

-- name: GetMap :one
SELECT id, campaign_id, name, image_key, image_type, width_px, height_px, hex_size_px, origin_x, origin_y, ambient, created_at, updated_at
FROM campaign.maps WHERE campaign_id = @campaign_id AND id = @id;

-- name: ListMaps :many
SELECT id, campaign_id, name, image_key, image_type, width_px, height_px, hex_size_px, origin_x, origin_y, ambient, created_at, updated_at
FROM campaign.maps WHERE campaign_id = $1 ORDER BY name, id;

-- name: UpdateMap :execrows
UPDATE campaign.maps SET name = @name, hex_size_px = @hex_size_px, origin_x = @origin_x, origin_y = @origin_y, ambient = @ambient,
    updated_at = @now
WHERE campaign_id = @campaign_id AND id = @id;

-- name: SetMapAmbient :exec
UPDATE campaign.maps SET ambient = @ambient, updated_at = @now WHERE id = @id;

-- name: MapWalls :many
SELECT q, r FROM campaign.map_walls WHERE map_id = $1;

-- name: AddWall :exec
INSERT INTO campaign.map_walls (map_id, q, r) VALUES (@map_id, @q, @r) ON CONFLICT DO NOTHING;

-- name: RemoveWall :exec
DELETE FROM campaign.map_walls WHERE map_id = @map_id AND q = @q AND r = @r;

-- name: MapLights :many
SELECT id, q, r, bright_ft, dim_ft FROM campaign.map_lights WHERE map_id = $1 ORDER BY id;

-- name: InsertLight :exec
INSERT INTO campaign.map_lights (id, map_id, q, r, bright_ft, dim_ft) VALUES (@id, @map_id, @q, @r, @bright_ft, @dim_ft);

-- name: DeleteLight :exec
DELETE FROM campaign.map_lights WHERE map_id = @map_id AND id = @id;

-- name: MapReveals :many
SELECT q, r FROM campaign.map_reveals WHERE map_id = $1;

-- name: AddReveal :exec
INSERT INTO campaign.map_reveals (map_id, q, r) VALUES (@map_id, @q, @r) ON CONFLICT DO NOTHING;

-- name: RemoveReveal :exec
DELETE FROM campaign.map_reveals WHERE map_id = @map_id AND q = @q AND r = @r;

-- name: SetSessionMap :exec
UPDATE play.sessions SET map_id = sqlc.narg(map_id) WHERE id = @id;

-- name: InsertHexEvent :exec
INSERT INTO play.action_hex_events (action_id, q, r) VALUES (@action_id, @q, @r) ON CONFLICT DO NOTHING;
