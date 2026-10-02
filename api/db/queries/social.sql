-- name: SocialAccountBySubject :one
SELECT id, username, nickname FROM identity.accounts WHERE subject = @subject AND NOT disabled;

-- name: SocialAccountByUsername :one
SELECT id, username, nickname FROM identity.accounts WHERE username = @username AND NOT disabled;

-- name: InsertFriendRequest :execrows
INSERT INTO social.friend_requests (id, from_account, to_account, status, created_at) VALUES (@id, @from_account, @to_account, 'pending', @now)
ON CONFLICT DO NOTHING;

-- name: PendingRequest :one
SELECT id, from_account, to_account FROM social.friend_requests WHERE id = @id AND status = 'pending';

-- name: PendingBetween :one
SELECT id FROM social.friend_requests WHERE from_account = @from_account AND to_account = @to_account AND status = 'pending';

-- name: DeleteFriendRequest :exec
DELETE FROM social.friend_requests WHERE id = @id;

-- name: DeclineFriendRequest :exec
UPDATE social.friend_requests SET status = 'declined', decided_at = @now WHERE id = @id;

-- name: InsertFriendship :exec
INSERT INTO social.friendships (a, b, since) VALUES (least(@x::uuid, @y::uuid), greatest(@x::uuid, @y::uuid), @now) ON CONFLICT DO NOTHING;

-- name: DeleteFriendship :execrows
DELETE FROM social.friendships WHERE a = least(@x::uuid, @y::uuid) AND b = greatest(@x::uuid, @y::uuid);

-- name: AreFriends :one
SELECT EXISTS (SELECT 1 FROM social.friendships WHERE a = least(@x::uuid, @y::uuid) AND b = greatest(@x::uuid, @y::uuid));

-- name: IsBlocked :one
SELECT EXISTS (SELECT 1 FROM social.blocks WHERE blocker = @blocker AND blocked = @blocked);

-- name: InsertBlock :exec
INSERT INTO social.blocks (blocker, blocked, created_at) VALUES (@blocker, @blocked, @now) ON CONFLICT DO NOTHING;

-- name: DeleteBlock :execrows
DELETE FROM social.blocks WHERE blocker = @blocker AND blocked = @blocked;

-- name: ListFriends :many
SELECT a.id, a.username, a.nickname, f.since FROM social.friendships f
JOIN identity.accounts a ON a.id = CASE WHEN f.a = @me THEN f.b ELSE f.a END
WHERE f.a = @me OR f.b = @me ORDER BY lower(a.nickname), a.username;

-- name: ListIncomingRequests :many
SELECT r.id, a.id AS account_id, a.username, a.nickname, r.created_at FROM social.friend_requests r
JOIN identity.accounts a ON a.id = r.from_account
WHERE r.to_account = @me AND r.status = 'pending'
    AND NOT EXISTS (SELECT 1 FROM social.blocks b WHERE b.blocker = @me AND b.blocked = r.from_account)
ORDER BY r.created_at DESC;

-- name: ListOutgoingRequests :many
SELECT r.id, a.id AS account_id, a.username, a.nickname, r.created_at FROM social.friend_requests r
JOIN identity.accounts a ON a.id = r.to_account
WHERE r.from_account = @me AND r.status = 'pending' ORDER BY r.created_at DESC;

-- name: ListBlocked :many
SELECT a.id, a.username, a.nickname, b.created_at FROM social.blocks b JOIN identity.accounts a ON a.id = b.blocked
WHERE b.blocker = @me ORDER BY lower(a.nickname), a.username;
