# Code Review — Compytition

**Date:** 2026-10-27  
**Scope:** `apps/api` (Go services, handlers, SQLC queries, middleware, config) and `apps/web` (auth/onboarding redirects, API client usage).  
**Method:** manual line-by-line audit against `rules.md`; CodeRabbit CLI was attempted but is still signed out (`coderabbit auth status` → `signed out`), so this review is the fallback agreed with the user.

Severity reflects user impact: **High** = data loss, security bypass, or broken core flow; **Medium** = incorrect behaviour or a realistic race; **Low** = hardening, consistency, or polish.

## Summary

| # | Severity | Area | Finding | Status |
|---|---|---|---|---|
| 1 | High | Teams | Leaving a team could cascade-delete a team submission through `registrations → submissions`. | Fixed |
| 2 | High | Judging | Split decisions were grouped by `pair_id`, so different judges comparing the same two entries were never detected as a disagreement. | Fixed |
| 3 | High | Web auth | `?next=` accepted `//evil.example`, `/\evil.example`, etc. (open redirect). | Fixed |
| 4 | Medium | Config | Malformed integer/boolean env vars silently fell back to defaults, hiding deployment mistakes. | Fixed |
| 5 | Medium | Events | Unpublish counted registrations outside a transaction, allowing a registration to race the status update. | Fixed |
| 6 | Medium | Judging | Undoing a conflict call left every pair it skipped permanently skipped. | Fixed |
| 7 | Medium | Judging | Accepted judges with zero judgments were omitted from reliability/progress output. | Fixed |
| 8 | Medium | Auth | After a lock expired, one new failure immediately re-locked because the old attempt count was reused. | Fixed |
| 9 | Medium | Teams | `GET /teams/{id}` exposed the full roster and pending invites/requests to any visitor. | Fixed |
| 10 | Medium | Staff | Judges could be invited, removed, or accept invites after judging started, creating unassigned/orphaned judging state. | Fixed |
| 11 | Medium | Observability | Staff invite bearer tokens were written to request logs in plain text. | Fixed |
| 12 | Medium | Dashboard | Database errors were swallowed section-by-section, returning a misleading empty dashboard. | Fixed |
| 13 | Low | Teams | Hosted-event query omitted events where the user is an accepted co-organizer. | Fixed |
| 14 | Low | Bookmarks | Draft events could be bookmarked by ID even though drafts are otherwise invisible. | Fixed |
| 15 | Low | Submissions | Arbitrary `status` query values were passed straight to SQL. | Fixed |
| 16 | Low | Auth | A user could explicitly choose a `user_xxxxxx` placeholder, then be blocked from finishing onboarding. | Fixed |
| 17 | Low | Maintenance | Expired OAuth states and stale login-attempt rows were never cleaned up. | Fixed |
| 18 | Low | Teams | Team detail had two identical pending-invite/request branches. | Fixed |
| 19 | Low | Web | Login dropped the intended destination when sending a new user to onboarding. | Fixed |

## Details

### 1. Team leave could destroy a submission (High)

`teams.Leave` deleted the caller's registration and *then* transferred ownership. `submissions.registration_id` uses `ON DELETE CASCADE`, so if the leaver was the submission creator the submission disappeared. Separately, the last member leaving deletes all team registrations.

**Fix:** leaving is still always allowed (P0 requirement). When a team has a submission and the leaver created it, the submission is reassigned to the longest-standing remaining member with a registered registration before the leaver's registration is deleted. If the leaver is the last member, the empty team is dissolved and its registrations are deleted deliberately, matching “the team dissolves when empty.”

**Files:** `internal/teams/service.go`, `db/queries/submissions.sql` (`ReassignSubmission`).

### 2. Split decisions were invisible (High)

`disagreements` grouped outcomes by `judging_pairs.id`. Each judge receives their own pair row for the same canonical submission pair, so a 2–1 split produced three groups of one winner and never appeared in the host review.

**Fix:** group by the canonical `(submission_a, submission_b)` pair. `pair_id` in the response is now a deterministic representative pair ID; consumers should not assume it is shared by all judges. Covered by `TestDisagreementsGroupJudgesAcrossPairRows`.

**Files:** `internal/judging/service.go`, `internal/judging/service_test.go`.

