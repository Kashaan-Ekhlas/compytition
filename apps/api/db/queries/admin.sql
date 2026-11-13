-- name: AdminOverview :one
SELECT
    (SELECT count(*)::int FROM users) AS users,
    (SELECT count(*)::int FROM events) AS events,
    (SELECT count(*)::int FROM submissions) AS submissions,
    (SELECT count(*)::int FROM judgments WHERE is_current = TRUE) AS judgments,
    (SELECT count(*)::int FROM uploads) AS uploads;

-- name: CountSeedEvents :one
SELECT count(*)::int AS count
FROM events
WHERE organizer_id = $1
  AND slug LIKE $2;

-- name: CountSeedUsers :one
SELECT count(*)::int AS count
FROM users
WHERE is_admin = FALSE
  AND email LIKE $1;

-- name: ListSeedEvents :many
SELECT id, slug, title, category, participation_format, status, is_remote,
       country, state, city, registration_opens_at, registration_closes_at,
       starts_at, ends_at, submission_deadline, judging_status,
       results_published_at, created_at
FROM events
WHERE organizer_id = $1
  AND slug LIKE $2
ORDER BY created_at ASC;

-- name: DeleteSeedEvents :execrows
DELETE FROM events
WHERE organizer_id = $1
  AND slug LIKE $2;

-- name: DeleteSeedUsers :execrows
-- Only remove seed accounts that no remaining event still references. Two
-- admins share the same participant and judge pool, so deleting a user who
-- still supports another admin's events would cascade their data away.
DELETE FROM users u
WHERE u.is_admin = FALSE
  AND u.email LIKE $1
  AND NOT EXISTS (SELECT 1 FROM event_staff s WHERE s.user_id = u.id)
  AND NOT EXISTS (SELECT 1 FROM registrations r WHERE r.user_id = u.id)
  AND NOT EXISTS (SELECT 1 FROM team_members m WHERE m.user_id = u.id)
  AND NOT EXISTS (SELECT 1 FROM submissions sub WHERE sub.created_by = u.id)
  AND NOT EXISTS (SELECT 1 FROM teams t WHERE t.owner_id = u.id);
