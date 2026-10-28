# Compytition Endpoints

All API routes are under `/api/v1` unless noted. JSON error envelope:

```json
{ "error": { "code": "validation_failed", "message": "Some fields need attention.", "fields": { "email": "..." } } }
```

Auth is a server-side session cookie (`compytition_session`, HttpOnly, SameSite=Lax). State-changing requests must come from `WEB_ORIGIN` (Origin/Referer check). List endpoints accept `limit` (max 100) and `offset`. The web client surfaces the first field error when `fields` is present, so validation failures read specifically instead of "Some fields need attention."

Status legend: ✅ implemented

## Health

| Status | Method | Path | Auth | Notes |
|---|---|---|---|---|
| ✅ | GET | `/healthz` | public | `{ status: "ok" }`; process liveness |
| ✅ | GET | `/readyz` | public | pings Postgres; 503 when the database is not ready |

## Auth & Account

| Status | Method | Path | Auth | Notes |
|---|---|---|---|---|
| ✅ | GET | `/auth/config` | public | `{ google_enabled }` |
| ✅ | GET | `/auth/me` | optional | `{ user, profile, google_enabled }` - 200 with `user: null` when logged out |
| ✅ | POST | `/auth/register` | public | `{ email, password, username? }`; sets cookie |
| ✅ | POST | `/auth/login` | public | `{ email, password }`; lockout after 8 failures / 15 min |
| ✅ | POST | `/auth/logout` | any | clears cookie |
| ✅ | GET | `/auth/username-available?username=` | public | `{ available, reason }` |
| ✅ | PATCH | `/auth/username` | user | `{ username }` |
| ✅ | PATCH | `/auth/onboarding` | user | `{ step, completed, intents: ["find","host","judge"] }`, resumable |
| ✅ | POST | `/auth/password` | user | `{ current_password, new_password }`; revokes other sessions |
| ✅ | GET | `/auth/google/start?redirect=/path` | public | 302 to Google (PKCE S256); 503 if unconfigured |
| ✅ | GET | `/auth/google/callback` | public | verifies OIDC, sets cookie, redirects to web |
| ✅ | GET | `/profile` | user | `{ user, profile }` |
| ✅ | PUT | `/profile` | user | `{ full_name, college, course, year_of_study, country, state, city }` |

## Events

| Status | Method | Path | Auth | Notes |
|---|---|---|---|---|
| ✅ | GET | `/events` | public | `q`, `category`, `format`, `status` (`upcoming`/`open`/`closed`/`ended`), `mode` (`online`/`onsite`), `country`, `state`, `city`, `limit`, `offset`; `{ events, total, categories }` |
| ✅ | GET | `/events/{id}` | optional | `{ event }`; drafts hidden unless organizer/staff |
| ✅ | POST | `/events` | user | create draft; event fields + `criteria`; accepts `is_remote`, `country`, `state`, `city`, `merge_registration_submission` |
| ✅ | PATCH | `/events/{id}` | organizer/staff | edit fields; blocked after results |
| ✅ | POST | `/events/{id}/publish` | organizer/staff | draft → published; requires start + deadlines |
| ✅ | POST | `/events/{id}/unpublish` | organizer/staff | published → draft; blocked once registrations exist |
| ✅ | POST | `/events/{id}/close` | organizer/staff | published → closed |
| ✅ | DELETE | `/events/{id}` | organizer/staff | draft only |
| ✅ | GET | `/events/{id}/manage` | organizer/staff | event detail for the manage screen |
| ✅ | GET | `/host/events` | user | events I own or co-organize |
| ✅ | GET | `/events/{id}/form?kind=registration\|submission` | public (drafts: organizer/staff/judge) | `{ fields }` for the requested form kind; draft events return 404 to everyone else |
| ✅ | PUT | `/events/{id}/form?kind=registration\|submission` | organizer/staff | `{ fields: [...] }` replaces that kind's schema |
| ✅ | GET | `/events/{id}/staff?role=judge\|co_organizer` | organizer/staff | staff + pending invite accept URL |
| ✅ | POST | `/events/{id}/staff` | organizer/staff | `{ email, role }`; email or username accepted; existing accounts are added as `accepted`, unknown emails receive an invite link; new judges only while judging is in `setup` |
| ✅ | DELETE | `/events/{id}/staff/{staffId}` | organizer/staff | remove judge (only before judging starts) / co-organizer |
| ✅ | POST | `/invites/staff/{token}/accept` | user | existing matching account accepts immediately; unknown email auto-accepts after signup; blocked after judging starts or results publish |
| ✅ | GET | `/events/{id}/judging-config` | organizer/staff/judge | `{ config, criteria }`; preview/assign derive the effective config from submission and judge counts |
| ✅ | GET | `/events/{id}/judging/preview` | organizer/staff | derived config; blocked until the submission deadline has passed |
| ✅ | GET | `/judge/events` | judge | accepted judge assignments |