### 3. Open redirect in auth `next` handling (High)

Login/register/onboarding used `next.startsWith("/")`, which accepts protocol-relative (`//host`) and backslash variants.

**Fix:** added `safeNext()` in `lib/utils.ts`, rejecting non-leading-slash values, `//`, backslashes, control characters, and absolute URLs. Applied to login, register, and both onboarding exits. Register/login now also preserve the destination through onboarding.

**Files:** `apps/web/lib/utils.ts`, `apps/web/lib/utils.test.ts`, `app/(auth)/login/page.tsx`, `app/(auth)/register/page.tsx`, `app/onboarding/page.tsx`.

### 4. Malformed config was silently ignored (Medium)

`envInt`/`envBool` returned defaults on parse errors. A typo such as `SESSION_TTL_HOURS=72O` or `COOKIE_SECURE=tru` produced a running service with unexpected settings.

**Fix:** parsing now returns errors and `Load` fails fast. Also validates `PORT` as 1–65535 and `WEB_ORIGIN` as an origin with no path/query/fragment. Covered by `config_test.go`.

**Files:** `internal/config/config.go`, `internal/config/config_test.go`.

### 5. Unpublish race (Medium)

The registration count and status update were separate statements, so a concurrent registration could be inserted after the count but before the update.

**Fix:** `SetStatus` now runs in a transaction that locks the event row (`LockEventForUpdate`), re-checks the transition, counts registrations, updates status, and audits. Registration creation also locks the event row and re-checks the registration window inside its transaction, so the two paths serialize.

**Files:** `internal/events/service.go`, `internal/registrations/service.go`, `db/queries/events.sql`.

### 6. Undo did not restore conflict-skipped pairs (Medium)

`Compare` with `conflict: true` marks the current pair and all pending pairs involving either submission as `skipped`. `Undo` only reset the one pair, so the skipped pairs never returned to the queue.

**Fix:** inside the undo transaction, read the current judgment; if it was a conflict, run `RestoreSkippedPairsForJudge`, which restores skipped pairs involving either submission unless they already have a current judgment.

**Files:** `internal/judging/service.go`, `db/queries/judging.sql`.

### 7. Zero-judgment judges were missing (Medium)

`reliability` iterated only `model.Judges`, which CrowdBT creates from outcomes. An accepted judge who had not judged yet disappeared from the host review, hiding the bottleneck.

**Fix:** union accepted staff rows with model judges. Judges with no outcomes appear with `comparisons: 0` and the documented Beta(3,1) prior (`alpha 3`, `beta 1`, reliability `0.75`). Covered by `TestReliabilityIncludesAcceptedJudgesWithNoJudgments`.

**Files:** `internal/judging/service.go`, `internal/judging/service_test.go`.

### 8. Lockout window reset (Medium)

`recordFailure` used the stored attempt count without checking `first_attempt_at`. After a 15-minute lock expired, the next failure reused the old count and immediately re-locked.

**Fix:** attempts are only carried forward while `first_attempt_at` is inside the 15-minute window. Named constants (`loginWindow`, `maxLoginFailures`, `loginLock`) replace magic numbers.

**Files:** `internal/auth/service.go`.

### 9. Team roster privacy (Medium)

`GET /teams/{id}` is public/optional-auth and returned member names and join timestamps to anyone, plus pending invites and join requests to the owner. That exposed participant identity beyond the event page.

**Fix:** roster and pending lists are only serialized for team members and event managers (`CanManageEvent`). Everyone else gets the public team card (`member_count`, owner username, event metadata). The duplicate pending-flag branches were collapsed.

**Files:** `internal/teams/service.go`, `docs` (`endpoints.md`).

### 10. Staff lifecycle after judging starts (Medium)

Removing a judge after assignment, or accepting a judge invite after judging went live, left pairs pointing at an inactive judge or produced a judge with no work. `RemoveStaff` also scanned both role lists linearly.

**Fix:** added `GetStaffByID`, require judging status `setup` to add/remove/accept a judge, and keep the existing block on results-published events. Co-organizer removal remains possible until judging locks.

**Files:** `internal/events/service.go`, `db/queries/events.sql`.

### 11. Invite tokens in logs (Medium)

