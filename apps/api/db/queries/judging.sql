-- name: DeleteJudgingPairsForEvent :exec
DELETE FROM judging_pairs WHERE event_id = $1;

-- name: CreateJudgingPair :one
INSERT INTO judging_pairs (
    event_id, judge_id, submission_a, submission_b, left_submission, position
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetJudgingPair :one
SELECT * FROM judging_pairs WHERE id = $1;

-- name: GetJudgingPairForJudge :one
SELECT * FROM judging_pairs WHERE id = $1 AND judge_id = $2;

-- name: UpdatePairStatus :exec
UPDATE judging_pairs SET status = $2 WHERE id = $1;

-- name: CountPairsForJudge :one
SELECT count(*)::int AS total,
       count(*) FILTER (WHERE status <> 'pending')::int AS done
FROM judging_pairs
WHERE event_id = $1 AND judge_id = $2;

-- name: ListPendingPairsForJudge :many
SELECT p.id AS pair_id, p.event_id, p.submission_a, p.submission_b,
       p.left_submission, p.position,
       a.title AS a_title, a.description AS a_description,
       a.repo_url AS a_repo_url, a.demo_url AS a_demo_url,
       a.file_url AS a_file_url, a.team_id AS a_team_id,
       ta.name AS a_team_name, ua.username AS a_creator,
       b.title AS b_title, b.description AS b_description,
       b.repo_url AS b_repo_url, b.demo_url AS b_demo_url,
       b.file_url AS b_file_url, b.team_id AS b_team_id,
       tb.name AS b_team_name, ub.username AS b_creator
FROM judging_pairs p
JOIN submissions a ON a.id = p.submission_a
JOIN submissions b ON b.id = p.submission_b
JOIN users ua ON ua.id = a.created_by
JOIN users ub ON ub.id = b.created_by
LEFT JOIN teams ta ON ta.id = a.team_id
LEFT JOIN teams tb ON tb.id = b.team_id
WHERE p.event_id = $1 AND p.judge_id = $2 AND p.status = 'pending'
ORDER BY p.position ASC, p.created_at ASC
LIMIT $3 OFFSET $4;

-- name: ListPairsForEvent :many
SELECT p.id, p.judge_id, p.submission_a, p.submission_b, p.left_submission,
       p.status, p.created_at
FROM judging_pairs p
WHERE p.event_id = $1
ORDER BY p.judge_id, p.position ASC
LIMIT $2 OFFSET $3;

-- name: SupersedeJudgment :exec
UPDATE judgments SET is_current = FALSE
WHERE pair_id = $1 AND is_current = TRUE;

-- name: CreateJudgment :one
INSERT INTO judgments (
    event_id, pair_id, judge_id, winner_submission_id, skipped, conflict, is_current
) VALUES ($1, $2, $3, $4, $5, $6, TRUE)
RETURNING *;

-- name: GetCurrentJudgmentForPair :one
SELECT * FROM judgments WHERE pair_id = $1 AND is_current = TRUE;

-- name: ListCurrentJudgmentsForEvent :many
SELECT j.id, j.pair_id, j.judge_id, j.winner_submission_id, j.skipped,
       j.conflict, j.created_at,
       p.submission_a, p.submission_b
FROM judgments j
JOIN judging_pairs p ON p.id = j.pair_id
WHERE j.event_id = $1 AND j.is_current = TRUE
ORDER BY j.created_at ASC;

-- name: ListJudgmentsForEvent :many
SELECT j.id, j.pair_id, j.judge_id, j.winner_submission_id, j.skipped,
       j.conflict, j.is_current, j.created_at,
       u.username AS judge_username,
       p.submission_a, p.submission_b,
       a.title AS a_title, b.title AS b_title
FROM judgments j
JOIN judging_pairs p ON p.id = j.pair_id
JOIN users u ON u.id = j.judge_id
JOIN submissions a ON a.id = p.submission_a
JOIN submissions b ON b.id = p.submission_b
WHERE j.event_id = $1 AND j.is_current = TRUE
ORDER BY j.created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListAcceptedJudgeIDs :many
SELECT user_id FROM event_staff
WHERE event_id = $1 AND role = 'judge' AND status = 'accepted' AND user_id IS NOT NULL
ORDER BY created_at ASC;

-- name: ListJudgeProgress :many
SELECT s.user_id, s.email, u.username, p.full_name,
       (SELECT count(*)::int FROM judging_pairs jp
        WHERE jp.event_id = s.event_id AND jp.judge_id = s.user_id) AS total,
       (SELECT count(*)::int FROM judging_pairs jp
        WHERE jp.event_id = s.event_id AND jp.judge_id = s.user_id
          AND jp.status <> 'pending') AS done
FROM event_staff s
LEFT JOIN users u ON u.id = s.user_id
LEFT JOIN profiles p ON p.user_id = s.user_id
WHERE s.event_id = $1 AND s.role = 'judge' AND s.status = 'accepted'
ORDER BY s.created_at ASC;

-- name: CreateReliabilitySnapshot :exec
INSERT INTO judge_reliability_snapshots (
    event_id, judge_id, alpha, beta, reliability, comparisons
) VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListReliabilitySnapshots :many
SELECT r.id, r.event_id, r.judge_id, r.alpha, r.beta, r.reliability,
       r.comparisons, r.created_at, u.username
FROM judge_reliability_snapshots r
JOIN users u ON u.id = r.judge_id
WHERE r.event_id = $1
ORDER BY r.created_at DESC, r.reliability DESC;

-- name: DeleteResultsForEvent :exec
DELETE FROM event_results WHERE event_id = $1;

-- name: UpsertEventResult :exec
INSERT INTO event_results (event_id, submission_id, rank, score, confidence, published_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (event_id, submission_id) DO UPDATE
SET rank = EXCLUDED.rank, score = EXCLUDED.score,
    confidence = EXCLUDED.confidence, published_at = now();

-- name: ListEventResults :many
SELECT r.event_id, r.submission_id, r.rank, r.score, r.confidence, r.published_at,
       s.title AS submission_title, s.team_id, s.created_by,
       t.name AS team_name, u.username AS creator_username
FROM event_results r
JOIN submissions s ON s.id = r.submission_id
LEFT JOIN teams t ON t.id = s.team_id
JOIN users u ON u.id = s.created_by
WHERE r.event_id = $1
ORDER BY r.rank ASC;

-- name: CountEventJudgments :one
SELECT count(*)::int AS count
FROM judgments WHERE event_id = $1 AND is_current = TRUE AND skipped = FALSE;

-- name: CountEventPendingPairs :one
SELECT count(*)::int AS count
FROM judging_pairs WHERE event_id = $1 AND status = 'pending';

-- name: ListJudgeSubmissionConflicts :many
SELECT DISTINCT s.id AS submission_id, tm.user_id AS judge_id
FROM submissions s
JOIN team_members tm ON tm.team_id = s.team_id
WHERE s.event_id = $1
UNION
SELECT s.id, s.created_by
FROM submissions s
WHERE s.event_id = $1;

-- name: SkipPendingPairsForJudge :exec
UPDATE judging_pairs SET status = 'skipped'
WHERE event_id = $1 AND judge_id = $2 AND status = 'pending'
  AND (submission_a = sqlc.arg('submission_a') OR submission_b = sqlc.arg('submission_a')
       OR submission_a = sqlc.arg('submission_b') OR submission_b = sqlc.arg('submission_b'));

-- name: ClearCurrentJudgment :exec
UPDATE judgments SET is_current = FALSE WHERE pair_id = $1 AND is_current = TRUE;

-- name: ResetPairStatus :exec
UPDATE judging_pairs SET status = $2 WHERE id = $1;

-- name: ListPairOutcomes :many
SELECT p.id AS pair_id, p.submission_a, p.submission_b, p.judge_id,
       j.winner_submission_id
FROM judging_pairs p
JOIN judgments j ON j.pair_id = p.id AND j.is_current = TRUE
WHERE p.event_id = $1
  AND j.winner_submission_id IS NOT NULL
  AND j.skipped = FALSE AND j.conflict = FALSE
ORDER BY p.id;

-- name: CountPairsForEvent :one
SELECT count(*)::int AS total,
       count(*) FILTER (WHERE status <> 'pending')::int AS done
FROM judging_pairs WHERE event_id = $1;

-- name: LastJudgedPairForJudge :one
SELECT p.id
FROM judging_pairs p
JOIN judgments j ON j.pair_id = p.id AND j.is_current = TRUE
WHERE p.event_id = $1 AND p.judge_id = $2
ORDER BY j.created_at DESC
LIMIT 1;
