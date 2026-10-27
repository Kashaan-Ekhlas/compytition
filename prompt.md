# MASTER PROMPT: Build Compytition End to End

You are a senior full-stack engineer building **Compytition**, an event hosting and judging platform. You own this project from empty repo to a working, tested, locally-running product. Read this entire prompt before writing any code, then work through the phases in order.

> **THIS IS A PRODUCTION APPLICATION, NOT A PROTOTYPE.**
> Write code as if real users, real organizers, and real data hit it tomorrow. No placeholder logic, no hardcoded secrets, no "TODO: add auth later", no mocked data paths left in shipping code, no skipped validation, no silent error swallowing. If something is incomplete, say so in `context.md` and track it. Do not fake it.

---

## 0. Working Agreements

1. **Read `design.md` before touching any UI.** It is the source of truth for visual style.
2. **Maintain `context.md` at the repo root (see Section 1).** Updating it is part of every task, not an afterthought.
3. **Execute the stack and decisions below as written.** If you believe something is wrong or a better option exists, note it in `context.md` under "Open Questions" and continue with the specified approach. Do not silently substitute.
4. **Work in small, verifiable increments.** After each phase: run tests, run linters, run the app, and only then move on.
5. **No scope creep into monetization.** Do NOT build or stub anything related to ads, payments, subscriptions, pricing tiers, billing, or hosting fees. No Stripe, no plan fields, no paywalls, no "upgrade" UI. (The PRD mentions hosting fees as a business model; it is explicitly out of scope for now.)
6. **Ask only when blocked.** Make reasonable decisions, document them in `context.md`, and keep moving.
7. **Local first.** Everything must run on localhost via Docker Compose. Cloud deployment (Vercel, Cloud Run, hosted Postgres) is deferred, but write code that will not need rewriting to deploy (12-factor config via env vars, stateless API, health checks).

---

## 1. `context.md` Protocol (mandatory)

Create `context.md` at the repo root in Phase 0 and keep it current. It is the memory of this project for any future agent or human.

Structure:

```
# Project Context
## Overview            (one paragraph: what this is, current state)
## Architecture        (services, folders, how they talk)
## Decisions Log       (date, decision, reason; append-only)
## Data Model Notes    (tables, key constraints, invariants)
## Judging System      (assignment algorithm + CrowdBT details as implemented)
## Phase Progress      (checklist of phases and PRD requirement IDs completed)
## How To Run          (exact commands: setup, dev, test, migrate, sqlc generate)
## Env Vars            (name, purpose, example; never real secrets)
## Known Issues / Tech Debt
## Open Questions
## Changelog           (newest first: what changed, which files, why)
```

Rules:
- Update `context.md` at the end of every meaningful unit of work (a feature, a migration, a bug fix, a decision).
- Log *why*, not just *what*.
- Never delete history from the Decisions Log or Changelog.
- Before starting any new session of work, re-read `context.md` and `design.md`.

---

## 2. Product Summary

**Compytition** is a simple, clutter-free, non-rigid, fast platform for hosting events (hackathons, competitions, contests) and judging them fairly.

**Problems we solve**
- *Organizers:* existing platforms are cluttered (UnStop) or rigid in scoring (Devfolio).
- *Participants:* unfair marks when some judges are strict and others lenient; events are hard to find; people can add you to teams without permission and you cannot leave.
- *Judges:* constantly re-checking score weights while scoring.

**Top 3 principles (every decision is measured against these)**
1. Minimalistic but configurable.
2. Fewest taps for judging and for finding events (low cognitive load).
3. Eliminate unfair scoring. **Judging is always pairwise. There are no numeric scores or weighted rubrics, ever.**

---

## 3. Tech Stack (fixed)

