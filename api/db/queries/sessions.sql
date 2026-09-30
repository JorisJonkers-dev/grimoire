-- name: NextSessionNumber :one
SELECT coalesce(max(number), 0)::int + 1 FROM play.sessions WHERE campaign_id = $1;

-- name: InsertSession :one
INSERT INTO play.sessions (campaign_id, number, status, started_at) VALUES (@campaign_id, @number, 'live', @now)
RETURNING id, campaign_id, number, status, seq, grid_radius, started_at, ended_at, map_id;

-- name: GetSession :one
SELECT id, campaign_id, number, status, seq, grid_radius, started_at, ended_at, map_id
FROM play.sessions WHERE campaign_id = @campaign_id AND id = @id;

-- name: SessionByID :one
SELECT id, campaign_id, number, status, seq, grid_radius, started_at, ended_at, map_id FROM play.sessions WHERE id = $1;

-- name: ListSessions :many
SELECT id, campaign_id, number, status, seq, grid_radius, started_at, ended_at, map_id
FROM play.sessions WHERE campaign_id = $1 ORDER BY number DESC LIMIT 50;

-- name: EndSession :execrows
UPDATE play.sessions SET status = 'ended', ended_at = @now WHERE campaign_id = @campaign_id AND id = @id AND status = 'live';

-- name: BumpSessionSeq :one
UPDATE play.sessions SET seq = seq + 1 WHERE id = $1 RETURNING seq;

-- name: SessionTokens :many
SELECT id, label, kind, q, r, hidden, darkvision_ft, controller_member_id, stat_source, armor_class, hp, hp_max FROM play.tokens WHERE session_id = $1 ORDER BY label, id;

-- name: InsertToken :exec
INSERT INTO play.tokens (id, session_id, label, kind, q, r, hidden, darkvision_ft, controller_member_id, stat_source, armor_class,
    hp, hp_max)
VALUES (@id, @session_id, @label, @kind, @q, @r, @hidden, @darkvision_ft, @controller_member_id, sqlc.narg(stat_source),
    sqlc.narg(armor_class), sqlc.narg(hp), sqlc.narg(hp_max));

-- name: UpdateToken :exec
UPDATE play.tokens SET q = @q, r = @r, hidden = @hidden WHERE session_id = @session_id AND id = @id;

-- name: DeleteToken :exec
DELETE FROM play.tokens WHERE session_id = @session_id AND id = @id;

-- name: InsertSessionAction :one
INSERT INTO play.actions (campaign_id, session_id, seq, kind, actor_member_id, actor_name, origin, client, created_at)
VALUES (@campaign_id, @session_id, @seq, @kind, @actor_member_id, @actor_name, @origin, @client, @now)
RETURNING id;

-- name: InsertTokenEvent :exec
INSERT INTO play.action_token_events (action_id, token_id, label, q, r, hidden) VALUES (@action_id, @token_id, @label, @q, @r, @hidden);

-- name: LockSessionOwner :one
SELECT pg_try_advisory_lock(hashtextextended(@lock_key::text, 0));

-- name: UnlockSessionOwner :one
SELECT pg_advisory_unlock(hashtextextended(@lock_key::text, 0));

-- name: RunningCombat :one
SELECT id, session_id, status, round, turn_count, started_at, ended_at FROM play.combats WHERE session_id = $1 AND status <> 'ended';

-- name: CombatCombatants :many
SELECT id, combat_id, token_id, roll_id, initiative_bonus, speed_ft, initiative, done, has_action, has_bonus_action, has_reaction, movement_ft
FROM play.combatants WHERE combat_id = $1 ORDER BY id;

-- name: SaveCombat :exec
INSERT INTO play.combats (id, session_id, status, round, turn_count, started_at, ended_at)
VALUES (@id, @session_id, @status, @round, sqlc.narg(turn_count), @started_at, sqlc.narg(ended_at))
ON CONFLICT (id) DO UPDATE SET status = excluded.status, round = excluded.round, turn_count = excluded.turn_count,
    ended_at = excluded.ended_at;

