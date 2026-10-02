-- name: InsertAccount :one
INSERT INTO identity.accounts (id, subject, username, nickname, email, password_hash, admin, created_at)
VALUES (@id, @subject, @username, @nickname, @email, sqlc.narg(password_hash), @admin, @now)
RETURNING id, subject, username, nickname, email, admin, disabled, created_at;

-- name: AccountByUsername :one
SELECT id, subject, username, nickname, email, password_hash, admin, disabled, created_at FROM identity.accounts WHERE username = @username;

-- name: AccountByEmail :one
SELECT id, subject, username, nickname, email, admin, disabled, created_at FROM identity.accounts WHERE lower(email) = lower(@email);

-- name: AccountBySubject :one
SELECT id, subject, username, nickname, email, admin, disabled, created_at FROM identity.accounts WHERE subject = @subject;

-- name: AccountByID :one
SELECT id, subject, username, nickname, email, admin, disabled, created_at FROM identity.accounts WHERE id = @id;

-- name: SetAccountPassword :exec
UPDATE identity.accounts SET password_hash = @password_hash WHERE id = @id;

-- name: InsertInvite :exec
INSERT INTO identity.invites (id, token_hash, created_by, admin, created_at, expires_at) VALUES (@id, @token_hash, @created_by, @admin, @now, @expires_at);

-- name: InviteByToken :one
SELECT id, created_by, admin, created_at, expires_at, used_at FROM identity.invites WHERE token_hash = @token_hash;

-- name: UseInvite :execrows
UPDATE identity.invites SET used_at = @now, account_id = @account_id WHERE id = @id AND used_at IS NULL AND expires_at > @now;

-- name: InsertAccountSession :exec
INSERT INTO identity.account_sessions (id, account_id, token_hash, user_agent, created_at, last_seen_at, expires_at)
VALUES (@id, @account_id, @token_hash, @user_agent, @now, @now, @expires_at);

-- name: SessionAccount :one
SELECT s.id, a.subject, a.disabled FROM identity.account_sessions s JOIN identity.accounts a ON a.id = s.account_id
WHERE s.token_hash = @token_hash AND s.revoked_at IS NULL AND s.expires_at > @now;

-- name: TouchAccountSession :exec
UPDATE identity.account_sessions SET last_seen_at = @now WHERE id = @id AND last_seen_at < @cutoff;

-- name: RevokeAccountSession :exec
UPDATE identity.account_sessions SET revoked_at = @now WHERE token_hash = @token_hash AND revoked_at IS NULL;

-- name: InsertSignInLink :exec
INSERT INTO identity.sign_in_links (token_hash, account_id, created_at, expires_at) VALUES (@token_hash, @account_id, @now, @expires_at);

-- name: UseSignInLink :one
UPDATE identity.sign_in_links SET used_at = @now WHERE token_hash = @token_hash AND used_at IS NULL AND expires_at > @now RETURNING account_id;
