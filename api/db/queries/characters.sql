-- name: InsertCharacter :one
INSERT INTO campaign.characters (campaign_id, owner_member_id, name, ruleset, species_slug, class_slug, background_slug,
    ability_method, hp_max, hp_current, armor_slug, shield, created_at, updated_at)
VALUES (@campaign_id, @owner_member_id, @name, @ruleset, @species_slug, @class_slug, @background_slug,
    @ability_method, @hp_max, @hp_max, sqlc.narg(armor_slug), @shield, @now, @now)
RETURNING id;

-- name: SetCharacterAbility :exec
INSERT INTO campaign.character_abilities (character_id, ability, base, bonus) VALUES (@character_id, @ability, @base, @bonus);

-- name: AddCharacterSkill :exec
INSERT INTO campaign.character_skills (character_id, skill, source) VALUES (@character_id, @skill, @source);

-- name: ClearCharacterWeapons :exec
DELETE FROM campaign.character_weapons WHERE character_id = $1;

-- name: AddCharacterWeapon :exec
INSERT INTO campaign.character_weapons (character_id, weapon_slug, ordering) VALUES (@character_id, @weapon_slug, @ordering);

-- name: GetCharacter :one
SELECT c.id, c.campaign_id, c.owner_member_id, m.display_name AS owner_name, m.auth_subject AS owner_subject, c.name,
       c.ruleset, c.species_slug, c.class_slug, c.background_slug, c.level, c.ability_method, c.hp_max, c.hp_current,
       c.armor_slug, c.shield, c.created_at, c.updated_at
FROM campaign.characters c JOIN campaign.members m ON m.id = c.owner_member_id
WHERE c.campaign_id = @campaign_id AND c.id = @id;

-- name: ListCharacters :many
SELECT c.id, c.owner_member_id, m.display_name AS owner_name, m.auth_subject AS owner_subject, c.name, c.ruleset,
       c.species_slug, c.class_slug, c.level, c.hp_max, c.hp_current
FROM campaign.characters c JOIN campaign.members m ON m.id = c.owner_member_id
WHERE c.campaign_id = $1
ORDER BY c.name, c.id;

-- name: CharacterAbilities :many
SELECT ability, base, bonus FROM campaign.character_abilities WHERE character_id = $1;

-- name: CharacterSkills :many
SELECT skill, source FROM campaign.character_skills WHERE character_id = $1 ORDER BY skill;

-- name: CharacterWeapons :many
SELECT weapon_slug FROM campaign.character_weapons WHERE character_id = $1 ORDER BY ordering;

-- name: UpdateCharacter :exec
UPDATE campaign.characters
SET name = @name, hp_current = @hp_current, armor_slug = sqlc.narg(armor_slug), shield = @shield, updated_at = @now
WHERE campaign_id = @campaign_id AND id = @id;

-- name: DeleteCharacter :exec
DELETE FROM campaign.characters WHERE campaign_id = @campaign_id AND id = @id;
