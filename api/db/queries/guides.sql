-- The guides of the public compendium read SRD documents only.

-- name: GuideSpells :many
WITH effective AS (
    SELECT DISTINCT ON (s.slug) s.slug, s.name, s.level, s.school_id
    FROM compendium.spells s JOIN compendium.documents d ON d.id = s.document_id
    WHERE d.key LIKE 'srd-%' AND (sqlc.narg(ruleset)::text IS NULL OR d.key = sqlc.narg(ruleset)::text)
    ORDER BY s.slug, d.precedence DESC
)
SELECT e.slug, e.name, e.level, sch.slug AS school
FROM effective e JOIN compendium.magic_schools sch ON sch.id = e.school_id
ORDER BY e.level, lower(e.name), e.slug;

-- name: GuideChallenges :many
WITH effective AS (
    SELECT DISTINCT ON (m.slug) m.id, m.challenge_rating
    FROM compendium.monsters m JOIN compendium.documents d ON d.id = m.document_id
    WHERE d.key LIKE 'srd-%' AND (sqlc.narg(ruleset)::text IS NULL OR d.key = sqlc.narg(ruleset)::text)
    ORDER BY m.slug, d.precedence DESC
)
SELECT challenge_rating::float8 AS challenge_rating, count(*)::bigint AS monsters
FROM effective GROUP BY challenge_rating ORDER BY challenge_rating;

-- name: GuideAttacks :many
WITH effective AS (
    SELECT DISTINCT ON (m.slug) m.id, m.challenge_rating
    FROM compendium.monsters m JOIN compendium.documents d ON d.id = m.document_id
    WHERE d.key LIKE 'srd-%' AND (sqlc.narg(ruleset)::text IS NULL OR d.key = sqlc.narg(ruleset)::text)
    ORDER BY m.slug, d.precedence DESC
)
SELECT e.challenge_rating::float8 AS challenge_rating, a.to_hit, coalesce(a.damage_dice, '')::text AS damage_dice, a.damage_bonus,
    coalesce(a.extra_dice, '')::text AS extra_dice
FROM effective e
JOIN compendium.monster_actions ac ON ac.monster_id = e.id
JOIN compendium.monster_attacks a ON a.action_id = ac.id
ORDER BY e.challenge_rating, a.to_hit, a.action_id, a.ordering;

-- name: GuideMagicItems :many
WITH effective AS (
    SELECT DISTINCT ON (i.slug) i.slug, i.name, i.magic, i.rarity
    FROM compendium.items i JOIN compendium.documents d ON d.id = i.document_id
    WHERE d.key LIKE 'srd-%' AND (sqlc.narg(ruleset)::text IS NULL OR d.key = sqlc.narg(ruleset)::text)
    ORDER BY i.slug, d.precedence DESC
)
SELECT slug, name, rarity::text AS rarity FROM effective
WHERE magic AND rarity IS NOT NULL
ORDER BY lower(name), slug;
