-- name: CreateRegistration :one
INSERT INTO registrations (event_id, user_id, team_id, form_answers)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetRegistrationByID :one
SELECT * FROM registrations WHERE id = $1;

-- name: GetRegistrationForEventUser :one
SELECT * FROM registrations WHERE event_id = $1 AND user_id = $2;

-- name: GetTeamRegistration :one
SELECT * FROM registrations
WHERE event_id = $1 AND team_id = $2
ORDER BY created_at ASC
LIMIT 1;

-- name: ListRegistrationsForEvent :many
SELECT r.id, r.event_id, r.user_id, r.team_id, r.form_answers, r.status,
       r.created_at, r.updated_at,
       u.username, u.email, p.full_name, p.college,
       t.name AS team_name,
       (s.id IS NOT NULL)::boolean AS has_submission,
       count(*) OVER() AS total_count
FROM registrations r
JOIN users u ON u.id = r.user_id
LEFT JOIN profiles p ON p.user_id = u.id
LEFT JOIN teams t ON t.id = r.team_id
LEFT JOIN submissions s ON s.registration_id = r.id
WHERE r.event_id = $1
  AND (sqlc.narg('status')::text IS NULL OR r.status = sqlc.narg('status')::text)
ORDER BY r.created_at DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: ListMyRegistrations :many
SELECT r.id, r.event_id, r.user_id, r.team_id, r.form_answers, r.status,
       r.created_at, r.updated_at,
       e.title AS event_title, e.slug AS event_slug, e.status AS event_status,
       e.starts_at, e.registration_closes_at, e.submission_deadline,
       e.participation_format, t.name AS team_name,
       (s.id IS NOT NULL)::boolean AS has_submission
FROM registrations r
JOIN events e ON e.id = r.event_id
LEFT JOIN teams t ON t.id = r.team_id
LEFT JOIN submissions s ON s.event_id = r.event_id
    AND (s.registration_id = r.id OR (r.team_id IS NOT NULL AND s.team_id = r.team_id))
WHERE r.user_id = $1
ORDER BY r.created_at DESC;

-- name: UpdateRegistrationAnswers :exec
UPDATE registrations
SET form_answers = $2, updated_at = now()
WHERE id = $1 AND user_id = $3;

-- name: UpdateRegistrationStatus :exec
UPDATE registrations SET status = $2, updated_at = now() WHERE id = $1;

-- name: UpdateRegistrationTeam :exec
UPDATE registrations SET team_id = $2, updated_at = now() WHERE event_id = $3 AND user_id = $1;

-- name: DeleteRegistration :exec
DELETE FROM registrations WHERE id = $1;

-- name: CountEventRegistrations :one
SELECT count(*)::int AS count FROM registrations WHERE event_id = $1 AND status = 'registered';

-- name: GetRegistrationAnswers :one
SELECT form_answers FROM registrations WHERE event_id = $1 AND user_id = $2;

-- name: CreateRegistrationForTeamMember :one
INSERT INTO registrations (event_id, user_id, team_id, form_answers)
VALUES ($1, $2, $3, $4)
ON CONFLICT (event_id, user_id) DO UPDATE
SET team_id = EXCLUDED.team_id, form_answers = EXCLUDED.form_answers,
    status = 'registered', updated_at = now()
RETURNING *;

-- name: UpdateTeamRegistrationAnswers :exec
UPDATE registrations SET form_answers = $2, updated_at = now()
WHERE event_id = $1 AND team_id = $3;

-- name: ListTeamMemberIDs :many
SELECT user_id FROM team_members WHERE team_id = $1;
