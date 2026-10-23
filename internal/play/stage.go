package play

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"

	"github.com/foutoucour/crackapack/internal/domain"
)

// ErrLocked is returned by Open when the set is in a stage the player has not unlocked yet.
var ErrLocked = errors.New("play: the set is locked")

// thresholdSlack absorbs the rounding of a share times a card count, so that 0.7 of 10 cards asks
// for 7 and not 8.
const thresholdSlack = 1e-9

// StageRules is how the sets of a game open up, see docs/core-features.md#stages. The sets rank
// from the quickest binder to the slowest, see [domain.BoostersToShare], and are cut into stages:
// the first StartSets sets, then Stages stages of equal size. The first stage is open from the
// start. A binder that reaches Threshold opens SetsPerUnlock sets of the next stage, once.
type StageRules struct {
	// Threshold is the share of a binder, in (0, 1], that unlocks sets. The sets rank by the energy
	// a player spends to reach it.
	Threshold     float64
	SetsPerUnlock int
	Stages        int
	StartSets     int
}

// UnlockedSet is a set that an opening unlocked.
type UnlockedSet struct {
	Code string
	Name string
}

// staging is the stages of a game and how far the player is through them.
type staging struct {
	rules StageRules
	// stages are the set codes of each stage, in the order the sets rank, quickest first.
	stages   [][]string
	stageOf  map[string]int
	unlocked map[string]bool
	// triggered are the sets whose binder has unlocked sets already: a binder unlocks once.
	triggered map[string]bool
	// stream draws the sets an unlock opens, and is saved so that a reload does not draw again.
	stream *Stream
	rng    *rand.Rand
}

// Stage cuts the sets of the game into stages under rules and locks every set after the first stage.
// The welcome boosters go to the sets of the first stage, and the sets are listed in the order of
// the stages. stream draws the sets each unlock opens. Without Stage every set is open. Stage is
// called once, after NewGame and before Persist.
func (g *Game) Stage(rules StageRules, stream *Stream) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if rules.Threshold <= 0 || rules.Threshold > 1 || rules.SetsPerUnlock < 1 || rules.Stages < 1 || rules.StartSets < 1 {
		return fmt.Errorf("play: stage rules out of range: %+v", rules)
	}
	ranked, err := g.rank(rules.Threshold)
	if err != nil {
		return err
	}
	s := &staging{
		rules:     rules,
		stages:    cutStages(ranked, rules.StartSets, rules.Stages),
		stageOf:   make(map[string]int, len(ranked)),
		unlocked:  make(map[string]bool, len(ranked)),
		triggered: make(map[string]bool),
		stream:    stream,
		rng:       rand.New(stream),
	}
	sets := make([]domain.SetCatalogue, 0, len(g.sets))
	for i, stage := range s.stages {
		for _, code := range stage {
			s.stageOf[code] = i + 1
			cat, _ := g.set(code)
			sets = append(sets, cat)
		}
	}
	for _, code := range s.stages[0] {
		s.unlocked[code] = true
	}
	g.sets, g.staging, g.packs = sets, s, welcomePacks(s.stages[0])
	return nil
}

// rank orders the set codes by the energy it takes to reach share of their binder, the quickest
// first, and by code when two take the same.
func (g *Game) rank(share float64) ([]string, error) {
	energy := make(map[string]int, len(g.sets))
	codes := make([]string, len(g.sets))
	for i, cat := range g.sets {
		boosters, err := domain.BoostersToShare(g.binders[cat.Code], share)
		if err != nil {
			return nil, fmt.Errorf("play: rank set %q: %w", cat.Code, err)
		}
		energy[cat.Code] = boosters * g.rules.Costs[cat.Code]
		codes[i] = cat.Code
	}
	slices.SortFunc(codes, func(a, b string) int { return cmp.Or(cmp.Compare(energy[a], energy[b]), cmp.Compare(a, b)) })
	return codes, nil
}

// cutStages makes the first stage of the first start codes, and cuts the rest into n stages of equal
// size, the last one smaller when they do not divide evenly. A stage is never empty.
func cutStages(ranked []string, start, n int) [][]string {
	start = min(start, len(ranked))
	stages := [][]string{ranked[:start]}
	rest := ranked[start:]
	size := (len(rest) + n - 1) / n
	for len(rest) > 0 {
		k := min(size, len(rest))
		stages = append(stages, rest[:k])
		rest = rest[k:]
	}
	return stages
}

// welcomePacks splits the welcome boosters across codes, in turn.
func welcomePacks(codes []string) map[string]int {
	packs := make(map[string]int, len(codes))
	for i := range WelcomeBoosters {
		packs[codes[i%len(codes)]]++
	}
	return packs
}

// locked reports whether the player cannot open code yet. g.mu must be held.
func (g *Game) locked(code string) bool {
	return g.staging != nil && !g.staging.unlocked[code]
}

// pastThreshold reports whether the binder of code has reached the unlock threshold at some point,
// not whether it holds that many cards now: an unlock is permanent. It is true even when no locked
// set was left for the binder to unlock. g.mu must be held.
func (g *Game) pastThreshold(code string) bool {
	return g.staging != nil && g.staging.triggered[code]
}

// stage is the 1-based stage of code, and 0 in a game without stages. g.mu must be held.
func (g *Game) stage(code string) int {
	if g.staging == nil {
		return 0
	}
	return g.staging.stageOf[code]
}

// unlockCaughtUp unlocks the sets that a catch-up, or a save from before the stages, earned, and
// saves the game when it did, so that a set shown unlocked is still unlocked after a restart. A
// failed save is reported as every save is, see SaveError. g.mu must be held.
func (g *Game) unlockCaughtUp() {
	if len(g.unlockEarned()) > 0 {
		g.save()
	}
}

// unlockEarned unlocks sets for every binder that has reached the threshold and not unlocked
// before, and returns them in the order they were drawn. Every method that adds cards to the
// collection calls it, before it saves. g.mu must be held.
func (g *Game) unlockEarned() []UnlockedSet {
	s := g.staging
	if s == nil {
		return nil
	}
	var out []UnlockedSet
	for _, cat := range g.sets {
		if s.triggered[cat.Code] {
			continue
		}
		p := g.collection.BinderProgress(g.binders[cat.Code])
		need := int(math.Ceil(s.rules.Threshold*float64(p.Total) - thresholdSlack))
		if p.Total == 0 || p.Owned < need {
			continue
		}
		s.triggered[cat.Code] = true
		for _, code := range s.draw() {
			s.unlocked[code] = true
			unlockedCat, _ := g.set(code)
			out = append(out, UnlockedSet{Code: code, Name: unlockedCat.Name})
		}
	}
	if len(out) > 0 {
		g.openCrowdSets(g.economy)
	}
	return out
}

// draw picks the sets one unlock opens: at random among the locked sets of the earliest stage that
// has any, and from the stages after it when that stage has too few.
func (s *staging) draw() []string {
	want := s.rules.SetsPerUnlock
	var out []string
	for _, stage := range s.stages {
		if want == 0 {
			break
		}
		var locked []string
		for _, code := range stage {
			if !s.unlocked[code] {
				locked = append(locked, code)
			}
		}
		s.rng.Shuffle(len(locked), func(i, j int) { locked[i], locked[j] = locked[j], locked[i] })
		n := min(want, len(locked))
		out = append(out, locked[:n]...)
		want -= n
	}
	return out
}
