-- name: CreateEvent :one
INSERT INTO events (
    slug, organizer_id, title, description, category, participation_format,
    status, eligibility, rules, contact_email, location, min_team_size,
    max_team_size, registration_opens_at, registration_closes_at, starts_at,
    ends_at, submission_deadline, criteria
) VALUES (
    $1, $2, $3, $4, $5, $6, 'draft', $7, $8, $9, $10, $11, $12, $13, $14,
    $15, $16, $17, $18
)
RETURNING id, slug, organizer_id, title, description, category,
    participation_format, status, eligibility, rules, contact_email, location,
    min_team_size, max_team_size, registration_opens_at, registration_closes_at,
    starts_at, ends_at, submission_deadline, criteria, judging_config,
    judging_status, judging_locked_at, results_published_at, created_at, updated_at;

-- name: GetEventByID :one
SELECT id, slug, organizer_id, title, description, category,
    participation_format, status, eligibility, rules, contact_email, location,
    min_team_size, max_team_size, registration_opens_at, registration_closes_at,
    starts_at, ends_at, submission_deadline, criteria, judging_config,
    judging_status, judging_locked_at, results_published_at, created_at, updated_at
FROM events WHERE id = $1;

-- name: GetEventBySlug :one
SELECT id, slug, organizer_id, title, description, category,
    participation_format, status, eligibility, rules, contact_email, location,
    min_team_size, max_team_size, registration_opens_at, registration_closes_at,
    starts_at, ends_at, submission_deadline, criteria, judging_config,
    judging_status, judging_locked_at, results_published_at, created_at, updated_at
FROM events WHERE slug = $1;

-- name: LockEventForUpdate :one
SELECT id, organizer_id, status, judging_status
FROM events WHERE id = $1 FOR UPDATE;

-- name: UpdateEvent :one
UPDATE events SET
    title = $2, description = $3, category = $4, participation_format = $5,
    eligibility = $6, rules = $7, contact_email = $8, location = $9,
    min_team_size = $10, max_team_size = $11, registration_opens_at = $12,
    registration_closes_at = $13, starts_at = $14, ends_at = $15,
    submission_deadline = $16, criteria = $17, updated_at = now()
WHERE id = $1
RETURNING id, slug, organizer_id, title, description, category,
    participation_format, status, eligibility, rules, contact_email, location,
    min_team_size, max_team_size, registration_opens_at, registration_closes_at,
    starts_at, ends_at, submission_deadline, criteria, judging_config,
    judging_status, judging_locked_at, results_published_at, created_at, updated_at;

-- name: UpdateEventStatus :exec
UPDATE events SET status = $2, updated_at = now() WHERE id = $1;

-- name: UpdateEventJudgingStatus :exec
UPDATE events
SET judging_status = $2,
    judging_locked_at = CASE WHEN $2 = 'locked' THEN now() ELSE judging_locked_at END,
    results_published_at = CASE WHEN $2 = 'published' THEN now() ELSE results_published_at END,
    updated_at = now()
WHERE id = $1;

-- name: UpdateEventJudgingConfig :exec
UPDATE events SET judging_config = $2, updated_at = now() WHERE id = $1;

-- name: DeleteEvent :exec
DELETE FROM events WHERE id = $1;

-- name: ListPublishedEvents :many
SELECT e.id, e.slug, e.organizer_id, e.title, e.description, e.category,
    e.participation_format, e.status, e.eligibility, e.rules, e.contact_email,
    e.location, e.min_team_size, e.max_team_size, e.registration_opens_at,
    e.registration_closes_at, e.starts_at, e.ends_at, e.submission_deadline,
    e.criteria, e.judging_config, e.judging_status, e.judging_locked_at,
    e.results_published_at, e.created_at, e.updated_at,
    count(*) OVER() AS total_count
FROM events e
WHERE e.status IN ('published', 'closed', 'results_published')
  AND (sqlc.narg('search')::text IS NULL
       OR e.search_vector @@ websearch_to_tsquery('english', sqlc.narg('search')::text)
       OR e.title ILIKE '%' || sqlc.narg('search')::text || '%')
  AND (sqlc.narg('category')::text IS NULL OR e.category = sqlc.narg('category')::text)
  AND (sqlc.narg('format')::text IS NULL
       OR e.participation_format = sqlc.narg('format')::text
       OR e.participation_format = 'both')
  AND (sqlc.narg('reg_status')::text IS NULL OR
       CASE
           WHEN e.status = 'results_published'
                OR (e.ends_at IS NOT NULL AND e.ends_at < now()) THEN 'ended'
           WHEN e.status = 'closed'
                OR (e.registration_closes_at IS NOT NULL AND e.registration_closes_at < now()) THEN 'closed'
           WHEN e.registration_opens_at IS NOT NULL AND e.registration_opens_at > now() THEN 'upcoming'
           ELSE 'open'
       END = sqlc.narg('reg_status')::text)
