DROP INDEX IF EXISTS teams_invite_token_idx;

ALTER TABLE submissions DROP COLUMN IF EXISTS answers;

ALTER TABLE teams DROP COLUMN IF EXISTS invite_token;

DROP INDEX IF EXISTS event_form_fields_event_idx;
CREATE INDEX event_form_fields_event_idx ON event_form_fields(event_id, position);
ALTER TABLE event_form_fields DROP COLUMN IF EXISTS form_kind;

ALTER TABLE events
    DROP COLUMN IF EXISTS merge_registration_submission,
    DROP COLUMN IF EXISTS city,
    DROP COLUMN IF EXISTS state,
    DROP COLUMN IF EXISTS country,
    DROP COLUMN IF EXISTS is_remote;

ALTER TABLE profiles
    DROP COLUMN IF EXISTS city,
    DROP COLUMN IF EXISTS state,
    DROP COLUMN IF EXISTS country;
