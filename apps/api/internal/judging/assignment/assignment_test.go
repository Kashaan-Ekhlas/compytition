package assignment

import (
	"testing"

	"github.com/google/uuid"
)

func subs(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.New()
	}
	return out
}

func judges(ids []uuid.UUID, conflicts map[uuid.UUID]map[uuid.UUID]bool) []Judge {
	out := make([]Judge, len(ids))
	for i, id := range ids {
		out[i] = Judge{ID: id, Conflicts: conflicts[id]}
	}
	return out
}

func checkInvariants(t *testing.T, plan *Plan, subs []uuid.UUID, js []Judge, budget int) {
	t.Helper()
	if !plan.Connected {
		t.Fatal("plan is not connected")
	}
	if plan.TotalPairs < len(subs)-1 {
		t.Fatalf("too few pairs: %d", plan.TotalPairs)
	}
	perJudge := map[uuid.UUID]map[string]bool{}
	for _, p := range plan.Pairs {
		if p.A == p.B {
			t.Fatal("self pair")
		}
		if p.Left != p.A && p.Left != p.B {
			t.Fatal("left must be one of the pair")
		}
		if perJudge[p.JudgeID] == nil {
			perJudge[p.JudgeID] = map[string]bool{}
		}
		key := p.A.String() + "|" + p.B.String()
		rev := p.B.String() + "|" + p.A.String()
		if perJudge[p.JudgeID][key] || perJudge[p.JudgeID][rev] {
			t.Fatalf("duplicate pair for judge: %s", key)
		}
		perJudge[p.JudgeID][key] = true
	}
	for _, j := range js {
		if plan.JudgeStats[0].Comparisons < 0 {
			t.Fatal("negative comparisons")
		}
		if len(perJudge[j.ID]) > budget {
			t.Fatalf("judge %s has %d pairs, budget %d", j.ID, len(perJudge[j.ID]), budget)
		}
		for _, p := range plan.Pairs {
			if p.JudgeID != j.ID {
				continue
			}
			if j.Conflicts[p.A] || j.Conflicts[p.B] {
				t.Fatal("conflicted submission assigned")
			}
		}
	}
	if plan.CoverageMin < 1 {
		t.Fatalf("coverage min = %d", plan.CoverageMin)
	}
}

func TestGenerateBalancedConnectedPlan(t *testing.T) {
	s := subs(20)
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	js := judges(ids, nil)
	plan, err := Generate(Input{
		Submissions: s, Judges: js, Seed: "event-1",
		Config: Config{TargetComparisons: 5, JudgeBudget: 14, AnchorCount: 3},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	checkInvariants(t, plan, s, js, 14)
	if plan.CoverageMin < 4 {
		t.Fatalf("coverage min = %d, want >= 4", plan.CoverageMin)
	}
	for _, o := range plan.Overlap {
		if o.Shared < 2 {
			t.Fatalf("judge overlap %d below 2", o.Shared)
		}
	}
	loads := make([]int, len(plan.JudgeStats))
	for i, st := range plan.JudgeStats {
		loads[i] = st.Comparisons
	}
	min, max := loads[0], loads[0]
	for _, l := range loads {
		if l < min {
			min = l
		}
		if l > max {
			max = l
		}
	}
	if max-min > 6 {
		t.Fatalf("judge workload imbalance too high: min=%d max=%d", min, max)
	}
}

func TestGenerateWithConflicts(t *testing.T) {
	s := subs(12)
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	conflicts := map[uuid.UUID]map[uuid.UUID]bool{}
	conflicts[ids[0]] = map[uuid.UUID]bool{s[0]: true, s[1]: true}
	conflicts[ids[1]] = map[uuid.UUID]bool{s[2]: true}
	js := judges(ids, conflicts)
	plan, err := Generate(Input{
		Submissions: s, Judges: js, Seed: "conflict",
		Config: Config{TargetComparisons: 4, JudgeBudget: 12, AnchorCount: 3},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	checkInvariants(t, plan, s, js, 12)
}

func TestSingleJudge(t *testing.T) {
	s := subs(8)
	js := judges([]uuid.UUID{uuid.New()}, nil)
	plan, err := Generate(Input{
		Submissions: s, Judges: js, Seed: "solo",
		Config: Config{TargetComparisons: 3, JudgeBudget: 20, AnchorCount: 3},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	checkInvariants(t, plan, s, js, 20)
}

func TestTwoSubmissionsManyJudges(t *testing.T) {
	s := subs(2)
	js := judges([]uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}, nil)
	plan, err := Generate(Input{
		Submissions: s, Judges: js, Seed: "small",
		Config: Config{TargetComparisons: 10, JudgeBudget: 2, AnchorCount: 3},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	checkInvariants(t, plan, s, js, 2)
}

func TestImpossibleWhenEveryJudgeConflicted(t *testing.T) {
	s := subs(3)
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	conflicts := map[uuid.UUID]map[uuid.UUID]bool{}
	for _, id := range ids {
		conflicts[id] = map[uuid.UUID]bool{s[0]: true}
	}
	_, err := Generate(Input{
		Submissions: s, Judges: judges(ids, conflicts), Seed: "impossible",
		Config: Config{TargetComparisons: 3, JudgeBudget: 5, AnchorCount: 2},
	})
	if err == nil {
		t.Fatal("expected impossible assignment error")
	}
}

func TestDeterministic(t *testing.T) {
	s := subs(10)
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	in := Input{
		Submissions: s, Judges: judges(ids, nil), Seed: "same",
		Config: Config{TargetComparisons: 4, JudgeBudget: 10, AnchorCount: 3},
	}
	a, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Pairs) != len(b.Pairs) {
		t.Fatalf("non-deterministic pair count")
	}
	for i := range a.Pairs {
		if a.Pairs[i] != b.Pairs[i] {
			t.Fatalf("non-deterministic pair at %d", i)
		}
	}
}
