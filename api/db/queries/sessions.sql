-- name: NextSessionNumber :one
SELECT coalesce(max(number), 0)::int + 1 FROM play.sessions WHERE campaign_id = $1;

-- name: InsertSession :one
INSERT INTO play.sessions (campaign_id, number, status, started_at) VALUES (@campaign_id, @number, 'live', @now)
RETURNING id, campaign_id, number, status, seq, grid_radius, started_at, ended_at, map_id, world_map_id;

-- name: GetSession :one
SELECT id, campaign_id, number, status, seq, grid_radius, started_at, ended_at, map_id, world_map_id
FROM play.sessions WHERE campaign_id = @campaign_id AND id = @id;

-- name: SessionByID :one
SELECT id, campaign_id, number, status, seq, grid_radius, started_at, ended_at, map_id, world_map_id FROM play.sessions WHERE id = $1;

-- name: ListSessions :many
SELECT id, campaign_id, number, status, seq, grid_radius, started_at, ended_at, map_id, world_map_id
FROM play.sessions WHERE campaign_id = $1 ORDER BY number DESC LIMIT 50;

-- name: EndSession :execrows
UPDATE play.sessions SET status = 'ended', ended_at = @now WHERE campaign_id = @campaign_id AND id = @id AND status = 'live';

-- name: BumpSessionSeq :one
UPDATE play.sessions SET seq = seq + 1 WHERE id = $1 RETURNING seq;

-- name: SessionTokens :many
SELECT id, label, kind, q, r, hidden, darkvision_ft, controller_member_id, stat_source, armor_class, hp, hp_max, intelligence, tactics, can_shield, spell_dc,
    stealth, perception, initiative, speed_ft, unarmed_dc, attacks_per_action FROM play.tokens WHERE session_id = $1 ORDER BY label, id;

-- name: InsertToken :exec
INSERT INTO play.tokens (id, session_id, label, kind, q, r, hidden, darkvision_ft, controller_member_id, stat_source, armor_class,
    hp, hp_max, intelligence, can_shield, spell_dc, stealth, perception, initiative, speed_ft, unarmed_dc, attacks_per_action)
VALUES (@id, @session_id, @label, @kind, @q, @r, @hidden, @darkvision_ft, @controller_member_id, sqlc.narg(stat_source),
    sqlc.narg(armor_class), sqlc.narg(hp), sqlc.narg(hp_max), sqlc.narg(intelligence), @can_shield, sqlc.narg(spell_dc), @stealth,
    @perception, @initiative, @speed_ft, @unarmed_dc, @attacks_per_action);

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
SELECT id, session_id, status, round, turn_count, started_at, ended_at, resume_token_id, resume_cost_ft FROM play.combats WHERE session_id = $1 AND status <> 'ended';

-- name: CombatCombatants :many
SELECT id, combat_id, token_id, roll_id, initiative_bonus, speed_ft, initiative, done, has_action, has_bonus_action, has_reaction, movement_ft,
       shielded, surprised, disengaged, readied_trigger, readied_who, readied_attack, attacks_left, light_attack, off_hand, interaction, cleave_from, cleaved
FROM play.combatants WHERE combat_id = $1 ORDER BY id;

-- name: SaveCombat :exec
INSERT INTO play.combats (id, session_id, status, round, turn_count, started_at, ended_at, resume_token_id, resume_cost_ft)
VALUES (@id, @session_id, @status, @round, sqlc.narg(turn_count), @started_at, sqlc.narg(ended_at), sqlc.narg(resume_token_id),
    sqlc.narg(resume_cost_ft))
ON CONFLICT (id) DO UPDATE SET status = excluded.status, round = excluded.round, turn_count = excluded.turn_count,
    ended_at = excluded.ended_at, resume_token_id = excluded.resume_token_id, resume_cost_ft = excluded.resume_cost_ft;

-- name: SaveCombatant :exec
INSERT INTO play.combatants (id, combat_id, token_id, roll_id, initiative_bonus, speed_ft, initiative, done, has_action,
    has_bonus_action, has_reaction, movement_ft, shielded, surprised, disengaged, readied_trigger, readied_who, readied_attack,
    attacks_left, light_attack, off_hand, interaction, cleave_from, cleaved)
VALUES (@id, @combat_id, @token_id, @roll_id, @initiative_bonus, @speed_ft, sqlc.narg(initiative), @done, @has_action,
    @has_bonus_action, @has_reaction, @movement_ft, @shielded, @surprised, @disengaged, sqlc.narg(readied_trigger), sqlc.narg(readied_who),
    sqlc.narg(readied_attack), @attacks_left, @light_attack, @off_hand, @interaction, sqlc.narg(cleave_from), @cleaved)
