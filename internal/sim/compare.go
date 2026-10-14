package sim

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// ReadResult decodes a Result from JSON produced by WriteJSON.
// Unknown fields in the JSON are rejected; errors name the problem.
func ReadResult(r io.Reader) (Result, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var result Result
	if err := dec.Decode(&result); err != nil {
		return Result{}, fmt.Errorf("decode result: %w", err)
	}
	return result, nil
}

// MetricDiff is the diff of one Summary metric between two results.
// APresent and BPresent indicate which sides contained the metric.
// Delta is B − A when both sides are present, and 0 otherwise.
type MetricDiff struct {
	APresent bool
	A        float64
	BPresent bool
	B        float64
	// Delta is B − A. It is 0 when either side is absent.
	Delta float64
}

// ColDelta is one column's (B − A) delta between two results for one day.
// Name matches the CSV column name emitted by WriteCSV (same stable order).
type ColDelta struct {
	Name  string
	Delta int64
}

// DayComparison is the per-day comparison between two results.
// Deltas lists every metric from the day CSV in the same stable column order
// as WriteCSV, so WriteComparisonCSV automatically covers every metric that
// WriteCSV emits.
type DayComparison struct {
	Day    int
	Deltas []ColDelta
}

// Comparison is the output of Compare: a Summary diff and per-day column deltas.
type Comparison struct {
	// Summary contains one MetricDiff per key in the union of both Summaries.
	// Map keys are sorted alphabetically by the caller of WriteComparisonSummary.
	Summary map[string]MetricDiff
	// Days lists per-day deltas in day order.
	Days []DayComparison
}

// Compare returns a Comparison of a and b.
//
// It returns an error when:
//   - the results have different day counts
//   - the results have different set codes (derived from day 0's SetProgress)
//   - only one of the results has an NPC crowd (derived from day 0's NPC metrics)
//   - only one of the results has a Counter supply (derived from day 0's Counter metrics)
//   - only one of the results has trades (derived from day 0's trade metrics)
//   - a per-day Day number differs at any index (index and both numbers are named)
func Compare(a, b Result) (Comparison, error) {
	// 1. Day count.
	if len(a.Days) != len(b.Days) {
		return Comparison{}, fmt.Errorf("results have different day counts: %d vs %d",
			len(a.Days), len(b.Days))
	}

	// 2. Set codes from day 0 (when non-empty).
	if len(a.Days) > 0 {
		aCodes := progressSetCodes(a.Days[0].SetProgress)
		bCodes := progressSetCodes(b.Days[0].SetProgress)
		if !equalStrings(aCodes, bCodes) {
			return Comparison{}, fmt.Errorf("results have different set codes: %v vs %v",
				aCodes, bCodes)
		}
	}

	// 2b. Both runs have the same optional columns: they would not line up otherwise.
	withA, withB := extrasOf(a.Days), extrasOf(b.Days)
	if withA.npc != withB.npc {
		return Comparison{}, fmt.Errorf("results differ in NPC crowd: only one of them has NPC metrics")
	}
	if withA.counter != withB.counter {
		return Comparison{}, fmt.Errorf("results differ in counter supply: only one of them has Counter metrics")
	}
	if withA.trades != withB.trades {
		return Comparison{}, fmt.Errorf("results differ in trades: only one of them has trade metrics")
	}

	// 3. Per-day Day number alignment.
	for i := range a.Days {
		if a.Days[i].Day != b.Days[i].Day {
			return Comparison{}, fmt.Errorf(
				"day number mismatch at index %d: %d vs %d",
				i, a.Days[i].Day, b.Days[i].Day,
			)
		}
	}

	// 4. Summary diff.
	summaryDiff := computeSummaryDiff(a.Summary, b.Summary)

	// 5. Per-day deltas via dayColumns (the shared column driver).
	var dayCmps []DayComparison
	if len(a.Days) > 0 {
		setCodes := progressSetCodes(a.Days[0].SetProgress)
		with := extrasOf(a.Days)
		dayCmps = make([]DayComparison, len(a.Days))
		for i := range a.Days {
			dayCmps[i] = computeDayComparison(a.Days[i], b.Days[i], setCodes, with)
		}
	}

	return Comparison{Summary: summaryDiff, Days: dayCmps}, nil
}

