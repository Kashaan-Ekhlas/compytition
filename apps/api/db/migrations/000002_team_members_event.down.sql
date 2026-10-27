DROP INDEX IF EXISTS team_members_event_idx;
DROP INDEX IF EXISTS team_members_one_team_per_event;

ALTER TABLE team_members DROP CONSTRAINT IF EXISTS team_members_event_id_fkey;
ALTER TABLE team_members DROP COLUMN IF EXISTS event_id;

CREATE UNIQUE INDEX team_members_one_team_per_event
    ON team_members(team_id, user_id);