ON CONFLICT (id) DO UPDATE SET initiative = excluded.initiative, done = excluded.done, has_action = excluded.has_action,
    has_bonus_action = excluded.has_bonus_action, has_reaction = excluded.has_reaction, movement_ft = excluded.movement_ft,
    shielded = excluded.shielded, disengaged = excluded.disengaged, readied_trigger = excluded.readied_trigger,
    readied_who = excluded.readied_who, readied_attack = excluded.readied_attack, attacks_left = excluded.attacks_left,
    light_attack = excluded.light_attack, off_hand = excluded.off_hand, interaction = excluded.interaction,
    cleave_from = excluded.cleave_from, cleaved = excluded.cleaved;

-- name: SetTokenHP :exec
UPDATE play.tokens SET hp = @hp WHERE session_id = @session_id AND id = @id;

-- name: SessionTokenAttacks :many
SELECT a.token_id, a.ordering, a.name, a.to_hit, a.reach_ft, a.range_ft, a.long_range_ft, a.damage_dice, a.damage_bonus, a.damage_type,
       a.light, a.damage_mod, a.mastery
FROM play.token_attacks a JOIN play.tokens t ON t.id = a.token_id WHERE t.session_id = $1 ORDER BY a.token_id, a.ordering;

-- name: InsertTokenAttack :exec
INSERT INTO play.token_attacks (token_id, ordering, name, to_hit, reach_ft, range_ft, long_range_ft, damage_dice, damage_bonus, damage_type,
    light, damage_mod, mastery)
VALUES (@token_id, @ordering, @name, @to_hit, @reach_ft, @range_ft, @long_range_ft, @damage_dice, @damage_bonus, @damage_type, @light, @damage_mod,
    sqlc.narg(mastery));

-- name: CombatAttack :one
SELECT id, combat_id, attacker_token_id, target_token_id, attack_no, mode, cover_bonus, stage, critical, roll_id, ranged, total,
       opportunity, off_hand, cleave
FROM play.attacks WHERE combat_id = $1;

-- name: SaveAttack :exec
INSERT INTO play.attacks (id, combat_id, attacker_token_id, target_token_id, attack_no, mode, cover_bonus, stage, critical, roll_id, ranged,
    total, opportunity, off_hand, cleave)
VALUES (@id, @combat_id, @attacker_token_id, @target_token_id, @attack_no, @mode, @cover_bonus, @stage, @critical, @roll_id, @ranged,
    sqlc.narg(total), @opportunity, @off_hand, @cleave)
ON CONFLICT (id) DO UPDATE SET stage = excluded.stage, critical = excluded.critical, roll_id = excluded.roll_id, total = excluded.total;

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
SELECT m.id, m.name, m.armor_class, m.hit_points, m.intelligence, m.strength, m.dexterity, m.constitution, m.wisdom, m.charisma,
       COALESCE(m.challenge_rating, 0)::float8 AS challenge_rating
FROM compendium.monsters m
JOIN compendium.documents d ON d.id = m.document_id
WHERE m.slug = @slug AND (sqlc.narg(ruleset)::text IS NULL OR d.key = sqlc.narg(ruleset)::text)
ORDER BY d.precedence DESC LIMIT 1;

-- name: MonsterAttackRows :many
SELECT k.name, k.to_hit, k.reach_feet, k.range_feet, k.long_range_feet, coalesce(k.damage_dice, '')::text AS damage_dice,
       k.damage_bonus, coalesce(k.damage_type, '')::text AS damage_type, coalesce(k.extra_dice, '')::text AS extra_dice,
       coalesce(k.extra_type, '')::text AS extra_type
FROM compendium.monster_attacks k JOIN compendium.monster_actions a ON a.id = k.action_id
WHERE a.monster_id = @monster_id ORDER BY a.ordering, k.ordering;

-- name: SetTokenTactics :exec
UPDATE play.tokens SET tactics = @tactics WHERE session_id = @session_id AND id = @id;

-- name: SessionObservations :many
SELECT o.observer_token_id, o.attacker_token_id, o.ranged_damage FROM play.observed_damage o
JOIN play.tokens t ON t.id = o.observer_token_id WHERE t.session_id = $1;

