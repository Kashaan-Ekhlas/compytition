package judging

import (
	"testing"

	"github.com/google/uuid"

	"compytition/internal/db"
	"compytition/internal/judging/crowdbt"
)

func TestDisagreementsGroupJudgesAcrossPairRows(t *testing.T) {
	// Fixed IDs keep subA canonical (lower UUID string) so the assertion does
	// not depend on random UUID ordering.
	subA := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	subB := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	pairOne, pairTwo := uuid.New(), uuid.New()
	judgeOne, judgeTwo := uuid.New(), uuid.New()

	outcomes := []db.ListPairOutcomesRow{
		{
			PairID: pairOne, SubmissionA: subA, SubmissionB: subB,
			JudgeID: judgeOne, WinnerSubmissionID: &subA,
		},
		{
			PairID: pairTwo, SubmissionA: subA, SubmissionB: subB,
			JudgeID: judgeTwo, WinnerSubmissionID: &subB,
		},
	}
	subs := []Submission{{ID: subA, Title: "Alpha"}, {ID: subB, Title: "Beta"}}

	got := disagreements(outcomes, subs)
	if len(got) != 1 {
		t.Fatalf("expected one split decision, got %d: %+v", len(got), got)
	}
	if got[0].Judgments != 2 {
		t.Fatalf("expected two judgments in the split, got %d", got[0].Judgments)
	}
	if got[0].TitleA != "Alpha" || got[0].TitleB != "Beta" {
		t.Fatalf("unexpected titles: %q vs %q", got[0].TitleA, got[0].TitleB)
	}
	if len(got[0].WinnerCount) != 2 {
		t.Fatalf("expected both submissions to have a vote, got %+v", got[0].WinnerCount)
	}
	if got[0].PairID != pairOne && got[0].PairID != pairTwo {
		t.Fatalf("expected a representative pair id, got %s", got[0].PairID)
	}
}

func TestReliabilityIncludesAcceptedJudgesWithNoJudgments(t *testing.T) {
	subA, subB := uuid.New(), uuid.New()
	active, idle := uuid.New(), uuid.New()
	activeName := "active"
	idleName := "idle"

	model := crowdbt.Recompute([]crowdbt.Outcome{{
		JudgeID: active, WinnerID: subA, LoserID: subB, Sequence: "one",
	}})
	progress := []db.ListJudgeProgressRow{
		{UserID: &active, Username: &activeName, Email: "active@example.com", Total: 3, Done: 1},
		{UserID: &idle, Username: &idleName, Email: "idle@example.com", Total: 3, Done: 0},
	}

	got := reliability(model, progress)
	if len(got) != 2 {
		t.Fatalf("expected both accepted judges, got %d", len(got))
	}
	var found *JudgeReliability
	for i := range got {
		if got[i].ID == idle {
			found = &got[i]
		}
	}
	if found == nil {
		t.Fatal("idle accepted judge was dropped from reliability")
	}
	if found.Comparisons != 0 {
		t.Fatalf("expected zero comparisons, got %d", found.Comparisons)
	}
	if found.Alpha != crowdbt.PriorAlpha || found.Beta != crowdbt.PriorBeta {
		t.Fatalf("expected the Beta(3,1) prior, got alpha=%v beta=%v", found.Alpha, found.Beta)
	}
	if found.Reliability != crowdbt.PriorAlpha/(crowdbt.PriorAlpha+crowdbt.PriorBeta) {
		t.Fatalf("expected prior reliability, got %v", found.Reliability)
	}
	if found.Username != idleName || found.Email != "idle@example.com" {
		t.Fatalf("expected contact details to be preserved, got %+v", found)
	}
}

