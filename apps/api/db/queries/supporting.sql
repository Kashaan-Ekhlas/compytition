-- name: CreateAuditEntry :exec
INSERT INTO audit_log (actor_id, event_id, action, metadata)
VALUES ($1, $2, $3, $4);

-- name: ListAuditForEvent :many
SELECT a.*, u.username AS actor_username
FROM audit_log a
LEFT JOIN users u ON u.id = a.actor_id
WHERE a.event_id = $1
ORDER BY a.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CreateFeedback :one
INSERT INTO feedback_reports (user_id, subject, body)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListFeedback :many
SELECT f.*, u.username
FROM feedback_reports f
LEFT JOIN users u ON u.id = f.user_id
ORDER BY f.created_at DESC
LIMIT $1 OFFSET $2;

-- name: AddBookmark :exec
INSERT INTO bookmarks (user_id, event_id) VALUES ($1, $2)
ON CONFLICT (user_id, event_id) DO NOTHING;

-- name: RemoveBookmark :exec
DELETE FROM bookmarks WHERE user_id = $1 AND event_id = $2;

-- name: ListBookmarkedEvents :many
SELECT e.id, e.slug, e.organizer_id, e.title, e.description, e.category,
       e.participation_format, e.status, e.eligibility, e.rules, e.contact_email,
       e.location, e.min_team_size, e.max_team_size, e.registration_opens_at,
       e.registration_closes_at, e.starts_at, e.ends_at, e.submission_deadline,
       e.criteria, e.judging_config, e.judging_status, e.judging_locked_at,
       e.results_published_at, e.created_at, e.updated_at,
       b.created_at AS bookmarked_at
FROM bookmarks b
JOIN events e ON e.id = b.event_id
WHERE b.user_id = $1
ORDER BY b.created_at DESC;

-- name: IsBookmarked :one
SELECT EXISTS (
    SELECT 1 FROM bookmarks WHERE user_id = $1 AND event_id = $2
) AS bookmarked;

-- name: CreateNotification :one
INSERT INTO notifications (user_id, kind, title, body, link)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListNotifications :many
SELECT * FROM notifications
WHERE user_id = $1
  AND (sqlc.narg('unread_only')::boolean IS NULL OR read_at IS NULL)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: MarkNotificationRead :exec
UPDATE notifications SET read_at = now() WHERE id = $1 AND user_id = $2;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL;

-- name: CountUnreadNotifications :one
SELECT count(*)::int AS count FROM notifications WHERE user_id = $1 AND read_at IS NULL;

-- name: CreateChatMessage :one
INSERT INTO chat_messages (team_id, user_id, body)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetChatMessage :one
SELECT m.id, m.team_id, m.user_id, m.body, m.created_at,
       u.username, p.full_name
FROM chat_messages m
JOIN users u ON u.id = m.user_id
LEFT JOIN profiles p ON p.user_id = u.id
WHERE m.id = $1;

-- name: ListTeamMessages :many
SELECT m.id, m.team_id, m.user_id, m.body, m.created_at,
       u.username, p.full_name
FROM chat_messages m
JOIN users u ON u.id = m.user_id
LEFT JOIN profiles p ON p.user_id = u.id
WHERE m.team_id = $1
  AND (sqlc.narg('before')::timestamptz IS NULL OR m.created_at < sqlc.narg('before')::timestamptz)
ORDER BY m.created_at DESC, m.id DESC
LIMIT $2 OFFSET $3;
