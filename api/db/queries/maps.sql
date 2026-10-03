-- name: InsertMap :one
INSERT INTO campaign.maps (campaign_id, name, kind, image_key, image_type, width_px, height_px, hex_size_px, origin_x, origin_y, grid_kind,
    grid_strength, scale_miles, created_at, updated_at)
VALUES (@campaign_id, @name, @kind, @image_key, @image_type, @width_px, @height_px, @hex_size_px, @origin_x, @origin_y, @grid_kind,
    @grid_strength, @scale_miles, @now, @now)
RETURNING id, campaign_id, name, image_key, image_type, width_px, height_px, hex_size_px, origin_x, origin_y, ambient, created_at, updated_at, kind, grid_kind, grid_strength, scale_miles;

-- name: GetMap :one
SELECT id, campaign_id, name, image_key, image_type, width_px, height_px, hex_size_px, origin_x, origin_y, ambient, created_at, updated_at, kind, grid_kind, grid_strength, scale_miles
FROM campaign.maps WHERE campaign_id = @campaign_id AND id = @id;

-- name: ListMaps :many
SELECT id, campaign_id, name, image_key, image_type, width_px, height_px, hex_size_px, origin_x, origin_y, ambient, created_at, updated_at, kind, grid_kind, grid_strength, scale_miles
FROM campaign.maps WHERE campaign_id = $1 ORDER BY name, id;

-- name: UpdateMap :execrows
UPDATE campaign.maps SET name = @name, hex_size_px = @hex_size_px, origin_x = @origin_x, origin_y = @origin_y, ambient = @ambient,
    grid_kind = @grid_kind, grid_strength = @grid_strength, scale_miles = @scale_miles, updated_at = @now
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

-- name: SetSessionWorld :exec
UPDATE play.sessions SET world_map_id = sqlc.narg(map_id) WHERE id = @id;

-- name: MapNodes :many
SELECT id, name, q, r FROM campaign.map_nodes WHERE map_id = $1 ORDER BY name, id;

-- name: InsertNode :exec
INSERT INTO campaign.map_nodes (id, map_id, name, q, r) VALUES (@id, @map_id, @name, @q, @r);

-- name: DeleteNode :exec
DELETE FROM campaign.map_nodes WHERE map_id = @map_id AND id = @id;

-- name: MapEdges :many
SELECT id, from_node_id, to_node_id, distance_mi FROM campaign.map_edges WHERE map_id = $1 ORDER BY id;

-- name: InsertEdge :exec
INSERT INTO campaign.map_edges (id, map_id, from_node_id, to_node_id, distance_mi) VALUES (@id, @map_id, @from_node_id, @to_node_id, @distance_mi);

-- name: DeleteEdge :exec
DELETE FROM campaign.map_edges WHERE map_id = @map_id AND id = @id;

-- name: MapParty :many
SELECT node_id FROM campaign.map_parties WHERE map_id = $1;

-- name: SetMapParty :exec
INSERT INTO campaign.map_parties (map_id, node_id) VALUES (@map_id, @node_id)
ON CONFLICT (map_id) DO UPDATE SET node_id = excluded.node_id;

-- name: InsertTravelLeg :exec
INSERT INTO play.travel_legs (action_id, session_id, map_id, from_name, to_name, pace, distance_mi, minutes, days)
VALUES (@action_id, @session_id, @map_id, @from_name, @to_name, @pace, @distance_mi, @minutes, @days);

-- name: SessionTravelLegs :many
SELECT l.from_name, l.to_name, l.pace, l.distance_mi, l.minutes, l.days
FROM play.travel_legs l JOIN play.actions a ON a.id = l.action_id
WHERE l.session_id = @session_id AND l.map_id = @map_id ORDER BY a.seq;