// WriteComparisonSummary writes a fixed-width text table of the Summary diff to w.
// Rows are sorted by metric name. Absent sides are shown as "-"; the Delta column
// shows "-" when either side is absent, and a signed value when both are present.
func WriteComparisonSummary(w io.Writer, c Comparison) error {
	keys := make([]string, 0, len(c.Summary))
	for k := range c.Summary {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Compute dynamic width for the metric name column.
	const minNameW = len("metric")
	nameW := minNameW
	for _, k := range keys {
		if len(k) > nameW {
			nameW = len(k)
		}
	}
	const colW = 12

	var sb strings.Builder

	// Header + separator.
	header := fmt.Sprintf("%-*s  %*s  %*s  %*s\n",
		nameW, "metric",
		colW, "a",
		colW, "b",
		colW, "delta",
	)
	sb.WriteString(header)
	sb.WriteString(strings.Repeat("-", len(header)-1) + "\n")

	for _, k := range keys {
		diff := c.Summary[k]

		aStr := "-"
		if diff.APresent {
			aStr = formatSummaryFloat(diff.A)
		}
		bStr := "-"
		if diff.BPresent {
			bStr = formatSummaryFloat(diff.B)
		}
		dStr := "-"
		if diff.APresent && diff.BPresent {
			if diff.Delta > 0 {
				dStr = "+" + formatSummaryFloat(diff.Delta)
			} else {
				dStr = formatSummaryFloat(diff.Delta)
			}
		}

		fmt.Fprintf(&sb, "%-*s  %*s  %*s  %*s\n",
			nameW, k,
			colW, aStr,
			colW, bStr,
			colW, dStr,
		)
	}

	_, err := io.WriteString(w, sb.String())
	if err != nil {
		return fmt.Errorf("write comparison summary: %w", err)
	}
	return nil
}

// WriteComparisonCSV writes the per-day deltas as CSV to w.
// Column order: "day", then "delta_<name>" for each column from DayComparison.Deltas
// in the same stable order as WriteCSV. Delta files can be told apart from raw
// days.csv files by the "delta_" prefix on every column except "day".
func WriteComparisonCSV(w io.Writer, c Comparison) error {
	cw := csv.NewWriter(w)

	// Header.
	var header []string
	header = append(header, "day")
	if len(c.Days) > 0 {
		for _, cd := range c.Days[0].Deltas {
			header = append(header, "delta_"+cd.Name)
		}
	}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("write comparison csv header: %w", err)
	}

	for _, day := range c.Days {
		row := make([]string, 0, 1+len(day.Deltas))
		row = append(row, strconv.Itoa(day.Day))
		for _, cd := range day.Deltas {
			row = append(row, strconv.FormatInt(cd.Delta, 10))
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("write comparison csv row day %d: %w", day.Day, err)
		}
	}

	cw.Flush()
	return cw.Error()
}

// progressSetCodes returns a sorted slice of set codes from a SetProgress map.
func progressSetCodes(sp map[string]SetDayProgress) []string {
	codes := make([]string, 0, len(sp))
	for code := range sp {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

// equalStrings reports whether a and b have the same length and elements.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// computeSummaryDiff returns a MetricDiff for every key in the union of a and b.
func computeSummaryDiff(a, b Summary) map[string]MetricDiff {
	all := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		all[k] = struct{}{}
	}
	for k := range b {
		all[k] = struct{}{}
	}

	result := make(map[string]MetricDiff, len(all))
	for k := range all {
		aVal, aOk := a[k]
		bVal, bOk := b[k]
		diff := MetricDiff{
			APresent: aOk,
			A:        aVal,
			BPresent: bOk,
			B:        bVal,
		}
		if aOk && bOk {
			diff.Delta = bVal - aVal
		}
		result[k] = diff
	}
	return result
}

// computeDayComparison produces the per-column deltas for one pair of days.
// setCodes must be sorted ascending, and with must match both days (same contract as dayColumns).
func computeDayComparison(a, b DayMetrics, setCodes []string, with extras) DayComparison {
	aCols := dayColumns(a, setCodes, with)
	bCols := dayColumns(b, setCodes, with)

	deltas := make([]ColDelta, len(aCols))
	for i, ac := range aCols {
		deltas[i] = ColDelta{
			Name:  ac.name,
			Delta: bCols[i].value - ac.value,
		}
	}
	return DayComparison{Day: a.Day, Deltas: deltas}
}

// formatSummaryFloat formats a float64 as an integer string when it is a whole
// number, otherwise as a minimal floating-point string.
func formatSummaryFloat(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}