`Logging` wrote `r.URL.Path` verbatim. `/api/v1/invites/staff/{token}/accept` therefore logged a bearer credential.

**Fix:** `redactPath` masks the token segment; covered by `TestRedactPathHidesStaffInviteTokens`.

**Files:** `internal/server/middleware.go`, `internal/server/middleware_test.go`.

### 12. Dashboard swallowed errors (Medium)

Each dashboard section used `if err == nil { ... }`, so a database failure looked like an empty dashboard.

**Fix:** each query now returns the error; the handler maps it to the standard envelope.

**Files:** `internal/dashboard/service.go`.

### 13–19. Smaller fixes

- `/host/events` now includes accepted co-organizers, not just owners (`ListEventsByOrganizer`).
- Draft events return 404 from bookmark creation.
- Submission list `status` accepts only `draft` or `submitted`.
- Placeholder usernames (`user_` + 6 hex chars) are rejected for explicit username selection/update, while auto-generated placeholders still work as the onboarding starting point.
- Hourly maintenance now also deletes expired OAuth states and login attempts older than 24 hours, and runs once at startup.
- Team detail pending-flag branches were deduplicated.
- Login `next` is carried through onboarding.

## Verification

Run after the fixes:

```bash
cd apps/api
gofmt -w .
/home/kash/go/bin/sqlc generate
go build ./...
go vet ./...
go test ./...
```

```bash
cd apps/web
export PATH="$HOME/.nvm/versions/node/v24.18.0/bin:$PATH"
npx tsc --noEmit
npm test -- --run
npm run build
```

Go: all packages build/vet clean; tests pass (including new config, judging, and middleware tests).  
Web: TypeScript clean, 22 Vitest tests pass, production build succeeds.

### Runtime smoke test (Docker Compose + Postgres)

The API and web images were rebuilt and exercised against the seeded database through the real HTTP API:

- Team submission hand-off: Alice (owner and submission creator) left a two-person team; the submission was reassigned to Bob with his registration, and Bob became owner. The roster stayed hidden from anonymous visitors and non-members while remaining visible to members, and a pending invitee saw only their invite flag.
- Roster/pending privacy held for a member of the public and for an invitee who was not yet a member.
- Event lifecycle: publish succeeded; unpublish returned `422` once a registration existed; draft deletion still returned `204`; a draft bookmark returned `404` while a published bookmark returned `201`.
- Co-organizer visibility: an accepted co-organizer saw the event in `GET /host/events`.
- Staff guards: judge invite and removal returned `422` once judging was live, and a pending judge invite could not be accepted (`409`).
- Judging review: two judges voted opposite ways on the same canonical pair and the review reported exactly one split decision; accepted judges with no judgments appeared at Beta(3,1) with `comparisons: 0`.
- Conflict undo: a conflict skipped two pairs; undo returned the pair to pending, restored the other skipped pair, and left zero skipped pairs.
- Request logs contained 8 redacted staff-invite paths and no raw tokens.

Smoke-test events and accounts were removed from the database afterwards; the seed event is unchanged.

## Remaining Risks / Not Changed

These are real but were outside this review's fix scope or need a product decision:

1. **No DB-backed integration tests.** Domain services and HTTP handlers still have no automated integration suite. The new behaviours (submission hand-off on leave, conflict undo, event status locking) were exercised in the manual Compose smoke test above but are not guarded by CI. Recommended next step: a small `testcontainers`/Compose-backed suite covering the judging and team flows.
2. **In-process rate limiter and chat hub.** Both are single-instance only. Horizontal scaling needs Redis/pub-sub or a shared limiter.
3. **Session touch on every request.** `TouchSession` writes per authenticated request; debounce (e.g. once per five minutes) if write load becomes a problem.
4. **No submission withdrawal endpoint.** Leaving a team no longer destroys a submission, but there is still no explicit “withdraw submission” route; participants can edit to draft status but cannot delete.
5. **CSP.** The web app deliberately omits a strict CSP because of the inline theme bootstrap script; the API sends a restrictive CSP for its JSON responses.
6. **Split-decision pair ID.** `disagreements[].pair_id` is now a representative ID, not an identifier shared by all judges. The UI only uses it as a React key, but future API consumers should use `title_a`/`title_b` or winner counts.
