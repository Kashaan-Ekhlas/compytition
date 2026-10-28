# Code Review - Compytition

**Date:** 2026-10-29 (second pass; first pass was 2026-10-27)  
**Scope:** `apps/api` (submission answers, judging payloads, event filters, staff/team invites) and `apps/web` (single-theme UI, auth/onboarding, discovery, forms, notifications, event cards).  
**Method:** manual line-by-line audit against `rules.md` plus browser smoke tests through Docker Compose; CodeRabbit CLI is still signed out (`coderabbit auth status` → `signed out`), so manual review remains the fallback agreed with the user.

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
| 20 | High | Submissions | Custom submission answers were stored but never validated server-side, so unknown or malformed answers could bypass the form schema. | Fixed |
| 21 | Medium | Organizer / Judge | Submission answers were omitted from organizer lists and judging payloads, so extra questions were invisible after registration. | Fixed |
| 22 | Medium | Event discovery | The status filter sent `published` / `results_published` while the API accepts `upcoming` / `open` / `closed` / `ended`, so several status filters returned a validation error. | Fixed |
| 23 | Medium | Web forms | The submit page posted registration answer keys alongside submission answers, and merged events did not prefill from registration. | Fixed |
| 24 | Low | Event UI | Event detail and organizer overview still displayed a minimum team size ("teams of 1-6") after the product moved to max-only team sizes. | Fixed |
| 25 | Low | Notifications | The 5-second poll set loading state and briefly replaced the open list with "Loading…" on every tick. | Fixed |
| 26 | Low | Tests | `TestDisagreementsGroupJudgesAcrossPairRows` used random UUIDs, so it failed about half the time when the canonical order flipped. | Fixed |
| 27 | Medium | Web auth | The Google buttons always redirected to `/dashboard` or `/onboarding` and dropped `?next=`, so staff/team invite links never returned to the invite for Google signups. | Fixed |
| 28 | High | Seed / CI | `go run ./cmd/seed` inserted submissions without `answers`, violating the new NOT NULL column, and the seed event's future deadline now blocked derived judging. CI failed at the seed step. | Fixed |

## Details

### 1. Team leave could destroy a submission (High)

`teams.Leave` deleted the caller's registration and *then* transferred ownership. `submissions.registration_id` uses `ON DELETE CASCADE`, so if the leaver was the submission creator the submission disappeared. Separately, the last member leaving deletes all team registrations.

**Fix:** leaving is still always allowed (P0 requirement). When a team has a submission and the leaver created it, the submission is reassigned to the longest-standing remaining member with a registered registration before the leaver's registration is deleted. If the leaver is the last member, the empty team is dissolved and its registrations are deleted deliberately, matching "the team dissolves when empty."

**Files:** `internal/teams/service.go`, `db/queries/submissions.sql` (`ReassignSubmission`).

### 2. Split decisions were invisible (High)

`disagreements` grouped outcomes by `judging_pairs.id`. Each judge receives their own pair row for the same canonical submission pair, so a 2-1 split produced three groups of one winner and never appeared in the host review.

**Fix:** group by the canonical `(submission_a, submission_b)` pair. `pair_id` in the response is now a deterministic representative pair ID; consumers should not assume it is shared by all judges. Covered by `TestDisagreementsGroupJudgesAcrossPairRows`.

**Files:** `internal/judging/service.go`, `internal/judging/service_test.go`.

### 3. Open redirect in auth `next` handling (High)

Login/register/onboarding used `next.startsWith("/")`, which accepts protocol-relative (`//host`) and backslash variants.

**Fix:** added `safeNext()` in `lib/utils.ts`, rejecting non-leading-slash values, `//`, backslashes, control characters, and absolute URLs. Applied to login, register, and both onboarding exits. Register/login now also preserve the destination through onboarding.

**Files:** `apps/web/lib/utils.ts`, `apps/web/lib/utils.test.ts`, `app/(auth)/login/page.tsx`, `app/(auth)/register/page.tsx`, `app/onboarding/page.tsx`.

### 4. Malformed config was silently ignored (Medium)

`envInt`/`envBool` returned defaults on parse errors. A typo such as `SESSION_TTL_HOURS=72O` or `COOKIE_SECURE=tru` produced a running service with unexpected settings.

**Fix:** parsing now returns errors and `Load` fails fast. Also validates `PORT` as 1-65535 and `WEB_ORIGIN` as an origin with no path/query/fragment. Covered by `config_test.go`.

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

### 13-19. Smaller fixes

- `/host/events` now includes accepted co-organizers, not just owners (`ListEventsByOrganizer`).
- Draft events return 404 from bookmark creation.
- Submission list `status` accepts only `draft` or `submitted`.
- Placeholder usernames (`user_` + 6 hex chars) are rejected for explicit username selection/update, while auto-generated placeholders still work as the onboarding starting point.
- Hourly maintenance now also deletes expired OAuth states and login attempts older than 24 hours, and runs once at startup.
- Team detail pending-flag branches were deduplicated.
- Login `next` is carried through onboarding.

### 20. Submission answers bypassed validation (High)

