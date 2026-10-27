// Package assignment builds a conflict-free pairwise judging plan. The plan is
// deliberately over-connected: every submission is covered several times, every
// judge shares anchor submissions with every other judge, and the submission
// comparison graph is connected. CrowdBT needs that structure to calibrate
// judge reliability.
package assignment

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"github.com/google/uuid"
)

type Config struct {
	TargetComparisons int
	JudgeBudget       int
	AnchorCount       int
}

type Judge struct {
	ID        uuid.UUID
	Conflicts map[uuid.UUID]bool // submissions this judge must not see
}

type Input struct {
	Submissions []uuid.UUID
	Judges      []Judge
	Config      Config
	// Seed makes generation deterministic. The judging service passes the
	// event ID so a preview matches the persisted plan.
	Seed string
}

type Pair struct {
	JudgeID uuid.UUID `json:"judge_id"`
	A       uuid.UUID `json:"submission_a"`
	B       uuid.UUID `json:"submission_b"`
	Left    uuid.UUID `json:"left_submission"`
	Overlap bool      `json:"anchor_pair"`
}

type JudgeStat struct {
	JudgeID     uuid.UUID `json:"judge_id"`
	Comparisons int       `json:"comparisons"`
	Seen        int       `json:"submissions_seen"`
}

type OverlapStat struct {
	A      uuid.UUID `json:"judge_a"`
	B      uuid.UUID `json:"judge_b"`
	Shared int       `json:"shared_submissions"`
}

type Plan struct {
	Pairs             []Pair            `json:"pairs"`
	JudgeStats        []JudgeStat       `json:"judge_stats"`
	Overlap           []OverlapStat     `json:"overlap"`
	Coverage          map[uuid.UUID]int `json:"coverage"`
	CoverageMin       int               `json:"coverage_min"`
	CoverageMax       int               `json:"coverage_max"`
	Connected         bool              `json:"connected"`
	Anchors           []uuid.UUID       `json:"anchors"`
	LeftCounts        map[uuid.UUID]int `json:"left_counts"`
	TotalPairs        int               `json:"total_pairs"`
	TargetCoverage    int               `json:"target_coverage"`
	BlockedSubmission []uuid.UUID       `json:"blocked_submissions,omitempty"`
}

type builder struct {
	subs      []uuid.UUID
	judges    []Judge
	cfg       Config
	rng       *rand.Rand
	pairs     []Pair
	perJudge  map[uuid.UUID]map[string]bool
	load      map[uuid.UUID]int
	coverage  map[uuid.UUID]int
	left      map[uuid.UUID]int
	seen      map[uuid.UUID]map[uuid.UUID]bool
	anchorSet map[uuid.UUID]bool
	anchors   []uuid.UUID
}

var ErrImpossible = errors.New("cannot build a conflict-free connected assignment")

func Generate(in Input) (*Plan, error) {
	if len(in.Submissions) < 2 {
		return nil, fmt.Errorf("%w: at least 2 submissions are required", ErrImpossible)
	}
	if len(in.Judges) == 0 {
		return nil, fmt.Errorf("%w: at least 1 judge is required", ErrImpossible)
	}
	if in.Config.TargetComparisons <= 0 {
		in.Config.TargetComparisons = 6
	}
	if in.Config.JudgeBudget <= 0 {
		in.Config.JudgeBudget = 20
	}
	if in.Config.AnchorCount <= 0 {
		in.Config.AnchorCount = 3
	}
	if in.Config.JudgeBudget < 1 {
		return nil, fmt.Errorf("%w: judge budget must be positive", ErrImpossible)
	}

	subs := append([]uuid.UUID(nil), in.Submissions...)
	judges := append([]Judge(nil), in.Judges...)
	b := &builder{
		subs: subs, judges: judges, cfg: in.Config,
		rng:       rand.New(rand.NewSource(seedInt(in.Seed, subs, judges))),
		perJudge:  map[uuid.UUID]map[string]bool{},
		load:      map[uuid.UUID]int{},
		coverage:  map[uuid.UUID]int{},
		left:      map[uuid.UUID]int{},
		seen:      map[uuid.UUID]map[uuid.UUID]bool{},
		anchorSet: map[uuid.UUID]bool{},
	}
	for _, j := range judges {
		b.perJudge[j.ID] = map[string]bool{}
		b.seen[j.ID] = map[uuid.UUID]bool{}
	}
	for _, s := range subs {
		b.coverage[s] = 0
	}

	// Anchors are shared by every judge and provide judge-overlap. Clamp the
	// anchor count so it cannot consume the whole per-judge budget.
	effectiveAnchors := in.Config.AnchorCount
	if effectiveAnchors > len(subs) {
		effectiveAnchors = len(subs)
	}
	if effectiveAnchors > in.Config.JudgeBudget+1 {
		effectiveAnchors = in.Config.JudgeBudget + 1
	}
	if effectiveAnchors < 1 {
		effectiveAnchors = 1
	}

	// 1) A spanning path guarantees the global submission graph is connected.
	order := append([]uuid.UUID(nil), subs...)
	b.rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	b.anchors = order
	for i := 0; i+1 < len(order); i++ {
		if !b.add(order[i], order[i+1]) {
			// Try the reverse orientation and any other judge; add() already
			// iterates all eligible judges, so failure means every judge is
			// conflicted with one of these submissions.
			if !b.add(order[i+1], order[i]) {
				return nil, fmt.Errorf("%w: no judge can compare two submissions (conflict on every judge?)", ErrImpossible)
			}
		}
	}

	// 2) Every judge sees the anchor chain (A-1 pairs). This is what makes
	// every pair of judges overlap on at least the anchors.
	anchors := order[:effectiveAnchors]
	for _, a := range anchors {
		b.anchorSet[a] = true
	}
	for _, j := range judges {
		for i := 0; i+1 < len(anchors); i++ {
			b.addForJudge(j.ID, anchors[i], anchors[i+1])
		}
	}

	// 3) Fill coverage. Target is capped by the total available slots.
	totalSlots := len(judges) * in.Config.JudgeBudget
	maxCoverage := (totalSlots * 2) / len(subs)
	if maxCoverage < 1 {
		maxCoverage = 1
	}
	target := in.Config.TargetComparisons
	if target > maxCoverage {
		target = maxCoverage
	}
	if target > len(subs)-1 {
		target = len(subs) - 1
	}
	if target < 1 {
		target = 1
	}
	b.fillCoverage(target)

	// 4) Repair judge overlap. Anchors normally suffice; conflicts or a low
	// anchor/budget setting can leave pairs below the threshold.
	b.repairOverlap(2)

	// 5) Attach disconnected components (rare, but explicit rather than silent).
	for i := 0; i < len(subs) && !b.connected(); i++ {
		b.rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		if !b.add(order[0], order[1]) {
			break
		}
	}
	if !b.connected() {
		return nil, fmt.Errorf("%w: comparison graph is not connected", ErrImpossible)
	}

	plan := b.plan(target)
	plan.Anchors = anchors
	plan.Connected = true
	return plan, nil
}

