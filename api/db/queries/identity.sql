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
INSERT INTO identity.account_sessions (id, account_id, token_hash, user_agent, created_at, last_seen_at, expires_at, strong)
VALUES (@id, @account_id, @token_hash, @user_agent, @now, @now, @expires_at, @strong);

-- name: SessionAccount :one
SELECT s.id, a.subject, a.disabled, s.strong FROM identity.account_sessions s JOIN identity.accounts a ON a.id = s.account_id
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

-- name: StrengthenSession :exec
UPDATE identity.account_sessions SET strong = true WHERE token_hash = @token_hash AND account_id = @account_id;

-- name: StartTOTP :execrows
INSERT INTO identity.totp_factors (account_id, secret, created_at) VALUES (@account_id, @secret, @now)
ON CONFLICT (account_id) DO UPDATE SET secret = EXCLUDED.secret, created_at = EXCLUDED.created_at
WHERE identity.totp_factors.confirmed_at IS NULL;

-- name: TOTPFactor :one
SELECT secret, confirmed_at, last_step FROM identity.totp_factors WHERE account_id = @account_id;

-- name: ConfirmTOTP :execrows
UPDATE identity.totp_factors SET confirmed_at = @now, last_step = @step WHERE account_id = @account_id AND confirmed_at IS NULL;

-- name: UseTOTPStep :execrows
UPDATE identity.totp_factors SET last_step = @step WHERE account_id = @account_id AND confirmed_at IS NOT NULL AND last_step < @step;

-- name: DeleteTOTP :exec
DELETE FROM identity.totp_factors WHERE account_id = @account_id;

-- name: DeleteRecoveryCodes :exec
DELETE FROM identity.recovery_codes WHERE account_id = @account_id;

-- name: InsertRecoveryCode :exec
INSERT INTO identity.recovery_codes (code_hash, account_id) VALUES (@code_hash, @account_id);

-- name: UseRecoveryCode :execrows
UPDATE identity.recovery_codes SET used_at = @now WHERE code_hash = @code_hash AND account_id = @account_id AND used_at IS NULL;

-- name: RecoveryCodesLeft :one
SELECT count(*)::integer AS left_count FROM identity.recovery_codes WHERE account_id = @account_id AND used_at IS NULL;

-- name: InsertTwoStepChallenge :exec
INSERT INTO identity.two_step_challenges (token_hash, account_id, created_at, expires_at) VALUES (@token_hash, @account_id, @now, @expires_at);

-- name: TryTwoStepChallenge :one
UPDATE identity.two_step_challenges SET attempts = attempts + 1
WHERE token_hash = @token_hash AND used_at IS NULL AND expires_at > @now AND attempts < @max_attempts::integer
RETURNING account_id;

-- name: UseTwoStepChallenge :exec
UPDATE identity.two_step_challenges SET used_at = @now WHERE token_hash = @token_hash;

-- name: InsertAccessToken :exec
INSERT INTO identity.access_tokens (id, account_id, name, scopes, token_hash, created_at, expires_at)
VALUES (@id, @account_id, @name, @scopes::text[], @token_hash, @now, @expires_at);

-- name: ListAccessTokens :many
SELECT id, name, scopes, created_at, expires_at, last_used_at FROM identity.access_tokens
WHERE account_id = @account_id AND revoked_at IS NULL AND expires_at > @now ORDER BY created_at DESC;

-- name: RevokeAccessToken :execrows
UPDATE identity.access_tokens SET revoked_at = @now WHERE id = @id AND account_id = @account_id AND revoked_at IS NULL;

-- name: AccessTokenAccount :one
SELECT t.id, t.scopes, t.last_used_at, a.subject, a.disabled FROM identity.access_tokens t JOIN identity.accounts a ON a.id = t.account_id
WHERE t.token_hash = @token_hash AND t.revoked_at IS NULL AND t.expires_at > @now;

-- name: TouchAccessToken :exec
UPDATE identity.access_tokens SET last_used_at = @now WHERE id = @id AND (last_used_at IS NULL OR last_used_at < @cutoff);

-- name: InsertAccountEvent :exec
INSERT INTO identity.account_events (id, account_id, actor, action, detail, at) VALUES (@id, @account_id, @actor, @action, @detail, @at);

-- name: ListAccountEvents :many
SELECT e.at, e.actor, coalesce(a.username, '')::text AS actor_name, e.action, e.detail
FROM identity.account_events e LEFT JOIN identity.accounts a ON a.subject = e.actor
WHERE e.account_id = @account_id ORDER BY e.at DESC, e.id LIMIT 100;

-- name: ListAccounts :many
SELECT a.id, a.subject, a.username, a.nickname, a.email, a.admin, a.disabled, a.created_at, max(s.last_seen_at) AS last_seen_at
FROM identity.accounts a LEFT JOIN identity.account_sessions s ON s.account_id = a.id
GROUP BY a.id ORDER BY lower(a.nickname), a.username;

-- name: ListUnusedInvites :many
SELECT id, created_by, admin, created_at, expires_at FROM identity.invites WHERE used_at IS NULL ORDER BY created_at DESC LIMIT 200;

-- name: SetAccountDisabled :exec
UPDATE identity.accounts SET disabled = @disabled WHERE id = @id;

-- name: RevokeAccountSessions :exec
UPDATE identity.account_sessions SET revoked_at = @now WHERE account_id = @account_id AND revoked_at IS NULL;

-- name: RevokeAccountTokens :exec
UPDATE identity.access_tokens SET revoked_at = @now WHERE account_id = @account_id AND revoked_at IS NULL;

-- name: CountLiveSessions :one
SELECT count(*)::integer AS live FROM identity.account_sessions WHERE account_id = @account_id AND revoked_at IS NULL AND expires_at > @now;

-- name: CountLiveTokens :one
SELECT count(*)::integer AS live FROM identity.access_tokens WHERE account_id = @account_id AND revoked_at IS NULL AND expires_at > @now;

-- name: SeenUserAgent :one
SELECT count(*) FILTER (WHERE s.user_agent = @user_agent)::integer AS seen, count(*)::integer AS sessions
FROM identity.account_sessions s WHERE s.account_id = @account_id;
