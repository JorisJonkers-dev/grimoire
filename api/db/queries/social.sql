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

-- name: InsertConversation :exec
INSERT INTO social.conversations (id, title, created_by, created_at, updated_at) VALUES (@id, @title, @created_by, @now, @now);

-- name: AddConversationMember :exec
INSERT INTO social.conversation_members (conversation_id, account_id, joined_at, last_read_at) VALUES (@conversation_id, @account_id, @now, @now);

-- name: DirectConversation :one
-- The one-to-one Conversation between two Accounts, if they have one.
SELECT c.id FROM social.conversations c
WHERE (SELECT count(*) FROM social.conversation_members m WHERE m.conversation_id = c.id) = 2
    AND EXISTS (SELECT 1 FROM social.conversation_members m WHERE m.conversation_id = c.id AND m.account_id = @x)
    AND EXISTS (SELECT 1 FROM social.conversation_members m WHERE m.conversation_id = c.id AND m.account_id = @y)
    AND c.title = ''
LIMIT 1;

-- name: IsConversationMember :one
SELECT EXISTS (SELECT 1 FROM social.conversation_members WHERE conversation_id = @conversation_id AND account_id = @account_id);

-- name: ListConversations :many
SELECT c.id, c.title, c.updated_at,
    (SELECT count(*) FROM social.messages msg WHERE msg.conversation_id = c.id AND msg.author <> @me AND msg.created_at > me.last_read_at)::integer AS unread,
    coalesce((SELECT left(msg.body, 120) FROM social.messages msg WHERE msg.conversation_id = c.id ORDER BY msg.created_at DESC, msg.id DESC LIMIT 1), '')::text AS last_body
FROM social.conversations c JOIN social.conversation_members me ON me.conversation_id = c.id AND me.account_id = @me
ORDER BY c.updated_at DESC, c.id;

-- name: ConversationMembers :many
SELECT m.conversation_id, a.id, a.username, a.nickname FROM social.conversation_members m JOIN identity.accounts a ON a.id = m.account_id
WHERE m.conversation_id = ANY(@ids::uuid[]) ORDER BY lower(a.nickname), a.username;

-- name: InsertMessage :exec
INSERT INTO social.messages (id, conversation_id, author, body, created_at) VALUES (@id, @conversation_id, @author, @body, @now);

-- name: TouchConversation :exec
UPDATE social.conversations SET updated_at = @now WHERE id = @id;

-- name: InsertMention :exec
INSERT INTO social.message_mentions (message_id, ordinal, kind, campaign_id, target_id) VALUES (@message_id, @ordinal, @kind, @campaign_id, @target_id);

-- name: ListMessages :many
SELECT msg.id, msg.author, a.username, a.nickname, msg.body, msg.created_at FROM social.messages msg JOIN identity.accounts a ON a.id = msg.author
WHERE msg.conversation_id = @conversation_id AND msg.created_at < @before ORDER BY msg.created_at DESC, msg.id DESC LIMIT @lim;

-- name: MessageMentions :many
SELECT message_id, ordinal, kind, campaign_id, target_id FROM social.message_mentions WHERE message_id = ANY(@ids::uuid[]) ORDER BY message_id, ordinal;

-- name: MarkConversationRead :exec
UPDATE social.conversation_members SET last_read_at = @now WHERE conversation_id = @conversation_id AND account_id = @account_id;

-- name: MentionedCharacter :one
-- A Campaign Character a reader may open: they are a Member of its Campaign.
SELECT c.name FROM campaign.characters c JOIN campaign.members m ON m.campaign_id = c.campaign_id
JOIN identity.accounts a ON a.subject = m.auth_subject
WHERE c.id = @target_id AND c.campaign_id = @campaign_id AND a.id = @reader;

-- name: MentionedLocation :one
-- A Location a reader may open: they are a DM of its Campaign.
SELECT n.name, mp.id AS map_id FROM campaign.map_nodes n JOIN campaign.maps mp ON mp.id = n.map_id
JOIN campaign.members m ON m.campaign_id = mp.campaign_id AND m.role = 'dm'
JOIN identity.accounts a ON a.subject = m.auth_subject
WHERE n.id = @target_id AND mp.campaign_id = @campaign_id AND a.id = @reader;

-- name: MentionableCharacters :many
SELECT c.id, c.name, c.campaign_id, cp.name AS campaign_name FROM campaign.characters c
JOIN campaign.campaigns cp ON cp.id = c.campaign_id
JOIN campaign.members m ON m.campaign_id = c.campaign_id JOIN identity.accounts a ON a.subject = m.auth_subject
WHERE a.id = @reader AND c.name ILIKE '%' || @q::text || '%' ORDER BY c.name LIMIT 20;

-- name: MentionableLocations :many
SELECT n.id, n.name, mp.campaign_id, cp.name AS campaign_name FROM campaign.map_nodes n
JOIN campaign.maps mp ON mp.id = n.map_id AND mp.kind = 'world' JOIN campaign.campaigns cp ON cp.id = mp.campaign_id
JOIN campaign.members m ON m.campaign_id = mp.campaign_id AND m.role = 'dm' JOIN identity.accounts a ON a.subject = m.auth_subject
WHERE a.id = @reader AND n.name ILIKE '%' || @q::text || '%' ORDER BY n.name LIMIT 20;

-- name: UpsertNotification :exec
INSERT INTO social.notifications (id, account_id, kind, title, body, action_label, action_path, dedupe_key, created_at)
VALUES (@id, @account_id, @kind, @title, @body, @action_label, @action_path, @dedupe_key, @now)
ON CONFLICT (account_id, dedupe_key) WHERE read_at IS NULL AND dedupe_key <> ''
DO UPDATE SET title = EXCLUDED.title, body = EXCLUDED.body, action_label = EXCLUDED.action_label,
    action_path = EXCLUDED.action_path, created_at = EXCLUDED.created_at;

-- name: ListNotifications :many
SELECT id, kind, title, body, action_label, action_path, created_at, read_at FROM social.notifications
WHERE account_id = @account_id ORDER BY created_at DESC, id LIMIT 50;

-- name: UnreadNotifications :one
SELECT count(*)::integer AS unread FROM social.notifications WHERE account_id = @account_id AND read_at IS NULL;

-- name: ReadNotification :execrows
UPDATE social.notifications SET read_at = @now WHERE id = @id AND account_id = @account_id AND read_at IS NULL;

-- name: ReadAllNotifications :exec
UPDATE social.notifications SET read_at = @now WHERE account_id = @account_id AND read_at IS NULL;

-- name: NotificationPreferences :many
SELECT kind, channel, enabled FROM social.notification_preferences WHERE account_id = @account_id;

-- name: SetNotificationPreference :exec
INSERT INTO social.notification_preferences (account_id, kind, channel, enabled) VALUES (@account_id, @kind, @channel, @enabled)
ON CONFLICT (account_id, kind, channel) DO UPDATE SET enabled = EXCLUDED.enabled;

-- name: ConversationTitle :one
SELECT title FROM social.conversations WHERE id = @id;
