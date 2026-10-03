-- name: ListFactions :many
SELECT id, campaign_id, name, archetype, goals, territory, notes, score, created_at, updated_at
FROM campaign.factions WHERE campaign_id = $1 ORDER BY name, id;

-- name: GetFaction :one
SELECT id, campaign_id, name, archetype, goals, territory, notes, score, created_at, updated_at
FROM campaign.factions WHERE campaign_id = @campaign_id AND id = @id;

-- name: InsertFaction :exec
INSERT INTO campaign.factions (id, campaign_id, name, archetype, goals, territory, notes, score, created_at, updated_at)
VALUES (@id, @campaign_id, @name, @archetype, @goals, @territory, @notes, 0, @now, @now);

-- name: UpdateFaction :execrows
UPDATE campaign.factions SET name = @name, archetype = @archetype, goals = @goals, territory = @territory, notes = @notes, updated_at = @now
WHERE campaign_id = @campaign_id AND id = @id;

-- name: DeleteFaction :execrows
DELETE FROM campaign.factions WHERE campaign_id = @campaign_id AND id = @id;

-- name: SetFactionScore :exec
UPDATE campaign.factions SET score = @score, updated_at = @now WHERE campaign_id = @campaign_id AND id = @id;

-- name: ListPersonalStandings :many
-- Every Personal Standing with a Faction of the Campaign, with whose it is.
SELECT p.faction_id, p.character_id, p.score, c.name, c.owner_member_id
FROM campaign.personal_standings p
JOIN campaign.factions f ON f.id = p.faction_id
JOIN campaign.characters c ON c.id = p.character_id AND c.campaign_id = f.campaign_id
WHERE f.campaign_id = $1 ORDER BY c.name, p.character_id;

-- name: PersonalStanding :many
SELECT score FROM campaign.personal_standings WHERE faction_id = @faction_id AND character_id = @character_id;

-- name: SetPersonalStanding :exec
-- Only a Character of the Faction's own Campaign carries a Personal Standing with it.
INSERT INTO campaign.personal_standings (faction_id, character_id, score)
SELECT f.id, c.id, @score FROM campaign.factions f JOIN campaign.characters c ON c.campaign_id = f.campaign_id
WHERE f.id = @faction_id AND c.id = @character_id AND f.campaign_id = @campaign_id
ON CONFLICT (faction_id, character_id) DO UPDATE SET score = excluded.score;

-- name: CampaignCharacterOwner :one
SELECT owner_member_id FROM campaign.characters WHERE campaign_id = @campaign_id AND id = @id;

-- name: ListStandingChanges :many
SELECT s.id, s.faction_id, s.character_id, s.delta, s.reason, s.share_reason, s.status, s.origin, s.client, s.proposed_by, s.created_at, s.decided_at
FROM campaign.standing_changes s JOIN campaign.factions f ON f.id = s.faction_id
WHERE f.campaign_id = $1 ORDER BY s.created_at DESC, s.id;

-- name: InsertStandingChange :exec
INSERT INTO campaign.standing_changes (id, faction_id, character_id, delta, reason, share_reason, status, origin, client, proposed_by, created_at)
VALUES (@id, @faction_id, sqlc.narg(character_id), @delta, @reason, @share_reason, 'pending', @origin, @client, @proposed_by, @now);

-- name: LockStandingChange :one
-- The change and its Faction, held for the DM's decision; only a change of this Campaign.
SELECT s.id, s.faction_id, s.character_id, s.delta, s.reason, s.share_reason, s.status, s.origin, s.client, s.proposed_by, s.created_at, s.decided_at
FROM campaign.standing_changes s JOIN campaign.factions f ON f.id = s.faction_id
WHERE f.campaign_id = @campaign_id AND s.id = @id FOR UPDATE OF s, f;

-- name: DecideStandingChange :exec
UPDATE campaign.standing_changes SET status = @status, delta = @delta, reason = @reason, share_reason = @share_reason, decided_at = @now
WHERE id = @id AND status = 'pending';
