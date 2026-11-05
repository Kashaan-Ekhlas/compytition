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
