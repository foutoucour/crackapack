package tune

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"sync"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/sim"
)

// InvalidLoss is the loss of a candidate whose config does not load or whose run fails. It sits far
// above any real loss, so an invalid candidate ranks last.
const InvalidLoss = 1e9

// Seeds of the two streams the search draws from, kept apart so that the candidates do not change
// when the number of seeds does.
const (
	candidateStream = 0x63616e64
	seedStream      = 0x73656564
)

// Runner simulates one run of a candidate. The config it receives already carries the seed and the
// scenario's visit rate and length. It is called from several goroutines at once.
type Runner interface {
	Run(cfg config.Config, run Run) (sim.Summary, error)
}

// Options sets the size of a search.
type Options struct {
	// Seed makes the search reproducible: the candidates and the simulation seeds derive from it.
	Seed uint64
	// Candidates is how many configs the screening stage draws.
	Candidates int
	// KeepFraction is the share of the valid candidates that survive screening, in (0, 1].
	KeepFraction float64
	// Seeds is how many simulation seeds score the survivors. The first one also screens.
	Seeds int
	// ValidationSeeds is how many fresh seeds re-score the best survivors.
	ValidationSeeds int
	// Top is how many of the best survivors are validated.
	Top int
	// Workers is how many simulations run at once.
	Workers int
	// Progress, when set, is told after each simulation job of a stage finishes.
	Progress func(stage string, done, total int)
}

func (o Options) validate() error {
	switch {
	case o.Candidates < 1:
		return errors.New("candidates must be at least 1")
	case o.KeepFraction <= 0 || o.KeepFraction > 1:
		return fmt.Errorf("keep fraction must be in (0, 1], got %v", o.KeepFraction)
	case o.Seeds < 1:
		return errors.New("seeds must be at least 1")
	case o.ValidationSeeds < 1:
		return errors.New("validation seeds must be at least 1")
	case o.Top < 1:
		return errors.New("top must be at least 1")
	case o.Workers < 1:
		return errors.New("workers must be at least 1")
	}
	return nil
}

// Candidate is one drawn config and how it scored.
type Candidate struct {
	ID        int               `json:"id"`
	Overrides []config.Override `json:"overrides"`
	// Invalid is why the candidate could not run; empty when it did.
	Invalid string `json:"invalid,omitempty"`
	// ScreenLoss is the loss on the first seed, which every valid candidate gets.
	ScreenLoss float64 `json:"screen_loss"`
	// Loss is the mean loss over the search seeds; only the survivors of screening have one.
	Loss *float64 `json:"loss,omitempty"`
	// ValidationLoss is the mean loss over seeds the search never used; only the best survivors
	// have one.
	ValidationLoss *float64 `json:"validation_loss,omitempty"`
	// Goals is the mean loss of each band over the search seeds, keyed "run metric". Only
	// survivors have it.
	Goals map[string]float64 `json:"goals,omitempty"`
}

// Report is what a search found.
type Report struct {
	Options         Options       `json:"-"`
	ScreenSeed      uint64        `json:"screen_seed"`
	SearchSeeds     []uint64      `json:"search_seeds"`
	ValidationSeeds []uint64      `json:"validation_seeds"`
	Baseline        Candidate     `json:"baseline"`
	Candidates      []Candidate   `json:"candidates"`
	Invalid         int           `json:"invalid"`
	Ranked          []int         `json:"ranked"`
	Feasible        bool          `json:"feasible"`
	Windows         []Window      `json:"windows"`
	Influence       []Influence   `json:"influence"`
	BestConfig      config.Config `json:"-"`
	Params          []Param       `json:"-"`
}

// Best returns the survivor with the lowest loss on the search seeds.
func (r Report) Best() Candidate { return r.Candidates[r.Ranked[0]] }

// searcher holds what the stages of one search share.
type searcher struct {
	base   config.Config
	runs   []Run
	shapes []Shape
	runner Runner
	opts   Options
	// configs are the built configs by candidate ID, filled by the single goroutine that scores.
	configs map[int]built
}

