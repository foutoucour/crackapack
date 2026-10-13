package main

import (
	"strconv"
	"strings"
	"testing"
)

// maxBoosterWait runs simulate on the shared catalogue directory with the given sets and returns
// the max_booster_wait_minutes line of its summary.
func maxBoosterWait(t *testing.T, sets ...string) float64 {
	t.Helper()
	dir := catalogueDirFS(t)
	var stdout strings.Builder
	a, _ := buildTestArgs(t, nil, nil, &stdout)
	a.catFS = dir
	a.pricesFS = dir
	a.setCodes = sets

	if err := runSimulate(a); err != nil {
		t.Fatalf("--sets %v: %v", sets, err)
	}

	const metric = "max_booster_wait_minutes"
	for _, line := range strings.Split(stdout.String(), "\n") {
		if value, ok := strings.CutPrefix(line, metric+" "); ok {
			wait, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil {
				t.Fatalf("parse %q: %v", line, err)
			}
			return wait
		}
	}
	t.Fatalf("%s missing from the summary:\n%s", metric, stdout.String())
	return 0
}

// CLS is the priciest set of the directory, so its booster costs the same energy whether it is
// opened alone or next to RAV, and that cost is above tier x1.
func TestSimulate_boosterWaitDoesNotDependOnTheSetsRun(t *testing.T) {
	t.Parallel()

	alone := maxBoosterWait(t, "CLS")
	together := maxBoosterWait(t, "CLS", "RAV")

	if alone != together {
		t.Errorf("max_booster_wait_minutes: CLS alone = %v, CLS with RAV = %v, want equal", alone, together)
	}
	const tierOneWait = 50 // 5 reserve ticks at 10 minutes each: a cost of 5, the x1 tier
	if alone <= tierOneWait {
		t.Errorf("max_booster_wait_minutes = %v for the priciest set, want above the x1 tier's %v", alone, tierOneWait)
	}
}
