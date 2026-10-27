package crowdbt

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/google/uuid"
)

func TestUpdateMovesWinnerUpAndLoserDown(t *testing.T) {
	m := New()
	winner, loser, judge := uuid.New(), uuid.New(), uuid.New()
	m.Update(judge, winner, loser)
	res := m.Result()
	if len(res.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(res.Items))
	}
	if res.Items[0].ID != winner {
		t.Fatalf("winner should rank first, got %s", res.Items[0].ID)
	}
	if res.Items[0].Mu <= res.Items[1].Mu {
		t.Fatalf("winner mean %f should exceed loser mean %f", res.Items[0].Mu, res.Items[1].Mu)
	}
	if res.Judges[0].Comparisons != 1 {
		t.Fatalf("judge comparisons = %d, want 1", res.Judges[0].Comparisons)
	}
}

func TestRecomputeIsDeterministicAndOrderIndependent(t *testing.T) {
	n := 12
	items := make([]uuid.UUID, n)
	for i := range items {
		items[i] = uuid.New()
	}
	rng := rand.New(rand.NewSource(42))
	judges := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	var outcomes []Outcome
	for i := 0; i < 120; i++ {
		a, b := rng.Intn(n), rng.Intn(n)
		if a == b {
			continue
		}
		winner, loser := items[a], items[b]
		if a < b {
			winner, loser = items[b], items[a]
		}
		outcomes = append(outcomes, Outcome{
			JudgeID: judges[i%len(judges)], WinnerID: winner, LoserID: loser,
			Sequence: timeSeq(i),
		})
	}
	first := Recompute(outcomes)
	shuffled := append([]Outcome(nil), outcomes...)
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	second := Recompute(shuffled)
	if len(first.Items) != len(second.Items) {
		t.Fatalf("item count differs")
	}
	for i := range first.Items {
		if first.Items[i].ID != second.Items[i].ID {
			t.Fatalf("ranking changed at %d: %s vs %s", i, first.Items[i].ID, second.Items[i].ID)
		}
		if first.Items[i].Mu != second.Items[i].Mu {
			t.Fatalf("score changed at %d: %f vs %f", i, first.Items[i].Mu, second.Items[i].Mu)
		}
	}
}

func TestSimulationRecoversGroundTruthAndDownweightsNoise(t *testing.T) {
	const (
		nItems      = 25
		nGood       = 3
		nNoisy      = 2
		comparisons = 150
	)
	items := make([]uuid.UUID, nItems)
	strength := map[uuid.UUID]float64{}
	for i := range items {
		items[i] = uuid.New()
		strength[items[i]] = float64(i)
	}
	type judgeSpec struct {
		id    uuid.UUID
		skill float64
	}
	judges := []judgeSpec{
		{uuid.New(), 0.95}, {uuid.New(), 0.90}, {uuid.New(), 0.85},
		{uuid.New(), 0.58}, {uuid.New(), 0.55},
	}
	rng := rand.New(rand.NewSource(7))
	var outcomes []Outcome
	seq := 0
	for _, j := range judges {
		for c := 0; c < comparisons; c++ {
			a, b := rng.Intn(nItems), rng.Intn(nItems)
			if a == b {
				continue
			}
			ia, ib := items[a], items[b]
			better, worse := ia, ib
			if strength[ia] < strength[ib] {
				better, worse = ib, ia
			}
			winner := better
			if rng.Float64() > j.skill {
				if rng.Intn(2) == 0 {
					winner = worse
				}
			}
			loser := better
			if winner == better {
				loser = worse
			}
			outcomes = append(outcomes, Outcome{
				JudgeID: j.id, WinnerID: winner, LoserID: loser, Sequence: timeSeq(seq),
			})
			seq++
		}
	}
	res := Recompute(outcomes)

	// Kendall tau against true strength order.
	got := make([]uuid.UUID, len(res.Items))
	truth := make([]uuid.UUID, nItems)
	for i := range res.Items {
		got[i] = res.Items[i].ID
	}
	for i, id := range items {
		truth[i] = id
	}
	sort.Slice(truth, func(i, j int) bool { return strength[truth[i]] > strength[truth[j]] })
	tau := kendallTau(got, truth)
	if tau < 0.65 {
		t.Fatalf("Kendall tau = %.3f, want >= 0.65; ranking=%v", tau, got)
	}

	rel := map[uuid.UUID]float64{}
	for _, j := range res.Judges {
		rel[j.ID] = j.Reliability
	}
	for i := 0; i < nGood; i++ {
		for k := nGood; k < nGood+nNoisy; k++ {
			if rel[judges[i].id] <= rel[judges[k].id] {
				t.Fatalf("good judge %.3f should beat noisy judge %.3f", rel[judges[i].id], rel[judges[k].id])
			}
		}
	}
}

func TestSingleJudgeStillRanks(t *testing.T) {
	j := uuid.New()
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	var outcomes []Outcome
	for i := 0; i < 5; i++ {
		outcomes = append(outcomes,
			Outcome{JudgeID: j, WinnerID: a, LoserID: b, Sequence: timeSeq(i)},
			Outcome{JudgeID: j, WinnerID: b, LoserID: c, Sequence: timeSeq(i + 10)},
		)
	}
	res := Recompute(outcomes)
	if len(res.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(res.Items))
	}
	if res.Items[0].ID != a || res.Items[2].ID != c {
		t.Fatalf("unexpected order: %+v", res.Items)
	}
}

func timeSeq(i int) string {
	// Fixed width so lexical ordering matches numeric ordering.
	const digits = "0000000000"
	s := digits
	pos := len(s) - 1
	for i > 0 {
		s = s[:pos] + string(rune('0'+i%10)) + s[pos+1:]
		i /= 10
		pos--
	}
	return s
}

func kendallTau(got, truth []uuid.UUID) float64 {
	rank := map[uuid.UUID]int{}
	for i, id := range truth {
		rank[id] = i
	}
	concordant, discordant := 0, 0
	for i := 0; i < len(got); i++ {
		for j := i + 1; j < len(got); j++ {
			if rank[got[i]] < rank[got[j]] {
				concordant++
			} else if rank[got[i]] > rank[got[j]] {
				discordant++
			}
		}
	}
	total := concordant + discordant
	if total == 0 {
		return 0
	}
	return float64(concordant-discordant) / float64(total)
}
