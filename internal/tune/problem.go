package tune

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"

	yaml "go.yaml.in/yaml/v3"

	"github.com/foutoucour/crackapack/internal/config"
)

// setPlaceholder stands for the set code, lower-cased, in the metric of a goal of a per-set
// scenario, such as days_to_100pct.{set}.
const setPlaceholder = "{set}"

// Scenario is one way of playing: a player policy, a visit rate, a length and the sets in play.
type Scenario struct {
	Name string `yaml:"name"`
	// Player is the name of a registered player policy.
	Player string `yaml:"player"`
	// SessionsPerDay and RunDays replace the config's own for this scenario. Zero keeps them.
	SessionsPerDay int `yaml:"sessions_per_day"`
	RunDays        int `yaml:"run_days"`
	// Sets are the openable set codes in play.
	Sets []string `yaml:"sets"`
	// Supplementary are card-pool-only codes, such as TSB for TSP.
	Supplementary []string `yaml:"supplementary"`
	// PerSet runs each set alone, one run per set, see ADR-0040. Its goals name the set's metric
	// with the placeholder {set}.
	PerSet bool `yaml:"per_set"`
}

// Goal is a band on a metric, judged on the runs of one scenario. Min and Max follow
// [config.CalibrationTarget].
type Goal struct {
	Scenario                 string `yaml:"scenario"`
	config.CalibrationTarget `yaml:",inline"`
}

// Shape is the form a metric should take across the sets of a per-set scenario: a bell curve of the
// given center and spread, see [ShapeDistance]. Where a goal asks each run to fall in a band, a shape
// asks the runs together to look like a bell.
type Shape struct {
	Scenario string `yaml:"scenario"`
	// Metric names the metric of each set's run, with the placeholder {set}.
	Metric string `yaml:"metric"`
	// Center is the mean of the bell and Spread its standard deviation, in the metric's unit.
	Center float64 `yaml:"center"`
	Spread float64 `yaml:"spread"`
}

// key names the shape in a candidate's goals.
func (s Shape) key() string { return s.Scenario + " shape " + s.Metric }

// Problem is what a search tries to satisfy.
type Problem struct {
	Scenarios []Scenario `yaml:"scenarios"`
	Goals     []Goal     `yaml:"goals"`
	Shapes    []Shape    `yaml:"shapes"`
}

// RunShape is the part a run plays in a shape: the metric of its set, and the key of the shape it
// belongs to, shared by the runs of the scenario.
type RunShape struct {
	Key    string
	Metric string
}

// Run is one simulation of a scenario, with the bands it is judged on and the shapes it feeds. A
// per-set scenario expands to one Run per set.
type Run struct {
	Name           string
	Player         string
	SessionsPerDay int
	RunDays        int
	Sets           []string
	Supplementary  []string
	Bands          []config.CalibrationTarget
	Shapes         []RunShape
}

// LoadProblem reads a problem from YAML and validates it. An unknown key is an error.
func LoadProblem(r io.Reader) (Problem, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Problem{}, fmt.Errorf("load problem: %w", err)
	}
	var p Problem
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil && !errors.Is(err, io.EOF) {
		return Problem{}, fmt.Errorf("load problem: %w", err)
	}
	if err := p.validate(); err != nil {
		return Problem{}, fmt.Errorf("load problem: %w", err)
	}
	return p, nil
}

func (p Problem) validate() error {
	if len(p.Scenarios) == 0 {
		return errors.New("scenarios must not be empty")
	}
	if len(p.Goals) == 0 && len(p.Shapes) == 0 {
		return errors.New("goals and shapes must not both be empty")
	}
	byName := make(map[string]Scenario, len(p.Scenarios))
	for _, s := range p.Scenarios {
		if err := s.validate(); err != nil {
			return err
		}
		if _, dup := byName[s.Name]; dup {
			return fmt.Errorf("scenario %s is listed twice", s.Name)
		}
		byName[s.Name] = s
	}
	for _, g := range p.Goals {
		s, ok := byName[g.Scenario]
		if !ok {
			return fmt.Errorf("goal on %s names an unknown scenario %q", g.Metric, g.Scenario)
		}
		if err := validateBand(g.CalibrationTarget); err != nil {
			return fmt.Errorf("goal %s on %s: %w", g.Metric, g.Scenario, err)
		}
		if !s.PerSet && strings.Contains(g.Metric, setPlaceholder) {
			return fmt.Errorf("goal %s on %s: only a per_set scenario can use %s", g.Metric, g.Scenario, setPlaceholder)
		}
	}
	for _, sh := range p.Shapes {
		if err := sh.validate(byName); err != nil {
			return fmt.Errorf("shape %s on %s: %w", sh.Metric, sh.Scenario, err)
		}
	}
	return nil
}

func (s Shape) validate(scenarios map[string]Scenario) error {
	scenario, ok := scenarios[s.Scenario]
	switch {
	case !ok:
		return fmt.Errorf("unknown scenario %q", s.Scenario)
	case !scenario.PerSet:
		return errors.New("a shape needs a per_set scenario, one run per set")
	case !strings.Contains(s.Metric, setPlaceholder):
		return fmt.Errorf("metric must contain %s, the metric of each set's run", setPlaceholder)
	case s.Center <= 0:
		return fmt.Errorf("center must be above 0, got %v", s.Center)
	case s.Spread <= 0:
		return fmt.Errorf("spread must be above 0, got %v", s.Spread)
	}
	return nil
}

