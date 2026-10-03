-- name: ListVehicles :many
SELECT id, name, kind, hull_hp, hull_max, threshold, miles_per_day
FROM campaign.vehicles WHERE campaign_id = $1 ORDER BY created_at, id;

-- name: ListVehicleComponents :many
SELECT c.id, c.vehicle_id, c.name, c.hp, c.hp_max, c.drives
FROM campaign.vehicle_components c JOIN campaign.vehicles v ON v.id = c.vehicle_id
WHERE v.campaign_id = $1 ORDER BY c.vehicle_id, c.position;

-- name: ListVehicleStations :many
SELECT s.id, s.vehicle_id, s.name, s.crew, s.posted
FROM campaign.vehicle_stations s JOIN campaign.vehicles v ON v.id = s.vehicle_id
WHERE v.campaign_id = $1 ORDER BY s.vehicle_id, s.position;

-- name: InsertVehicle :exec
INSERT INTO campaign.vehicles (id, campaign_id, name, kind, hull_hp, hull_max, threshold, miles_per_day, created_at)
VALUES (@id, @campaign_id, @name, @kind, @hull_max, @hull_max, @threshold, @miles_per_day, @now);

-- name: InsertVehicleComponent :exec
INSERT INTO campaign.vehicle_components (id, vehicle_id, position, name, hp, hp_max, drives)
VALUES (@id, @vehicle_id, @position, @name, @hp_max, @hp_max, @drives);

-- name: InsertVehicleStation :exec
INSERT INTO campaign.vehicle_stations (id, vehicle_id, position, name, crew, posted)
VALUES (@id, @vehicle_id, @position, @name, @crew, 0);

-- name: DeleteVehicle :execrows
DELETE FROM campaign.vehicles WHERE campaign_id = @campaign_id AND id = @id;

-- name: SetVehicleHull :exec
UPDATE campaign.vehicles SET hull_hp = @hull_hp WHERE campaign_id = @campaign_id AND id = @id;

-- name: SetVehicleComponentHP :exec
UPDATE campaign.vehicle_components SET hp = @hp WHERE vehicle_id = @vehicle_id AND id = @id;

-- name: SetVehicleStationPosted :exec
UPDATE campaign.vehicle_stations SET posted = @posted WHERE vehicle_id = @vehicle_id AND id = @id;