func (b *builder) add(a, c uuid.UUID) bool {
	if a == c {
		return false
	}
	// Prefer the least-loaded eligible judges, with random tie-breaking.
	type candidate struct {
		j    Judge
		load int
		rand int
	}
	var candidates []candidate
	for _, j := range b.judges {
		if b.canAdd(j, a, c) {
			candidates = append(candidates, candidate{j: j, load: b.load[j.ID], rand: b.rng.Intn(1000)})
		}
	}
	if len(candidates) == 0 {
		return false
	}
	sort.Slice(candidates, func(i, k int) bool {
		if candidates[i].load != candidates[k].load {
			return candidates[i].load < candidates[k].load
		}
		return candidates[i].rand < candidates[k].rand
	})
	return b.addForJudge(candidates[0].j.ID, a, c)
}

func (b *builder) addForJudge(judgeID, a, c uuid.UUID) bool {
	j := b.judgeByID(judgeID)
	if j == nil || !b.canAdd(*j, a, c) {
		return false
	}
	x, y := a, c
	if b.rng.Intn(2) == 0 {
		x, y = y, x
	}
	left := x
	// Balance left/right exposure for each submission.
	if b.left[x] > b.left[y] {
		left = y
	}
	b.pairs = append(b.pairs, Pair{
		JudgeID: judgeID, A: a, B: c, Left: left,
		Overlap: b.anchorSet[a] || b.anchorSet[c],
	})
	key := pairKey(a, c)
	b.perJudge[judgeID][key] = true
	b.load[judgeID]++
	b.coverage[a]++
	b.coverage[c]++
	b.seen[judgeID][a] = true
	b.seen[judgeID][c] = true
	b.left[left]++
	return true
}

func (b *builder) canAdd(j Judge, a, c uuid.UUID) bool {
	if a == c {
		return false
	}
	if b.load[j.ID] >= b.cfg.JudgeBudget {
		return false
	}
	if j.Conflicts[a] || j.Conflicts[c] {
		return false
	}
	return !b.perJudge[j.ID][pairKey(a, c)]
}

func (b *builder) fillCoverage(target int) {
	if target < 1 {
		return
	}
	for attempts := 0; attempts < len(b.subs)*target*4; attempts++ {
		// Lowest-coverage submission first.
		sort.SliceStable(b.subs, func(i, j int) bool {
			if b.coverage[b.subs[i]] != b.coverage[b.subs[j]] {
				return b.coverage[b.subs[i]] < b.coverage[b.subs[j]]
			}
			return b.subs[i].String() < b.subs[j].String()
		})
		var chosen *uuid.UUID
		for i := range b.subs {
			if b.coverage[b.subs[i]] < target {
				chosen = &b.subs[i]
				break
			}
		}
		if chosen == nil {
			return
		}
		// Opponents with the lowest coverage first, random among ties.
		opponents := make([]uuid.UUID, 0, len(b.subs)-1)
		for _, other := range b.subs {
			if other != *chosen {
				opponents = append(opponents, other)
			}
		}
		b.rng.Shuffle(len(opponents), func(i, j int) { opponents[i], opponents[j] = opponents[j], opponents[i] })
		sort.SliceStable(opponents, func(i, j int) bool {
			return b.coverage[opponents[i]] < b.coverage[opponents[j]]
		})
		progress := false
		for _, opp := range opponents {
			if b.add(*chosen, opp) {
				progress = true
				break
			}
		}
		if !progress {
			return
		}
	}
}

