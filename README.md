# Compytition

Event hosting and **pairwise judging** for hackathons, design jams, science fairs and any competition where "score it out of ten" produces noise instead of signal.

Organizers publish events and build registration forms. Participants enter solo or as a team and submit work. Judges see two submissions at a time and pick the stronger one. [CrowdBT](https://arxiv.org/abs/1802.08924) turns those comparisons into a ranking, with per-judge reliability, coverage checks and split-decision review before results are published.

No numeric scores, no weighted rubrics, no payments, ads or billing.

## Stack

| Layer | Choice |
|---|---|
| Web | Next.js 15 App Router, TypeScript strict, Tailwind, shadcn-style components |
| API | Go stdlib `net/http` + `ServeMux`, pgx, SQLC |
| Data | PostgreSQL 16, golang-migrate migrations |
| Realtime | gorilla/websocket team chat |
| Runtime | Docker Compose |

## Quick start

Requires Docker (with Compose) and `make`.

```bash
cp .env.example .env
make up            # postgres + api + web, migrations run automatically
make seed          # demo organizer, judges, participants and a finished event
```

Open **http://localhost:3100**.

Demo accounts (all use password `demo1234`):

| Role | Email |
|---|---|
| Organizer | `host@compytition.local` |
| Judges | `ava@compytition.local`, `milo@compytition.local`, `rin@compytition.local` |
| Participants | `alice@compytition.local` … `frank@compytition.local` |

The seeded event is already judged and published. Create a new event as the organizer to walk through the full flow, or reset with `make clean && make up && make seed`.

### Run without Docker

```bash
# Postgres on :5432 (or point DATABASE_URL at the compose database on :5433)
cd apps/api
export DATABASE_URL='postgres://compytition:compytition@localhost:5432/compytition?sslmode=disable'
export WEB_ORIGIN='http://localhost:3000'
go run ./cmd/api                 # :8080, auto-migrates

cd apps/web
export PATH="$HOME/.nvm/versions/node/v24.18.0/bin:$PATH"
export NEXT_PUBLIC_WS_BASE='ws://localhost:8080'
npm ci
npm run dev                      # :3000, proxies /api/v1 to :8080
```

## Commands

| Command | What it does |
|---|---|
| `make up` | Build and start db, API and web |
| `make down` | Stop the stack, keep the database volume |
| `make logs` | Follow service logs |
| `make migrate-up` / `make migrate-down` | Apply / roll back all migrations |
| `make sqlc` | Regenerate SQLC code from `apps/api/db/queries` |
| `make test` | Go tests + Vitest |
| `make lint` | `go vet` + TypeScript typecheck |
| `make seed` | Load demo data (idempotent) |
| `make build` | Build API and production web bundle |
| `make clean` | Stop the stack and delete the database volume |

## How judging works

1. **Assign** — the organizer previews a deterministic plan: each submission gets a target number of comparisons (default 6), each judge gets a budget (default 20), and a small set of shared anchors keeps the comparison graph connected.
2. **Start** — judges can now work. Pairs are randomized left/right so position does not leak preference.
3. **Compare** — each judgment picks `a` or `b` against the canonical submission pair. Judges can skip a pair, flag a conflict (which skips every remaining pair involving either entry), and undo their latest call.
4. **Review** — CrowdBT is replayed from the append-only judgment log. The organizer sees ranking, coverage, wins/losses, judge reliability, split decisions and warnings.
5. **Publish** — writes `event_results`, snapshots reliability, locks judging, sets the event to `results_published` and audits the action.

Judgments are append-only: editing supersedes the current row instead of overwriting history.

## Project layout

```
apps/api/
  cmd/api/            API entrypoint (graceful shutdown, optional auto-migrate)
  cmd/migrate/        up/down migration command
  cmd/seed/           idempotent demo data
  db/migrations/      embedded SQL migrations
  db/queries/         SQLC query sources
  internal/
    auth/             argon2id, sessions, onboarding, Google OIDC
    events/           CRUD, lifecycle, forms, staff invites
    teams/            teams, invites, join requests
    registrations/    registration forms and organizer status
    submissions/      submission editing and deadlines
    judging/          assignment, CrowdBT, judge flow, review/publish
    dashboard/        aggregated user home
    chat/             persisted team chat + websocket hub
    support/          bookmarks, notifications, feedback
    server/           ServeMux wiring and middleware chain
apps/web/
  app/                App Router pages
  components/         UI, host tabs, judge comparison, team chat
  lib/                API client, DTOs, formatters
```

## Configuration

See `.env.example`. Important values:

- `DATABASE_URL` — Postgres DSN.
- `WEB_ORIGIN` — allowed origin for CORS, CSRF and OAuth redirects.
- `COOKIE_SECURE` — set `true` behind HTTPS.
- `AUTO_MIGRATE` — run embedded migrations when the API boots.
- `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` — optional Google sign-in.
- `ADMIN_EMAILS` — comma-separated emails allowed into the feedback queue.
- `NEXT_PUBLIC_WS_BASE` — websocket origin for team chat (defaults to same-origin).

## Security notes

- Passwords: argon2id (`m=65536`, `t=3`, `p=2`), PHC encoded.
- Sessions: 32-byte opaque token, only its SHA-256 stored, HttpOnly + SameSite=Lax cookie.
- CSRF: Origin/Referer allowlist plus SameSite cookies.
- Login lockout: 8 failures per IP+email hash in 15 minutes → 15-minute lock.
- Every handler re-checks authorization; organizer, staff and judge permissions are resolved server-side.
- Staff invites are email-bound and require the logged-in account to match.
- Rate limiting is in-process (240 req/min global, 20 req/min auth). A shared store is required for horizontal scaling.

## API contract

`endpoints.md` is the living endpoint inventory. All routes live under `/api/v1` and use a consistent error envelope:

```json
{ "error": { "code": "validation_failed", "message": "Some fields need attention.", "fields": { "email": "..." } } }
```

`context.md` records architecture decisions, data-model invariants and known tech debt.

## Tests

```bash
make test
```

Go: assignment invariants, CrowdBT replay/simulation, service packages.
Web: API client error mapping, formatters and date helpers via Vitest.

## License

Private project. All rights reserved.