-- name: SaveCombatant :exec
INSERT INTO play.combatants (id, combat_id, token_id, roll_id, initiative_bonus, speed_ft, initiative, done, has_action,
    has_bonus_action, has_reaction, movement_ft)
VALUES (@id, @combat_id, @token_id, @roll_id, @initiative_bonus, @speed_ft, sqlc.narg(initiative), @done, @has_action,
    @has_bonus_action, @has_reaction, @movement_ft)
ON CONFLICT (id) DO UPDATE SET initiative = excluded.initiative, done = excluded.done, has_action = excluded.has_action,
    has_bonus_action = excluded.has_bonus_action, has_reaction = excluded.has_reaction, movement_ft = excluded.movement_ft;

-- name: SetTokenHP :exec
UPDATE play.tokens SET hp = @hp WHERE session_id = @session_id AND id = @id;

-- name: SessionTokenAttacks :many
SELECT a.token_id, a.ordering, a.name, a.to_hit, a.reach_ft, a.range_ft, a.long_range_ft, a.damage_dice, a.damage_bonus, a.damage_type
FROM play.token_attacks a JOIN play.tokens t ON t.id = a.token_id WHERE t.session_id = $1 ORDER BY a.token_id, a.ordering;

-- name: InsertTokenAttack :exec
INSERT INTO play.token_attacks (token_id, ordering, name, to_hit, reach_ft, range_ft, long_range_ft, damage_dice, damage_bonus, damage_type)
VALUES (@token_id, @ordering, @name, @to_hit, @reach_ft, @range_ft, @long_range_ft, @damage_dice, @damage_bonus, @damage_type);

-- name: CombatAttack :one
SELECT id, combat_id, attacker_token_id, target_token_id, attack_no, mode, cover_bonus, stage, critical, roll_id
FROM play.attacks WHERE combat_id = $1;

-- name: SaveAttack :exec
INSERT INTO play.attacks (id, combat_id, attacker_token_id, target_token_id, attack_no, mode, cover_bonus, stage, critical, roll_id)
VALUES (@id, @combat_id, @attacker_token_id, @target_token_id, @attack_no, @mode, @cover_bonus, @stage, @critical, @roll_id)
ON CONFLICT (id) DO UPDATE SET stage = excluded.stage, critical = excluded.critical, roll_id = excluded.roll_id;

-- name: ClearAttacks :exec
DELETE FROM play.attacks WHERE combat_id = $1;

-- name: InsertHPEvent :exec
INSERT INTO play.action_hp_events (action_id, token_id, hp_before, hp_after, undoes_action_id)
VALUES (@action_id, @token_id, @hp_before, @hp_after, sqlc.narg(undoes_action_id));

-- name: LastDamage :one
SELECT a.id, h.token_id, h.hp_before, h.hp_after FROM play.actions a
JOIN play.action_hp_events h ON h.action_id = a.id
WHERE a.session_id = $1 AND a.kind = 'damage_dealt'
  AND NOT EXISTS (SELECT 1 FROM play.action_hp_events u WHERE u.undoes_action_id = a.id)
ORDER BY a.seq DESC LIMIT 1;

-- name: CampaignRuleset :one
SELECT ruleset_pref FROM campaign.campaigns WHERE id = $1;

-- name: MonsterStatblock :one
SELECT m.id, m.name, m.armor_class, m.hit_points FROM compendium.monsters m
JOIN compendium.documents d ON d.id = m.document_id
WHERE m.slug = @slug AND (sqlc.narg(ruleset)::text IS NULL OR d.key = sqlc.narg(ruleset)::text)
ORDER BY d.precedence DESC LIMIT 1;

-- name: MonsterAttackRows :many
SELECT k.name, k.to_hit, k.reach_feet, k.range_feet, k.long_range_feet, coalesce(k.damage_dice, '')::text AS damage_dice,
       k.damage_bonus, coalesce(k.damage_type, '')::text AS damage_type, coalesce(k.extra_dice, '')::text AS extra_dice,
       coalesce(k.extra_type, '')::text AS extra_type
FROM compendium.monster_attacks k JOIN compendium.monster_actions a ON a.id = k.action_id
WHERE a.monster_id = @monster_id ORDER BY a.ordering, k.ordering;
