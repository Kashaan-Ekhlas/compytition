-- name: CreateUpload :one
INSERT INTO uploads (
    owner_id, kind, event_id, original_name, mime_type, size_bytes,
    storage_key, preview_key
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetUpload :one
SELECT * FROM uploads WHERE id = $1;

-- name: DeleteUpload :exec
DELETE FROM uploads WHERE id = $1;

-- name: SetProfileAvatar :exec
INSERT INTO profiles (user_id, avatar_url)
VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE
SET avatar_url = EXCLUDED.avatar_url, updated_at = now();

-- name: SumUploadBytesByOwner :one
SELECT COALESCE(sum(size_bytes), 0)::bigint FROM uploads WHERE owner_id = $1;

-- name: HasSubmissionAttachmentAccess :one
SELECT EXISTS (
    SELECT 1
    FROM submissions s
    WHERE s.event_id = sqlc.arg(event_id)
      AND s.file_url = '/api/v1/uploads/' || sqlc.arg(upload_id)::text
      AND (
          s.created_by = sqlc.arg(user_id)
          OR (
              s.team_id IS NOT NULL
              AND EXISTS (
                  SELECT 1 FROM team_members m
                  WHERE m.team_id = s.team_id AND m.user_id = sqlc.arg(user_id)
              )
          )
      )
) AS allowed;

-- name: CountEventsUsingBanner :one
SELECT count(*)::int FROM events WHERE banner_url = $1;
