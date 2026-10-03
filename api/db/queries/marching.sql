-- name: MarchingOrder :many
SELECT character_id FROM campaign.marching_order WHERE campaign_id = $1 ORDER BY position;

-- name: ClearMarchingOrder :exec
DELETE FROM campaign.marching_order WHERE campaign_id = $1;

-- name: InsertMarcher :exec
-- Only a Character of the Campaign takes a place in its Marching Order.
INSERT INTO campaign.marching_order (campaign_id, position, character_id)
SELECT c.campaign_id, @position, c.id FROM campaign.characters c WHERE c.id = @character_id AND c.campaign_id = @campaign_id;
