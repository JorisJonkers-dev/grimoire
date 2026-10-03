-- name: InsertCharacter :one
INSERT INTO campaign.characters (campaign_id, owner_member_id, character_id, name, ruleset, species_slug, class_slug, background_slug,
    ability_method, hp_max, hp_current, armor_slug, shield, created_at, updated_at)
VALUES (@campaign_id, @owner_member_id, @character_id, @name, @ruleset, @species_slug, @class_slug, @background_slug,
    @ability_method, @hp_max, @hp_max, sqlc.narg(armor_slug), @shield, @now, @now)
RETURNING id;

-- name: InsertAccountCharacter :exec
INSERT INTO campaign.account_characters (id, owner_subject, name, ruleset, species_slug, class_slug, background_slug, appearance, backstory, created_at, updated_at)
VALUES (@id, @owner_subject, @name, @ruleset, @species_slug, @class_slug, @background_slug, @appearance, @backstory, @now, @now);

-- name: SetCharacterAbility :exec
INSERT INTO campaign.character_abilities (character_id, ability, base, bonus) VALUES (@character_id, @ability, @base, @bonus);

-- name: AddCharacterSkill :exec
INSERT INTO campaign.character_skills (character_id, skill, source) VALUES (@character_id, @skill, @source);

-- name: ClearCharacterWeapons :exec
DELETE FROM campaign.character_weapons WHERE character_id = $1;

-- name: AddCharacterWeapon :exec
INSERT INTO campaign.character_weapons (character_id, weapon_slug, ordering) VALUES (@character_id, @weapon_slug, @ordering);

-- name: GetCharacter :one
SELECT c.id, c.campaign_id, c.character_id, c.owner_member_id, m.display_name AS owner_name, m.auth_subject AS owner_subject, c.name,
       c.ruleset, c.species_slug, c.class_slug, c.background_slug, c.level, c.ability_method, c.hp_max, c.hp_current,
       c.armor_slug, c.shield, c.created_at, c.updated_at, c.portrait_key, c.portrait_type, c.token_key, c.token_type, c.temp_hp, c.level_up_ready, c.can_prepare, c.heroic_inspiration
FROM campaign.characters c JOIN campaign.members m ON m.id = c.owner_member_id
WHERE c.campaign_id = @campaign_id AND c.id = @id;

-- name: ListCharacters :many
SELECT c.id, c.character_id, c.owner_member_id, m.display_name AS owner_name, m.auth_subject AS owner_subject, c.name, c.ruleset,
       c.species_slug, c.class_slug, c.level, c.hp_max, c.hp_current, c.token_key, c.heroic_inspiration
FROM campaign.characters c JOIN campaign.members m ON m.id = c.owner_member_id
WHERE c.campaign_id = $1
ORDER BY c.name, c.id;

-- name: CharacterAbilities :many
SELECT ability, base, bonus, increase FROM campaign.character_abilities WHERE character_id = $1;

-- name: CharacterSkills :many
SELECT skill, source FROM campaign.character_skills WHERE character_id = $1 ORDER BY skill;

-- name: CharacterWeapons :many
SELECT weapon_slug FROM campaign.character_weapons WHERE character_id = $1 ORDER BY ordering;

-- name: UpdateCharacter :exec
UPDATE campaign.characters
SET name = @name, hp_current = @hp_current, temp_hp = @temp_hp, armor_slug = sqlc.narg(armor_slug), shield = @shield, updated_at = @now
WHERE campaign_id = @campaign_id AND id = @id;

-- name: DeleteCharacter :exec
DELETE FROM campaign.characters WHERE campaign_id = @campaign_id AND id = @id;

-- name: SetCharacterPortrait :exec
UPDATE campaign.characters SET portrait_key = sqlc.narg(key), portrait_type = sqlc.narg(content_type), updated_at = @now
WHERE campaign_id = @campaign_id AND id = @id;

-- name: SetCharacterToken :exec
UPDATE campaign.characters SET token_key = sqlc.narg(key), token_type = sqlc.narg(content_type), updated_at = @now
WHERE campaign_id = @campaign_id AND id = @id;

