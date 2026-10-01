-- name: UpsertPushSubscription :one
INSERT INTO campaign.push_subscriptions (subject, endpoint, p256dh, auth)
VALUES (@subject, @endpoint, @p256dh, @auth)
ON CONFLICT (endpoint) DO UPDATE SET subject = EXCLUDED.subject, p256dh = EXCLUDED.p256dh, auth = EXCLUDED.auth
RETURNING id;

-- name: DeletePushSubscription :execrows
DELETE FROM campaign.push_subscriptions WHERE id = @id AND subject = @subject;

-- name: PushSubscriptions :many
SELECT id, endpoint, p256dh, auth FROM campaign.push_subscriptions WHERE subject = @subject ORDER BY created_at, id;

-- name: DropPushEndpoint :exec
DELETE FROM campaign.push_subscriptions WHERE endpoint = @endpoint;