## Teams

| Status | Method | Path | Auth | Notes |
|---|---|---|---|---|
| ✅ | POST | `/events/{id}/teams` | user | create team; owner auto-joins |
| ✅ | GET | `/events/{id}/teams` | optional | list teams (+ viewer invite/request flags) |
| ✅ | GET | `/events/{id}/teams/mine` | user | my team for this event |
| ✅ | GET | `/teams` | user | my teams |
| ✅ | GET | `/teams/{id}` | optional | detail; members + pending lists visible only to members and event managers |
| ✅ | PATCH | `/teams/{id}` | owner | name, looking-for-members |
| ✅ | DELETE | `/teams/{id}` | owner | dissolve when safe |
| ✅ | POST | `/teams/{id}/invites` | owner | `{ username }`; explicit acceptance required; unknown usernames return `username not found` |
| ✅ | GET | `/teams/{id}/invite-link` | owner | `{ invite_link }`; stable shareable `/invites/team/{token}` link |
| ✅ | GET | `/team-invites/preview?token=` | public | team, event and member count for the invite landing page |
| ✅ | POST | `/team-invites/accept-link` | user | `{ token }`; existing accounts join immediately, new accounts return after signup |
| ✅ | GET | `/teams/{id}/invites` | owner | pending invites |
| ✅ | DELETE | `/teams/{id}/invites/{inviteId}` | owner | cancel invite |
| ✅ | POST | `/team-invites/{id}/accept` | invitee | transactional size check |
| ✅ | POST | `/team-invites/{id}/reject` | invitee | |
| ✅ | POST | `/teams/{id}/join-requests` | user | `{ message }` |
| ✅ | GET | `/teams/{id}/join-requests` | owner | pending requests |
| ✅ | POST | `/team-join-requests/{id}/accept` | owner | transactional size check |
| ✅ | POST | `/team-join-requests/{id}/reject` | owner | |
| ✅ | POST | `/teams/{id}/leave` | member | always allowed; keeps a team submission by transferring it, transfers ownership, or dissolves an empty team |
| ✅ | GET | `/team-invites` | user | my pending invites |
| ✅ | GET | `/team-join-requests` | user | my pending requests |

## Registrations

| Status | Method | Path | Auth | Notes |
|---|---|---|---|---|
| ✅ | POST | `/events/{id}/registrations` | user | individual or team; merges profile autofill + answers |
| ✅ | GET | `/events/{id}/registrations/me` | user | my registration |
| ✅ | PATCH | `/events/{id}/registrations/me` | user | update answers before registration closes |
| ✅ | DELETE | `/events/{id}/registrations/me` | user | cancel before registration closes |
| ✅ | GET | `/events/{id}/registrations` | organizer/staff | participants, pagination, status filter |
| ✅ | PATCH | `/registrations/{id}/status` | organizer/staff | registered / waitlisted / cancelled / disqualified |
| ✅ | GET | `/registrations` | user | my registered events |

## Submissions

| Status | Method | Path | Auth | Notes |
|---|---|---|---|---|
| ✅ | GET | `/events/{id}/submissions/me` | participant | my / my team's submission |
| ✅ | POST | `/events/{id}/submissions` | participant | create before submission deadline; `answers` are validated against submission form fields |
| ✅ | PATCH | `/submissions/{id}` | owner | update before deadline; blocked after judging locks; same answer validation |
| ✅ | GET | `/events/{id}/submissions` | organizer/staff | all submissions for the event, including custom `answers`; optional `status=draft\|submitted` |
| ✅ | GET | `/submissions` | user | my submissions, including custom `answers` |

## Judging

