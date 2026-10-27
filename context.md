# Project Context

## Overview

**Compytition** is an event hosting and pairwise-judging platform. Organizers create events, participants register solo or in teams, judges compare submissions two at a time, and CrowdBT aggregates those comparisons into a ranking that organizers review and publish. No numeric scores, no weighted rubrics.

Current state (checkpoint): **feature-complete end to end**. The Go API, PostgreSQL schema/migrations, SQLC queries, pairwise assignment, CrowdBT ranking, judge flow, review/publish, chat, bookmarks, notifications and feedback are implemented. The Next.js app covers public discovery, registration, teams, submissions, the organizer console, the judge comparison screen and settings. Docker Compose, Make targets, seeds, CI and tests are in place. `context.md` and `endpoints.md` are kept current at every meaningful unit of work.

## Architecture

Monorepo:

```
compytition/
  apps/api/          Go 1.22+ stdlib HTTP API
    cmd/api/         API entrypoint (migrations + graceful shutdown)
    cmd/migrate/     up/down migration command
    db/migrations/   embedded SQL migrations (golang-migrate)
    db/queries/      SQLC query sources
    db/embed.go      embeds migrations for the API/migrate binaries
    internal/
      apperr/        domain errors
      auth/          argon2id, sessions, onboarding, Google OIDC
      chat/          persisted team chat + websocket hub
      config/        env config, fail-fast validation
      db/            SQLC generated code + pgx pool + tx helper + migrator
      events/        event CRUD, lifecycle, form builder, staff invites
      support/       bookmarks, notifications, feedback admin queue
      httpx/         JSON/error envelope, request helpers, auth context
      judging/       CrowdBT + assignment + judging handlers
      registrations/ registrations and organizer status
      server/        ServeMux wiring, middleware chain
      submissions/   submissions and deadlines
      teams/         team lifecycle, invites, join requests
      dashboard/     aggregated user home
  apps/web/          Next.js App Router + TS strict + Tailwind + shadcn-style UI
  design.md          visual source of truth
  prompt.md          product/engineering brief
  rules.md           code-review rules
  endpoints.md       API + UI endpoint inventory (kept current)
  review.md          code review output
```

Data flow: browser → Next.js `app/api` rewrite proxy → Go API `/api/v1/*` → SQLC/pgx → Postgres. Session cookie is an opaque random token; only its SHA-256 lives in `sessions`. WebSocket chat upgrades on the Go API.

## Decisions Log

- **2026-10-27** — Follow the prompt's `apps/api` + `apps/web` layout; removed the empty `backend/`/`frontend/` folders.
- **2026-10-27** — Migration tool: `golang-migrate` with embedded `iofs` source, applied automatically at API boot (`AUTO_MIGRATE`) and available via `cmd/migrate`.
- **2026-10-27** — SQLC `pgx/v5` output in `internal/db`; UUID overrides to `uuid.UUID`; nullable UUID/timestamp overrides to pointers. JSONB is generated as `[]byte` and parsed at the service boundary.
- **2026-10-27** — Passwords: argon2id, m=65536 KiB, t=3, p=2, 16-byte salt, 32-byte key, PHC encoding.
- **2026-10-27** — Sessions: 32-byte random URL-safe token, SHA-256 stored, HttpOnly + SameSite=Lax + Secure-in-prod cookie, 30-day default TTL, hourly expired-session cleanup.
- **2026-10-27** — CSRF: Origin/Referer allowlist + SameSite=Lax (prompt allows Origin checks). CORS allows exactly `WEB_ORIGIN` with credentials.
- **2026-10-27** — Login lockout: 8 failures per (IP,email) hash within 15 minutes → 15-minute lock. Generic auth errors prevent enumeration.
- **2026-10-27** — Google sign-in: `coreos/go-oidc` verifier (signature, issuer, audience, expiry) plus `email_verified` check. `oauth2` handles PKCE S256.
- **2026-10-27** — Onboarding is resumable via `users.onboarding_step`, `onboarding_completed`, and `intents`; registration auto-generates a placeholder username (`user_xxxxxx`) that must be replaced before completion.
- **2026-10-27** — Event lifecycle: draft → published → closed → results_published (results status set by judging finalization). Unpublish only before registrations. Only drafts are deletable.
- **2026-10-27** — Event staff (judges/co-organizers) invited by email with bearer invite tokens; acceptance requires the logged-in account email to match. This prevents forwarded-link takeover.
- **2026-10-27** — Team invites are account-based (by username); join requests are user-initiated. Both require explicit acceptance. Registration rows are created for team members when a team registers; late joiners inherit the team registration answers.
- **2026-10-27** — Judging judgments are append-only. Each pair has at most one `is_current` row; edits supersede rather than overwrite, preserving an audit trail. Immutability enforced after judging locks.
- **2026-10-27** — Rate limiting is an in-process fixed-window limiter (240 req/min global, 20 req/min auth). Horizontal scaling would move this to a shared store; documented as tech debt.
- **2026-10-27** — Added `coreos/go-oidc` and `golang-migrate` beyond the minimal dependency list; both are explicitly justified by the security and migration requirements.
- **2026-10-27** — Frontend is client-fetched through a shared `api()` helper and `AuthProvider`; Next rewrites `/api/v1/*` to `API_INTERNAL_URL` so the browser stays same-origin and the session cookie works without CORS in the normal path.
- **2026-10-27** — Compose publishes web on `3100`, API on `8080` and Postgres on `5433`, avoiding collisions with local dev servers (notably an existing `:3000` app on this machine).
- **2026-10-27** — Team chat connects directly to `NEXT_PUBLIC_WS_BASE` because websocket upgrades cannot go through Next rewrites reliably; Origin is checked server-side.
- **2026-10-27** — Review/publish responses serialize empty collections as `[]` rather than `null`; the client also normalizes arrays defensively so a rendering path never assumes a non-nil slice.
- **2026-10-27** — Judge progress includes `last_pair_id`, allowing undo to survive queue completion and page reloads without a separate endpoint.

