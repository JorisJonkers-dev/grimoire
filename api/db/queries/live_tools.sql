-- name: InsertSpawnEvent :exec
INSERT INTO play.action_spawn_events (action_id, token_id, label) VALUES (@action_id, @token_id, @label);

-- name: InsertEffectEvent :exec
INSERT INTO play.action_effect_events (action_id, effect_id) VALUES (@action_id, @effect_id);

-- name: InsertUndo :exec
INSERT INTO play.action_undos (action_id, undoes_action_id) VALUES (@action_id, @undoes_action_id);

-- name: SessionActionBySeq :one
SELECT a.id, a.kind,
    (EXISTS (SELECT 1 FROM play.action_undos u WHERE u.undoes_action_id = a.id)
        OR EXISTS (SELECT 1 FROM play.action_hp_events h WHERE h.undoes_action_id = a.id))::boolean AS undone
FROM play.actions a
WHERE a.session_id = @session_id AND a.seq = @seq;

-- name: ActionTokenEvent :one
SELECT token_id, label FROM play.action_token_events WHERE action_id = @action_id;

-- name: ActionHexEvents :many
SELECT q, r FROM play.action_hex_events WHERE action_id = @action_id ORDER BY q, r;

-- name: ActionHPEvent :one
SELECT token_id, hp_before, hp_after FROM play.action_hp_events WHERE action_id = @action_id;

-- name: ActionSpawnEvents :many
SELECT token_id, label FROM play.action_spawn_events WHERE action_id = @action_id ORDER BY label, token_id;

-- name: ActionEffectEvent :one
SELECT effect_id FROM play.action_effect_events WHERE action_id = @action_id;

-- name: SessionLog :many
SELECT a.seq, a.kind, a.actor_name, a.origin, a.client, a.created_at,
    -- The one token the Action touched; the nil UUID when it touched none.
    coalesce(t.token_id, (SELECT h.token_id FROM play.action_hp_events h WHERE h.action_id = a.id), '00000000-0000-0000-0000-000000000000')::uuid AS token_id,
    coalesce(t.label, (SELECT string_agg(s.label, ', ' ORDER BY s.label) FROM play.action_spawn_events s WHERE s.action_id = a.id), '')::text AS label,
    (EXISTS (SELECT 1 FROM play.action_undos u WHERE u.undoes_action_id = a.id)
        OR EXISTS (SELECT 1 FROM play.action_hp_events h WHERE h.undoes_action_id = a.id))::boolean AS undone
FROM play.actions a
LEFT JOIN play.action_token_events t ON t.action_id = a.id
WHERE a.session_id = @session_id
ORDER BY a.seq DESC
LIMIT @lim;
