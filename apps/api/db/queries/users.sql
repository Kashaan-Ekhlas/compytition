-- name: CreateUser :one
INSERT INTO users (email, username)
VALUES ($1, $2)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: UsernameTaken :one
SELECT EXISTS (SELECT 1 FROM users WHERE username = $1) AS taken;

-- name: UpdateUsername :exec
UPDATE users SET username = $2, updated_at = now() WHERE id = $1;

-- name: UpdateOnboarding :exec
UPDATE users
SET onboarding_step = $2, onboarding_completed = $3, intents = $4, updated_at = now()
WHERE id = $1;

-- name: CreateIdentity :exec
INSERT INTO auth_identities (user_id, provider, provider_user_id, secret_hash)
VALUES ($1, $2, $3, $4);

-- name: GetIdentity :one
SELECT * FROM auth_identities WHERE provider = $1 AND provider_user_id = $2;

-- name: GetIdentityForUser :one
SELECT * FROM auth_identities WHERE user_id = $1 AND provider = $2;

-- name: UpdateIdentitySecret :exec
UPDATE auth_identities SET secret_hash = $2 WHERE id = $1;

-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetSessionByHash :one
SELECT s.id, s.user_id, s.expires_at, s.created_at, s.last_seen_at,
       u.email, u.username, u.onboarding_completed, u.onboarding_step,
       u.intents, u.is_admin
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1 AND s.expires_at > now();

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = now() WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: DeleteSessionByID :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < now();

-- name: GetProfile :one
SELECT * FROM profiles WHERE user_id = $1;

-- name: UpsertProfile :one
INSERT INTO profiles (user_id, full_name, college, course, year_of_study)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id) DO UPDATE
SET full_name = EXCLUDED.full_name,
    college = EXCLUDED.college,
    course = EXCLUDED.course,
    year_of_study = EXCLUDED.year_of_study,
    updated_at = now()
RETURNING *;

-- name: CreateOAuthState :exec
INSERT INTO oauth_states (state, code_verifier, redirect_to, expires_at)
VALUES ($1, $2, $3, $4);

-- name: ConsumeOAuthState :one
DELETE FROM oauth_states
WHERE state = $1 AND expires_at > now()
RETURNING *;

-- name: GetLoginAttempt :one
SELECT * FROM login_attempts WHERE key = $1;

-- name: RecordLoginFailure :one
INSERT INTO login_attempts (key, attempts, first_attempt_at, locked_until)
VALUES ($1, 1, now(), $2)
ON CONFLICT (key) DO UPDATE
SET attempts = CASE
        WHEN login_attempts.first_attempt_at < now() - interval '15 minutes' THEN 1
        ELSE login_attempts.attempts + 1
    END,
    first_attempt_at = CASE
        WHEN login_attempts.first_attempt_at < now() - interval '15 minutes' THEN now()
        ELSE login_attempts.first_attempt_at
    END,
    locked_until = $2
RETURNING *;

-- name: ClearLoginAttempts :exec
DELETE FROM login_attempts WHERE key = $1;

-- name: MakeUserAdmin :exec
UPDATE users SET is_admin = TRUE WHERE id = $1;

-- name: DeleteOtherSessions :exec
DELETE FROM sessions WHERE user_id = $1 AND id <> $2;