## Data Model Notes

Key tables and invariants:

- `users` — CITEXT unique email + username; username regex `^[a-zA-Z0-9_]{3,30}$`; onboarding step 1–6.
- `auth_identities` — one row per provider (`password`, `google`); password hash nullable for OAuth-only accounts.
- `sessions` — `token_hash BYTEA UNIQUE`; expiry index; cascade on user delete.
- `profiles` — 1:1 with users; name/college/course/year.
- `events` — slug unique, organizer, lifecycle status, judging status, JSONB `criteria` (array of `{name,description}`), JSONB `judging_config` (`{target_comparisons,judge_budget,anchor_count}`), generated `search_vector` + GIN index.
- `event_form_fields` — schema-driven registration fields; type CHECK; options JSONB; ordered.
- `event_staff` — `(event_id,email,role)` unique; `invite_token` unique; accepted rows carry `user_id`.
- `teams` — `(event_id,name)` unique; owner. `team_members` PK `(team_id,user_id)`; service enforces one team per user per event and max size under `SELECT … FOR UPDATE`.
- `team_invites` / `team_join_requests` — `(team_id,user)` unique; pending → accepted/rejected/cancelled.
- `registrations` — `(event_id,user_id)` unique; `team_id` links team entry; answers JSONB.
- `submissions` — one per registration; partial unique `(event_id,team_id) WHERE team_id IS NOT NULL`.
- `judging_pairs` — canonical `submission_a < submission_b`; left/right randomization via `left_submission`; unique `(judge_id,a,b)`; status pending/judged/skipped.
- `judgments` — append-only; partial unique current per pair; winner must be one of the pair (service invariant); `skipped`/`conflict` flags.
- `event_results` — `(event_id,rank)` unique; `(event_id,submission_id)` PK; only written when publishing.
- `audit_log` — append-only JSONB metadata.
- Supporting: bookmarks, notifications, chat_messages, feedback_reports.

## Judging System

Implemented in `internal/judging`:

- **Assignment** (`assignment/`): target comparisons per submission `k` (default 6), judge budget (default 20), shared anchors (default 3). The planner guarantees a connected comparison graph, coverage ≥ `k` where the budget allows, per-judge overlap via anchors, balanced left/right ordering, conflict exclusion, and a single-judge fallback. Seed is the event ID, so plans are deterministic. Assignment is only allowed while `judging_status='setup'`.
- **CrowdBT** (`crowdbt/`): latent Gaussian score per submission (mean, variance) and Beta reliability per judge. Online updates plus a deterministic full recompute from the judgment log. Hyperparameters: prior mean 0, prior variance 1, `BetaSq=0.25`, `Beta(3,1)`, minimum variance 0.05. Unreliable judges are down-weighted. A simulation test checks ranking quality.
- **Judge flow**: only accepted judges may judge; `next` returns the next pending pair with randomized left/right order, criteria and progress. Winners are sent as `"a"`/`"b"` relative to the canonical `submission_a`/`submission_b`, never the displayed side. Skip marks only that pair; conflict skips that pair and every remaining pair involving either submission for that judge. Undo clears the current judgment and returns the pair to pending; `last_pair_id` in progress keeps the action available after a reload.
- **Review/publish**: exposes ranking, coverage, wins/losses, confidence, per-judge reliability, split decisions and warnings. Publishing requires `judging_status=live`, at least one outcome and either all pairs done or `force:true`. It writes `event_results`, snapshots reliability, sets `judging_status='published'` and the event to `results_published`, then audits the action. Results are public only after publication; organizers can preview beforehand.

## Phase Progress

- [x] Phase 0 foundations (repo, schema, config, middleware, Docker, Make targets, CI)
- [x] Phase 1 accounts/auth (register/login/logout/session/username/profile/onboarding/Google)
- [x] Phase 2 events (CRUD, lifecycle, search/filter, form builder, staff invites, judging config)
- [x] Phase 3 teams + registration (team lifecycle, invites, join requests, registration inheritance)
- [x] Phase 4 submissions (deadline locking, team-aware ownership, organizer lists)
- [x] Phase 5 judging (assignment, CrowdBT, comparison UI, review/publish, public results)
- [x] Phase 6 team chat (persisted history + websocket, membership checks)
- [x] Phase 7 P1 features delivered: bookmarks, notifications, feedback + admin queue, dashboard aggregation
- [ ] Phase 7 P1 still deferred: event duplication, CSV export, emailed deadline reminders, teams-looking-for-members discovery polish
- [x] Phase 8 hardening: `docker compose` stack, CI workflow, seeds, Vitest + Go tests, browser smoke test
- [ ] Phase 8 polish: OpenAPI file, Lighthouse/axe pass, real email provider

PRD coverage:
- 1.1 register/login/logout — done
- 1.2 unique username / change password — done
- 1.3 view/edit profile — done
- 1.4 profile autofill — done
- 1.5 authorization — server-side checks on every endpoint
- 1.6 Google sign-in — done (needs credentials to exercise)
- 2.1–2.4 discovery/detail/search/filters — done
- 3.1–3.5 hosting CRUD/lifecycle/forms/staff/criteria/finalize — done
- 4.x teams and registration — done
- 5.x submissions — done
- 6.x pairwise judging, CrowdBT, review/publish — done
- 7.1–7.5 dashboard/settings/feedback — done (feedback email delivery is P1)

## How To Run

Docker Compose (recommended):

```bash
cp .env.example .env
make up                 # db :5433, api :8080, web :3100
make seed               # idempotent demo data
make logs
make down
make clean              # stop and delete the volume
```

Backend directly (requires Postgres; compose DB is on 5433):

```bash
cd apps/api
export DATABASE_URL='postgres://compytition:compytition@localhost:5433/compytition?sslmode=disable'
export WEB_ORIGIN='http://localhost:3100'
go run ./cmd/api            # auto-migrates, listens on :8080
go run ./cmd/migrate -down  # rollback
/home/kash/go/bin/sqlc generate
go test ./...
```

Frontend directly:

```bash
export PATH="$HOME/.nvm/versions/node/v24.18.0/bin:$PATH"
cd apps/web
npm ci
API_INTERNAL_URL=http://localhost:8080 npm run dev -- -p 3100
npm run build
npm run lint
npm test -- --run
```

Demo accounts: `host@compytition.local` (organizer), `ava@`, `milo@`, `rin@compytition.local` (judges) and `alice@`…`frank@compytition.local` (participants). Password is `demo1234` for all.

## Env Vars

