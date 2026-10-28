// Package judging turns submissions plus pairwise judgments into a ranking.
//
// The flow is:
//
//	preview  → deterministic assignment plan, nothing persisted
//	assign   → persist judging_pairs
//	start    → judging_status=live
//	compare  → append-only judgments (edits supersede the previous row)
//	review   → full CrowdBT replay with reliability, coverage, disagreements
//	publish  → event_results snapshot, reliability snapshots, results status
package judging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"compytition/internal/apperr"
	"compytition/internal/config"
	"compytition/internal/db"
	"compytition/internal/events"
	"compytition/internal/judging/assignment"
	"compytition/internal/judging/crowdbt"
)

type Service struct {
	store *db.Store
	cfg   *config.Config
	log   *slog.Logger
}

func NewService(store *db.Store, cfg *config.Config, log *slog.Logger) *Service {
	return &Service{store: store, cfg: cfg, log: log}
}

type Submission struct {
	ID              uuid.UUID      `json:"id"`
	Title           string         `json:"title"`
	Description     string         `json:"description"`
	RepoURL         string         `json:"repo_url"`
	DemoURL         string         `json:"demo_url"`
	FileURL         string         `json:"file_url"`
	Answers         map[string]any `json:"answers"`
	TeamID          *uuid.UUID     `json:"team_id"`
	TeamName        string         `json:"team_name,omitempty"`
	CreatorUsername string         `json:"creator_username"`
	CreatorName     string         `json:"creator_name,omitempty"`
	CreatedBy       uuid.UUID      `json:"created_by"`
}

type JudgeSummary struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Load     int       `json:"load,omitempty"`
}

type Preview struct {
	Config      events.JudgingConfig `json:"config"`
	Submissions []Submission         `json:"submissions"`
	Judges      []JudgeSummary       `json:"judges"`
	Plan        *assignment.Plan     `json:"plan"`
	Warnings    []string             `json:"warnings"`
}

type AssignResult struct {
	TotalPairs int                      `json:"total_pairs"`
	Coverage   map[string]int           `json:"coverage"`
	Overlap    []assignment.OverlapStat `json:"overlap"`
	Anchors    []uuid.UUID              `json:"anchors"`
}

type PairView struct {
	ID       uuid.UUID  `json:"id"`
	Position int16      `json:"position"`
	Status   string     `json:"status"`
	LeftID   uuid.UUID  `json:"left_submission"`
	A        Submission `json:"a"`
	B        Submission `json:"b"`
}

type NextPair struct {
	Pair     *PairView          `json:"pair"`
	Progress JudgeProgress      `json:"progress"`
	Criteria []events.Criterion `json:"criteria"`
}

type JudgeProgress struct {
	Total      int32      `json:"total"`
	Done       int32      `json:"done"`
	LastPairID *uuid.UUID `json:"last_pair_id"`
}

type RankingEntry struct {
	SubmissionID    uuid.UUID `json:"submission_id"`
	Title           string    `json:"title"`
	TeamName        string    `json:"team_name,omitempty"`
	CreatorUsername string    `json:"creator_username"`
	Score           float64   `json:"score"`
	Variance        float64   `json:"variance"`
	Confidence      float64   `json:"confidence"`
	Rank            int32     `json:"rank"`
	Coverage        int       `json:"coverage"`
	Wins            int       `json:"wins"`
	Losses          int       `json:"losses"`
}

type JudgeReliability struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username,omitempty"`
	Email       string    `json:"email,omitempty"`
	Reliability float64   `json:"reliability"`
	Alpha       float64   `json:"alpha"`
	Beta        float64   `json:"beta"`
	Comparisons int       `json:"comparisons"`
	Total       int32     `json:"assigned"`
	Done        int32     `json:"done"`
}

type Disagreement struct {
	PairID      uuid.UUID      `json:"pair_id"`
	TitleA      string         `json:"title_a"`
	TitleB      string         `json:"title_b"`
	Judgments   int            `json:"judgments"`
	WinnerCount map[string]int `json:"winner_counts"`
}

type Review struct {
	JudgingStatus  string             `json:"judging_status"`
	TotalPairs     int32              `json:"total_pairs"`
	DonePairs      int32              `json:"done_pairs"`
	PendingPairs   int32              `json:"pending_pairs"`
	TotalJudgments int                `json:"total_judgments"`
	CoverageMin    int                `json:"coverage_min"`
	CoverageMax    int                `json:"coverage_max"`
	Criteria       []events.Criterion `json:"criteria"`
	Rankings       []RankingEntry     `json:"rankings"`
	Judges         []JudgeReliability `json:"judges"`
	Disagreements  []Disagreement     `json:"disagreements"`
	Warnings       []string           `json:"warnings"`
}

