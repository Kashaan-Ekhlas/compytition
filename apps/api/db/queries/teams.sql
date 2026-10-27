-- name: CreateTeam :one
INSERT INTO teams (event_id, name, owner_id, is_looking_for_members)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetTeamByID :one
SELECT * FROM teams WHERE id = $1;

-- name: LockTeamForUpdate :one
SELECT * FROM teams WHERE id = $1 FOR UPDATE;

-- name: GetTeamForEventUser :one
SELECT t.* FROM teams t
JOIN team_members m ON m.team_id = t.id
WHERE t.event_id = $1 AND m.user_id = $2;

-- name: ListTeamsForEvent :many
SELECT t.id, t.event_id, t.name, t.owner_id, t.is_looking_for_members,
       t.created_at, t.updated_at,
       u.username AS owner_username,
       count(m.user_id)::int AS member_count
FROM teams t
JOIN users u ON u.id = t.owner_id
LEFT JOIN team_members m ON m.team_id = t.id
WHERE t.event_id = $1
GROUP BY t.id, u.username
ORDER BY t.created_at ASC;

-- name: ListMyTeams :many
SELECT t.id, t.event_id, t.name, t.owner_id, t.is_looking_for_members,
       t.created_at, t.updated_at,
       e.title AS event_title, e.slug AS event_slug, e.status AS event_status,
       owner.username AS owner_username,
       (SELECT count(*)::int FROM team_members tm WHERE tm.team_id = t.id) AS member_count
FROM teams t
JOIN events e ON e.id = t.event_id
JOIN users owner ON owner.id = t.owner_id
JOIN team_members m ON m.team_id = t.id
WHERE m.user_id = $1
ORDER BY t.created_at DESC;

-- name: UpdateTeam :exec
UPDATE teams SET name = $2, is_looking_for_members = $3, updated_at = now() WHERE id = $1;

-- name: DeleteTeam :exec
DELETE FROM teams WHERE id = $1;

-- name: UpdateTeamOwner :exec
UPDATE teams SET owner_id = $2, updated_at = now() WHERE id = $1;

-- name: AddTeamMember :exec
INSERT INTO team_members (team_id, user_id) VALUES ($1, $2);

-- name: RemoveTeamMember :exec
DELETE FROM team_members WHERE team_id = $1 AND user_id = $2;

-- name: GetTeamMember :one
SELECT * FROM team_members WHERE team_id = $1 AND user_id = $2;

-- name: CountTeamMembers :one
SELECT count(*)::int AS count FROM team_members WHERE team_id = $1;

-- name: ListTeamMembers :many
SELECT m.team_id, m.user_id, m.joined_at,
       u.username, u.email, p.full_name
FROM team_members m
JOIN users u ON u.id = m.user_id
LEFT JOIN profiles p ON p.user_id = u.id
WHERE m.team_id = $1
ORDER BY m.joined_at ASC;

-- name: ListTeamMembershipInEvent :many
SELECT m.team_id, m.user_id, m.joined_at, t.name AS team_name, t.owner_id
FROM team_members m
JOIN teams t ON t.id = m.team_id
WHERE t.event_id = $1 AND m.user_id = $2;

-- name: CreateTeamInvite :one
INSERT INTO team_invites (team_id, invited_user_id, invited_by)
VALUES ($1, $2, $3)
ON CONFLICT (team_id, invited_user_id) DO UPDATE
SET status = 'pending', invited_by = EXCLUDED.invited_by, created_at = now(), responded_at = NULL
RETURNING *;

-- name: GetTeamInvite :one
SELECT * FROM team_invites WHERE id = $1;

-- name: ListTeamInvites :many
SELECT i.id, i.team_id, i.invited_user_id, i.invited_by, i.status,
       i.created_at, i.responded_at,
       t.name AS team_name, t.event_id, e.title AS event_title,
       u.username AS invited_by_username
FROM team_invites i
JOIN teams t ON t.id = i.team_id
JOIN events e ON e.id = t.event_id
JOIN users u ON u.id = i.invited_by
WHERE i.team_id = $1
ORDER BY i.created_at DESC;

-- name: ListMyTeamInvites :many
SELECT i.id, i.team_id, i.invited_user_id, i.invited_by, i.status,
       i.created_at, i.responded_at,
       t.name AS team_name, t.event_id, e.title AS event_title,
       u.username AS invited_by_username
FROM team_invites i
JOIN teams t ON t.id = i.team_id
JOIN events e ON e.id = t.event_id
JOIN users u ON u.id = i.invited_by
WHERE i.invited_user_id = $1 AND i.status = 'pending'
ORDER BY i.created_at DESC;

-- name: UpdateTeamInviteStatus :exec
UPDATE team_invites SET status = $2, responded_at = now() WHERE id = $1;

-- name: CreateJoinRequest :one
INSERT INTO team_join_requests (team_id, user_id, message)
VALUES ($1, $2, $3)
ON CONFLICT (team_id, user_id) DO UPDATE
SET status = 'pending', message = EXCLUDED.message, created_at = now(), responded_at = NULL
RETURNING *;

-- name: GetJoinRequest :one
SELECT * FROM team_join_requests WHERE id = $1;

-- name: ListJoinRequestsForTeam :many
SELECT r.id, r.team_id, r.user_id, r.message, r.status, r.created_at,
       r.responded_at, u.username, p.full_name
FROM team_join_requests r
JOIN users u ON u.id = r.user_id
LEFT JOIN profiles p ON p.user_id = u.id
WHERE r.team_id = $1 AND r.status = 'pending'
ORDER BY r.created_at ASC;

-- name: ListMyJoinRequests :many
SELECT r.id, r.team_id, r.user_id, r.message, r.status, r.created_at,
       r.responded_at, t.name AS team_name, t.event_id, e.title AS event_title
FROM team_join_requests r
JOIN teams t ON t.id = r.team_id
JOIN events e ON e.id = t.event_id
WHERE r.user_id = $1 AND r.status = 'pending'
ORDER BY r.created_at DESC;

-- name: UpdateJoinRequestStatus :exec
UPDATE team_join_requests SET status = $2, responded_at = now() WHERE id = $1;

-- name: CountPendingTeamInvites :one
SELECT count(*)::int AS count FROM team_invites WHERE invited_user_id = $1 AND status = 'pending';

-- name: DeleteRegistrationForTeamUser :exec
DELETE FROM registrations WHERE event_id = $1 AND user_id = $2;

-- name: CancelTeamInvitesForEventUser :exec
UPDATE team_invites SET status = 'cancelled', responded_at = now()
WHERE invited_user_id = $1 AND status = 'pending'
  AND team_id IN (SELECT id FROM teams WHERE event_id = $2);

-- name: CancelJoinRequestsForEventUser :exec
UPDATE team_join_requests SET status = 'cancelled', responded_at = now()
WHERE user_id = $1 AND status = 'pending'
  AND team_id IN (SELECT id FROM teams WHERE event_id = $2);

-- name: GetPendingJoinRequestForTeamUser :one
SELECT * FROM team_join_requests
WHERE team_id = $1 AND user_id = $2 AND status = 'pending';

-- name: GetPendingTeamInviteForTeamUser :one
SELECT * FROM team_invites
WHERE team_id = $1 AND invited_user_id = $2 AND status = 'pending';

-- name: CountPendingJoinRequests :one
SELECT count(*)::int AS count FROM team_join_requests WHERE team_id = $1 AND status = 'pending';

-- name: DeleteRegistrationsForTeam :exec
DELETE FROM registrations WHERE team_id = $1;