The new `submissions.answers` column was written straight from the request body. A participant could post arbitrary keys, skip required submission questions, or store values that did not match the field type.

**Fix:** `submissions.Service.validateAnswers` loads the event's `submission` form kind and rejects unknown keys, missing required values, oversized text, invalid URLs, non-numeric numbers, and options outside the declared list. Create and update both call it before writing. Covered by the same pattern used for registration answers.

**Files:** `internal/submissions/service.go`, `db/queries/submissions.sql`.

### 21. Organizer and judge payloads hid custom answers (Medium)

`ListSubmissionsForEvent`, `ListSubmissionsForJudging` and `ListMySubmissions` did not select `answers`. Organizers saw only title/description/links, and judges could not see the extra questions the event asked.

**Fix:** the three queries now select `answers`; the submission views unmarshal them, `judging.Submission` carries `Answers`, and a shared `SubmissionAnswers` component renders labelled values on the organizer submissions tab and both judge panels.

**Files:** `db/queries/submissions.sql`, `internal/submissions/service.go`, `internal/judging/service.go`, `components/forms/submission-answers.tsx`, `components/host/tab-submissions.tsx`, `app/judge/events/[id]/page.tsx`.

### 22. Discovery status filter used the wrong vocabulary (Medium)

The UI offered `published` and `results_published`, but `events.List` validates `upcoming`, `open`, `closed`, `ended`. Selecting those options returned 422 instead of filtering.

**Fix:** the status options now use the API vocabulary (`Upcoming`, `Registration open`, `Registration closed`, `Ended`) and `endpoints.md` documents it.

**Files:** `app/events/page.tsx`, `endpoints.md`.

### 23. Submit page mixed registration and submission answers (Medium)

The submit page sent every answer from the merged registration form, including registration-only keys, and did not prefill merged submission answers when the submission was first created. The API correctly rejected the unknown keys.

**Fix:** loading builds a set of submission field IDs, prefills from registration answers filtered to that set, then overlays the saved submission answers. On submit the same filter is applied, so only submission keys leave the browser.

**Files:** `app/events/[id]/submit/page.tsx`.

### 24. Team size copy still showed a minimum (Low)

The backend now fixes `min_team_size` at 1, but event detail and the organizer overview rendered `teams of 1-N`.

**Fix:** both screens say `Solo or teams up to N` or `Teams up to N`.

**Files:** `app/events/[id]/page.tsx`, `components/host/tab-overview.tsx`.

### 25. Notification polling flickered the open menu (Low)

Every 5-second poll set `loading = true`, and the list rendered the loading line instead of existing items until the response arrived.

**Fix:** the loading state only occupies the panel when the list is still empty. Existing notifications stay visible during background refreshes.

**Files:** `components/layout/notifications-menu.tsx`.

### 26. Flaky split-decision test (Low)

The test created `subA` and `subB` with `uuid.New()`. `canonicalA` orders by UUID string, so the assertion `TitleA == "Alpha"` failed whenever `subB` sorted first.

**Fix:** fixed UUIDs keep `subA` canonical.

**Files:** `internal/judging/service_test.go`.

### 27. Google OAuth dropped the invite destination (Medium)

The password flows carried `next` through login, registration, and onboarding, but both Google buttons hard-coded `redirect=/dashboard` or `/onboarding`. A new user arriving from a staff or team invite and choosing Google landed on onboarding without a return path, so the invite was never auto-accepted.

**Fix:** both buttons now pass the current safe `next` into the OAuth `redirect` parameter. Registration wraps the destination in `/onboarding?next=...`, matching the password signup path; login uses `next` when present and falls back to `/dashboard`.

**Files:** `app/(auth)/login/page.tsx`, `app/(auth)/register/page.tsx`.

### 28. CI seed failed on the new submission column and deadline gate (High)

The CI workflow runs `go run ./cmd/seed` against a fresh database. The seed called `CreateSubmission` without `answers`, so the generated insert sent `NULL` to the new `NOT NULL` column and failed with SQLSTATE 23502. With that fixed, the seed still failed at `judging.Assign` because the demo event's submission deadline was 20 days in the future and derived judging is now blocked until the deadline passes.

**Fix:** the seed passes `{}` for `answers`, and the demo event is now a recently finished event (registration closed 10 days ago, submissions closed 3 days ago) so pre-judging and publishing match its published results.

**Files:** `cmd/seed/main.go`.

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

Go: all packages build/vet clean; tests pass (including new config, judging, and middleware tests). The flaky split-decision test is now deterministic.  
Web: TypeScript clean, 22 Vitest tests pass, production build succeeds.

Second pass (2026-10-29) also exercised the new flows in the browser: seven-step onboarding with curated college/course suggestions and sequential country/state/city, correct own-username availability, password checklist and eye toggles, red login toast with X, remote-first event creation and location search, merged registration with custom fields, submission answers saved while merged and shown to the organizer/judge payloads, staff and team invite auto-accept for existing and new accounts, and the bell notification trigger. Submission answer validation and the corrected discovery status filter were covered after the fixes.

