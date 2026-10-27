-- Enforce one team per user per event in the database, not only in the
-- service layer. Concurrent creates/joins could otherwise race past the
-- application check and leave a user on two teams for the same event.

ALTER TABLE team_members ADD COLUMN event_id UUID;

UPDATE team_members tm
SET event_id = t.event_id
FROM teams t
WHERE t.id = tm.team_id;

ALTER TABLE team_members ALTER COLUMN event_id SET NOT NULL;
ALTER TABLE team_members
    ADD CONSTRAINT team_members_event_id_fkey
    FOREIGN KEY (event_id) REFERENCES events(id) ON DELETE CASCADE;

-- The old index duplicated the (team_id, user_id) primary key.
DROP INDEX IF EXISTS team_members_one_team_per_event;
CREATE UNIQUE INDEX team_members_one_team_per_event
    ON team_members(event_id, user_id);
CREATE INDEX team_members_event_idx ON team_members(event_id);