func (b *builder) repairOverlap(minShared int) {
	for i := 0; i < len(b.judges); i++ {
		for k := i + 1; k < len(b.judges); k++ {
			ji, jk := b.judges[i], b.judges[k]
			for b.overlap(ji.ID, jk.ID) < minShared {
				progress := false
				// Compare both judges against a shared submission using two
				// fresh opponents, which adds it to both seen sets.
				for _, shared := range b.subs {
					if ji.Conflicts[shared] || jk.Conflicts[shared] {
						continue
					}
					var oppI, oppK *uuid.UUID
					for _, s := range b.subs {
						if s != shared && !b.seen[ji.ID][s] {
							v := s
							oppI = &v
							break
						}
					}
					for _, s := range b.subs {
						if s != shared && !b.seen[jk.ID][s] && (oppI == nil || s != *oppI) {
							v := s
							oppK = &v
							break
						}
					}
					if oppI != nil && b.addForJudge(ji.ID, shared, *oppI) {
						if oppK == nil {
							oppK = oppI
						}
						progress = b.addForJudge(jk.ID, shared, *oppK) || progress
					}
					if b.overlap(ji.ID, jk.ID) >= minShared {
						break
					}
				}
				if !progress {
					break
				}
			}
		}
	}
}

func (b *builder) overlap(a, c uuid.UUID) int {
	count := 0
	for s := range b.seen[a] {
		if b.seen[c][s] {
			count++
		}
	}
	return count
}

func (b *builder) connected() bool {
	if len(b.subs) == 0 {
		return true
	}
	adj := map[uuid.UUID][]uuid.UUID{}
	for _, p := range b.pairs {
		adj[p.A] = append(adj[p.A], p.B)
		adj[p.B] = append(adj[p.B], p.A)
	}
	seen := map[uuid.UUID]bool{}
	queue := []uuid.UUID{b.subs[0]}
	seen[b.subs[0]] = true
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, nxt := range adj[cur] {
			if !seen[nxt] {
				seen[nxt] = true
				queue = append(queue, nxt)
			}
		}
	}
	return len(seen) == len(b.subs)
}

func (b *builder) plan(target int) *Plan {
	coverage := map[uuid.UUID]int{}
	minC, maxC := 1<<30, 0
	for _, s := range b.subs {
		cvg := b.coverage[s]
		coverage[s] = cvg
		if cvg < minC {
			minC = cvg
		}
		if cvg > maxC {
			maxC = cvg
		}
	}
	if len(b.subs) == 0 {
		minC = 0
	}
	stats := make([]JudgeStat, 0, len(b.judges))
	for _, j := range b.judges {
		stats = append(stats, JudgeStat{
			JudgeID: j.ID, Comparisons: b.load[j.ID], Seen: len(b.seen[j.ID]),
		})
	}
	var overlaps []OverlapStat
	for i := 0; i < len(b.judges); i++ {
		for k := i + 1; k < len(b.judges); k++ {
			overlaps = append(overlaps, OverlapStat{
				A: b.judges[i].ID, B: b.judges[k].ID,
				Shared: b.overlap(b.judges[i].ID, b.judges[k].ID),
			})
		}
	}
	leftCounts := map[uuid.UUID]int{}
	for _, s := range b.subs {
		leftCounts[s] = b.left[s]
	}
	pairs := append([]Pair(nil), b.pairs...)
	return &Plan{
		Pairs: pairs, JudgeStats: stats, Overlap: overlaps, Coverage: coverage,
		CoverageMin: minC, CoverageMax: maxC, TotalPairs: len(pairs),
		TargetCoverage: target, LeftCounts: leftCounts,
	}
}

func (b *builder) judgeByID(id uuid.UUID) *Judge {
	for i := range b.judges {
		if b.judges[i].ID == id {
			return &b.judges[i]
		}
	}
	return nil
}

func pairKey(a, c uuid.UUID) string {
	if a.String() > c.String() {
		a, c = c, a
	}
	return a.String() + ":" + c.String()
}

func seedInt(seed string, subs []uuid.UUID, judges []Judge) int64 {
	h := sha256.New()
	h.Write([]byte(seed))
	sortedSubs := append([]uuid.UUID(nil), subs...)
	sort.Slice(sortedSubs, func(i, j int) bool { return sortedSubs[i].String() < sortedSubs[j].String() })
	for _, s := range sortedSubs {
		h.Write(s[:])
	}
	for _, j := range judges {
		h.Write(j.ID[:])
	}
	sum := h.Sum(nil)
	// int64 seed; math/rand accepts negative values fine.
	return int64(binary.BigEndian.Uint64(sum[:8]))
}
