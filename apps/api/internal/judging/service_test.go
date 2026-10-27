package judging

import (
	"testing"

	"github.com/google/uuid"

	"compytition/internal/db"
	"compytition/internal/judging/crowdbt"
)

func TestDisagreementsGroupJudgesAcrossPairRows(t *testing.T) {
	subA, subB := uuid.New(), uuid.New()
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
