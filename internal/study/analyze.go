// Package study compares ways of playing against the speeds the product wants: how much faster the
// Counter and the market make a binder than opening boosters alone. It reads the days each run
// took, and judges them.
package study

import (
	"cmp"
	"math"
	"slices"
	"strconv"

	"github.com/foutoucour/crackapack/internal/config"
)

// The three ways of playing the study compares, each a registered player policy.
const (
	BoostersOnly = "boosters-only"
	Counter      = "collector"
	Market       = "market-trader"
)

// Names of the checks a variant is judged on.
const (
	CheckCounterSpeed      = "counter speed"
	CheckMarketSpeed       = "market speed"
	CheckMarketOverCounter = "market over counter"
	CheckFloor             = "fastest set"
)

// p10 is the fraction of the sets that finish faster than the tenth percentile.
const p10 = 0.1

// SetRun is what one run of one set reported.
type SetRun struct {
	Set string `json:"set"`
	// Seed is the seed of the run: a study that runs several pools the runs of all of them.
	Seed uint64 `json:"seed"`
	// Days is the first day the whole binder was owned, or the horizon plus one when it never was.
	Days float64 `json:"days"`
	// CUDays is the same for the commons and uncommons alone.
	CUDays float64 `json:"cu_days"`
	// Asks, Listings and Deals count what a market player did, and are zero for the others.
	Asks     float64 `json:"asks,omitempty"`
	Listings float64 `json:"listings,omitempty"`
	Deals    float64 `json:"deals,omitempty"`
}

// Outcome is every run of one variant: the sets of each player.
type Outcome struct {
	Name string `json:"name"`
	Note string `json:"note,omitempty"`
	// Overrides are the config keys the variant changed.
	Overrides []config.Override `json:"overrides,omitempty"`
	// Runs maps a player to its runs, one per set.
	Runs map[string][]SetRun `json:"runs"`
}

// Band is a closed range of speeds.
type Band struct{ Min, Max float64 }

// Targets are the speeds a variant is judged on. A speed is the median days of a player who opens
// boosters alone, over the median days of the player judged.
type Targets struct {
	// CounterSpeed is the band the Counter's speed must fall in: the Counter helps, but stays the
	// slow road.
	CounterSpeed Band
	// MarketSpeedMin is the least speed the market must reach.
	MarketSpeedMin float64
	// MarketOverCounterMin is how many times faster than the Counter the market must be.
	MarketOverCounterMin float64
	// FloorDays is the least days the fastest set may take with the market.
	FloorDays float64
}

// Stats summarises the days a player took over the sets.
type Stats struct {
	Median float64 `json:"median"`
	P10    float64 `json:"p10"`
	Min    float64 `json:"min"`
	// Unfinished is the share of the sets whose binder was never complete.
	Unfinished float64 `json:"unfinished"`
	N          int     `json:"n"`
}

// Check is one target and whether a variant meets it.
type Check struct {
	Name   string
	Pass   bool
	Detail string
	// Slack is the room the variant has on the target, relative to it: positive when it passes, and
	// the miss, negated, when it does not.
	Slack float64
}

// Miss is how far the variant is from the target, relative to it, and zero when it passes.
func (c Check) Miss() float64 { return math.Max(0, -c.Slack) }

// Verdict is how a variant fares against the targets.
type Verdict struct {
	Name         string
	Note         string
	Stats        map[string]Stats
	CounterSpeed float64
	MarketSpeed  float64
	// BoostersLowerBound is true when most sets never finished with boosters alone, so that the
	// speeds are lower bounds: the real boosters-only median is above the horizon.
	BoostersLowerBound bool
	Checks             []Check
	Pass               bool
	// Distance is the sum of the misses, the measure a variant that fails is ranked by.
	Distance float64
	// Slack is the least slack of the speed checks: how much room the variant has on its tightest
	// speed target. A variant that passes is ranked by it, since one at the edge of a band may leave
	// it on another seed.
	Slack float64
}

// StatsOf summarises runs, whose unfinished sets carry horizon+1 days.
func StatsOf(runs []SetRun, horizon int) Stats {
	days := make([]float64, len(runs))
	unfinished := 0
	for i, r := range runs {
		days[i] = r.Days
		if r.Days > float64(horizon) {
			unfinished++
		}
	}
	slices.Sort(days)
	if len(days) == 0 {
		return Stats{}
	}
	mid := len(days) / 2
	median := days[mid]
	if len(days)%2 == 0 {
		median = (days[mid-1] + days[mid]) / 2
	}
	return Stats{
		Median:     median,
		P10:        days[int(p10*float64(len(days)-1))],
		Min:        days[0],
		Unfinished: float64(unfinished) / float64(len(days)),
		N:          len(days),
	}
}

// Analyze judges each outcome against targets and ranks them: the variants that meet every target
// first, the one with the most room on its tightest target at the top, then the others by their
// distance from the targets, closest first. horizon is the run length, in days.
func Analyze(outcomes []Outcome, targets Targets, horizon int) []Verdict {
	verdicts := make([]Verdict, len(outcomes))
	for i, o := range outcomes {
		verdicts[i] = judge(o, targets, horizon)
	}
	slices.SortStableFunc(verdicts, rank)
	return verdicts
}

func rank(a, b Verdict) int {
	switch {
	case a.Pass && !b.Pass:
		return -1
	case !a.Pass && b.Pass:
		return 1
	case a.Pass:
		return cmp.Or(cmp.Compare(b.Slack, a.Slack), cmp.Compare(a.Name, b.Name))
	}
	return cmp.Or(cmp.Compare(a.Distance, b.Distance), cmp.Compare(a.Name, b.Name))
}

func judge(o Outcome, t Targets, horizon int) Verdict {
	stats := map[string]Stats{}
	for player, runs := range o.Runs {
		stats[player] = StatsOf(runs, horizon)
	}
	boosters := stats[BoostersOnly]
	v := Verdict{
		Name:               o.Name,
		Note:               o.Note,
		Stats:              stats,
		CounterSpeed:       boosters.Median / stats[Counter].Median,
		MarketSpeed:        boosters.Median / stats[Market].Median,
		BoostersLowerBound: boosters.Unfinished > 0.5,
	}
	v.Checks = []Check{
		inBand(CheckCounterSpeed, v.CounterSpeed, t.CounterSpeed),
		atLeast(CheckMarketSpeed, v.MarketSpeed, t.MarketSpeedMin),
		atLeast(CheckMarketOverCounter, v.MarketSpeed/v.CounterSpeed, t.MarketOverCounterMin),
		atLeast(CheckFloor, stats[Market].Min, t.FloorDays),
	}
	v.Pass = true
	v.Slack = math.Inf(1)
	for _, c := range v.Checks {
		v.Distance += c.Miss()
		v.Pass = v.Pass && c.Pass
		// The fastest set is a whole number of days that the pace of the energy fixes, so it sits
		// on its floor in almost every variant: it is pass or fail, and leaves the room to the speeds.
		if c.Name != CheckFloor {
			v.Slack = math.Min(v.Slack, c.Slack)
		}
	}
	return v
}

func inBand(name string, value float64, b Band) Check {
	slack := math.Min((value-b.Min)/b.Min, (b.Max-value)/b.Max)
	return Check{Name: name, Pass: value >= b.Min && value <= b.Max, Detail: "speed " + format(value), Slack: slack}
}

func atLeast(name string, value, floor float64) Check {
	return Check{Name: name, Pass: value >= floor, Detail: format(value), Slack: (value - floor) / floor}
}

// format shows a speed or a number of days with two decimals.
func format(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