| Name | Purpose | Example |
|---|---|---|
| `APP_ENV` | development/production | `development` |
| `PORT` | API port | `8080` |
| `DATABASE_URL` | Postgres DSN (required) | `postgres://compytition:compytition@db:5432/compytition?sslmode=disable` |
| `WEB_ORIGIN` | CORS/CSRF/OAuth redirects | `http://localhost:3100` |
| `SESSION_TTL_HOURS` | Session lifetime | `720` |
| `COOKIE_SECURE` | Secure cookies | `false` locally, `true` prod |
| `AUTO_MIGRATE` | Run migrations on boot | `true` |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Google OAuth | empty locally |
| `GOOGLE_REDIRECT_URL` | OAuth callback | `http://localhost:8080/api/v1/auth/google/callback` |
| `ADMIN_EMAILS` | comma-separated admin emails | `you@example.com` |
| `LOG_LEVEL` | slog level | `info` |
| `RATE_LIMIT_PER_MIN` | global per-IP budget | `240` |
| `API_INTERNAL_URL` | Next rewrite target | `http://api:8080` in Compose |
| `NEXT_PUBLIC_API_BASE` | browser API prefix | `/api/v1` |
| `NEXT_PUBLIC_WS_BASE` | browser websocket origin | `ws://localhost:8080` |
| `WEB_PORT` / `API_PORT` / `POSTGRES_PORT` | host ports | `3100` / `8080` / `5433` |

## Known Issues / Tech Debt

- Go toolchain is 1.26.x locally; the API Dockerfile pins `golang:1.27.1-bookworm`. Keep them in step when bumping `go.mod`.
- Rate limiting is per-process only. Horizontal scaling needs a shared store.
- No OpenAPI file yet; `endpoints.md` is the interim contract.
- Session `TouchSession` writes on every authenticated request; could debounce.
- Event search uses `websearch_to_tsquery` + title ILIKE fallback; no trigram index for ILIKE yet.
- Profile `year_of_study` is a pointer int16; send `null`, not `0`.
- Team invites are by username, not email. Staff invites are email-bound links.
- Deadline reminders and emailed invites are P1; no email provider abstraction yet.
- Chat is a single in-process hub; multi-instance deployments need pub/sub.
- The seeded demo event is pre-judged and published, so create a new event to exercise the judge flow end to end.
- Docker image `golang:1.27.1-bookworm` is newer than the local toolchain; verify the tag exists before the first CI/CD image build.

## Open Questions

- Production cookie sharing between `web` and `api` subdomains will need `COOKIE_DOMAIN`; localhost works because cookies ignore ports.
- Whether to allow team changes after registration closes (current behavior: no new teams/joins after close, leaving always allowed).
- Exact CrowdBT hyperparameters and Kendall-tau threshold will be documented when implemented.
- Whether event organizers should be forced through onboarding before creating events (currently only auth is required).

## Changelog

Newest first.

- **2026-10-27** — Fixed WebSocket upgrades through the logging middleware: the response wrapper now preserves `http.Hijacker` and `http.Flusher` (plus `Unwrap`), with unit coverage. Before this, team chat upgrades failed with “response does not implement http.Hijacker”.
- **2026-10-27** — Fixed the Compose web build: `API_INTERNAL_URL` is now a build arg because Next bakes rewrite destinations into the standalone build. The image previously proxied `/api/v1` to `localhost:8080` inside the web container and returned 500s.
- **2026-10-27** — Completed the Next.js app: landing, discovery, event detail with bookmarks, registration, team management + websocket chat, submission editor, dashboard, settings, organizer home and the tabbed manage console (overview/form/participants/submissions/judges/judging), judge queue and pairwise comparison screen, staff invite acceptance. Added Vitest coverage for the API client and formatters, fixed Next config loading under TypeScript 5.9, and pinned the toolchain.
- **2026-10-27** — Browser smoke test found and fixed: judging review rendering crashed when `warnings` was JSON `null` (server now emits `[]`, client defaults arrays); judges can now undo their last call after the queue is empty or the page reloads (`last_pair_id` in progress); seed no longer ships two submissions titled “Loopback”. Added Docker Compose, Make targets, `.env.example`, CI workflow, README, and refreshed this file + `endpoints.md`.

- **2026-10-27** — Implemented initial schema (26 tables), SQLC generation, config, pgx store/tx helper, embedded golang-migrate runner, middleware chain (request ID, logging, recovery, security headers, CORS, CSRF origin checks, rate limit, session loading), auth service (argon2id, sessions, lockout, Google OIDC, profile/onboarding/username/password), event service (CRUD, lifecycle, public discovery with search/filters, form builder, staff invites/acceptance, judging config), auth/events HTTP handlers, server wiring, API entrypoint. Added `apperr` + centralized error mapping. Created `endpoints.md` and this context file.
- **2026-10-27** — Repo restructured to `apps/api` and `apps/web`; installed Go/Node dependencies and sqlc v1.31.1.