// Search draws candidates from space, screens them on one seed, scores the survivors on the
// search seeds and validates the best on seeds it never used.
func Search(ctx context.Context, base config.Config, space Space, problem Problem, runner Runner, opts Options) (Report, error) {
	if err := opts.validate(); err != nil {
		return Report{}, fmt.Errorf("search: %w", err)
	}
	s := &searcher{base: base, runs: problem.Runs(), shapes: problem.Shapes, runner: runner, opts: opts, configs: make(map[int]built)}
	searchSeeds, validationSeeds := deriveSeeds(opts.Seed, opts.Seeds, opts.ValidationSeeds)
	report := Report{
		Options: opts, ScreenSeed: searchSeeds[0], SearchSeeds: searchSeeds, ValidationSeeds: validationSeeds,
		Params: space.Params,
	}

	baseline, err := s.score(ctx, "baseline", []Candidate{{}}, searchSeeds)
	if err != nil {
		return Report{}, err
	}
	if baseline[0].Invalid != "" {
		return Report{}, fmt.Errorf("search: the base config does not run: %s", baseline[0].Invalid)
	}
	baselineLoss := baseline[0].Loss
	report.Baseline = Candidate{Loss: &baselineLoss, ScreenLoss: baselineLoss, Goals: baseline[0].Goals}

	rng := rand.New(rand.NewPCG(opts.Seed, candidateStream))
	report.Candidates = make([]Candidate, opts.Candidates)
	for i := range report.Candidates {
		report.Candidates[i] = Candidate{ID: i + 1, Overrides: space.Sample(rng)}
	}
	screened, err := s.score(ctx, "screening", report.Candidates, searchSeeds[:1])
	if err != nil {
		return Report{}, err
	}
	for i, c := range screened {
		report.Candidates[i].Invalid = c.Invalid
		report.Candidates[i].ScreenLoss = c.screenLoss()
		if c.Invalid != "" {
			report.Invalid++
		}
	}

	survivors := screeningSurvivors(report.Candidates, opts.KeepFraction)
	if len(survivors) == 0 {
		return Report{}, errors.New("search: no candidate is valid; widen the space or check the base config")
	}
	scored, err := s.score(ctx, "scoring", pick(report.Candidates, survivors), searchSeeds)
	if err != nil {
		return Report{}, err
	}
	for k, i := range survivors {
		loss := scored[k].Loss
		report.Candidates[i].Loss = &loss
		report.Candidates[i].Goals = scored[k].Goals
	}
	report.Ranked = rankByLoss(report.Candidates, survivors)

	top := report.Ranked[:min(opts.Top, len(report.Ranked))]
	validated, err := s.score(ctx, "validation", pick(report.Candidates, top), validationSeeds)
	if err != nil {
		return Report{}, err
	}
	for k, i := range top {
		loss := validated[k].Loss
		report.Candidates[i].ValidationLoss = &loss
	}

	report.BestConfig, err = config.ApplyOverrides(base, report.Best().Overrides)
	if err != nil {
		return Report{}, fmt.Errorf("search: rebuild the best config: %w", err)
	}
	report.analyse(survivors)
	return report, nil
}

// scored is the outcome of scoring one candidate.
type scored struct {
	Loss    float64
	Goals   map[string]float64
	Invalid string
}

// screenLoss returns the loss, or InvalidLoss when the candidate did not run.
func (s scored) screenLoss() float64 {
	if s.Invalid != "" {
		return InvalidLoss
	}
	return s.Loss
}

func pick(all []Candidate, ids []int) []Candidate {
	out := make([]Candidate, len(ids))
	for k, i := range ids {
		out[k] = all[i]
	}
	return out
}

