CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ─────────────────────────────────────────────────────────────
-- Users, identities, profiles, sessions
-- ─────────────────────────────────────────────────────────────
CREATE TABLE users (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email                 CITEXT NOT NULL UNIQUE,
    username              CITEXT NOT NULL UNIQUE,
    onboarding_step       SMALLINT NOT NULL DEFAULT 1 CHECK (onboarding_step BETWEEN 1 AND 6),
    onboarding_completed  BOOLEAN NOT NULL DEFAULT FALSE,
    intents               JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_admin              BOOLEAN NOT NULL DEFAULT FALSE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT users_email_format CHECK (email ~* '^[^@\s]+@[^@\s]+\.[^@\s]+$'),
    CONSTRAINT users_username_format CHECK (username ~ '^[a-zA-Z0-9_]{3,30}$')
);

-- One row per sign-in method. Password hashes live here so an account can
-- gain/lose providers without touching the core user row.
CREATE TABLE auth_identities (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider          TEXT NOT NULL CHECK (provider IN ('password', 'google')),
    provider_user_id  TEXT NOT NULL,
    secret_hash       TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_user_id),
    UNIQUE (user_id, provider)
);
CREATE INDEX auth_identities_user_id_idx ON auth_identities(user_id);

CREATE TABLE profiles (
    user_id       UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    full_name     TEXT NOT NULL DEFAULT '',
    college       TEXT NOT NULL DEFAULT '',
    course        TEXT NOT NULL DEFAULT '',
    year_of_study SMALLINT CHECK (year_of_study BETWEEN 1 AND 8),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Opaque session tokens are stored only as SHA-256 hashes.
CREATE TABLE sessions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   BYTEA NOT NULL UNIQUE,
    expires_at   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);

CREATE TABLE oauth_states (
    state         TEXT PRIMARY KEY,
    code_verifier TEXT NOT NULL,
    redirect_to   TEXT NOT NULL,
    expires_at    TIMESTAMPTZ NOT NULL
);
CREATE INDEX oauth_states_expires_at_idx ON oauth_states(expires_at);

-- Login throttling. Key is a salted hash of (ip + email) so raw details are
-- not persisted longer than needed.
CREATE TABLE login_attempts (
    key          TEXT PRIMARY KEY,
    attempts     INT NOT NULL DEFAULT 0,
    first_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_until TIMESTAMPTZ
);

-- ─────────────────────────────────────────────────────────────
-- Events
-- ─────────────────────────────────────────────────────────────
CREATE TABLE events (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                 CITEXT NOT NULL UNIQUE,
    organizer_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title                TEXT NOT NULL CHECK (char_length(title) BETWEEN 3 AND 160),
    description          TEXT NOT NULL DEFAULT '',
    category             TEXT NOT NULL DEFAULT 'Hackathon',
    participation_format TEXT NOT NULL DEFAULT 'both'
                         CHECK (participation_format IN ('individual', 'team', 'both')),
    status               TEXT NOT NULL DEFAULT 'draft'
                         CHECK (status IN ('draft', 'published', 'closed', 'results_published')),
    eligibility          TEXT NOT NULL DEFAULT '',
    rules                TEXT NOT NULL DEFAULT '',
    contact_email        TEXT NOT NULL DEFAULT '',
    location             TEXT NOT NULL DEFAULT 'Online',
    min_team_size        SMALLINT NOT NULL DEFAULT 1 CHECK (min_team_size >= 1),
    max_team_size        SMALLINT NOT NULL DEFAULT 4 CHECK (max_team_size >= 1),
    registration_opens_at  TIMESTAMPTZ,
    registration_closes_at TIMESTAMPTZ,
    starts_at            TIMESTAMPTZ,
    ends_at              TIMESTAMPTZ,
    submission_deadline  TIMESTAMPTZ,
    criteria             JSONB NOT NULL DEFAULT '[]'::jsonb,
    judging_config       JSONB NOT NULL DEFAULT '{}'::jsonb,
    judging_status       TEXT NOT NULL DEFAULT 'setup'
                         CHECK (judging_status IN ('setup', 'live', 'locked', 'published')),
    judging_locked_at    TIMESTAMPTZ,
    results_published_at TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    search_vector        TSVECTOR GENERATED ALWAYS AS (
        to_tsvector('english',
            coalesce(title, '') || ' ' || coalesce(description, '') || ' ' ||
            coalesce(category, '') || ' ' || coalesce(eligibility, '') || ' ' ||
            coalesce(rules, ''))
    ) STORED,
    CONSTRAINT events_team_size_order CHECK (max_team_size >= min_team_size)
);
CREATE INDEX events_search_idx ON events USING GIN(search_vector);
CREATE INDEX events_published_idx ON events(status) WHERE status IN ('published', 'closed', 'results_published');
CREATE INDEX events_organizer_idx ON events(organizer_id, created_at DESC);
CREATE INDEX events_category_idx ON events(category);
CREATE INDEX events_dates_idx ON events(starts_at, registration_closes_at);

