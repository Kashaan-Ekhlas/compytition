-- Make sure the denormalized event_id on team_members always matches the
-- event of its team, so the one-team-per-event unique index cannot be
-- sidestepped by writing an inconsistent row.

ALTER TABLE teams
    ADD CONSTRAINT teams_event_id_id_key UNIQUE (event_id, id);

ALTER TABLE team_members
    ADD CONSTRAINT team_members_event_team_fkey
    FOREIGN KEY (event_id, team_id) REFERENCES teams(event_id, id) ON DELETE CASCADE;
