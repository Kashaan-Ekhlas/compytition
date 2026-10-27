// Package crowdbt implements a compact, deterministic CrowdBT-style model for
// pairwise ranking aggregation (Chen et al., 2013).
//
// Model
//   - Every submission has a latent score s_i ~ N(mu_i, sigma_i^2).
//   - Every judge has a reliability r_j ~ Beta(alpha_j, beta_j), the probability
//     that their pick follows the latent ordering. A judge picking at random has
//     r = 0.5; reliable judges approach 1.
//   - A comparison is modelled as: with probability r_j the judge picks the
//     higher-scoring item, otherwise they pick uniformly at random.
//
// Updates
//   - Item scores use a TrueSkill-style Gaussian update on the score difference.
//   - The update is weighted by (2r_j - 1), so low-reliability judges move the
//     ranking less.
//   - Judge reliability is updated with the posterior probability that the judge
//     was reliable given the observed pick (Beta pseudo-counts).
//
// Priors and hyperparameters are documented in context.md. The package is pure
// (no DB or clock access), so a full recompute from the log is reproducible and
// auditable.
package crowdbt

import (
	"math"
	"sort"

	"github.com/google/uuid"
)

const (
	PriorMean     = 0.0
	PriorVariance = 1.0
	// BetaSq is the performance-noise variance (beta = 0.5).
	BetaSq = 0.25
	// Beta prior: mean 0.75, strength 4. Judges are presumed somewhat reliable
	// until the data says otherwise; this also bootstraps score movement.
	PriorAlpha = 3.0
	PriorBeta  = 1.0
	// Minimum variance keeps confidence from collapsing to certainty.
	MinVariance = 0.05
	MaxStep     = 10.0
)

type Item struct {
	ID       uuid.UUID
	Mu       float64
	Variance float64
}

type Judge struct {
	ID          uuid.UUID
	Alpha       float64
	Beta        float64
	Comparisons int
}

type Outcome struct {
	JudgeID  uuid.UUID
	WinnerID uuid.UUID
	LoserID  uuid.UUID
	// Sequence provides a deterministic tie-breaker when timestamps match.
	Sequence string
}

type ItemResult struct {
	ID         uuid.UUID `json:"id"`
	Mu         float64   `json:"score"`
	Variance   float64   `json:"variance"`
	Rank       int       `json:"rank"`
	Confidence float64   `json:"confidence"`
}

type JudgeResult struct {
	ID          uuid.UUID `json:"id"`
	Alpha       float64   `json:"alpha"`
	Beta        float64   `json:"beta"`
	Reliability float64   `json:"reliability"`
	Comparisons int       `json:"comparisons"`
}

type Result struct {
	Items  []ItemResult  `json:"items"`
	Judges []JudgeResult `json:"judges"`
}

type Model struct {
	items  map[uuid.UUID]*Item
	judges map[uuid.UUID]*Judge
}

func New() *Model {
	return &Model{items: map[uuid.UUID]*Item{}, judges: map[uuid.UUID]*Judge{}}
}

func (m *Model) item(id uuid.UUID) *Item {
	if it, ok := m.items[id]; ok {
		return it
	}
	it := &Item{ID: id, Mu: PriorMean, Variance: PriorVariance}
	m.items[id] = it
	return it
}

func (m *Model) judge(id uuid.UUID) *Judge {
	if j, ok := m.judges[id]; ok {
		return j
	}
	j := &Judge{ID: id, Alpha: PriorAlpha, Beta: PriorBeta}
	m.judges[id] = j
	return j
}

