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
SELECT id, label, kind, q, r, hidden, darkvision_ft, controller_member_id FROM play.tokens WHERE session_id = $1 ORDER BY label, id;

-- name: InsertToken :exec
INSERT INTO play.tokens (id, session_id, label, kind, q, r, hidden, darkvision_ft, controller_member_id)
VALUES (@id, @session_id, @label, @kind, @q, @r, @hidden, @darkvision_ft, @controller_member_id);

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