type ResultEntry struct {
	Rank            int32     `json:"rank"`
	SubmissionID    uuid.UUID `json:"submission_id"`
	Title           string    `json:"title"`
	TeamName        string    `json:"team_name,omitempty"`
	CreatorUsername string    `json:"creator_username"`
	Score           float64   `json:"score"`
	Confidence      float64   `json:"confidence"`
}

// ── Preview / assignment ────────────────────────────────────────────────────

func (s *Service) Preview(ctx context.Context, userID, eventID uuid.UUID) (*Preview, error) {
	ev, err := s.requireManage(ctx, userID, eventID)
	if err != nil {
		return nil, err
	}
	if ev.JudgingStatus == "locked" || ev.JudgingStatus == "published" {
		return nil, apperr.Invalid("judging", "Judging is already finalized for this event.")
	}
	subs, err := s.loadSubmissions(ctx, eventID)
	if err != nil {
		return nil, err
	}
	judges, err := s.loadJudges(ctx, eventID)
	if err != nil {
		return nil, err
	}
	cfg, _, err := s.config(ev)
	if err != nil {
		return nil, err
	}
	cfg = deriveJudgingConfig(len(subs), len(judges))

	plan, err := s.buildPlan(ctx, eventID, ev, subs, judges, cfg)
	if err != nil {
		return nil, err
	}
	warnings := planWarnings(plan, len(subs), len(judges))
	return &Preview{Config: cfg, Submissions: subs, Judges: judges, Plan: plan, Warnings: warnings}, nil
}