### Runtime smoke test (Docker Compose + Postgres)

The API and web images were rebuilt and exercised against the seeded database through the real HTTP API and the browser:

- Team submission hand-off: Alice (owner and submission creator) left a two-person team; the submission was reassigned to Bob with his registration, and Bob became owner. The roster stayed hidden from anonymous visitors and non-members while remaining visible to members, and a pending invitee saw only their invite flag.
- Roster/pending privacy held for a member of the public and for an invitee who was not yet a member.
- Event lifecycle: publish succeeded; unpublish returned `422` once a registration existed; draft deletion still returned `204`; a draft bookmark returned `404` while a published bookmark returned `201`.
- Co-organizer visibility: an accepted co-organizer saw the event in `GET /host/events`.
- Staff guards: judge invite and removal returned `422` once judging was live, and a pending judge invite could not be accepted (`409`).
- Judging review: two judges voted opposite ways on the same canonical pair and the review reported exactly one split decision; accepted judges with no judgments appeared at Beta(3,1) with `comparisons: 0`.
- Conflict undo: a conflict skipped two pairs; undo returned the pair to pending, restored the other skipped pair, and left zero skipped pairs.
- Request logs contained 8 redacted staff-invite paths and no raw tokens.
- Second pass: onboarding saved country/state/city, own usernames reported available, a merged event accepted a custom registration field, the same field appeared on the submission form, and the submission payload returned it. Submission-answer validation rejected unknown keys. Staff and team invite links auto-accepted for both existing and newly registered accounts.

Smoke-test events and accounts were removed from the database afterwards; the seed event is unchanged.

### 300-entry stress test (2026-10-29)

The full competitor path was also exercised at scale through the HTTP API, not just the UI:

- Created a published remote event, 310 accounts, and 5 teams of three using the team invite link (existing accounts auto-joined).
- Registered 310 participants (5 team registrations plus 295 solo), then created 300 submissions: 295 solo and 5 team entries.
- Added the three seeded judges by email, moved the deadline into the past, assigned 1,200 pairs (8 comparisons per entry), started judging, and recorded all 1,200 comparisons. Judge accuracy was simulated at 0.92, 0.78 and 0.58 against a known latent quality order.
- Review showed 1,200 done, 0 pending, 5 split decisions, and a sparse-coverage warning.
- CrowdBT ranking quality: Spearman 0.68, Kendall tau 0.48 against the known order. The winning entry was not the latent best. Reliability put the noisiest judge last (0.733 vs 0.748 and 0.756) but did not clearly separate the two better judges, and confidence was about 50% for most entries.
- Publishing wrote 300 result rows and the public results page rendered them.

The model itself is sound (isolated simulation with the same counts recovers the order and separates reliability), but 8 comparisons per entry is too sparse to resolve the top of a 300-entry field. The review now warns organizers about this and recommends adding judges. The test event remains at `/events/f07f577f-2ee2-4330-9db0-ce5e5f0772a8` for inspection.

## Remaining Risks / Not Changed

These are real but were outside this review's fix scope or need a product decision:

1. **No DB-backed integration tests.** Domain services and HTTP handlers still have no automated integration suite. The new behaviours (submission answer validation, merged forms, invite auto-accept, conflict undo, event status locking) were exercised manually but are not guarded by CI. Recommended next step: a small `testcontainers`/Compose-backed suite covering judging, team, and submission flows.
2. **In-process rate limiter and chat hub.** Both are single-instance only. Horizontal scaling needs Redis/pub-sub or a shared limiter.
3. **Session touch on every request.** `TouchSession` writes per authenticated request; debounce (e.g. once per five minutes) if write load becomes a problem.
4. **No submission withdrawal endpoint.** Leaving a team no longer destroys a submission, but there is still no explicit "withdraw submission" route; participants can edit to draft status but cannot delete.
5. **CSP.** The inline theme bootstrap script is gone now that there is one theme, so a strict CSP for the web app is feasible but not yet configured; the API sends a restrictive CSP for its JSON responses.
6. **Bookmark status per card.** Each event card calls `GET /events/{id}/bookmark` when signed in, so a page of 12 cards makes 12 small requests. A `bookmarked` flag in the event list payload would remove that N+1.
7. **Notification polling.** Five-second polling is simple and works across instances, but it keeps making requests on idle tabs. A shared SSE/WebSocket channel or a longer interval after inactivity would scale better.
8. **Split-decision pair ID.** `disagreements[].pair_id` is a representative ID, not an identifier shared by all judges. The UI only uses it as a React key, but future API consumers should use `title_a`/`title_b` or winner counts.
9. **Stored judging config GET.** `GET /events/{id}/judging-config` still returns the stored legacy JSON, while preview/assign derive the effective values from counts and the deadline. The web app does not read it; either derive it there too or remove the route.
10. **Sparse coverage on large events.** For events over 20 entries the derived plan targets 8 comparisons per entry, which is enough for a coarse global order but not for a reliable top ten. The 300-entry stress test scored Kendall tau 0.48 and a ~50% winner confidence. Scale coverage with the entry count or recommend more judges before publishing.