CREATE TABLE event_form_fields (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id   UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    label      TEXT NOT NULL CHECK (char_length(label) BETWEEN 1 AND 120),
    field_type TEXT NOT NULL CHECK (field_type IN ('text','long_text','select','multi_select','url','number','file')),
    required   BOOLEAN NOT NULL DEFAULT FALSE,
    options    JSONB NOT NULL DEFAULT '[]'::jsonb,
    help_text  TEXT NOT NULL DEFAULT '',
    position   INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX event_form_fields_event_idx ON event_form_fields(event_id, position);

-- Co-organizers and judges. `user_id` is filled once an invited person
-- accepts (or immediately for a direct assignment by username/email match).
CREATE TABLE event_staff (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id    UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id     UUID REFERENCES users(id) ON DELETE CASCADE,
    email       CITEXT NOT NULL,
    role        TEXT NOT NULL CHECK (role IN ('co_organizer', 'judge')),
    status      TEXT NOT NULL DEFAULT 'invited' CHECK (status IN ('invited', 'accepted', 'declined', 'removed')),
    invite_token TEXT UNIQUE,
    invited_by  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (event_id, email, role)
);
CREATE INDEX event_staff_event_idx ON event_staff(event_id, role);
CREATE INDEX event_staff_user_idx ON event_staff(user_id, status);
CREATE INDEX event_staff_email_idx ON event_staff(email);

-- ─────────────────────────────────────────────────────────────
-- Teams
-- ─────────────────────────────────────────────────────────────
CREATE TABLE teams (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id   UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name       TEXT NOT NULL CHECK (char_length(name) BETWEEN 2 AND 80),
    owner_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_looking_for_members BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (event_id, name)
);
CREATE INDEX teams_event_idx ON teams(event_id);
CREATE INDEX teams_owner_idx ON teams(owner_id);

CREATE TABLE team_members (
    team_id   UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, user_id)
);
CREATE INDEX team_members_user_idx ON team_members(user_id);
CREATE UNIQUE INDEX team_members_one_team_per_event
    ON team_members(team_id, user_id);

CREATE TABLE team_invites (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id         UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    invited_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invited_by      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected', 'cancelled')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    responded_at    TIMESTAMPTZ,
    UNIQUE (team_id, invited_user_id)
);
CREATE INDEX team_invites_user_idx ON team_invites(invited_user_id, status);

CREATE TABLE team_join_requests (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id     UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message     TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected', 'cancelled')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    responded_at TIMESTAMPTZ,
    UNIQUE (team_id, user_id)
);
CREATE INDEX team_join_requests_team_idx ON team_join_requests(team_id, status);
CREATE INDEX team_join_requests_user_idx ON team_join_requests(user_id, status);