-- name: FlowCharacterIdentity :exec
-- A Campaign Character's name, Portrait and token become its Character's, and every other Campaign's.
WITH source AS (
    SELECT character_id, name, portrait_key, portrait_type, token_key, token_type FROM campaign.characters WHERE id = @id
), owned AS (
    UPDATE campaign.account_characters a
    SET name = source.name, portrait_key = source.portrait_key, portrait_type = source.portrait_type,
        token_key = source.token_key, token_type = source.token_type, updated_at = @now
    FROM source WHERE a.id = source.character_id
)
UPDATE campaign.characters c
SET name = source.name, portrait_key = source.portrait_key, portrait_type = source.portrait_type,
    token_key = source.token_key, token_type = source.token_type
FROM source WHERE c.character_id = source.character_id AND c.id <> @id;

-- name: ListAccountCharacters :many
SELECT id, name, ruleset, species_slug, class_slug, background_slug, backstory, portrait_key, created_at, updated_at
FROM campaign.account_characters WHERE owner_subject = @owner_subject ORDER BY lower(name), id;

-- name: GetAccountCharacter :one
SELECT id, owner_subject, name, ruleset, species_slug, class_slug, background_slug, backstory, portrait_key, created_at, updated_at
FROM campaign.account_characters WHERE id = @id;

-- name: AccountCharacterCampaigns :many
SELECT c.character_id, c.id, c.campaign_id, cp.name AS campaign_name, c.level, c.hp_current, c.hp_max, c.updated_at
FROM campaign.characters c JOIN campaign.campaigns cp ON cp.id = c.campaign_id
WHERE c.character_id = ANY(@ids::uuid[]) ORDER BY c.updated_at DESC, c.id;

-- name: UpdateAccountCharacter :exec
UPDATE campaign.account_characters SET name = @name, backstory = @backstory, updated_at = @now WHERE id = @id;

-- name: RenameCampaignCharacters :exec
UPDATE campaign.characters SET name = @name WHERE character_id = @character_id;

-- name: AdoptCharacterIdentity :exec
-- A new Campaign Character takes its Character's name, Portrait and token.
UPDATE campaign.characters c
SET name = a.name, portrait_key = a.portrait_key, portrait_type = a.portrait_type, token_key = a.token_key, token_type = a.token_type
FROM campaign.account_characters a WHERE c.id = @id AND a.id = c.character_id;

-- name: GetCharacterDraft :one
SELECT step, build, rolled, updated_at FROM campaign.character_drafts WHERE campaign_id = @campaign_id AND owner_subject = @owner_subject;

-- name: SaveCharacterDraft :exec
INSERT INTO campaign.character_drafts (campaign_id, owner_subject, step, build, updated_at) VALUES (@campaign_id, @owner_subject, @step, @build, @now)
ON CONFLICT (campaign_id, owner_subject) DO UPDATE SET step = EXCLUDED.step, build = EXCLUDED.build, updated_at = EXCLUDED.updated_at;

-- name: RollCharacterDraft :execrows
-- Rolled scores are set once per draft; a draft is started if there is none.
INSERT INTO campaign.character_drafts (campaign_id, owner_subject, step, build, rolled, updated_at) VALUES (@campaign_id, @owner_subject, 3, '{}', @rolled, @now)
ON CONFLICT (campaign_id, owner_subject) DO UPDATE SET rolled = EXCLUDED.rolled, updated_at = EXCLUDED.updated_at
WHERE campaign.character_drafts.rolled IS NULL;

-- name: DeleteCharacterDraft :exec
DELETE FROM campaign.character_drafts WHERE campaign_id = @campaign_id AND owner_subject = @owner_subject;

-- name: CharacterClasses :many
SELECT class_slug, subclass_slug, level FROM campaign.character_classes WHERE character_id = $1 ORDER BY position;

-- name: CharacterPicks :many
SELECT level, choice, value FROM campaign.character_picks WHERE character_id = $1 ORDER BY level, choice, value;

-- name: CharacterSpells :many
SELECT class_slug, spell_slug, learned_level, prepared, spellbook FROM campaign.character_spells WHERE character_id = $1 ORDER BY class_slug, spell_slug;

-- name: ClearCharacterClasses :exec
DELETE FROM campaign.character_classes WHERE character_id = $1;

-- name: InsertCharacterClass :exec
INSERT INTO campaign.character_classes (character_id, class_slug, subclass_slug, level, position)
VALUES (@character_id, @class_slug, sqlc.narg(subclass_slug), @level, @position);

-- name: InsertCharacterPick :exec
INSERT INTO campaign.character_picks (character_id, level, choice, value) VALUES (@character_id, @level, @choice, @value);