// score runs every valid candidate on every seed and returns, per candidate, the mean loss. A
// candidate whose config is refused, or whose run fails, is Invalid. A band on a metric the run
// does not report aborts the search: no candidate can fix it.
func (s *searcher) score(ctx context.Context, stage string, cands []Candidate, seeds []uint64) ([]scored, error) {
	cfgs := make([]config.Config, len(cands))
	out := make([]scored, len(cands))
	for i, c := range cands {
		cfg, err := s.build(c)
		if err != nil {
			out[i].Invalid = err.Error()
			continue
		}
		cfgs[i] = cfg
	}

	type job struct{ cand, seed int }
	var jobs []job
	for i := range cands {
		if out[i].Invalid != "" {
			continue
		}
		for j := range seeds {
			jobs = append(jobs, job{i, j})
		}
	}
	results := make([]seedResult, len(jobs))
	var mu sync.Mutex
	done := 0
	err := parallel(ctx, s.opts.Workers, len(jobs), func(n int) error {
		j := jobs[n]
		res, err := s.runAll(cfgs[j.cand], seeds[j.seed])
		if err != nil {
			return err
		}
		results[n] = res
		if s.opts.Progress != nil {
			mu.Lock()
			done++
			s.opts.Progress(stage, done, len(jobs))
			mu.Unlock()
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("search: %s: %w", stage, err)
	}

	bySeeds := make(map[int][]seedResult)
	for n, j := range jobs {
		bySeeds[j.cand] = append(bySeeds[j.cand], results[n])
	}
	for i, rs := range bySeeds {
		out[i] = mean(rs)
	}
	return out, nil
}

// built is a candidate's config, or why it could not be built.
type built struct {
	cfg config.Config
	err error
}

// build returns the config of a candidate. Loading a config is the dearest step outside the
// simulations, so each candidate's is built once and kept for the stages that follow.
func (s *searcher) build(c Candidate) (config.Config, error) {
	if b, ok := s.configs[c.ID]; ok {
		return b.cfg, b.err
	}
	cfg, err := config.ApplyOverrides(s.base, c.Overrides)
	s.configs[c.ID] = built{cfg: cfg, err: err}
	return cfg, err
}

// seedResult is the loss of one candidate on one seed, over all the runs.
type seedResult struct {
	loss    float64
	goals   map[string]float64
	invalid string
}

// runAll simulates every run of the problem with one seed and sums the distance outside the bands,
// and the distance of each shape from its bell.
func (s *searcher) runAll(cfg config.Config, seed uint64) (seedResult, error) {
	res := seedResult{goals: make(map[string]float64)}
	shaped := make(map[string][]float64)
	for _, run := range s.runs {
		runCfg := cfg
		runCfg.Seed = seed
		if run.SessionsPerDay > 0 {
			runCfg.SessionsPerDay = run.SessionsPerDay
		}
		if run.RunDays > 0 {
			runCfg.RunDays = run.RunDays
		}
		summary, err := s.runner.Run(runCfg, run)
		if err != nil {
			return seedResult{invalid: fmt.Sprintf("run %s: %v", run.Name, err)}, nil
		}
		for _, band := range run.Bands {
			value, ok := summary[band.Metric]
			if !ok {
				return seedResult{}, fmt.Errorf("run %s reports no metric %q", run.Name, band.Metric)
			}
			d := Distance(value, band)
			res.loss += d
			res.goals[run.Name+" "+band.Metric] += d
		}
		for _, shape := range run.Shapes {
			value, ok := summary[shape.Metric]
			if !ok {
				return seedResult{}, fmt.Errorf("run %s reports no metric %q", run.Name, shape.Metric)
			}
			shaped[shape.Key] = append(shaped[shape.Key], value)
		}
	}
	for _, shape := range s.shapes {
		d := ShapeDistance(shaped[shape.key()], shape.Center, shape.Spread)
		res.loss += d
		res.goals[shape.key()] += d
	}
	return res, nil
}

// mean averages the results of one candidate over its seeds. One failed seed makes the candidate
// invalid.
func mean(rs []seedResult) scored {
	for _, r := range rs {
		if r.invalid != "" {
			return scored{Invalid: r.invalid}
		}
	}
	n := float64(len(rs))
	total := 0.0
	goals := make(map[string]float64)
	for _, r := range rs {
		total += r.loss
		for k, v := range r.goals {
			goals[k] += v / n
		}
	}
	return scored{Loss: total / n, Goals: goals}
}

// screeningSurvivors returns the indexes of the best KeepFraction of the valid candidates by
// screening loss, ties broken by ID.
func screeningSurvivors(cands []Candidate, fraction float64) []int {
	var valid []int
	for i, c := range cands {
		if c.Invalid == "" {
			valid = append(valid, i)
		}
	}
	slices.SortStableFunc(valid, func(a, b int) int { return cmp.Compare(cands[a].ScreenLoss, cands[b].ScreenLoss) })
	keep := max(1, int(math.Ceil(fraction*float64(len(valid)))))
	return valid[:min(keep, len(valid))]
}

// rankByLoss returns the indexes in ids ordered by loss on the search seeds, ties broken by ID.
func rankByLoss(cands []Candidate, ids []int) []int {
	ranked := slices.Clone(ids)
	slices.SortStableFunc(ranked, func(a, b int) int { return cmp.Compare(*cands[a].Loss, *cands[b].Loss) })
	return ranked
}

// deriveSeeds draws the search seeds, then the validation seeds, from one stream, skipping repeats
// so the two sets never overlap.
func deriveSeeds(seed uint64, search, validation int) ([]uint64, []uint64) {
	rng := rand.New(rand.NewPCG(seed, seedStream))
	seen := make(map[uint64]bool, search+validation)
	draw := func(n int) []uint64 {
		out := make([]uint64, 0, n)
		for len(out) < n {
			if v := rng.Uint64(); !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
		return out
	}
	return draw(search), draw(validation)
}

// parallel calls do for each index below n on workers goroutines. The first error cancels the rest
// and is returned.
func parallel(ctx context.Context, workers, n int, do func(int) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	indexes := make(chan int)
	var wg sync.WaitGroup
	var once sync.Once
	var firstErr error
	for range min(workers, max(n, 1)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range indexes {
				if err := do(i); err != nil {
					once.Do(func() { firstErr = err; cancel() })
				}
			}
		}()
	}
feed:
	for i := range n {
		select {
		case indexes <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(indexes)
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}
