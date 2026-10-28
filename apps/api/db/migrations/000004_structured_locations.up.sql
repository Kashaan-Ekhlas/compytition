-- Structured locations, merged registration/submission forms and shareable
-- team invite links. Existing free-text locations stay as the display value.

ALTER TABLE profiles
    ADD COLUMN country TEXT NOT NULL DEFAULT '',
    ADD COLUMN state   TEXT NOT NULL DEFAULT '',
    ADD COLUMN city    TEXT NOT NULL DEFAULT '';

ALTER TABLE events
    ADD COLUMN is_remote BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN country   TEXT NOT NULL DEFAULT '',
    ADD COLUMN state     TEXT NOT NULL DEFAULT '',
    ADD COLUMN city      TEXT NOT NULL DEFAULT '',
    ADD COLUMN merge_registration_submission BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE events
SET is_remote = (location ILIKE '%online%' OR location ILIKE '%remote%');

ALTER TABLE event_form_fields
    ADD COLUMN form_kind TEXT NOT NULL DEFAULT 'registration'
    CHECK (form_kind IN ('registration', 'submission'));

DROP INDEX IF EXISTS event_form_fields_event_idx;
CREATE INDEX event_form_fields_event_idx ON event_form_fields(event_id, form_kind, position);

ALTER TABLE teams ADD COLUMN invite_token TEXT UNIQUE;
CREATE INDEX teams_invite_token_idx ON teams(invite_token) WHERE invite_token IS NOT NULL;

ALTER TABLE submissions ADD COLUMN answers JSONB NOT NULL DEFAULT '{}'::jsonb;