func TestRankByWinsOrdersByWinsThenLosses(t *testing.T) {
	subA := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	subB := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	subC := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	judge := uuid.New()
	outcomes := []db.ListPairOutcomesRow{
		{PairID: uuid.New(), SubmissionA: subA, SubmissionB: subB, JudgeID: judge, WinnerSubmissionID: &subA},
		{PairID: uuid.New(), SubmissionA: subA, SubmissionB: subC, JudgeID: judge, WinnerSubmissionID: &subA},
		{PairID: uuid.New(), SubmissionA: subB, SubmissionB: subC, JudgeID: judge, WinnerSubmissionID: &subB},
	}
	// Deliberately not in expected order so a missing sort cannot pass.
	subs := []Submission{
		{ID: subC, Title: "Gamma"}, {ID: subA, Title: "Alpha"}, {ID: subB, Title: "Beta"},
	}

	got, minCov, maxCov := rankByWins(subs, outcomes)
	if minCov != 2 || maxCov != 2 {
		t.Fatalf("coverage = %d..%d, want 2..2", minCov, maxCov)
	}
	want := []uuid.UUID{subA, subB, subC}
	for i, id := range want {
		if got[i].SubmissionID != id {
			t.Fatalf("rank %d = %s, want %s", i+1, got[i].SubmissionID, id)
		}
		if got[i].Rank != int32(i+1) {
			t.Fatalf("entry %d rank = %d, want %d", i, got[i].Rank, i+1)
		}
	}
	if got[0].Wins != 2 || got[0].Losses != 0 || got[0].Score != 2 {
		t.Fatalf("winner = %+v, want two wins", got[0])
	}
	if got[0].Confidence != 1 {
		t.Fatalf("winner confidence = %v, want 1", got[0].Confidence)
	}
}

func TestRankByWinsBreaksTiesByLossesAndID(t *testing.T) {
	low := uuid.MustParse("00000000-0000-0000-0000-00000000000a")
	high := uuid.MustParse("00000000-0000-0000-0000-00000000000b")
	other := uuid.MustParse("00000000-0000-0000-0000-00000000000c")
	judge := uuid.New()
	outcomes := []db.ListPairOutcomesRow{
		// Both entries have one win. The low-UUID entry also has one loss.
		{PairID: uuid.New(), SubmissionA: low, SubmissionB: other, JudgeID: judge, WinnerSubmissionID: &low},
		{PairID: uuid.New(), SubmissionA: low, SubmissionB: high, JudgeID: judge, WinnerSubmissionID: &high},
	}
	got, _, _ := rankByWins([]Submission{{ID: low}, {ID: high}, {ID: other}}, outcomes)
	if got[0].SubmissionID != high {
		t.Fatalf("rank 1 = %s, want the entry with fewer losses", got[0].SubmissionID)
	}

	// Equal wins and losses fall through to the UUID tie-break.
	first := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	second := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	tied := []db.ListPairOutcomesRow{
		{PairID: uuid.New(), SubmissionA: first, SubmissionB: other, JudgeID: judge, WinnerSubmissionID: &first},
		{PairID: uuid.New(), SubmissionA: second, SubmissionB: other, JudgeID: judge, WinnerSubmissionID: &second},
	}
	got, _, _ = rankByWins([]Submission{{ID: second}, {ID: first}, {ID: other}}, tied)
	if got[0].SubmissionID != first || got[1].SubmissionID != second {
		t.Fatalf("tie-break order = %s, %s, want UUID ascending", got[0].SubmissionID, got[1].SubmissionID)
	}
}

func TestRankByWinsHandlesSkippedAndEmptyInput(t *testing.T) {
	subA := uuid.New()
	skipped := []db.ListPairOutcomesRow{{
		PairID: uuid.New(), SubmissionA: subA, SubmissionB: uuid.New(),
		JudgeID: uuid.New(), WinnerSubmissionID: nil,
	}}
	got, minCov, maxCov := rankByWins([]Submission{{ID: subA}}, skipped)
	if len(got) != 1 || got[0].Coverage != 0 || got[0].Score != 0 {
		t.Fatalf("skipped outcome = %+v, want no coverage or score", got)
	}
	if minCov != 0 || maxCov != 0 {
		t.Fatalf("coverage = %d..%d, want 0..0", minCov, maxCov)
	}
	if got, minCov, maxCov := rankByWins(nil, nil); len(got) != 0 || minCov != 0 || maxCov != 0 {
		t.Fatalf("empty input = %d entries, coverage %d..%d, want 0 and 0..0", len(got), minCov, maxCov)
	}
}

