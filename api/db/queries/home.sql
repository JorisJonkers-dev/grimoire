-- name: HomeLiveSessions :many
SELECT c.id AS campaign_id, c.name AS campaign, m.role, s.id AS session_id, s.number
FROM play.sessions s
JOIN campaign.campaigns c ON c.id = s.campaign_id
JOIN campaign.members m ON m.campaign_id = c.id AND m.auth_subject = @subject
WHERE s.status = 'live' AND s.parent_session_id IS NULL
ORDER BY s.started_at DESC, s.id LIMIT 20;

-- name: HomeLevelUps :many
SELECT c.id AS campaign_id, c.name AS campaign, ch.id AS character_id, ch.name
FROM campaign.characters ch
JOIN campaign.members m ON m.id = ch.owner_member_id AND m.auth_subject = @subject
JOIN campaign.campaigns c ON c.id = ch.campaign_id
WHERE ch.level_up_ready
ORDER BY lower(c.name), lower(ch.name), ch.id LIMIT 20;

-- name: HomeDowntime :many
SELECT c.id AS campaign_id, c.name AS campaign, ch.name, ch.downtime_days
FROM campaign.characters ch
JOIN campaign.members m ON m.id = ch.owner_member_id AND m.auth_subject = @subject
JOIN campaign.campaigns c ON c.id = ch.campaign_id
WHERE ch.downtime_days > 0
ORDER BY lower(c.name), lower(ch.name), ch.id LIMIT 20;

-- name: HomeProposalsToReview :many
SELECT c.id AS campaign_id, c.name AS campaign, count(*) AS waiting
FROM library.proposals p
JOIN campaign.campaigns c ON c.id = p.campaign_id
JOIN campaign.members m ON m.campaign_id = c.id AND m.auth_subject = @subject AND m.role = 'dm'
WHERE p.status = 'pending'
GROUP BY c.id, c.name ORDER BY lower(c.name), c.id LIMIT 20;

-- name: HomeProposalsToRevise :many
SELECT c.id AS campaign_id, c.name AS campaign, p.id AS proposal_id, p.name
FROM library.proposals p
JOIN campaign.campaigns c ON c.id = p.campaign_id
JOIN campaign.members m ON m.campaign_id = c.id AND m.auth_subject = @subject
WHERE p.author_subject = @subject AND p.status = 'changes_requested'
ORDER BY p.updated_at DESC, p.id LIMIT 20;

-- name: HomeRollsWaiting :many
SELECT c.id AS campaign_id, c.name AS campaign, count(*) AS waiting
FROM play.roll_requests r
JOIN campaign.members m ON m.id = r.roller_member_id AND m.auth_subject = @subject
JOIN campaign.campaigns c ON c.id = r.campaign_id
WHERE r.status = 'pending'
GROUP BY c.id, c.name ORDER BY lower(c.name), c.id LIMIT 20;

-- name: HomeFriendRequests :one
SELECT count(*) FROM social.friend_requests r
JOIN identity.accounts a ON a.id = r.to_account
WHERE a.subject = @subject AND r.status = 'pending'
    AND NOT EXISTS (SELECT 1 FROM social.blocks b WHERE b.blocker = a.id AND b.blocked = r.from_account);

-- name: SearchSpells :many
WITH effective AS (
    SELECT DISTINCT ON (s.slug) s.slug, s.name, s.level, s.school_id
    FROM compendium.spells s JOIN compendium.documents d ON d.id = s.document_id
    ORDER BY s.slug, d.precedence DESC
)
SELECT e.slug, e.name, e.level, sch.name AS school
FROM effective e JOIN compendium.magic_schools sch ON sch.id = e.school_id
WHERE e.name ILIKE @pattern::text ESCAPE '\'
ORDER BY strpos(lower(e.name), lower(@needle::text)), char_length(e.name), lower(e.name), e.slug LIMIT @lim;

-- name: SearchMonsters :many
WITH effective AS (
    SELECT DISTINCT ON (m.slug) m.slug, m.name, m.size, m.creature_type, m.challenge_rating
    FROM compendium.monsters m JOIN compendium.documents d ON d.id = m.document_id
    ORDER BY m.slug, d.precedence DESC
)
SELECT e.slug, e.name, e.size, e.creature_type, e.challenge_rating::float8 AS challenge_rating
FROM effective e
WHERE e.name ILIKE @pattern::text ESCAPE '\'
ORDER BY strpos(lower(e.name), lower(@needle::text)), char_length(e.name), lower(e.name), e.slug LIMIT @lim;

-- name: SearchItems :many
WITH effective AS (
    SELECT DISTINCT ON (i.slug) i.slug, i.name, i.category, i.magic, i.rarity
    FROM compendium.items i JOIN compendium.documents d ON d.id = i.document_id
    ORDER BY i.slug, d.precedence DESC
)
SELECT e.slug, e.name, e.category, e.magic, coalesce(e.rarity, '')::text AS rarity
FROM effective e
WHERE e.name ILIKE @pattern::text ESCAPE '\'
ORDER BY strpos(lower(e.name), lower(@needle::text)), char_length(e.name), lower(e.name), e.slug LIMIT @lim;

-- name: SearchLibrary :many
SELECT id, kind, name, shared, owner_subject = @subject AS mine
FROM library.entries
WHERE (owner_subject = @subject OR shared) AND name ILIKE @pattern::text ESCAPE '\'
ORDER BY (owner_subject = @subject) DESC, strpos(lower(name), lower(@needle::text)), char_length(name), lower(name), id LIMIT @lim;

-- name: SearchCampaigns :many
SELECT c.id, c.name, m.role
FROM campaign.campaigns c
JOIN campaign.members m ON m.campaign_id = c.id AND m.auth_subject = @subject
WHERE c.name ILIKE @pattern::text ESCAPE '\'
ORDER BY strpos(lower(c.name), lower(@needle::text)), char_length(c.name), lower(c.name), c.id LIMIT @lim;

-- name: SearchCharacters :many
SELECT c.id AS campaign_id, c.name AS campaign, ch.id, ch.name, ch.level, ch.class_slug
FROM campaign.characters ch
JOIN campaign.campaigns c ON c.id = ch.campaign_id
JOIN campaign.members m ON m.campaign_id = c.id AND m.auth_subject = @subject
WHERE ch.name ILIKE @pattern::text ESCAPE '\'
ORDER BY strpos(lower(ch.name), lower(@needle::text)), char_length(ch.name), lower(ch.name), ch.id LIMIT @lim;

-- name: SearchNpcs :many
SELECT c.id AS campaign_id, c.name AS campaign, n.id, n.name
FROM campaign.npcs n
JOIN campaign.campaigns c ON c.id = n.campaign_id
JOIN campaign.members m ON m.campaign_id = c.id AND m.auth_subject = @subject AND m.role = 'dm'
WHERE n.name ILIKE @pattern::text ESCAPE '\'
ORDER BY strpos(lower(n.name), lower(@needle::text)), char_length(n.name), lower(n.name), n.id LIMIT @lim;

-- name: SearchFriends :many
SELECT a.id, a.username, a.nickname
FROM identity.accounts me
JOIN social.friendships f ON f.a = me.id OR f.b = me.id
JOIN identity.accounts a ON a.id = CASE WHEN f.a = me.id THEN f.b ELSE f.a END
WHERE me.subject = @subject AND (a.nickname ILIKE @pattern::text ESCAPE '\' OR a.username ILIKE @pattern::text ESCAPE '\')
ORDER BY lower(a.nickname), a.username LIMIT @lim;