ORDER BY COALESCE(e.starts_at, e.created_at) ASC, e.created_at DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: ListEventsByOrganizer :many
SELECT id, slug, organizer_id, title, description, category,
    participation_format, status, eligibility, rules, contact_email, location,
    min_team_size, max_team_size, registration_opens_at, registration_closes_at,
    starts_at, ends_at, submission_deadline, criteria, judging_config,
    judging_status, judging_locked_at, results_published_at, created_at, updated_at
FROM events
WHERE organizer_id = $1
ORDER BY created_at DESC;

-- name: CanManageEvent :one
SELECT EXISTS (
    SELECT 1 FROM events e
    WHERE e.id = $2
      AND (
          e.organizer_id = $1
          OR EXISTS (
              SELECT 1 FROM event_staff s
              WHERE s.event_id = e.id AND s.user_id = $1
                AND s.status = 'accepted' AND s.role = 'co_organizer'
          )
      )
) AS allowed;

-- name: IsEventOrganizerOwner :one
SELECT EXISTS (SELECT 1 FROM events WHERE id = $1 AND organizer_id = $2) AS allowed;

-- name: IsJudgeForEvent :one
SELECT EXISTS (
    SELECT 1 FROM event_staff
    WHERE event_id = $1 AND user_id = $2 AND role = 'judge' AND status = 'accepted'
) AS allowed;

-- name: DeleteFormFieldsForEvent :exec
DELETE FROM event_form_fields WHERE event_id = $1;

-- name: CreateFormField :one
INSERT INTO event_form_fields (event_id, label, field_type, required, options, help_text, position)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListFormFields :many
SELECT * FROM event_form_fields WHERE event_id = $1 ORDER BY position ASC, created_at ASC;

-- name: CreateStaffInvite :one
INSERT INTO event_staff (event_id, user_id, email, role, invite_token, invited_by)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (event_id, email, role) DO UPDATE
SET user_id = EXCLUDED.user_id, invite_token = EXCLUDED.invite_token,
    status = 'invited', invited_by = EXCLUDED.invited_by, updated_at = now()
RETURNING *;

-- name: GetStaffByToken :one
SELECT * FROM event_staff WHERE invite_token = $1;

-- name: GetStaffMember :one
SELECT * FROM event_staff WHERE event_id = $1 AND user_id = $2 AND role = $3;

-- name: ListEventStaff :many
SELECT * FROM event_staff
WHERE event_id = $1 AND role = $2 AND status IN ('invited', 'accepted')
ORDER BY created_at ASC;

-- name: AcceptStaffInvite :exec
UPDATE event_staff
SET user_id = $2, status = 'accepted', invite_token = NULL, updated_at = now()
WHERE id = $1 AND status = 'invited';

-- name: AcceptStaffInviteByEmail :exec
UPDATE event_staff
SET user_id = $2, status = 'accepted', invite_token = NULL, updated_at = now()
WHERE event_id = $1 AND email = $3 AND role = $4 AND status = 'invited';

-- name: UpdateStaffStatus :exec
UPDATE event_staff SET status = $2, updated_at = now() WHERE id = $1;

-- name: RemoveStaff :exec
DELETE FROM event_staff WHERE id = $1;

-- name: ListJudgeEvents :many
SELECT e.id, e.slug, e.organizer_id, e.title, e.description, e.category,
    e.participation_format, e.status, e.eligibility, e.rules, e.contact_email,
    e.location, e.min_team_size, e.max_team_size, e.registration_opens_at,
    e.registration_closes_at, e.starts_at, e.ends_at, e.submission_deadline,
    e.criteria, e.judging_config, e.judging_status, e.judging_locked_at,
    e.results_published_at, e.created_at, e.updated_at
FROM event_staff s
JOIN events e ON e.id = s.event_id
WHERE s.user_id = $1 AND s.role = 'judge' AND s.status = 'accepted'
ORDER BY e.starts_at DESC NULLS LAST, e.created_at DESC;
