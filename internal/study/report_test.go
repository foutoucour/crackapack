package study_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/study"
)

func reportOf(horizon int, outcomes ...study.Outcome) study.Report {
	return study.Report{
		Scenario: "pace-3-visits", SessionsPerDay: 3, Horizon: horizon, Seeds: []uint64{42},
		Targets:  userTargets(),
		Verdicts: study.Analyze(outcomes, userTargets(), horizon),
		Outcomes: outcomes,
	}
}

func render(t *testing.T, r study.Report) string {
	t.Helper()
	var b bytes.Buffer
	if err := study.WriteReport(&b, r); err != nil {
		t.Fatalf("WriteReport: %v", err)
	}
	return b.String()
}

func TestWriteReport_namesEveryVariantItsOverridesAndTheVerdict(t *testing.T) {
	t.Parallel()
	on := outcome("on-target", runsOf(60, 60, 60), runsOf(45, 45, 45), runsOf(6, 25, 25))
	on.Overrides = []config.Override{{Path: "npcs.population", Value: 24}}
	off := outcome("too-generous", runsOf(60, 60, 60), runsOf(15, 15, 15), runsOf(10, 10, 10))

	got := render(t, reportOf(120, on, off))

	for _, want := range []string{"on-target", "too-generous", "npcs.population", "24", "meets every target", "misses"} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q", want)
		}
	}
}

func TestWriteReport_headlineSaysWhatIsClosestWhenNothingMeetsTheTargets(t *testing.T) {
	t.Parallel()
	off := outcome("too-generous", runsOf(60, 60, 60), runsOf(15, 15, 15), runsOf(10, 10, 10))

	got := render(t, reportOf(120, off))

	if !strings.Contains(got, "No variant meets every target") || !strings.Contains(got, "counter speed") {
		t.Errorf("headline does not say that nothing passes and which target is missed:\n%s", got)
	}
}

func TestWriteReport_headlineNamesTheMostRobustVariantThatPasses(t *testing.T) {
	t.Parallel()
	edge := outcome("a-edge", runsOf(60, 60, 60), runsOf(40, 40, 40), runsOf(10, 10, 10))
	roomy := outcome("b-roomy", runsOf(60, 60, 60), runsOf(45, 45, 45), runsOf(10, 10, 10))

	got := render(t, reportOf(120, edge, roomy))

	// The room of b-roomy is on its Counter speed, 1.33 in a band of 1.2 to 1.5, which is 11%.
	if !strings.Contains(got, "2 of 2 variants meet every target. The most robust is b-roomy") || !strings.Contains(got, "11%") {
		t.Errorf("headline does not name b-roomy as the most robust, with its room:\n%s", got)
	}
}

func TestWriteReport_escapesWhatTheVariantsFileSays(t *testing.T) {
	t.Parallel()
	o := outcome(`<script>alert(1)</script>`, runsOf(60, 60, 60), runsOf(45, 45, 45), runsOf(25, 25, 25))
	o.Note = `<img src=x onerror=alert(1)>`

	got := render(t, reportOf(120, o))

	if strings.Contains(got, "<script>alert(1)") || strings.Contains(got, "<img src=x") {
		t.Error("report carries the markup of a variant's name or note unescaped")
	}
}

func TestWriteReport_flagsSpeedsThatAreLowerBounds(t *testing.T) {
	t.Parallel()
	o := outcome("slow", runsOf(101, 101, 50), runsOf(40, 40, 40), runsOf(20, 20, 20))

	got := render(t, reportOf(100, o))

	if !strings.Contains(got, "lower bound") {
		t.Error("report does not warn that boosters-only never finished most sets")
	}
}

func TestWriteReport_showsHowFarEachVariantMovedFromTheFirst(t *testing.T) {
	t.Parallel()
	base := outcome("baseline", runsOf(60, 60, 60), runsOf(15, 15, 15), runsOf(10, 10, 10))
	moved := outcome("fewer-boosters", runsOf(60, 60, 60), runsOf(30, 30, 30), runsOf(10, 10, 10))

	got := render(t, reportOf(120, base, moved))

	// The Counter's speed goes from 4.00 to 2.00 between the two.
	if !strings.Contains(got, "−2.00") {
		t.Errorf("report does not show the change of the Counter's speed against the baseline:\n%s", got)
	}
}