-- name: ObserveDamage :exec
INSERT INTO play.observed_damage (observer_token_id, attacker_token_id, ranged_damage) VALUES (@observer, @attacker, @amount)
ON CONFLICT (observer_token_id, attacker_token_id) DO UPDATE SET ranged_damage = play.observed_damage.ranged_damage + excluded.ranged_damage;

-- name: ClearResumePath :exec
DELETE FROM play.combat_resume_path WHERE combat_id = $1;

-- name: AddResumeHex :exec
INSERT INTO play.combat_resume_path (combat_id, ordering, q, r) VALUES (@combat_id, @ordering, @q, @r);

-- name: ResumePath :many
SELECT q, r FROM play.combat_resume_path WHERE combat_id = $1 ORDER BY ordering;

-- name: CombatPrompt :one
SELECT id, kind, reactor_token_id, trigger_token_id, attack_no, effect, deadline FROM play.reaction_prompts WHERE combat_id = $1;

-- name: SavePrompt :exec
INSERT INTO play.reaction_prompts (id, combat_id, kind, reactor_token_id, trigger_token_id, attack_no, effect, deadline)
VALUES (@id, @combat_id, @kind, @reactor_token_id, @trigger_token_id, @attack_no, @effect, @deadline)
ON CONFLICT (id) DO NOTHING;

-- name: ClearPrompts :exec
DELETE FROM play.reaction_prompts WHERE combat_id = $1;

-- name: CampaignReactionTimeout :one
SELECT reaction_timeout_s FROM campaign.campaigns WHERE id = $1;

-- name: SessionEffects :many
SELECT id, target_token_id, source_token_id, slug, name, concentration, rounds_left, save_ability, save_dc, level
FROM play.active_effects WHERE session_id = $1 ORDER BY id;

-- name: ClearEffects :exec
DELETE FROM play.active_effects WHERE session_id = $1;

-- name: InsertEffect :exec
INSERT INTO play.active_effects (id, session_id, target_token_id, source_token_id, slug, name, concentration, rounds_left, save_ability, save_dc, level)
VALUES (@id, @session_id, @target_token_id, sqlc.narg(source_token_id), @slug, @name, @concentration, sqlc.narg(rounds_left),
    sqlc.narg(save_ability), sqlc.narg(save_dc), @level);

-- name: SessionManuals :many
SELECT id, text FROM play.manual_prompts WHERE session_id = $1 ORDER BY ordering;

-- name: ClearManuals :exec
DELETE FROM play.manual_prompts WHERE session_id = $1;

-- name: InsertManual :exec
INSERT INTO play.manual_prompts (id, session_id, ordering, text) VALUES (@id, @session_id, @ordering, @text);

-- name: SessionPendingSaves :many
SELECT roll_id, effect_id, dc FROM play.pending_saves WHERE session_id = $1;

-- name: ClearPendingSaves :exec
DELETE FROM play.pending_saves WHERE session_id = $1;

-- name: InsertPendingSave :exec
INSERT INTO play.pending_saves (roll_id, effect_id, session_id, dc) VALUES (@roll_id, @effect_id, @session_id, @dc);

-- name: SessionTokenSaves :many
SELECT s.token_id, s.ability, s.bonus FROM play.token_saves s JOIN play.tokens t ON t.id = s.token_id WHERE t.session_id = $1;

-- name: InsertTokenSave :exec
INSERT INTO play.token_saves (token_id, ability, bonus) VALUES (@token_id, @ability, @bonus);

-- name: MonsterSaves :many
SELECT name, value FROM compendium.monster_stats WHERE monster_id = @monster_id AND kind = 'save';

-- name: SessionSurfaces :many
SELECT q, r, kind, rounds_left FROM play.surfaces WHERE session_id = $1;

-- name: ClearSurfaces :exec
DELETE FROM play.surfaces WHERE session_id = $1;

-- name: InsertSurface :exec
INSERT INTO play.surfaces (session_id, q, r, kind, rounds_left) VALUES (@session_id, @q, @r, @kind, sqlc.narg(rounds_left));

-- name: SessionCast :one
SELECT id, caster_token_id, spell, dc, damage_roll_id FROM play.area_casts WHERE session_id = $1;

-- name: CastHexes :many
SELECT q, r FROM play.area_hexes WHERE cast_id = $1 ORDER BY q, r;

-- name: CastTargets :many
SELECT token_id, save_roll_id FROM play.area_targets WHERE cast_id = $1 ORDER BY token_id;

-- name: ClearCasts :exec
DELETE FROM play.area_casts WHERE session_id = $1;

