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

-- name: AccountHasPassword :one
SELECT (password_hash IS NOT NULL)::boolean AS has_password FROM identity.accounts WHERE id = @id;

-- name: UpdateAccountProfile :exec
UPDATE identity.accounts SET username = @username, nickname = @nickname, email = @email WHERE id = @id;

-- name: SetAccountAdmin :exec
UPDATE identity.accounts SET admin = @admin WHERE id = @id;

-- name: InsertOIDCRequest :exec
INSERT INTO identity.oidc_requests (state_hash, nonce, verifier, account_id, created_at, expires_at)
VALUES (@state_hash, @nonce, @verifier, sqlc.narg(account_id), @now, @expires_at);

-- name: UseOIDCRequest :one
UPDATE identity.oidc_requests SET used_at = @now WHERE state_hash = @state_hash AND used_at IS NULL AND expires_at > @now
RETURNING nonce, verifier, account_id;

-- name: OIDCLinkByAccount :one
SELECT account_id, issuer, subject, email, username, name, linked_at FROM identity.oidc_links WHERE account_id = @account_id;

-- name: OIDCLinkBySubject :one
SELECT account_id, issuer, subject, email, username, name, linked_at FROM identity.oidc_links WHERE issuer = @issuer AND subject = @subject;

-- name: InsertOIDCLink :exec
INSERT INTO identity.oidc_links (account_id, issuer, subject, email, username, name, linked_at)
VALUES (@account_id, @issuer, @subject, @email, @username, @name, @now);

-- name: UpdateOIDCLink :exec
UPDATE identity.oidc_links SET email = @email, username = @username, name = @name WHERE account_id = @account_id;

-- name: DeleteOIDCLink :execrows
DELETE FROM identity.oidc_links WHERE account_id = @account_id;

-- name: InsertOIDCPending :exec
INSERT INTO identity.oidc_pending (token_hash, issuer, subject, email, username, name, admin, created_at, expires_at)
VALUES (@token_hash, @issuer, @subject, @email, @username, @name, @admin, @now, @expires_at);

-- name: UseOIDCPending :one
UPDATE identity.oidc_pending SET used_at = @now WHERE token_hash = @token_hash AND used_at IS NULL AND expires_at > @now
RETURNING issuer, subject, email, username, name, admin;
