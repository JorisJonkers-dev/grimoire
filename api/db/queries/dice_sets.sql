-- name: InsertDiceSet :exec
INSERT INTO social.dice_sets (id, owner_account, name, design, image_key, image_type, sharing, review, copied_from, made_by, created_at, updated_at)
VALUES (@id, @owner_account, @name, @design, sqlc.narg(image_key), sqlc.narg(image_type), 'private', @review, sqlc.narg(copied_from), @made_by, @now, @now);

-- name: DiceSet :one
SELECT id, owner_account, name, design, image_key, image_type, sharing, review, copied_from, made_by, created_at, updated_at
FROM social.dice_sets WHERE id = @id;

-- name: DiceSetsOf :many
SELECT id, owner_account, name, design, image_key, image_type, sharing, review, copied_from, made_by, created_at, updated_at
FROM social.dice_sets WHERE owner_account = @owner_account ORDER BY lower(name), id;

-- name: DiceSetsSharedWith :many
-- Sets of others that an Account may see: those of its Friends shared with Friends or with everyone,
-- and those shared with everyone that carry no picture or a picture an Admin approved.
SELECT d.id, d.owner_account, d.name, d.design, d.image_key, d.image_type, d.sharing, d.review, d.copied_from, d.made_by, d.created_at, d.updated_at
FROM social.dice_sets d
WHERE d.owner_account <> @me AND d.copied_from IS NULL AND (
    (d.sharing = 'everyone' AND (d.image_key IS NULL OR d.review = 'approved'))
    OR (d.sharing IN ('friends', 'everyone') AND EXISTS (
        SELECT 1 FROM social.friendships f
        WHERE (f.a = d.owner_account AND f.b = @me) OR (f.b = d.owner_account AND f.a = @me)))
)
ORDER BY lower(d.name), d.id
LIMIT 200;

-- name: DiceSetsAwaitingReview :many
SELECT id, owner_account, name, design, image_key, image_type, sharing, review, copied_from, made_by, created_at, updated_at
FROM social.dice_sets WHERE review = 'pending' ORDER BY updated_at, id LIMIT 200;

-- name: UpdateDiceSet :exec
UPDATE social.dice_sets SET name = @name, design = @design, updated_at = @now WHERE id = @id;

-- name: SetDiceSetSharing :exec
-- The review follows from the row as it is now, so a picture uploaded meanwhile is never missed.
UPDATE social.dice_sets SET sharing = @sharing,
    review = CASE WHEN @sharing::text = 'everyone' AND image_key IS NOT NULL THEN 'pending' ELSE 'none' END, updated_at = @now
WHERE id = @id;

-- name: SetDiceSetImage :exec
UPDATE social.dice_sets SET image_key = sqlc.narg(image_key), image_type = sqlc.narg(image_type),
    review = CASE WHEN sharing = 'everyone' AND sqlc.narg(image_key)::text IS NOT NULL THEN 'pending' ELSE 'none' END, updated_at = @now
WHERE id = @id;

-- name: SetDiceSetReview :execrows
-- Decides on a set that waits, and only on the picture the Admin looked at.
UPDATE social.dice_sets SET review = @review, updated_at = @now WHERE id = @id AND review = 'pending' AND image_key = @image_key::text;

-- name: DeleteDiceSet :exec
DELETE FROM social.dice_sets WHERE id = @id;

-- name: ChooseDiceSet :exec
INSERT INTO social.dice_set_choices (account_id, dice_set_id) VALUES (@account_id, @dice_set_id)
ON CONFLICT (account_id) DO UPDATE SET dice_set_id = excluded.dice_set_id;

-- name: UnchooseDiceSet :exec
DELETE FROM social.dice_set_choices WHERE account_id = @account_id;

-- name: ChosenDiceSet :one
SELECT d.id, d.owner_account, d.name, d.design, d.image_key, d.image_type, d.sharing, d.review, d.copied_from, d.made_by, d.created_at, d.updated_at
FROM social.dice_set_choices c JOIN social.dice_sets d ON d.id = c.dice_set_id WHERE c.account_id = @account_id;