-- name: InsertCharacterSpell :exec
INSERT INTO campaign.character_spells (character_id, class_slug, spell_slug, learned_level, prepared, spellbook)
VALUES (@character_id, @class_slug, @spell_slug, @learned_level, @prepared, @spellbook);

-- name: SetAbilityIncrease :exec
UPDATE campaign.character_abilities SET increase = @increase WHERE character_id = @character_id AND ability = @ability;

-- name: LevelUpCharacter :execrows
-- Takes the next level once, only while it is unlocked.
UPDATE campaign.characters
SET level = level + 1, hp_max = hp_max + @gain, hp_current = hp_current + @gain, level_up_ready = false, can_prepare = true, updated_at = @now
WHERE campaign_id = @campaign_id AND id = @id AND level_up_ready AND level = @level;

-- name: SetLevelUpReady :exec
UPDATE campaign.characters SET level_up_ready = @ready AND level < 20, updated_at = @now WHERE campaign_id = @campaign_id AND id = @id;

-- name: ClearClassSpells :exec
DELETE FROM campaign.character_spells WHERE character_id = @character_id AND class_slug = @class_slug;

-- name: SetCanPrepare :exec
UPDATE campaign.characters SET can_prepare = @can_prepare WHERE id = @id;

-- name: CampaignClock :one
SELECT game_day, game_minute FROM campaign.campaigns WHERE id = $1;

-- name: SetCampaignClock :exec
UPDATE campaign.campaigns SET game_day = @game_day, game_minute = @game_minute WHERE id = @id;

-- name: CharacterPurse :many
-- The coins in a Character's own container; none when it has never held any.
SELECT c.id AS container_id, k.coin, k.amount
FROM campaign.containers c LEFT JOIN campaign.container_coins k ON k.container_id = c.id
WHERE c.character_id = $1;

-- name: SetHeroicInspiration :exec
UPDATE campaign.characters SET heroic_inspiration = @inspired WHERE campaign_id = @campaign_id AND id = @id;

-- name: GiveUpInspiration :execrows
UPDATE campaign.characters SET heroic_inspiration = false WHERE campaign_id = @campaign_id AND id = @id AND heroic_inspiration;

-- name: TakeInspiration :execrows
UPDATE campaign.characters SET heroic_inspiration = true WHERE campaign_id = @campaign_id AND id = @id AND NOT heroic_inspiration;

-- name: CharacterCarried :one
-- What a Character carries: every item in its own Container and the bags inside it, and its coins.
SELECT (coalesce((SELECT sum(i.quantity * coalesce((SELECT w.weight_lb FROM compendium.items w WHERE w.slug = i.item_slug ORDER BY w.id LIMIT 1), 0))
        FROM campaign.item_instances i JOIN campaign.containers ic ON ic.id = i.container_id
        LEFT JOIN campaign.containers ip ON ip.id = ic.parent_id
        WHERE ic.character_id = @owner OR ip.character_id = @owner), 0)
    + coalesce((SELECT sum(k.amount) FROM campaign.container_coins k JOIN campaign.containers kc ON kc.id = k.container_id
        LEFT JOIN campaign.containers kp ON kp.id = kc.parent_id
        WHERE kc.character_id = @owner OR kp.character_id = @owner), 0) / 50.0)::float8 AS weight_lb;

-- name: CampaignHomebrewDesigns :many
-- The homebrew designs of one kind a Campaign sees: linked directly or through a Collection switched
-- on, at the Revision a link pins.
WITH visible AS (
    SELECT l.entry_id FROM library.campaign_links l WHERE l.campaign_id = @campaign_id AND l.direct
    UNION
    SELECT ce.entry_id FROM library.collection_entries ce
    JOIN library.campaign_collections cc ON cc.collection_id = ce.collection_id WHERE cc.campaign_id = @campaign_id
)
SELECT e.id, coalesce(r.name, e.name)::text AS name, coalesce(r.design, e.design)::jsonb AS design
FROM visible v
JOIN library.entries e ON e.id = v.entry_id AND e.kind = @kind
LEFT JOIN library.campaign_links l ON l.entry_id = e.id AND l.campaign_id = @campaign_id
LEFT JOIN library.entry_revisions r ON r.entry_id = e.id AND r.no = l.pinned_revision
WHERE coalesce(r.design, e.design) IS NOT NULL
ORDER BY e.id;