func (s Scenario) validate() error {
	switch {
	case s.Name == "":
		return errors.New("a scenario has no name")
	case s.Player == "":
		return fmt.Errorf("scenario %s: player is required", s.Name)
	case len(s.Sets) == 0:
		return fmt.Errorf("scenario %s: sets must not be empty", s.Name)
	case s.SessionsPerDay < 0 || s.RunDays < 0:
		return fmt.Errorf("scenario %s: sessions_per_day and run_days must not be negative", s.Name)
	}
	return nil
}

func validateBand(t config.CalibrationTarget) error {
	switch t.Comparison {
	case config.ComparisonAtLeast:
		if t.Min == nil {
			return errors.New("at_least needs min")
		}
	case config.ComparisonAtMost:
		if t.Max == nil {
			return errors.New("at_most needs max")
		}
	case config.ComparisonBetween:
		if t.Min == nil || t.Max == nil {
			return errors.New("between needs min and max")
		}
		if *t.Min > *t.Max {
			return fmt.Errorf("min (%v) is above max (%v)", *t.Min, *t.Max)
		}
	default:
		return fmt.Errorf("unknown comparison %q", t.Comparison)
	}
	if t.Metric == "" {
		return errors.New("metric must not be empty")
	}
	return nil
}

// Runs expands the scenarios into the simulations a candidate is scored on, in file order. A
// per-set scenario gives one run per set, named "scenario/SET", and a set is never its own pool, so
// a donor set can be listed as a pool for the sets that borrow its cards.
func (p Problem) Runs() []Run {
	var runs []Run
	for _, s := range p.Scenarios {
		base := Run{Name: s.Name, Player: s.Player, SessionsPerDay: s.SessionsPerDay, RunDays: s.RunDays, Supplementary: s.Supplementary}
		if !s.PerSet {
			base.Sets = s.Sets
			base.Bands = p.bandsOf(s.Name, "")
			runs = append(runs, base)
			continue
		}
		for _, set := range s.Sets {
			run := base
			run.Name = s.Name + "/" + set
			run.Sets = []string{set}
			run.Supplementary = slices.DeleteFunc(slices.Clone(s.Supplementary), func(code string) bool { return code == set })
			run.Bands = p.bandsOf(s.Name, strings.ToLower(set))
			run.Shapes = p.shapesOf(s.Name, strings.ToLower(set))
			runs = append(runs, run)
		}
	}
	return runs
}

func (p Problem) shapesOf(scenario, set string) []RunShape {
	var shapes []RunShape
	for _, sh := range p.Shapes {
		if sh.Scenario == scenario {
			shapes = append(shapes, RunShape{Key: sh.key(), Metric: strings.ReplaceAll(sh.Metric, setPlaceholder, set)})
		}
	}
	return shapes
}

func (p Problem) bandsOf(scenario, set string) []config.CalibrationTarget {
	var bands []config.CalibrationTarget
	for _, g := range p.Goals {
		if g.Scenario != scenario {
			continue
		}
		band := g.CalibrationTarget
		band.Metric = strings.ReplaceAll(band.Metric, setPlaceholder, set)
		bands = append(bands, band)
	}
	return bands
}

// Distance is how far value falls outside the band, divided by the band's size, so that goals
// on metrics of different magnitudes add up. It is 0 inside the band. The size of a between band
// is its width; a band with one bound, or a zero width, uses the magnitude of its bound, and at
// least 1, so a goal such as sub_coin_cards at most 0 counts each offending card as 1.
func Distance(value float64, band config.CalibrationTarget) float64 {
	var below, above float64
	if band.Min != nil {
		below = math.Max(*band.Min-value, 0)
	}
	if band.Max != nil {
		above = math.Max(value-*band.Max, 0)
	}
	return (below + above) / bandSize(band)
}

// ShapeDistance is how far values fall from a bell curve of the given center and spread. The sorted
// values are laid against the quantiles of that curve, the value the i-th of n sets would take in a
// perfect bell, and each gap is counted in spreads. It is 0 for a perfect bell, and every set that
// sits one spread from its place adds 1, as a set one band-width outside a band does in [Distance].
// A pile of equal values, a skew and a long tail all cost, and the order of the values does not.
func ShapeDistance(values []float64, center, spread float64) float64 {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	n := float64(len(sorted))
	total := 0.0
	for i, v := range sorted {
		quantile := center + spread*math.Sqrt2*math.Erfinv(2*(float64(i)+0.5)/n-1)
		total += math.Abs(v-quantile) / spread
	}
	return total
}

func bandSize(band config.CalibrationTarget) float64 {
	if band.Min != nil && band.Max != nil && *band.Max > *band.Min {
		return *band.Max - *band.Min
	}
	size := 1.0
	if band.Min != nil {
		size = math.Max(size, math.Abs(*band.Min))
	}
	if band.Max != nil {
		size = math.Max(size, math.Abs(*band.Max))
	}
	return size
}