| Layer | Choice | Notes |
|---|---|---|
| Frontend | Next.js (App Router) + TypeScript (strict) + Tailwind CSS + shadcn/ui | Event discovery, profiles, dashboards, judging, onboarding |
| Backend | Go + `net/http` only | Use stdlib `http.ServeMux` (Go 1.22+ method and path patterns, e.g. `"GET /events/{id}"`), stdlib middleware pattern, `encoding/json`. **No Chi, Gin, Echo, Fiber, Gorilla Mux, or any router dependency.** |
| Database | PostgreSQL + `pgx` (pgxpool) + **SQLC** | SQL-first. Generated type-safe Go from `.sql` query files. Schema managed by migrations (use `golang-migrate` or `goose`; pick one, log it). |
| Realtime | `gorilla/websocket` | Team chat between teammates only |
| Containers | Docker + Docker Compose | Compose runs: `api`, `db` (Postgres), `web` (Next.js). One command to bring up everything. |
| CI | GitHub Actions | Go tests + lint, frontend typecheck + lint + tests, SQLC drift check, migration check |
| Frontend hosting (later) | Vercel | Do not configure yet |
| Backend hosting (later) | Google Cloud Run | Do not configure yet; just keep the API Dockerfile Cloud-Run-compatible (reads `PORT`, graceful shutdown, `/healthz`) |

**Allowed additional Go dependencies (keep the list minimal and justify any others in `context.md`):** `pgx`, `golang.org/x/crypto' or wtv it is which has bcrypt, `golang.org/x/oauth2` (Google sign-in), `gorilla/websocket`, a migration tool, a UUID library if needed, and test helpers (e.g. `testcontainers-go` or a Compose-based test DB).

---

## 4. Repository Layout

```
compytition/
  context.md
  design.md
  README.md
  docker-compose.yml
  .env.example
  .github/workflows/        ci.yml
  apps/
    web/                    Next.js app
      app/                  routes (App Router)
      components/           ui/ (shadcn), feature components
      lib/                  api client, utils, hooks
    api/                    Go service
      cmd/api/main.go
      internal/
        config/
        server/             router, middleware (logging, recover, request-id, auth, CORS, rate limit)
        auth/               sessions, password hashing, Google OAuth
        users/ events/ teams/ registrations/ submissions/
        judging/            assignment + crowdbt + sessions
        chat/               websocket hub
        db/                 sqlc generated code
      db/
        migrations/
        queries/            *.sql for sqlc
        sqlc.yaml
      Dockerfile