| Status | Method | Path | Auth | Notes |
|---|---|---|---|---|
| ✅ | GET | `/events/{id}/judging/preview` | organizer/staff | derived config from submission/judge counts; deterministic plan: coverage, connectivity, per-judge load, overlap, warnings; blocked before the submission deadline |
| ✅ | POST | `/events/{id}/judging/assign` | organizer/staff | persist pairs using the derived config; only while `judging_status=setup` and after the submission deadline |
| ✅ | POST | `/events/{id}/judging/start` | organizer/staff | requires at least one pair; sets `live` |
| ✅ | GET | `/judge/events/{id}/next` | judge | next pending pair + progress (`last_pair_id`) + criteria; submissions include custom `answers` |
| ✅ | POST | `/judge/events/{id}/comparisons/{pairId}` | judge | `{ winner: "a"\|"b" }` or `{ skipped: true }` or `{ conflict: true }` |
| ✅ | POST | `/judge/events/{id}/comparisons/{pairId}/undo` | judge | clears current judgment and returns pair to pending; undoing a conflict restores skipped pairs |
| ✅ | GET | `/judge/events/{id}/progress` | judge | `{ total, done, last_pair_id }` |
| ✅ | GET | `/events/{id}/judging/review` | organizer/staff | ranking, reliability (including zero-judgment judges), split decisions grouped by submission pair, warnings, confidence |
| ✅ | POST | `/events/{id}/judging/publish` | organizer/staff | `{ force? }`; writes results, snapshots reliability, locks judging, audits |
| ✅ | GET | `/events/{id}/results` | public / organizer | published ranking; organizer can preview before publication |

## Chat

| Status | Method | Path | Auth | Notes |
|---|---|---|---|---|
| ✅ | GET | `/teams/{id}/messages?before=&limit=` | member | persisted history |
| ✅ | GET | `/teams/{id}/chat` | member | WebSocket upgrade; Origin checked; membership re-checked per message |

## Dashboard / Support

| Status | Method | Path | Auth | Notes |
|---|---|---|---|---|
| ✅ | GET | `/dashboard` | user | registrations, teams, invites, hosted events, judging assignments |
| ✅ | GET | `/notifications` | user | `unread_only` filter |
| ✅ | POST | `/notifications/{id}/read` | user | mark one read |
| ✅ | POST | `/notifications/read-all` | user | mark all read |
| ✅ | POST | `/feedback` | optional | bug reports / feedback |
| ✅ | GET | `/admin/feedback` | admin | protected feedback queue |
| ✅ | GET | `/bookmarks` | user | saved events |
| ✅ | POST | `/events/{id}/bookmark` | user | save published/closed event; drafts return 404 |
| ✅ | DELETE | `/events/{id}/bookmark` | user | remove bookmark |

## Not implemented (P1)

These are intentionally deferred and have no routes yet: event duplication, CSV result export, email deadline reminders, and emailed staff invites (invite links are displayed in the organizer UI instead).

## Frontend routes

| Status | Route | Notes |
|---|---|---|
| ✅ | `/` | single-theme landing page with hero copy and CTAs |
| ✅ | `/login` | email/password + Google button when configured; red error toast; honours `?next=` |
| ✅ | `/register` | account creation with password checklist, eye toggles and gated submit; hands off to onboarding with `?next=` |
| ✅ | `/onboarding` | seven resumable steps: username, name, college, course, year, country/state/city, intents |
| ✅ | `/events` | discovery with search, category, format, status, mode and location filters plus "use my location" |
| ✅ | `/events/[id]` | public detail, criteria, form preview, results, save/share actions |
| ✅ | `/events/[id]/register` | autofilled registration form with team choice; includes submission fields when forms are merged |
| ✅ | `/events/[id]/team` | team create/join/invite/manage, copyable invite link + websocket chat |
| ✅ | `/events/[id]/submit` | submission editor with deadline state; prefills merged answers and shows custom fields |
| ✅ | `/dashboard` | registrations, teams, invites, drafts, hosting and judging queues |
| ✅ | `/settings` | profile, username, password, bookmarks, feedback, logout |
| ✅ | `/host` | organizer home grouped by draft / running / finished |
| ✅ | `/host/events/new` | event creation |
| ✅ | `/host/events/[id]` | manage tabs: overview, form, participants, submissions, judges, judging |
| ✅ | `/judge` | judge queue with progress bars |
| ✅ | `/judge/events/[id]` | pairwise comparison screen with custom answers, keyboard 1/2, skip, conflict, undo |
| ✅ | `/invites/staff/[token]` | staff invite acceptance; existing accounts accept now, new accounts return after signup |
| ✅ | `/invites/team/[token]` | public team invite preview; acceptance requires sign-in and auto-joins existing accounts |

## Agent notes

- Update this file whenever a route is added, renamed, or its payload changes.
- Keep Go wiring in `apps/api/internal/server/*_routes.go`; handler methods live in the domain packages.
- `context.md` holds architecture decisions, data invariants and tech debt.