func TestRankByWinsPlacesUnjudgedEntriesLast(t *testing.T) {
	subA, subB, subC := uuid.New(), uuid.New(), uuid.New()
	outcomes := []db.ListPairOutcomesRow{{
		PairID: uuid.New(), SubmissionA: subA, SubmissionB: subB,
		JudgeID: uuid.New(), WinnerSubmissionID: &subA,
	}}
	// The unjudged entry comes first so a missing partition cannot pass.
	subs := []Submission{
		{ID: subC, Title: "Gamma"}, {ID: subA, Title: "Alpha"}, {ID: subB, Title: "Beta"},
	}

	got, _, _ := rankByWins(subs, outcomes)
	if len(got) != 3 {
		t.Fatalf("entries = %d, want 3", len(got))
	}
	if got[2].SubmissionID != subC || got[2].Coverage != 0 || got[2].Score != 0 {
		t.Fatalf("unjudged entry = %+v, want Gamma last with zero score", got[2])
	}
}

func TestSimpleRankingRequiresExactlyOneJudge(t *testing.T) {
	first := uuid.New()
	second := uuid.New()
	oneJudge := []db.ListJudgeProgressRow{{UserID: &first}}
	if !simpleRanking(oneJudge, nil) {
		t.Fatal("one judge with no outcomes should use simple ranking")
	}
	if !simpleRanking(oneJudge, []db.ListPairOutcomesRow{{JudgeID: first}}) {
		t.Fatal("one judge should use simple ranking when every outcome is theirs")
	}
	if simpleRanking(oneJudge, []db.ListPairOutcomesRow{{JudgeID: second}}) {
		t.Fatal("outcomes from a removed judge should force CrowdBT")
	}
	if simpleRanking([]db.ListJudgeProgressRow{{UserID: &first}, {UserID: &second}}, nil) {
		t.Fatal("two judges should use CrowdBT")
	}
	if simpleRanking(nil, nil) {
		t.Fatal("no judges should not report a single-judge ranking")
	}
}

func TestReviewWarnsWhenSingleJudgeUsesWins(t *testing.T) {
	low := JudgeReliability{ID: uuid.New(), Username: "judge_low", Reliability: 0.4, Comparisons: 8}
	wins := &Review{RankingMode: "wins", Judges: []JudgeReliability{low}}
	got := reviewWarnings(wins, []Submission{{ID: uuid.New()}}, 1)
	if !containsWarning(got, "Single judge: entries are ranked by head-to-head wins, so CrowdBT reliability is not used.") {
		t.Fatalf("expected a single-judge warning, got %v", got)
	}
	if containsWarning(got, "Low reliability: judge_low.") {
		t.Fatalf("wins mode should not emit a reliability warning, got %v", got)
	}

	crowdbt := &Review{RankingMode: "crowdbt", Judges: []JudgeReliability{low}}
	got = reviewWarnings(crowdbt, []Submission{{ID: uuid.New()}}, 2)
	if !containsWarning(got, "Low reliability: judge_low.") {
		t.Fatalf("expected a low-reliability warning, got %v", got)
	}
	if containsWarning(got, "Single judge: entries are ranked by head-to-head wins, so CrowdBT reliability is not used.") {
		t.Fatalf("crowdbt mode should not emit a single-judge warning, got %v", got)
	}
}

func containsWarning(warnings []string, want string) bool {
	for _, warning := range warnings {
		if warning == want {
			return true
		}
	}
	return false
}

func TestReviewWarnsWhenLargeEventCoverageIsSparse(t *testing.T) {
	subs := make([]Submission, 300)
	for i := range subs {
		subs[i] = Submission{ID: uuid.New()}
	}
	review := &Review{CoverageMin: 8, CoverageMax: 8}
	got := reviewWarnings(review, subs, 3)
	found := false
	for _, warning := range got {
		if warning == "Each entry is compared about 8 times. With 300 entries, add judges so the top of the ranking is more reliable." {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a sparse-coverage warning, got %v", got)
	}
}
