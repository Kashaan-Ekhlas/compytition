-- Uploaded files are tracked so avatars and event banners can be served
-- publicly while submission attachments stay restricted to judges, hosts and
-- the owner.
CREATE TABLE uploads (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind          TEXT NOT NULL CHECK (kind IN ('avatar', 'banner', 'attachment')),
    event_id      UUID REFERENCES events(id) ON DELETE CASCADE,
    original_name TEXT NOT NULL DEFAULT '',
    mime_type     TEXT NOT NULL DEFAULT '',
    size_bytes    BIGINT NOT NULL DEFAULT 0,
    storage_key   TEXT NOT NULL UNIQUE,
    preview_key   TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX uploads_owner_id_idx ON uploads(owner_id);
CREATE INDEX uploads_event_id_idx ON uploads(event_id);

ALTER TABLE profiles ADD COLUMN avatar_url TEXT NOT NULL DEFAULT '';
ALTER TABLE events ADD COLUMN banner_url TEXT NOT NULL DEFAULT '';
ALTER TABLE submissions
    ADD COLUMN file_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN file_mime TEXT NOT NULL DEFAULT '',
    ADD COLUMN file_preview_url TEXT NOT NULL DEFAULT '';