```

Feature packages are organized by domain, with handlers -> service -> sqlc queries. Keep handlers thin and put business rules in services so they are unit-testable.

---

## 5. Roles and Permissions

A user is a single account. Roles are **contextual**, not global:
- **Participant:** default. Registers for events, joins teams, submits work.
- **Organizer:** for events they created (and co-organizers they add). Full control over that event only.
- **Judge:** assigned per event by an organizer (via invite link or email match). Can only see what is assigned to them.

Enforce authorization **server-side on every endpoint** with a central permission-check layer (e.g. `canManageEvent(user, event)`, `isJudgeFor(user, event)`). Never rely on the frontend to hide things. Return 404 (not 403) where revealing existence would leak information. Write tests for permission boundaries.

---

## 6. Onboarding Flow (first screen the user sees)

The app opens on an **Onboarding screen** (retro-styled, large text, minimal). Flow:

1. **Welcome screen:** one-line pitch, two big actions: "Sign in with Google" and "Use email". A secondary "Browse events" link lets unauthenticated users view the public discovery page (read-only).
2. **Account:** email + password (argon2id) or Google OAuth.
3. **Pick a username:** unique, validated live (debounced availability check).
4. **Profile basics:** name, college, course, year of study. All saved to the profile and reused for autofill. Skippable, but nudged.
5. **What brings you here?** (multi-select, changeable later): *Find events*, *Host events*, *Judge events*. This only tunes the default dashboard landing view. It does not change permissions.
6. **Land on Dashboard.** If the user arrived through a judge invite link or team invite link, return them to that flow after onboarding.

Onboarding progress must persist (user can close the tab and resume). No dark patterns; every step has a clear back button.

---

## 7. Functional Requirements

Implement **every PRD requirement below**. P0 = MVP (must ship), P1 = important (ship after all P0 is done and tested), P2 = nice-to-have (only if explicitly time-boxed later). Track completion by requirement in `context.md`.

### 7.1 User Accounts and Profiles
- [P0] Register, log in, log out.
- [P0] Unique username; change password.
- [P0] View and edit profile.
- [P0] Save common info: name, college, course, year of study.
- [P0] Securely access only permitted resources.
- [P0] Sign in with Google (design the auth layer so other OAuth providers can be added without refactoring).

### 7.2 Event Discovery
- [P0] Dedicated page listing published events.
- [P0] Search by name and keywords (use Postgres full-text search with a GIN index).
- [P0] Filter by category, participation format (individual / team / both), registration status (upcoming / open / closed / ended).
- [P0] Event detail page: details, deadlines, eligibility, team requirements.
- [P1] Bookmark events.

### 7.3 Event Hosting
- [P0] Create, edit, publish, unpublish, manage events (draft -> published -> closed -> results published).
- [P0] Configure details, registration deadlines, team size limits (min/max).
- [P0] Custom registration forms with required and optional fields (text, long text, select, multi-select, URL, number, file link; schema-driven, stored as JSONB with validation).
- [P0] Manage participants, submissions, judges.
- [P0] Configure judging (see Section 8) and finalize results.
- [P1] Duplicate a previous event; export results (CSV).

### 7.4 Team Formation
- [P0] Create a team with a name (scoped to an event).
- [P0] Team owner invites members and approves or rejects join requests.
- [P0] **Anyone can leave a team at any time without approval.** If the owner leaves, ownership transfers (to the longest-standing member) or the team dissolves if empty.
- [P0] **Nobody can be added to a team without their explicit acceptance.** Invitations are pending until accepted. (This directly addresses a core participant pain point.)
- [P0] Only authorized members can modify team details and membership.
- [P0] Enforce the event's team size limits transactionally (no race conditions that exceed max).
- [P1] "Teams looking for members" discovery per event.
- **Team chat:** websocket chat between members of a team (Section 9).

### 7.5 Registration and Submissions
- [P0] Register individually or as a team, per event settings.
- [P0] Registration form auto-populates from profile; user reviews and edits autofilled values and answers event-specific questions.
- [P0] View registered events and registration status.
- [P0] Submit and update work before the deadline where permitted (title, description, links, optional repo / demo / file URLs). Lock after deadline.
- [P1] Deadline reminders (in-app notifications first; email provider abstraction is optional and can be a stub interface with a logging implementation, clearly noted in `context.md`).

### 7.6 Pairwise Judging
See Section 8 (this is the heart of the product).

### 7.7 Dashboard and Settings
- [P0] Dashboard: my events, registrations, teams, pending invitations.
- [P0] Organizer view: events I manage.
- [P0] Judge view: assigned events and pending comparisons.
- [P0] Edit profile settings, change password, log out.
- [P0] Report bugs / submit feedback (simple in-app form stored in DB, viewable via a protected admin query or logged; keep minimal).

---

## 8. Pairwise Judging System (critical, build carefully)

### 8.1 Rules
- **Judging is always pairwise.** A judge sees two submissions side by side and picks the stronger one. No numeric scores. No weighted criteria math.
- **Criteria are guidance, not weights.** The organizer writes a short list of criteria (e.g. "Innovation", "Execution"). During comparison the criteria are shown as a compact, always-visible, pinned strip so the judge never has to go look them up. This removes the "checking weights constantly" pain point.
- Judges can **skip** a pair ("can't decide / conflict of interest") and flag a conflict of interest (judges must never be shown submissions from their own team or flagged conflicts).
- Judgments are saved immediately (every click persists). Judges can close the tab and **resume** exactly where they left off. Judgments are editable until the organizer locks judging.
- Show judge progress ("34 of 60 comparisons") and keep the UI to the fewest taps possible: ideally one tap or keypress (`1`/`2` or left/right arrow) per comparison, with auto-advance to the next pair.

### 8.2 Aggregation: CrowdBT
Implement **CrowdBT** (Chen et al., 2013, "Pairwise Ranking Aggregation in a Crowdsourced Setting") in Go as a self-contained package `internal/judging/crowdbt`:
- Each submission has a latent score (Gaussian posterior, mean and variance).
- Each judge has a reliability parameter (Beta prior), so noisy, strict, lenient, or inconsistent judges are down-weighted automatically. This is how unfair-scoring bias is mitigated.
- Support the online update step per judgment and a **deterministic full recompute from the complete judgment log** (used for finalization so results are reproducible and auditable).
- Pure functions, no DB calls inside the package. Fully unit-tested, including a **simulation test**: generate synthetic submissions with known true ranks, simulate judges with varying noise, accuracy bias and strictness, and assert the recovered ranking correlates strongly (e.g. Kendall tau above a documented threshold) with ground truth, and that unreliable judges receive lower reliability.
- Document the exact hyperparameters, priors, and convergence criteria in `context.md`.

### 8.3 Assignment With Overlap (essential for CrowdBT)
CrowdBT can only calibrate judge reliability if judges **overlap** on the same submissions and the global comparison graph is **connected**. Therefore assignment must NOT be "split submissions evenly among judges". Implement an assignment service that, given N submissions, J judges, and organizer config (target comparisons per submission `k`, comparisons per judge budget), generates pairs such that:

1. **Connectivity:** the comparison graph over all submissions is a single connected component (verify programmatically; fail loudly if not).
2. **Coverage:** every submission appears in at least `k` comparisons (default configurable, sensible default like 6-10, capped by what is feasible).
3. **Judge overlap:** every pair of judges shares a meaningful number of common submissions (include a shared "anchor set" that every judge sees, plus randomized overlapping blocks), so each judge's reliability and leniency can be estimated relative to the others.
4. **Balance:** per-judge workload is roughly equal; per-submission exposure is roughly equal; each submission is compared against a spread of opponents (not the same opponent repeatedly); position (left/right) is randomized and balanced to avoid ordering bias.
5. **Conflict-free:** exclude pairs involving a judge's own/conflicted submissions; re-balance if exclusions break coverage.
6. **Single judge case:** works with one judge (then reliability is simply not identifiable; fall back gracefully, still produce a ranking).
7. **Optional adaptive phase (P1):** after the base round, offer organizers a "refine top of ranking" round that selects additional pairs where the ranking is most uncertain (highest posterior overlap / information gain), still maintaining judge overlap.

Expose the plan as an **assignment preview** to the organizer before judging starts (coverage stats, connectivity check, per-judge counts, overlap matrix). Include thorough tests for the assignment algorithm across edge cases (few submissions, many judges, conflicts, odd counts).

### 8.4 Organizer Review (P1)
- Inspect judge disagreement (pairs where judges conflict), per-judge reliability, comparison coverage and per-submission confidence.
- Review ranking, then **explicitly publish** final results. Publishing is a deliberate, audited action; after publishing, judgments are immutable.
- Judging log is append-only with timestamps (audit trail).

---

## 9. Team Chat (WebSockets)

- `gorilla/websocket` on top of `net/http` upgrade handler.
- One hub per team; only current team members may connect (authenticate via session cookie; verify membership on connect and on every message; validate `Origin`).
- Persist messages in Postgres; load recent history over REST (paginated) and stream live over WS.
- Handle ping/pong, read limits, write deadlines, graceful close, reconnect with backoff on the client, and clean up when a member leaves a team (disconnect immediately).
- Basic message validation (length limit, rate limit per connection).
- Scope: team chat only. No DMs, no global chat.

---

## 10. Database

- PostgreSQL, managed entirely through migrations (never edit applied migrations; add new ones). Every migration has up and down.
- Use SQLC: write queries in `db/queries/*.sql`, generate into `internal/db`. CI fails if generated code is stale.
- Use `pgxpool`. Use transactions for multi-step invariants (team size enforcement, registration, ownership transfer, judging assignment creation).
- Use UUID primary keys, `created_at` / `updated_at`, appropriate foreign keys with sensible `ON DELETE`, unique constraints (username case-insensitive via `citext` or lower-index, one registration per user per event, one judgment per judge per pair), CHECK constraints, and indexes for every query path (list events, search, filters, dashboard queries).
- Core entities (extend as needed, document in `context.md`): users, auth_identities (password / google), sessions, profiles, events, event_form_fields (or JSONB schema), registrations, teams, team_members, team_invites, team_join_requests, submissions, judge_assignments (event_judges), judging_pairs, judgments, judge_reliability snapshots, event_results, bookmarks, notifications, chat_messages, feedback_reports, audit_log.

---

## 11. API Conventions

- REST-ish JSON under `/api/v1`. Method-specific `ServeMux` patterns.
- Consistent error envelope: `{ "error": { "code": "...", "message": "...", "fields": {...} } }` with correct HTTP status codes.
- Request validation on every input, with explicit size limits (`http.MaxBytesReader`), strict JSON decoding (reject unknown fields where appropriate).
- Middleware chain: request ID, structured logging (`log/slog`), panic recovery, CORS (strict allowlist from env), auth/session loading, rate limiting (stdlib-based token bucket; stricter on auth endpoints), security headers.
- Cursor or limit/offset pagination on all list endpoints.
- Context propagation and timeouts everywhere. Graceful shutdown. `/healthz` and `/readyz`.
- Document endpoints in `context.md` or an `openapi.yaml` (preferred, kept in sync).

---

## 12. Security Baseline (non-negotiable)

- Passwords: argon2id with sane parameters. Constant-time comparisons. Never log secrets or tokens.
- Sessions: server-side sessions stored in Postgres, opaque random tokens, `HttpOnly`, `Secure` (in prod), `SameSite=Lax` cookies. CSRF protection for state-changing requests (double-submit token or Origin checks).
- Google OAuth with `state` and PKCE; verify ID token claims.
- Authorization checks on every route (Section 5). No IDOR: always scope queries by the authenticated user / event membership.
- Rate limiting and lockout/backoff on login. Generic error messages on auth failures (no user enumeration).
- All SQL via SQLC parameterized queries. No string-built SQL.
- Sanitize and escape user content on render (React escapes by default; never use `dangerouslySetInnerHTML` on user content). Validate URLs submitted by users (http/https only).
- Secrets only via env vars; `.env.example` committed, `.env` gitignored.
- Containers run as non-root, minimal final image (distroless or alpine) for the Go API.

---

## 13. Frontend Requirements

- Next.js App Router, TypeScript `strict`, server components where sensible, client components only where interaction demands.
- Tailwind + shadcn/ui, **re-themed per `design.md`** (shadcn components must be restyled to the retro-tech look, not left default).
- Routes (suggested): `/` (onboarding or dashboard redirect), `/events` (discovery), `/events/[id]`, `/events/[id]/register`, `/events/[id]/team`, `/events/[id]/submit`, `/dashboard`, `/profile`, `/settings`, `/host` (organizer home), `/host/events/new`, `/host/events/[id]` (manage: details, form builder, participants, submissions, judges, judging setup, results), `/judge` (assigned events), `/judge/events/[id]` (comparison screen), plus auth and onboarding routes.
- A typed API client (generate types from OpenAPI or hand-write carefully), centralized error handling, loading and empty states for **every** view, optimistic updates where safe.
- Forms: React Hook Form + Zod with server error mapping.
- Accessibility: WCAG AA contrast, full keyboard navigation, visible focus rings, semantic HTML, proper labels and ARIA, `prefers-reduced-motion` respected. Readability is a core requirement: **large base font size, generous spacing**.
- Responsive from mobile (360px) up. The judging screen must work well on a phone.
- Use the Next.js proxy/rewrites to call the API so local dev has no CORS pain, while keeping API CORS strict.

---

## 14. Testing, CI, and Quality Gates

- **Go:** table-driven unit tests for services; integration tests against a real Postgres (Compose or testcontainers); handler tests with `httptest`; permission-boundary tests; CrowdBT simulation tests; assignment algorithm tests; websocket tests.
- **Frontend:** TypeScript typecheck, ESLint, component tests (Vitest + Testing Library) for critical components (onboarding, registration form, comparison screen), and a small Playwright suite for the golden paths if time allows (P1).
- **GitHub Actions `ci.yml`:** on every PR and push: `go vet`, `staticcheck` or `golangci-lint`, `go test ./... -race`, SQLC generate-and-diff check, migration up/down check against a Postgres service, frontend `tsc --noEmit`, lint, test, and `next build`. Cache dependencies.
- Seed script (`make seed`) generating realistic demo data (users, events, submissions, judges) for local development and for exercising the judging flow. Seed data must be clearly separated from production code paths.
- A `Makefile` (or `justfile`) with: `up`, `down`, `migrate-up`, `migrate-down`, `sqlc`, `test`, `lint`, `seed`.

---

## 15. Docker / Local Environment

- `docker-compose.yml` with `db` (Postgres with healthcheck and a named volume), `api` (multi-stage build, depends on healthy db, runs migrations on start or via a one-shot service), `web` (Next.js dev container with hot reload).
- `docker compose up` must give a fully working app at `http://localhost:3000` with API at `http://localhost:8080`.
- Provide `.env.example` documenting every variable. Fail fast with clear messages when required config is missing.

---

## 16. Design

Follow `design.md` strictly. Summary: **very minimalistic retro-tech, but always readable.** Big text, high contrast, restrained palette, pixel/CRT flavor used as seasoning (headings, badges, borders, small illustrations), never at the cost of legibility. Reference screenshots were provided (halftone/CRT-monitor collaboration landing page; 80s synthwave arcade pixel-art page). Take the *attitude* from them (chunky outlines, hard shadows, sticker labels, pixel headings, halftone texture in decorative areas), not their visual noise.

---

## 17. Phased Execution Plan

Complete each phase fully (code + tests + `context.md` update) before the next.

- **Phase 0, Foundations:** repo, monorepo layout, Docker Compose, Go skeleton with stdlib router + middleware + config + health, Next.js skeleton with themed shadcn setup per `design.md`, SQLC + migrations tooling, CI pipeline, `context.md`, `README.md`, Makefile.
- **Phase 1, Accounts and Onboarding:** schema, auth (email + Google), sessions, profile, username, onboarding flow, protected routes.
- **Phase 2, Events:** hosting CRUD, lifecycle, form builder, discovery page with search and filters, event detail.
- **Phase 3, Teams and Registration:** teams, invites, join requests, leave rules, size limits, registration with autofill, dashboard (participant view).
- **Phase 4, Submissions:** submit and update, deadline locking, organizer participant and submission management.
- **Phase 5, Judging core:** judge invites and assignment, assignment algorithm with overlap and preview, comparison UI, resumable sessions, CrowdBT package, aggregation, results review and publish.
- **Phase 6, Team chat:** websocket hub, persistence, client.
- **Phase 7, P1 features:** bookmarks, duplicate event, CSV export, reminders, teams-looking-for-members, judging disagreement inspection, adaptive refinement round.
- **Phase 8, Hardening:** security review pass, performance pass (indexes, N+1 queries, query plans on discovery and dashboard), accessibility audit, error/empty/loading state audit, docs, final `context.md` cleanup.

At the end of each phase, post a short summary: what was built, which PRD requirement IDs were completed, what was tested, what is outstanding.

---

## 18. Explicitly Out of Scope (for now)

Ads, payments, subscriptions, pricing, billing, hosting-fee logic, plan tiers, cloud deployment (Vercel / Cloud Run / managed Postgres setup), global chat, native mobile apps, and non-pairwise scoring methods.

---

## 19. Definition of Done

- All P0 requirements implemented, tested, and working end to end via `docker compose up`.
- CI green. No lint errors. No flaky tests.
- No hardcoded secrets, no dead code, no leftover mock data in production paths.
- `context.md` accurate and current; `README.md` lets a new developer run everything in under 10 minutes.
- The core story works flawlessly: **an organizer creates and publishes an event, participants form teams and register and submit, multiple judges with overlapping assignments complete pairwise comparisons, CrowdBT produces a ranking, and the organizer publishes results.**

Remember: **production quality, every time.**

## 20 Code Review 
Use Code Rabbit CLI for review and ask it to put the review in a file review.md. Give it rules for review from rules.md