// Update applies one judgment. `winner` and `loser` are submission IDs.
func (m *Model) Update(judgeID, winnerID, loserID uuid.UUID) {
	if winnerID == loserID {
		return
	}
	w := m.item(winnerID)
	l := m.item(loserID)
	j := m.judge(judgeID)
	j.Comparisons++

	r := j.Alpha / (j.Alpha + j.Beta)
	diff := w.Mu - l.Mu
	c2 := w.Variance + l.Variance + 2*BetaSq
	if c2 < 1e-9 {
		c2 = 1e-9
	}
	p := normalCDF(diff / math.Sqrt(c2))

	// Reliability posterior: P(reliable | observed winner).
	denom := r*p + (1-r)*0.5
	if denom < 1e-12 {
		denom = 1e-12
	}
	q := (r * p) / denom
	if q < 0 {
		q = 0
	}
	if q > 1 {
		q = 1
	}
	j.Alpha += q
	j.Beta += 1 - q

	// Weight the score update by reliability above chance.
	weight := 2*r - 1
	if weight < 0 {
		weight = 0
	}
	if weight > 1 {
		weight = 1
	}
	if weight == 0 {
		return
	}

	// TrueSkill-style update for a decisive win.
	c := math.Sqrt(c2)
	t := diff / c
	cdf := normalCDF(t)
	if cdf < 1e-12 {
		cdf = 1e-12
	}
	pdf := math.Exp(-0.5*t*t) / math.Sqrt(2*math.Pi)
	v := pdf / cdf
	if v > MaxStep {
		v = MaxStep
	}
	wFunc := v * (v + t)

	muDelta := weight * (w.Variance / c) * v
	w.Mu += muDelta
	l.Mu -= weight * (l.Variance / c) * v

	w.Variance *= 1 - weight*(w.Variance/c2)*wFunc
	l.Variance *= 1 - weight*(l.Variance/c2)*wFunc
	if w.Variance < MinVariance {
		w.Variance = MinVariance
	}
	if l.Variance < MinVariance {
		l.Variance = MinVariance
	}
}

// Recompute replays the complete judgment log in deterministic order. It is the
// audit path used for finalization, and it matches the online update exactly.
func Recompute(outcomes []Outcome) *Result {
	ordered := append([]Outcome(nil), outcomes...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Sequence != ordered[j].Sequence {
			return ordered[i].Sequence < ordered[j].Sequence
		}
		return false
	})
	m := New()
	for _, o := range ordered {
		if o.WinnerID == uuid.Nil || o.LoserID == uuid.Nil || o.JudgeID == uuid.Nil {
			continue
		}
		m.Update(o.JudgeID, o.WinnerID, o.LoserID)
	}
	return m.Result()
}

func (m *Model) Result() *Result {
	items := make([]ItemResult, 0, len(m.items))
	for _, it := range m.items {
		items = append(items, ItemResult{ID: it.ID, Mu: it.Mu, Variance: it.Variance})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Mu != items[j].Mu {
			return items[i].Mu > items[j].Mu
		}
		return items[i].ID.String() < items[j].ID.String()
	})
	for i := range items {
		items[i].Rank = i + 1
		items[i].Confidence = confidence(items, i)
	}
	judges := make([]JudgeResult, 0, len(m.judges))
	for _, j := range m.judges {
		judges = append(judges, JudgeResult{
			ID: j.ID, Alpha: j.Alpha, Beta: j.Beta,
			Reliability: j.Alpha / (j.Alpha + j.Beta),
			Comparisons: j.Comparisons,
		})
	}
	sort.Slice(judges, func(i, j int) bool {
		if judges[i].Reliability != judges[j].Reliability {
			return judges[i].Reliability > judges[j].Reliability
		}
		return judges[i].ID.String() < judges[j].ID.String()
	})
	return &Result{Items: items, Judges: judges}
}

// confidence is the model's probability that this item beats its nearest
// neighbour in the ranking (for the last item: probability it loses to the one
// above). It is bounded to [0,1] and is a display aid, not a p-value.
func confidence(items []ItemResult, i int) float64 {
	n := len(items)
	if n < 2 {
		return 0.5
	}
	if i < n-1 {
		diff := items[i].Mu - items[i+1].Mu
		sigma := math.Sqrt(items[i].Variance + items[i+1].Variance)
		if sigma < 1e-9 {
			return 1
		}
		return clamp01(normalCDF(diff / sigma))
	}
	diff := items[i-1].Mu - items[i].Mu
	sigma := math.Sqrt(items[i-1].Variance + items[i].Variance)
	if sigma < 1e-9 {
		return 1
	}
	return clamp01(normalCDF(diff / sigma))
}

func normalCDF(x float64) float64 {
	return 0.5 * math.Erfc(-x/math.Sqrt2)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
