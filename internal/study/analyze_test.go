package study_test

import (
	"math"
	"testing"

	"github.com/foutoucour/crackapack/internal/study"
)

// runsOf is one run per day in days, on sets named s0, s1, and so on.
func runsOf(days ...float64) []study.SetRun {
	runs := make([]study.SetRun, len(days))
	for i, d := range days {
		runs[i] = study.SetRun{Set: string(rune('a' + i)), Days: d}
	}
	return runs
}

func outcome(name string, boosters, counter, market []study.SetRun) study.Outcome {
	return study.Outcome{Name: name, Runs: map[string][]study.SetRun{
		study.BoostersOnly: boosters, study.Counter: counter, study.Market: market,
	}}
}

// userTargets are the speeds the maintainers asked for: the Counter 1.33 times faster than
// boosters alone, the market at least twice, and no set done in under five days.
func userTargets() study.Targets {
	return study.Targets{
		CounterSpeed:         study.Band{Min: 1.2, Max: 1.5},
		MarketSpeedMin:       2,
		MarketOverCounterMin: 1.5,
		FloorDays:            5,
	}
}

func TestAnalyze_speedsAreTheMedianDaysOfBoostersOverThePlayer(t *testing.T) {
	t.Parallel()
	o := outcome("v", runsOf(40, 60, 80), runsOf(30, 45, 60), runsOf(20, 30, 40))

	got := study.Analyze([]study.Outcome{o}, userTargets(), 120)[0]

	if math.Abs(got.CounterSpeed-60.0/45.0) > 1e-9 {
		t.Errorf("CounterSpeed = %v, want 60/45", got.CounterSpeed)
	}
	if math.Abs(got.MarketSpeed-2.0) > 1e-9 {
		t.Errorf("MarketSpeed = %v, want 60/30", got.MarketSpeed)
	}
}

func TestAnalyze_aVariantOnTargetPassesEveryCheck(t *testing.T) {
	t.Parallel()
	// Speeds 1.33 and 2.4, and the fastest set takes 6 days.
	o := outcome("on-target", runsOf(60, 60, 60), runsOf(45, 45, 45), runsOf(6, 25, 25))

	got := study.Analyze([]study.Outcome{o}, userTargets(), 120)[0]

	if !got.Pass {
		t.Errorf("Pass = false, checks: %+v", got.Checks)
	}
	if got.Distance != 0 {
		t.Errorf("Distance = %v, want 0 on target", got.Distance)
	}
}

func TestAnalyze_eachTargetFailsOnItsOwn(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		o    study.Outcome
		fail string
	}{
		{"counter too strong", outcome("a", runsOf(60, 60, 60), runsOf(15, 15, 15), runsOf(10, 10, 10)), study.CheckCounterSpeed},
		{"counter too weak", outcome("b", runsOf(60, 60, 60), runsOf(58, 58, 58), runsOf(25, 25, 25)), study.CheckCounterSpeed},
		// Speeds 1.2 and 1.9: under the 2 asked, yet 1.58 times the Counter's.
		{"market too slow", outcome("c", runsOf(60, 60, 60), runsOf(50, 50, 50), runsOf(31.5, 31.5, 31.5)), study.CheckMarketSpeed},
		// Speeds 1.5 and 2: both met, yet only 1.33 times the Counter's.
		{"market barely beats the counter", outcome("d", runsOf(60, 60, 60), runsOf(40, 40, 40), runsOf(30, 30, 30)), study.CheckMarketOverCounter},
		{"a set done too fast", outcome("e", runsOf(60, 60, 60), runsOf(45, 45, 45), runsOf(3, 25, 25)), study.CheckFloor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := study.Analyze([]study.Outcome{tt.o}, userTargets(), 120)[0]

			if got.Pass {
				t.Fatalf("Pass = true, want %s to fail", tt.fail)
			}
			failed := ""
			for _, c := range got.Checks {
				if !c.Pass {
					failed += c.Name + ";"
				}
			}
			if failed != tt.fail+";" {
				t.Errorf("failed checks = %q, want only %q", failed, tt.fail)
			}
		})
	}
}

