-- name: GetInstanceCreatedAt :one
SELECT created_at FROM ops.instance WHERE id = 1;
