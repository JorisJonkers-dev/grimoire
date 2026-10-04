-- name: InsertSpawnEvent :exec
INSERT INTO play.action_spawn_events (action_id, token_id, label) VALUES (@action_id, @token_id, @label);

-- name: InsertEffectEvent :exec
INSERT INTO play.action_effect_events (action_id, effect_id) VALUES (@action_id, @effect_id);

-- name: InsertUndo :exec
INSERT INTO play.action_undos (action_id, undoes_action_id) VALUES (@action_id, @undoes_action_id);

-- name: SessionActionBySeq :one
SELECT a.id, a.kind,
    (EXISTS (SELECT 1 FROM play.action_undos u WHERE u.undoes_action_id = a.id)
        OR EXISTS (SELECT 1 FROM play.action_hp_events h WHERE h.undoes_action_id = a.id))::boolean AS undone,
    -- A rewind to before this Action, made after it, took it back.
    EXISTS (SELECT 1 FROM play.rewinds w JOIN play.actions wa ON wa.id = w.action_id
        WHERE w.session_id = a.session_id AND w.to_action_seq < a.seq AND wa.seq > a.seq)::boolean AS rewound
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
        OR EXISTS (SELECT 1 FROM play.action_hp_events h WHERE h.undoes_action_id = a.id)
        -- A rewind to before this Action, made after it, took it back with everything else.
        OR EXISTS (SELECT 1 FROM play.rewinds w JOIN play.actions wa ON wa.id = w.action_id
            WHERE w.session_id = a.session_id AND w.to_action_seq < a.seq AND wa.seq > a.seq))::boolean AS undone
FROM play.actions a
LEFT JOIN play.action_token_events t ON t.action_id = a.id
WHERE a.session_id = @session_id
ORDER BY a.seq DESC
LIMIT @lim;

-- name: ListCheckpoints :many
SELECT id, name, kind, round, action_seq, created_at FROM play.checkpoints WHERE session_id = @session_id ORDER BY action_seq, created_at, id;

-- name: DropLaterCheckpoints :exec
-- A rewind leaves no Checkpoint of the future it took back.
DELETE FROM play.checkpoints WHERE session_id = @session_id AND action_seq > @action_seq;

-- name: InsertRewind :exec
INSERT INTO play.rewinds (action_id, session_id, to_action_seq) VALUES (@action_id, @session_id, @to_action_seq);

-- name: CampaignNoUndo :one
SELECT no_undo FROM campaign.campaigns WHERE id = $1;

-- name: DropOldRounds :exec
-- A Session keeps the start of its latest rounds only.
DELETE FROM play.checkpoints c WHERE c.session_id = @session_id AND c.kind = 'round' AND c.id NOT IN (
    SELECT k.id FROM play.checkpoints k WHERE k.session_id = @session_id AND k.kind = 'round' ORDER BY k.action_seq DESC, k.created_at DESC LIMIT @keep);

-- name: CampaignShowDCs :one
SELECT show_dcs FROM campaign.campaigns WHERE id = $1;

-- name: CampaignDifficulty :one
SELECT difficulty FROM campaign.campaigns WHERE id = $1;

-- name: SessionAttitudes :many
-- The attitudes of the creatures on a Session's map towards the Campaign's Characters.
SELECT a.token_id, a.character_id, a.attitude
FROM play.token_attitudes a JOIN play.tokens t ON t.id = a.token_id
WHERE t.session_id = $1 ORDER BY a.token_id, a.character_id;

-- name: SetAttitude :exec
-- Only a creature of the Session takes an attitude, and only towards a Character of the Session's Campaign.
INSERT INTO play.token_attitudes (token_id, character_id, attitude)
SELECT t.id, c.id, @attitude
FROM play.tokens t JOIN play.sessions s ON s.id = t.session_id JOIN campaign.characters c ON c.campaign_id = s.campaign_id
WHERE t.session_id = @session_id AND t.id = @token_id AND c.id = @character_id
ON CONFLICT (token_id, character_id) DO UPDATE SET attitude = excluded.attitude;