-- name: InsertCast :exec
INSERT INTO play.area_casts (id, session_id, caster_token_id, spell, dc, damage_roll_id)
VALUES (@id, @session_id, @caster_token_id, @spell, @dc, sqlc.narg(damage_roll_id));

-- name: InsertCastHex :exec
INSERT INTO play.area_hexes (cast_id, q, r) VALUES (@cast_id, @q, @r);

-- name: InsertCastTarget :exec
INSERT INTO play.area_targets (cast_id, token_id, save_roll_id) VALUES (@cast_id, @token_id, sqlc.narg(save_roll_id));

-- name: MapElevations :many
SELECT q, r, elevation_ft FROM campaign.map_elevations WHERE map_id = $1;

-- name: SetElevation :exec
INSERT INTO campaign.map_elevations (map_id, q, r, elevation_ft) VALUES (@map_id, @q, @r, @elevation_ft)
ON CONFLICT (map_id, q, r) DO UPDATE SET elevation_ft = excluded.elevation_ft;

-- name: ClearElevation :exec
DELETE FROM campaign.map_elevations WHERE map_id = @map_id AND q = @q AND r = @r;

-- name: CampaignHighGround :one
SELECT high_ground FROM campaign.campaigns WHERE id = $1;

-- name: SessionTable :one
SELECT camera, q, r, zoom_pct, scene, title, body, map_id, blackout FROM play.table_displays WHERE session_id = $1;

-- name: SaveTable :exec
INSERT INTO play.table_displays (session_id, camera, q, r, zoom_pct, scene, title, body, map_id, blackout)
VALUES (@session_id, @camera, @q, @r, @zoom_pct, @scene, @title, @body, sqlc.narg(map_id), @blackout)
ON CONFLICT (session_id) DO UPDATE SET camera = excluded.camera, q = excluded.q, r = excluded.r, zoom_pct = excluded.zoom_pct,
    scene = excluded.scene, title = excluded.title, body = excluded.body, map_id = excluded.map_id, blackout = excluded.blackout;

-- name: MonsterAmbushStats :many
SELECT kind, name, value FROM compendium.monster_stats
WHERE monster_id = @monster_id AND ((kind = 'skill' AND name IN ('stealth', 'perception')) OR (kind = 'speed' AND name = 'walk'));

-- name: SessionZones :many
SELECT id, name, q, r, radius_hexes, dm_only, held, status, dc FROM play.encounter_zones WHERE session_id = $1 ORDER BY name, id;

-- name: SaveZone :exec
INSERT INTO play.encounter_zones (id, session_id, name, q, r, radius_hexes, dm_only, held, status, dc)
VALUES (@id, @session_id, @name, @q, @r, @radius_hexes, @dm_only, @held, @status, @dc)
ON CONFLICT (id) DO UPDATE SET held = excluded.held, status = excluded.status, dc = excluded.dc;

-- name: DeleteZone :exec
DELETE FROM play.encounter_zones WHERE session_id = @session_id AND id = @id;

-- name: SessionZoneChecks :many
SELECT c.zone_id, c.token_id, c.roll_id, c.noticed
FROM play.zone_checks c JOIN play.encounter_zones z ON z.id = c.zone_id
WHERE z.session_id = $1 ORDER BY c.zone_id, c.token_id;

-- name: SaveZoneCheck :exec
INSERT INTO play.zone_checks (zone_id, token_id, roll_id, noticed) VALUES (@zone_id, @token_id, sqlc.narg(roll_id), sqlc.narg(noticed))
ON CONFLICT (zone_id, token_id) DO UPDATE SET roll_id = excluded.roll_id, noticed = excluded.noticed;

-- name: SessionZoneCreatures :many
SELECT c.zone_id, c.token_id
FROM play.zone_creatures c JOIN play.encounter_zones z ON z.id = c.zone_id
WHERE z.session_id = $1 ORDER BY c.zone_id, c.token_id;

-- name: AddZoneCreature :exec
INSERT INTO play.zone_creatures (zone_id, token_id) VALUES (@zone_id, @token_id) ON CONFLICT DO NOTHING;

-- name: SessionPendingActions :many
SELECT roll_id, actor_token_id, target_token_id, action, dc FROM play.pending_actions WHERE session_id = $1 ORDER BY roll_id;

-- name: InsertPendingAction :exec
INSERT INTO play.pending_actions (roll_id, session_id, actor_token_id, target_token_id, action, dc)
VALUES (@roll_id, @session_id, @actor_token_id, sqlc.narg(target_token_id), @action, @dc);

-- name: DeletePendingAction :exec
DELETE FROM play.pending_actions WHERE roll_id = $1;