func TestAnalyze_aMedianOfSetsThatNeverFinishedIsALowerBound(t *testing.T) {
	t.Parallel()
	// With a horizon of 100 days, 101 stands for a set not finished.
	o := outcome("slow", runsOf(101, 101, 50), runsOf(40, 40, 40), runsOf(20, 20, 20))

	got := study.Analyze([]study.Outcome{o}, userTargets(), 100)[0]

	if !got.BoostersLowerBound {
		t.Error("BoostersLowerBound = false, want true: most sets never finished")
	}
	if got.Stats[study.BoostersOnly].Unfinished != 2.0/3.0 {
		t.Errorf("Unfinished = %v, want 2/3", got.Stats[study.BoostersOnly].Unfinished)
	}
}

func TestAnalyze_ranksTheClosestVariantFirstAndThePassingOnesBeforeTheRest(t *testing.T) {
	t.Parallel()
	far := outcome("far", runsOf(60, 60, 60), runsOf(15, 15, 15), runsOf(10, 10, 10))
	near := outcome("near", runsOf(60, 60, 60), runsOf(55, 55, 55), runsOf(28, 28, 28))
	on := outcome("on", runsOf(60, 60, 60), runsOf(45, 45, 45), runsOf(6, 25, 25))

	got := study.Analyze([]study.Outcome{far, near, on}, userTargets(), 120)

	order := []string{got[0].Name, got[1].Name, got[2].Name}
	want := []string{"on", "near", "far"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("ranking = %v, want %v", order, want)
		}
	}
}

func TestAnalyze_amongVariantsThatPassTheOneWithMoreRoomOnItsTightestCheckComesFirst(t *testing.T) {
	t.Parallel()
	// Both meet every target. The first has its Counter speed at 1.5, the edge of the band, and the
	// second at 1.33, with room on both sides.
	edge := outcome("a-edge", runsOf(60, 60, 60), runsOf(40, 40, 40), runsOf(10, 10, 10))
	roomy := outcome("b-roomy", runsOf(60, 60, 60), runsOf(45, 45, 45), runsOf(10, 10, 10))

	got := study.Analyze([]study.Outcome{edge, roomy}, userTargets(), 120)

	if !got[0].Pass || !got[1].Pass {
		t.Fatalf("both variants should pass: %+v / %+v", got[0].Checks, got[1].Checks)
	}
	if got[0].Name != "b-roomy" {
		t.Errorf("first = %s, want b-roomy: more room than a-edge on its tightest check (slack %v against %v)", got[0].Name, got[0].Slack, got[1].Slack)
	}
	if got[0].Slack <= got[1].Slack {
		t.Errorf("Slack = %v and %v, want the first to be larger", got[0].Slack, got[1].Slack)
	}
}

func TestAnalyze_theFastestSetIsPassOrFailAndLeavesTheRoomToTheSpeedChecks(t *testing.T) {
	t.Parallel()
	// The fastest set takes exactly the 5 days asked, which is no room on that check, and the
	// speeds have room: the variant must not be ranked as if it had none.
	o := outcome("on-the-floor", runsOf(60, 60, 60), runsOf(45, 45, 45), runsOf(5, 25, 25))

	got := study.Analyze([]study.Outcome{o}, userTargets(), 120)[0]

	if !got.Pass {
		t.Fatalf("Pass = false, checks: %+v", got.Checks)
	}
	if got.Slack < 0.1 {
		t.Errorf("Slack = %v, want the 11%% the Counter's speed has: the 5 days of the fastest set leave it out", got.Slack)
	}
}

func TestStatsOf_medianPercentileAndMinimumOfAnEvenNumberOfSets(t *testing.T) {
	t.Parallel()

	got := study.StatsOf(runsOf(10, 20, 30, 40), 120)

	if got.Median != 25 {
		t.Errorf("Median = %v, want 25", got.Median)
	}
	if got.Min != 10 {
		t.Errorf("Min = %v, want 10", got.Min)
	}
	if got.N != 4 {
		t.Errorf("N = %d, want 4", got.N)
	}
}