func (s *Service) Assign(ctx context.Context, userID, eventID uuid.UUID) (*AssignResult, error) {
	ev, err := s.requireManage(ctx, userID, eventID)
	if err != nil {
		return nil, err
	}
	if ev.JudgingStatus != "setup" {
		return nil, apperr.Invalid("judging", "Comparisons can only be regenerated before judging starts.")
	}
	if err := judgingDeadlineOpen(ev); err != nil {
		return nil, err
	}
	subs, err := s.loadSubmissions(ctx, eventID)
	if err != nil {
		return nil, err
	}
	judges, err := s.loadJudges(ctx, eventID)
	if err != nil {
		return nil, err
	}
	cfg, _, err := s.config(ev)
	if err != nil {
		return nil, err
	}
	cfg = deriveJudgingConfig(len(subs), len(judges))
	plan, err := s.buildPlan(ctx, eventID, ev, subs, judges, cfg)
	if err != nil {
		return nil, err
	}
	positions := map[uuid.UUID]int16{}
	err = s.store.InTx(ctx, func(q *db.Queries) error {
		if err := q.DeleteJudgingPairsForEvent(ctx, eventID); err != nil {
			return err
		}
		for _, p := range plan.Pairs {
			positions[p.JudgeID]++
			if _, err := q.CreateJudgingPair(ctx, db.CreateJudgingPairParams{
				EventID: eventID, JudgeID: p.JudgeID,
				SubmissionA: canonicalA(p.A, p.B), SubmissionB: canonicalB(p.A, p.B),
				LeftSubmission: p.Left, Position: positions[p.JudgeID],
			}); err != nil {
				return err
			}
		}
		meta, _ := json.Marshal(map[string]any{
			"pairs": len(plan.Pairs), "submissions": len(subs), "judges": len(judges),
		})
		return q.CreateAuditEntry(ctx, db.CreateAuditEntryParams{
			ActorID: &userID, EventID: &eventID, Action: "judging.assign", Metadata: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	coverage := map[string]int{}
	for id, n := range plan.Coverage {
		coverage[id.String()] = n
	}
	return &AssignResult{
		TotalPairs: len(plan.Pairs), Coverage: coverage,
		Overlap: plan.Overlap, Anchors: plan.Anchors,
	}, nil
}

func (s *Service) Start(ctx context.Context, userID, eventID uuid.UUID) error {
	ev, err := s.requireManage(ctx, userID, eventID)
	if err != nil {
		return err
	}
	if ev.JudgingStatus != "setup" {
		return apperr.Invalid("judging", "Judging has already started.")
	}
	if err := judgingDeadlineOpen(ev); err != nil {
		return err
	}
	counts, err := s.store.CountPairsForEvent(ctx, eventID)
	if err != nil {
		return err
	}
	if counts.Total == 0 {
		return apperr.Invalid("judging", "Generate comparisons before starting judging.")
	}
	return s.store.UpdateEventJudgingStatus(ctx, db.UpdateEventJudgingStatusParams{
		ID: eventID, JudgingStatus: "live",
	})
}

func (s *Service) buildPlan(ctx context.Context, eventID uuid.UUID, ev db.GetEventByIDRow, subs []Submission, judges []JudgeSummary, cfg events.JudgingConfig) (*assignment.Plan, error) {
	if len(subs) < 2 {
		return nil, apperr.Invalid("submissions", "At least two submitted entries are required.")
	}
	if len(judges) == 0 {
		return nil, apperr.Invalid("judges", "Invite at least one judge who has accepted before assigning comparisons.")
	}
	conflicts, err := s.store.ListJudgeSubmissionConflicts(ctx, eventID)
	if err != nil {
		return nil, err
	}
	conflictMap := map[uuid.UUID]map[uuid.UUID]bool{}
	for _, c := range conflicts {
		if conflictMap[c.JudgeID] == nil {
			conflictMap[c.JudgeID] = map[uuid.UUID]bool{}
		}
		conflictMap[c.JudgeID][c.SubmissionID] = true
	}
	subIDs := make([]uuid.UUID, len(subs))
	for i, sub := range subs {
		subIDs[i] = sub.ID
	}
	judgeInputs := make([]assignment.Judge, len(judges))
	for i, j := range judges {
		judgeInputs[i] = assignment.Judge{ID: j.ID, Conflicts: conflictMap[j.ID]}
	}
	plan, err := assignment.Generate(assignment.Input{
		Submissions: subIDs, Judges: judgeInputs, Seed: eventID.String(),
		Config: assignment.Config{
			TargetComparisons: cfg.TargetComparisons,
			JudgeBudget:       cfg.JudgeBudget,
			AnchorCount:       cfg.AnchorCount,
		},
	})
	if err != nil {
		if errors.Is(err, assignment.ErrImpossible) {
			return nil, apperr.Invalid("judging", "Comparisons could not be generated: "+err.Error())
		}
		return nil, err
	}
	loads := map[uuid.UUID]int{}
	for _, st := range plan.JudgeStats {
		loads[st.JudgeID] = st.Comparisons
	}
	for i := range judges {
		judges[i].Load = loads[judges[i].ID]
	}
	return plan, nil
}

func planWarnings(plan *assignment.Plan, subCount, judgeCount int) []string {
	warnings := make([]string, 0)
	if plan.CoverageMin < plan.TargetCoverage {
		warnings = append(warnings, fmt.Sprintf(
			"Some entries are compared %d times instead of the target %d. Increase the judge budget or add judges.",
			plan.CoverageMin, plan.TargetCoverage))
	}
	if judgeCount == 1 {
		warnings = append(warnings, "Only one judge is assigned, so reliability cannot be cross-checked.")
	}
	if subCount < 3 {
		warnings = append(warnings, "With fewer than three entries the ranking is mostly a single comparison.")
	}
	for _, o := range plan.Overlap {
		if o.Shared == 0 {
			warnings = append(warnings, "Two judges share no submissions, so their reliability cannot be compared.")
			break
		}
	}
	return warnings
}

// ── Judge flow ──────────────────────────────────────────────────────────────

func (s *Service) Next(ctx context.Context, userID, eventID uuid.UUID) (*NextPair, error) {
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if err := s.requireJudge(ctx, eventID, userID); err != nil {
		return nil, err
	}
	if ev.JudgingStatus != "live" {
		return nil, apperr.Invalid("judging", "Judging is not live for this event.")
	}
	progress, err := s.progress(ctx, eventID, userID)
	if err != nil {
		return nil, err
	}
	out := &NextPair{Progress: progress, Criteria: parseCriteria(ev.Criteria)}
	pairs, err := s.store.ListPendingPairsForJudge(ctx, db.ListPendingPairsForJudgeParams{
		EventID: eventID, JudgeID: userID, Limit: 1, Offset: 0,
	})
	if err != nil {
		return nil, err
	}
	if len(pairs) == 0 {
		return out, nil
	}
	p := pairs[0]
	a := Submission{
		ID: p.SubmissionA, Title: p.ATitle, Description: p.ADescription,
		RepoURL: p.ARepoUrl, DemoURL: p.ADemoUrl, FileURL: p.AFileUrl,
		TeamID: p.ATeamID, TeamName: deref(p.ATeamName), CreatorUsername: p.ACreator,
	}
	b := Submission{
		ID: p.SubmissionB, Title: p.BTitle, Description: p.BDescription,
		RepoURL: p.BRepoUrl, DemoURL: p.BDemoUrl, FileURL: p.BFileUrl,
		TeamID: p.BTeamID, TeamName: deref(p.BTeamName), CreatorUsername: p.BCreator,
	}
	out.Pair = &PairView{
		ID: p.PairID, Position: p.Position, Status: "pending",
		LeftID: p.LeftSubmission, A: a, B: b,
	}
	return out, nil
}

// Compare records the judge's decision. `winner` is "a" or "b" relative to the
// canonical pair ordering. Setting conflict skips this pair and every remaining
// pair involving either submission for this judge.
func (s *Service) Compare(ctx context.Context, userID, eventID, pairID uuid.UUID, winner string, skipped, conflict bool) error {
	pair, err := s.store.GetJudgingPairForJudge(ctx, db.GetJudgingPairForJudgeParams{ID: pairID, JudgeID: userID})
	if err != nil {
		return apperr.ErrNotFound
	}
	if pair.EventID != eventID {
		return apperr.ErrNotFound
	}
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return apperr.ErrNotFound
	}
	if ev.JudgingStatus != "live" {
		return apperr.Invalid("judging", "Judging is not live for this event.")
	}
	if !conflict && !skipped && winner != "a" && winner != "b" {
		return apperr.Invalid("winner", "Choose one of the two submissions.")
	}

	var winnerID *uuid.UUID
	status := "judged"
	switch {
	case conflict:
		status = "skipped"
		skipped = true
	case skipped:
		status = "skipped"
	default:
		id := pair.SubmissionA
		if winner == "b" {
			id = pair.SubmissionB
		}
		winnerID = &id
	}

	return s.store.InTx(ctx, func(q *db.Queries) error {
		if err := q.SupersedeJudgment(ctx, pair.ID); err != nil {
			return err
		}
		if _, err := q.CreateJudgment(ctx, db.CreateJudgmentParams{
			EventID: eventID, PairID: pair.ID, JudgeID: userID,
			WinnerSubmissionID: winnerID, Skipped: skipped, Conflict: conflict,
		}); err != nil {
			return err
		}
		if err := q.UpdatePairStatus(ctx, db.UpdatePairStatusParams{ID: pair.ID, Status: status}); err != nil {
			return err
		}
		if conflict {
			return q.SkipPendingPairsForJudge(ctx, db.SkipPendingPairsForJudgeParams{
				EventID: eventID, JudgeID: userID,
				SubmissionA: pair.SubmissionA, SubmissionB: pair.SubmissionB,
			})
		}
		return nil
	})
}

func (s *Service) Undo(ctx context.Context, userID, eventID, pairID uuid.UUID) error {
	pair, err := s.store.GetJudgingPairForJudge(ctx, db.GetJudgingPairForJudgeParams{ID: pairID, JudgeID: userID})
	if err != nil {
		return apperr.ErrNotFound
	}
	if pair.EventID != eventID {
		return apperr.ErrNotFound
	}
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return apperr.ErrNotFound
	}
	if ev.JudgingStatus != "live" {
		return apperr.Invalid("judging", "Judging is not live for this event.")
	}
	return s.store.InTx(ctx, func(q *db.Queries) error {
		judgment, err := q.GetCurrentJudgmentForPair(ctx, pair.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err := q.ClearCurrentJudgment(ctx, pair.ID); err != nil {
			return err
		}
		if err := q.ResetPairStatus(ctx, db.ResetPairStatusParams{ID: pair.ID, Status: "pending"}); err != nil {
			return err
		}
		// Undoing a conflict call must bring back the pairs that conflict
		// skipped, otherwise they stay hidden from the judge forever.
		if err == nil && judgment.Conflict {
			return q.RestoreSkippedPairsForJudge(ctx, db.RestoreSkippedPairsForJudgeParams{
				EventID: eventID, JudgeID: userID,
				SubmissionA: pair.SubmissionA, SubmissionB: pair.SubmissionB,
			})
		}
		return nil
	})
}

func (s *Service) Progress(ctx context.Context, userID, eventID uuid.UUID) (JudgeProgress, error) {
	if err := s.requireJudge(ctx, eventID, userID); err != nil {
		return JudgeProgress{}, err
	}
	return s.progress(ctx, eventID, userID)
}

func (s *Service) progress(ctx context.Context, eventID, userID uuid.UUID) (JudgeProgress, error) {
	row, err := s.store.CountPairsForJudge(ctx, db.CountPairsForJudgeParams{EventID: eventID, JudgeID: userID})
	if err != nil {
		return JudgeProgress{}, err
	}
	out := JudgeProgress{Total: row.Total, Done: row.Done}
	last, err := s.store.LastJudgedPairForJudge(ctx, db.LastJudgedPairForJudgeParams{
		EventID: eventID, JudgeID: userID,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return JudgeProgress{}, err
	}
	if err == nil {
		out.LastPairID = &last
	}
	return out, nil
}

// ── Review / publish / results ──────────────────────────────────────────────

func (s *Service) Review(ctx context.Context, userID, eventID uuid.UUID) (*Review, error) {
	ev, err := s.requireManage(ctx, userID, eventID)
	if err != nil {
		return nil, err
	}
	subs, err := s.loadSubmissions(ctx, eventID)
	if err != nil {
		return nil, err
	}
	outcomes, err := s.store.ListPairOutcomes(ctx, eventID)
	if err != nil {
		return nil, err
	}
	model := s.recompute(outcomes)
	counts, err := s.store.CountPairsForEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	judgeRows, err := s.store.ListJudgeProgress(ctx, eventID)
	if err != nil {
		return nil, err
	}
	review := &Review{
		JudgingStatus: ev.JudgingStatus, TotalPairs: counts.Total, DonePairs: counts.Done,
		PendingPairs: counts.Total - counts.Done, TotalJudgments: len(outcomes),
		Criteria:      parseCriteria(ev.Criteria),
		Disagreements: disagreements(outcomes, subs),
	}
	review.Rankings, review.CoverageMin, review.CoverageMax = rankEntries(model, subs, outcomes)
	review.Judges = reliability(model, judgeRows)
	review.Warnings = reviewWarnings(review, subs, len(judgeRows))
	return review, nil
}

type publishInput struct {
	Force bool `json:"force"`
}

func (s *Service) Publish(ctx context.Context, userID, eventID uuid.UUID, force bool) ([]ResultEntry, error) {
	ev, err := s.requireManage(ctx, userID, eventID)
	if err != nil {
		return nil, err
	}
	if ev.JudgingStatus != "live" {
		return nil, apperr.Invalid("judging", "Start judging before publishing results.")
	}
	if ev.Status != "published" && ev.Status != "closed" {
		return nil, apperr.Invalid("event", "Publish the event before publishing results.")
	}
	counts, err := s.store.CountPairsForEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if counts.Total == 0 {
		return nil, apperr.Invalid("judging", "No comparisons have been assigned.")
	}
	if counts.Done < counts.Total && !force {
		return nil, apperr.Invalid("judging",
			fmt.Sprintf("%d of %d comparisons are still pending. Finish them or publish anyway.", counts.Total-counts.Done, counts.Total))
	}
	subs, err := s.loadSubmissions(ctx, eventID)
	if err != nil {
		return nil, err
	}
	outcomes, err := s.store.ListPairOutcomes(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if len(outcomes) == 0 {
		return nil, apperr.Invalid("judging", "There are no completed comparisons to publish.")
	}
	model := s.recompute(outcomes)
	rankings, _, _ := rankEntries(model, subs, outcomes)
	entries := make([]ResultEntry, 0, len(rankings))
	for _, r := range rankings {
		entries = append(entries, ResultEntry{
			Rank: r.Rank, SubmissionID: r.SubmissionID, Title: r.Title,
			TeamName: r.TeamName, CreatorUsername: r.CreatorUsername,
			Score: r.Score, Confidence: r.Confidence,
		})
	}
	judgeRows, err := s.store.ListJudgeProgress(ctx, eventID)
	if err != nil {
		return nil, err
	}
	reliabilities := reliability(model, judgeRows)

	err = s.store.InTx(ctx, func(q *db.Queries) error {
		if err := q.DeleteResultsForEvent(ctx, eventID); err != nil {
			return err
		}
		for _, e := range entries {
			if err := q.UpsertEventResult(ctx, db.UpsertEventResultParams{
				EventID: eventID, SubmissionID: e.SubmissionID, Rank: e.Rank,
				Score: e.Score, Confidence: e.Confidence,
			}); err != nil {
				return err
			}
		}
		for _, j := range reliabilities {
			if j.ID == uuid.Nil {
				continue
			}
			if err := q.CreateReliabilitySnapshot(ctx, db.CreateReliabilitySnapshotParams{
				EventID: eventID, JudgeID: j.ID, Alpha: j.Alpha, Beta: j.Beta,
				Reliability: j.Reliability, Comparisons: int32(j.Comparisons),
			}); err != nil {
				return err
			}
		}
		if err := q.UpdateEventJudgingStatus(ctx, db.UpdateEventJudgingStatusParams{
			ID: eventID, JudgingStatus: "published",
		}); err != nil {
			return err
		}
		if err := q.UpdateEventStatus(ctx, db.UpdateEventStatusParams{ID: eventID, Status: "results_published"}); err != nil {
			return err
		}
		meta, _ := json.Marshal(map[string]any{
			"results": len(entries), "comparisons": len(outcomes), "forced": force,
		})
		return q.CreateAuditEntry(ctx, db.CreateAuditEntryParams{
			ActorID: &userID, EventID: &eventID, Action: "judging.publish", Metadata: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *Service) Results(ctx context.Context, userID *uuid.UUID, eventID uuid.UUID) ([]ResultEntry, error) {
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if ev.Status != "results_published" {
		if userID == nil {
			return nil, apperr.ErrNotFound
		}
		allowed, err := s.store.CanManageEvent(ctx, db.CanManageEventParams{OrganizerID: *userID, ID: eventID})
		if err != nil || !allowed {
			return nil, apperr.ErrNotFound
		}
	}
	rows, err := s.store.ListEventResults(ctx, eventID)
	if err != nil {
		return nil, err
	}
	out := make([]ResultEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, ResultEntry{
			Rank: r.Rank, SubmissionID: r.SubmissionID, Title: r.SubmissionTitle,
			TeamName: deref(r.TeamName), CreatorUsername: r.CreatorUsername,
			Score: r.Score, Confidence: r.Confidence,
		})
	}
	return out, nil
}

// ── Internals ───────────────────────────────────────────────────────────────

func (s *Service) requireManage(ctx context.Context, userID, eventID uuid.UUID) (db.GetEventByIDRow, error) {
	ev, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return db.GetEventByIDRow{}, apperr.ErrNotFound
	}
	allowed, err := s.store.CanManageEvent(ctx, db.CanManageEventParams{OrganizerID: userID, ID: eventID})
	if err != nil {
		return db.GetEventByIDRow{}, err
	}
	if !allowed {
		return db.GetEventByIDRow{}, apperr.ErrNotFound
	}
	return ev, nil
}

func (s *Service) requireJudge(ctx context.Context, eventID, userID uuid.UUID) error {
	allowed, err := s.store.IsJudgeForEvent(ctx, db.IsJudgeForEventParams{EventID: eventID, UserID: &userID})
	if err != nil {
		return err
	}
	if !allowed {
		return apperr.ErrNotFound
	}
	return nil
}

func (s *Service) loadSubmissions(ctx context.Context, eventID uuid.UUID) ([]Submission, error) {
	rows, err := s.store.ListSubmissionsForJudging(ctx, eventID)
	if err != nil {
		return nil, err
	}
	out := make([]Submission, 0, len(rows))
	for _, r := range rows {
		out = append(out, Submission{
			ID: r.ID, Title: r.Title, Description: r.Description,
			RepoURL: r.RepoUrl, DemoURL: r.DemoUrl, FileURL: r.FileUrl,
			Answers: unmarshalAnswers(r.Answers),
			TeamID:  r.TeamID, TeamName: deref(r.TeamName),
			CreatorUsername: r.CreatorUsername, CreatorName: deref(r.CreatorName),
			CreatedBy: r.CreatedBy,
		})
	}
	return out, nil
}

func (s *Service) loadJudges(ctx context.Context, eventID uuid.UUID) ([]JudgeSummary, error) {
	ids, err := s.store.ListAcceptedJudgeIDs(ctx, eventID)
	if err != nil {
		return nil, err
	}
	out := make([]JudgeSummary, 0, len(ids))
	for _, id := range ids {
		if id == nil {
			continue
		}
		summary := JudgeSummary{ID: *id}
		if u, err := s.store.GetUserByID(ctx, *id); err == nil {
			summary.Username = u.Username
			summary.Email = u.Email
		}
		out = append(out, summary)
	}
	return out, nil
}

func deriveJudgingConfig(subCount, judgeCount int) events.JudgingConfig {
	if subCount < 2 {
		subCount = 2
	}
	if judgeCount < 1 {
		judgeCount = 1
	}
	target := 6
	switch {
	case subCount <= 3:
		target = 3
	case subCount <= 8:
		target = 5
	case subCount <= 20:
		target = 6
	default:
		target = 8
	}
	budget := (target*subCount + judgeCount - 1) / judgeCount
	if budget < 2 {
		budget = 2
	}
	if budget > 500 {
		budget = 500
	}
	anchors := target / 2
	if anchors < 1 {
		anchors = 1
	}
	if anchors > subCount {
		anchors = subCount
	}
	return events.JudgingConfig{
		TargetComparisons: target, JudgeBudget: budget, AnchorCount: anchors,
	}
}

func judgingDeadlineOpen(ev db.GetEventByIDRow) error {
	if ev.SubmissionDeadline == nil {
		return apperr.Invalid("submission_deadline", "Set a submission deadline before judging.")
	}
	if time.Now().Before(*ev.SubmissionDeadline) {
		return apperr.Invalid("judging", "Judging opens after the submission deadline.")
	}
	return nil
}

func (s *Service) config(ev db.GetEventByIDRow) (events.JudgingConfig, []events.Criterion, error) {
	cfg := events.DefaultJudgingConfig
	if len(ev.JudgingConfig) > 0 {
		if err := json.Unmarshal(ev.JudgingConfig, &cfg); err != nil {
			return cfg, nil, err
		}
	}
	if cfg.TargetComparisons <= 0 {
		cfg.TargetComparisons = events.DefaultJudgingConfig.TargetComparisons
	}
	if cfg.JudgeBudget <= 0 {
		cfg.JudgeBudget = events.DefaultJudgingConfig.JudgeBudget
	}
	if cfg.AnchorCount <= 0 {
		cfg.AnchorCount = events.DefaultJudgingConfig.AnchorCount
	}
	return cfg, parseCriteria(ev.Criteria), nil
}

func (s *Service) recompute(outcomes []db.ListPairOutcomesRow) *crowdbt.Result {
	items := make([]crowdbt.Outcome, 0, len(outcomes))
	for _, o := range outcomes {
		if o.WinnerSubmissionID == nil {
			continue
		}
		winner := *o.WinnerSubmissionID
		loser := o.SubmissionA
		if winner == o.SubmissionA {
			loser = o.SubmissionB
		}
		items = append(items, crowdbt.Outcome{
			JudgeID: o.JudgeID, WinnerID: winner, LoserID: loser,
			Sequence: o.PairID.String(),
		})
	}
	return crowdbt.Recompute(items)
}

func rankEntries(model *crowdbt.Result, subs []Submission, outcomes []db.ListPairOutcomesRow) ([]RankingEntry, int, int) {
	byID := map[uuid.UUID]Submission{}
	for _, sub := range subs {
		byID[sub.ID] = sub
	}
	coverage := map[uuid.UUID]int{}
	wins := map[uuid.UUID]int{}
	losses := map[uuid.UUID]int{}
	for _, o := range outcomes {
		if o.WinnerSubmissionID == nil {
			continue
		}
		coverage[o.SubmissionA]++
		coverage[o.SubmissionB]++
		wins[*o.WinnerSubmissionID]++
		loser := o.SubmissionA
		if *o.WinnerSubmissionID == o.SubmissionA {
			loser = o.SubmissionB
		}
		losses[loser]++
	}
	entries := make([]RankingEntry, 0, len(subs))
	seen := map[uuid.UUID]bool{}
	for _, it := range model.Items {
		sub, ok := byID[it.ID]
		if !ok {
			continue
		}
		seen[it.ID] = true
		entries = append(entries, RankingEntry{
			SubmissionID: it.ID, Title: sub.Title, TeamName: sub.TeamName,
			CreatorUsername: sub.CreatorUsername, Score: it.Mu, Variance: it.Variance,
			Confidence: it.Confidence, Rank: int32(it.Rank),
			Coverage: coverage[it.ID], Wins: wins[it.ID], Losses: losses[it.ID],
		})
	}
	// Unjudged submissions still appear, below everything that was compared.
	rest := make([]Submission, 0)
	for _, sub := range subs {
		if !seen[sub.ID] {
			rest = append(rest, sub)
		}
	}
	sort.Slice(rest, func(i, j int) bool { return rest[i].ID.String() < rest[j].ID.String() })
	for _, sub := range rest {
		entries = append(entries, RankingEntry{
			SubmissionID: sub.ID, Title: sub.Title, TeamName: sub.TeamName,
			CreatorUsername: sub.CreatorUsername, Rank: int32(len(entries) + 1),
		})
	}
	minCov, maxCov := 0, 0
	if len(subs) > 0 {
		minCov = 1 << 30
	}
	for _, sub := range subs {
		c := coverage[sub.ID]
		if c < minCov {
			minCov = c
		}
		if c > maxCov {
			maxCov = c
		}
	}
	if len(subs) == 0 {
		minCov = 0
	}
	return entries, minCov, maxCov
}

func reliability(model *crowdbt.Result, progress []db.ListJudgeProgressRow) []JudgeReliability {
	known := map[uuid.UUID]crowdbt.JudgeResult{}
	for _, j := range model.Judges {
		known[j.ID] = j
	}
	out := make([]JudgeReliability, 0, len(progress)+len(model.Judges))
	seen := map[uuid.UUID]bool{}
	add := func(id uuid.UUID) JudgeReliability {
		entry := JudgeReliability{ID: id}
		if j, ok := known[id]; ok {
			entry.Reliability = j.Reliability
			entry.Alpha = j.Alpha
			entry.Beta = j.Beta
			entry.Comparisons = j.Comparisons
		} else {
			// Accepted judges who have not judged yet still appear, at the
			// Beta(3,1) prior, so the host can see who is holding things up.
			entry.Alpha = crowdbt.PriorAlpha
			entry.Beta = crowdbt.PriorBeta
			entry.Reliability = crowdbt.PriorAlpha / (crowdbt.PriorAlpha + crowdbt.PriorBeta)
		}
		return entry
	}
	for _, row := range progress {
		if row.UserID == nil || seen[*row.UserID] {
			continue
		}
		entry := add(*row.UserID)
		entry.Username = deref(row.Username)
		entry.Email = row.Email
		entry.Total = row.Total
		entry.Done = row.Done
		out = append(out, entry)
		seen[*row.UserID] = true
	}
	// A judge removed mid-event can still have judgments in the log; keep them
	// visible so the review matches the recomputed model.
	for _, j := range model.Judges {
		if seen[j.ID] {
			continue
		}
		out = append(out, add(j.ID))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Reliability != out[j].Reliability {
			return out[i].Reliability > out[j].Reliability
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	return out
}

func disagreements(outcomes []db.ListPairOutcomesRow, subs []Submission) []Disagreement {
	titles := map[uuid.UUID]string{}
	for _, s := range subs {
		titles[s.ID] = s.Title
	}
	type group struct {
		a       uuid.UUID
		b       uuid.UUID
		pairID  uuid.UUID
		winners map[uuid.UUID]int
	}
	// Judges are assigned the same canonical pair in separate rows, so split
	// decisions must be grouped by the submission pair, not by pair id.
	groups := map[string]*group{}
	for _, o := range outcomes {
		if o.WinnerSubmissionID == nil {
			continue
		}
		a, b := canonicalA(o.SubmissionA, o.SubmissionB), canonicalB(o.SubmissionA, o.SubmissionB)
		key := a.String() + "|" + b.String()
		g := groups[key]
		if g == nil {
			g = &group{a: a, b: b, pairID: o.PairID, winners: map[uuid.UUID]int{}}
			groups[key] = g
		} else if o.PairID.String() < g.pairID.String() {
			g.pairID = o.PairID
		}
		g.winners[*o.WinnerSubmissionID]++
	}
	out := make([]Disagreement, 0)
	for _, g := range groups {
		if len(g.winners) < 2 {
			continue
		}
		counts := map[string]int{}
		total := 0
		for winner, n := range g.winners {
			counts[winner.String()] = n
			total += n
		}
		out = append(out, Disagreement{
			PairID: g.pairID, TitleA: titles[g.a], TitleB: titles[g.b],
			Judgments: total, WinnerCount: counts,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PairID.String() < out[j].PairID.String() })
	return out
}

func reviewWarnings(r *Review, subs []Submission, judgeCount int) []string {
	warnings := make([]string, 0)
	if r.PendingPairs > 0 {
		warnings = append(warnings, fmt.Sprintf("%d comparisons are still pending.", r.PendingPairs))
	}
	if r.CoverageMin < 2 && len(subs) > 2 {
		warnings = append(warnings, "Some entries have fewer than two comparisons; their ranking is unstable.")
	}
	if len(subs) >= 50 && r.CoverageMax < 12 {
		warnings = append(warnings, fmt.Sprintf(
			"Each entry is compared about %d times. With %d entries, add judges so the top of the ranking is more reliable.",
			r.CoverageMax, len(subs)))
	}
	low := []string{}
	for _, j := range r.Judges {
		if j.Comparisons >= 5 && j.Reliability < 0.6 {
			low = append(low, usernameOrEmail(j))
		}
	}
	if len(low) > 0 {
		warnings = append(warnings, "Low reliability: "+strings.Join(low, ", ")+".")
	}
	if judgeCount == 0 {
		warnings = append(warnings, "No accepted judges are recorded for this event.")
	}
	if len(r.Disagreements) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d pairs have split decisions.", len(r.Disagreements)))
	}
	return warnings
}

func usernameOrEmail(j JudgeReliability) string {
	if j.Username != "" {
		return j.Username
	}
	if j.Email != "" {
		return j.Email
	}
	return j.ID.String()
}

func parseCriteria(raw []byte) []events.Criterion {
	var out []events.Criterion
	if len(raw) == 0 {
		return []events.Criterion{}
	}
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return []events.Criterion{}
	}
	return out
}

func canonicalA(a, b uuid.UUID) uuid.UUID {
	if a.String() < b.String() {
		return a
	}
	return b
}

func canonicalB(a, b uuid.UUID) uuid.UUID {
	if a.String() < b.String() {
		return b
	}
	return a
}

func unmarshalAnswers(raw []byte) map[string]any {
	out := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