-- ─────────────────────────────────────────────────────────────
-- Registrations and submissions
-- ─────────────────────────────────────────────────────────────
CREATE TABLE registrations (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id     UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    team_id      UUID REFERENCES teams(id) ON DELETE SET NULL,
    form_answers JSONB NOT NULL DEFAULT '{}'::jsonb,
    status       TEXT NOT NULL DEFAULT 'registered' CHECK (status IN ('registered', 'waitlisted', 'cancelled', 'disqualified')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (event_id, user_id)
);
CREATE INDEX registrations_event_idx ON registrations(event_id, created_at DESC);
CREATE INDEX registrations_user_idx ON registrations(user_id, created_at DESC);
CREATE INDEX registrations_team_idx ON registrations(team_id);

CREATE TABLE submissions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id        UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    registration_id UUID UNIQUE REFERENCES registrations(id) ON DELETE CASCADE,
    team_id         UUID REFERENCES teams(id) ON DELETE SET NULL,
    created_by      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title           TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 160),
    description     TEXT NOT NULL DEFAULT '',
    repo_url        TEXT NOT NULL DEFAULT '',
    demo_url        TEXT NOT NULL DEFAULT '',
    file_url        TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'submitted' CHECK (status IN ('draft', 'submitted')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX submissions_one_per_team ON submissions(event_id, team_id) WHERE team_id IS NOT NULL;
CREATE INDEX submissions_event_idx ON submissions(event_id, created_at);

-- ─────────────────────────────────────────────────────────────
-- Judging
-- ─────────────────────────────────────────────────────────────
CREATE TABLE judging_pairs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id       UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    judge_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    submission_a   UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    submission_b   UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    left_submission UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    position       SMALLINT NOT NULL DEFAULT 1,
    status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'judged', 'skipped')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT judging_pairs_distinct CHECK (submission_a <> submission_b),
    CONSTRAINT judging_pairs_ordered CHECK (submission_a < submission_b),
    CONSTRAINT judging_pairs_left_member CHECK (left_submission IN (submission_a, submission_b)),
    UNIQUE (judge_id, submission_a, submission_b)
);
CREATE INDEX judging_pairs_judge_idx ON judging_pairs(judge_id, event_id, status);
CREATE INDEX judging_pairs_event_idx ON judging_pairs(event_id);
CREATE INDEX judging_pairs_submission_a_idx ON judging_pairs(submission_a);
CREATE INDEX judging_pairs_submission_b_idx ON judging_pairs(submission_b);

-- Append-only: edits are new rows superseding the previous one. `is_current`
-- points at the effective judgment; the full history remains auditable.
CREATE TABLE judgments (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id              UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    pair_id               UUID NOT NULL REFERENCES judging_pairs(id) ON DELETE CASCADE,
    judge_id              UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    winner_submission_id  UUID REFERENCES submissions(id) ON DELETE CASCADE,
    skipped               BOOLEAN NOT NULL DEFAULT FALSE,
    conflict              BOOLEAN NOT NULL DEFAULT FALSE,
    is_current            BOOLEAN NOT NULL DEFAULT TRUE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX judgments_one_current_per_pair ON judgments(pair_id) WHERE is_current;
CREATE INDEX judgments_event_current_idx ON judgments(event_id) WHERE is_current;
CREATE INDEX judgments_judge_idx ON judgments(judge_id, created_at DESC);

CREATE TABLE judge_reliability_snapshots (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id     UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    judge_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    alpha        DOUBLE PRECISION NOT NULL,
    beta         DOUBLE PRECISION NOT NULL,
    reliability  DOUBLE PRECISION NOT NULL,
    comparisons  INT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX judge_reliability_event_idx ON judge_reliability_snapshots(event_id, created_at DESC);

CREATE TABLE event_results (
    event_id      UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    submission_id UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    rank          INT NOT NULL CHECK (rank >= 1),
    score         DOUBLE PRECISION NOT NULL,
    confidence    DOUBLE PRECISION NOT NULL DEFAULT 0,
    published_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, submission_id),
    UNIQUE (event_id, rank)
);

-- ─────────────────────────────────────────────────────────────
-- Supporting features
-- ─────────────────────────────────────────────────────────────
CREATE TABLE bookmarks (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_id   UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, event_id)
);
CREATE INDEX bookmarks_user_idx ON bookmarks(user_id, created_at DESC);

CREATE TABLE notifications (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL DEFAULT '',
    link       TEXT NOT NULL DEFAULT '',
    read_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_idx ON notifications(user_id, created_at DESC);
CREATE INDEX notifications_unread_idx ON notifications(user_id) WHERE read_at IS NULL;

CREATE TABLE chat_messages (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id    UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body       TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX chat_messages_team_idx ON chat_messages(team_id, created_at DESC, id DESC);

CREATE TABLE feedback_reports (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    subject    TEXT NOT NULL CHECK (char_length(subject) BETWEEN 3 AND 160),
    body       TEXT NOT NULL CHECK (char_length(body) BETWEEN 3 AND 5000),
    status     TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'triaged', 'closed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX feedback_reports_created_idx ON feedback_reports(created_at DESC);

CREATE TABLE audit_log (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id   UUID REFERENCES users(id) ON DELETE SET NULL,
    event_id   UUID REFERENCES events(id) ON DELETE CASCADE,
    action     TEXT NOT NULL,
    metadata   JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_event_idx ON audit_log(event_id, created_at DESC);
CREATE INDEX audit_log_actor_idx ON audit_log(actor_id, created_at DESC);
