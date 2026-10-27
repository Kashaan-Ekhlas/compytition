-- name: CreateSubmission :one
INSERT INTO submissions (
    event_id, registration_id, team_id, created_by, title, description,
    repo_url, demo_url, file_url, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetSubmissionByID :one
SELECT * FROM submissions WHERE id = $1;

-- name: GetSubmissionByRegistration :one
SELECT * FROM submissions WHERE registration_id = $1;

-- name: GetSubmissionByTeam :one
SELECT * FROM submissions WHERE event_id = $1 AND team_id = $2;

-- name: GetSubmissionForUser :one
SELECT s.* FROM submissions s
WHERE s.event_id = $1
  AND (
      s.created_by = $2
      OR (s.team_id IS NOT NULL AND EXISTS (
          SELECT 1 FROM team_members m WHERE m.team_id = s.team_id AND m.user_id = $2
      ))
  )
LIMIT 1;

-- name: UpdateSubmission :one
UPDATE submissions
SET title = $2, description = $3, repo_url = $4, demo_url = $5,
    file_url = $6, status = $7, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateSubmissionStatus :exec
UPDATE submissions SET status = $2, updated_at = now() WHERE id = $1;

-- name: ListSubmissionsForEvent :many
SELECT s.id, s.event_id, s.registration_id, s.team_id, s.created_by, s.title,
       s.description, s.repo_url, s.demo_url, s.file_url, s.status,
       s.created_at, s.updated_at,
       u.username AS creator_username, p.full_name AS creator_name,
       t.name AS team_name,
       count(*) OVER() AS total_count
FROM submissions s
JOIN users u ON u.id = s.created_by
LEFT JOIN profiles p ON p.user_id = s.created_by
LEFT JOIN teams t ON t.id = s.team_id
WHERE s.event_id = $1
  AND (sqlc.narg('status')::text IS NULL OR s.status = sqlc.narg('status')::text)
ORDER BY s.created_at ASC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: ListSubmissionsForJudging :many
SELECT s.id, s.event_id, s.registration_id, s.team_id, s.created_by, s.title,
       s.description, s.repo_url, s.demo_url, s.file_url, s.status,
       s.created_at, s.updated_at,
       u.username AS creator_username, p.full_name AS creator_name,
       t.name AS team_name
FROM submissions s
JOIN users u ON u.id = s.created_by
LEFT JOIN profiles p ON p.user_id = s.created_by
LEFT JOIN teams t ON t.id = s.team_id
WHERE s.event_id = $1
  AND s.status = 'submitted'
  AND EXISTS (
      SELECT 1 FROM registrations r
      WHERE r.id = s.registration_id AND r.status = 'registered'
  )
ORDER BY s.created_at ASC;

-- name: ListMySubmissions :many
SELECT s.id, s.event_id, s.registration_id, s.team_id, s.created_by, s.title,
       s.description, s.repo_url, s.demo_url, s.file_url, s.status,
       s.created_at, s.updated_at,
       e.title AS event_title, e.slug AS event_slug, e.submission_deadline,
       t.name AS team_name
FROM submissions s
JOIN events e ON e.id = s.event_id
LEFT JOIN teams t ON t.id = s.team_id
WHERE s.created_by = $1
   OR (s.team_id IS NOT NULL AND EXISTS (
       SELECT 1 FROM team_members m WHERE m.team_id = s.team_id AND m.user_id = $1
   ))
ORDER BY s.created_at DESC;

-- name: CountEventSubmissions :one
SELECT count(*)::int AS count FROM submissions WHERE event_id = $1 AND status = 'submitted';

-- name: CountSubmissionJudgments :one
SELECT count(DISTINCT p.id)::int AS count
FROM judging_pairs p
LEFT JOIN judgments j ON j.pair_id = p.id AND j.is_current
WHERE (p.submission_a = $1 OR p.submission_b = $1) AND j.winner_submission_id IS NOT NULL;
