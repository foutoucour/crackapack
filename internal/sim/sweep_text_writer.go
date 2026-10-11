package sim

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/foutoucour/crackapack/internal/config"
)

// headlineMetrics is the fixed set of per-rate summary metrics always shown in
// the table, unless the metric is already displayed as a target column.
func headlineMetrics() []string {
	return []string{
		"final_balance",
		"sub_coin_cards",
		"daily_reward_commons",
		"unsold_duplicate_copies",
	}
}

// WriteSweepText writes a human-readable sweep report to w.
//
// The table has one row per rate. Columns are:
//  1. rate (right-aligned)
//  2. one column per target: "value PASS" or "value FAIL"
//  3. headline metrics not already in a target column
//
// After the table a passing-rate summary line is written:
//   - "Passing rates: none" when no rate passes.
//   - "Passing rates: 250, 500 (range 250–500, contiguous)" or "… (gap)" when some pass.
//   - "no targets configured" when Sweep was called with an empty targets slice.
func WriteSweepText(w io.Writer, sr SweepResult, targets []config.CalibrationTarget) error {
	if sr.NoTargets {
		return writeNoTargetsReport(w, sr)
	}

	// Build column definitions.
	type column struct {
		header string
		value  func(rr RateResult) string
	}

	// Determine which headline metrics are not already target columns.
	targetMetrics := make(map[string]bool, len(targets))
	for _, t := range targets {
		targetMetrics[t.Metric] = true
	}

	cols := make([]column, 0, len(targets)+len(headlineMetrics()))

	// Target columns.
	for i, t := range targets {
		i := i
		header := t.Metric + " (" + boundLabel(t) + ")"
		cols = append(cols, column{
			header: header,
			value: func(rr RateResult) string {
				if i >= len(rr.Outcomes) {
					return "?"
				}
				o := rr.Outcomes[i]
				verdict := "FAIL"
				if o.Pass {
					verdict = "PASS"
				}
				return formatFloat(o.Value) + " " + verdict
			},
		})
	}

	// Headline columns (skip metrics already in target columns).
	for _, m := range headlineMetrics() {
		if targetMetrics[m] {
			continue
		}
		m := m
		cols = append(cols, column{
			header: m,
			value: func(rr RateResult) string {
				v, ok := rr.Result.Summary[m]
				if !ok {
					return "-"
				}
				return formatFloat(v)
			},
		})
	}

	// Compute column widths (max of header and each data cell, minimum 4).
	rows := make([][]string, len(sr.Rates))
	for i, rr := range sr.Rates {
		row := make([]string, len(cols))
		for j, col := range cols {
			row[j] = col.value(rr)
		}
		rows[i] = row
	}

	// Use rune count for widths: headers may contain multi-byte runes (≤, ≥, –).
	// Data cells are ASCII so rune count == byte count there, but the same
	// measurement keeps the logic consistent.
	widths := make([]int, len(cols))
	for j, col := range cols {
		w := utf8.RuneCountInString(col.header)
		if w < 4 {
			w = 4
		}
		widths[j] = w
	}
	rateWidth := 4
	for _, rr := range sr.Rates {
		if n := len(fmt.Sprintf("%d", rr.Rate)); n > rateWidth {
			rateWidth = n
		}
	}
	for i := range sr.Rates {
		for j := range cols {
			if n := utf8.RuneCountInString(rows[i][j]); n > widths[j] {
				widths[j] = n
			}
		}
	}

	var sb strings.Builder

	sb.WriteString(padLeft("rate", rateWidth))
	for j, col := range cols {
		sb.WriteString(" | ")
		sb.WriteString(padLeft(col.header, widths[j]))
	}
	sb.WriteByte('\n')

	sb.WriteString(strings.Repeat("-", rateWidth))
	for j := range cols {
		sb.WriteString("-+-")
		sb.WriteString(strings.Repeat("-", widths[j]))
	}
	sb.WriteByte('\n')

	for i, rr := range sr.Rates {
		sb.WriteString(padLeft(fmt.Sprintf("%d", rr.Rate), rateWidth))
		for j := range cols {
			sb.WriteString(" | ")
			sb.WriteString(padLeft(rows[i][j], widths[j]))
		}
		sb.WriteByte('\n')
	}

	sb.WriteByte('\n')
	sb.WriteString(passingRateLine(sr))
	sb.WriteByte('\n')

	_, err := io.WriteString(w, sb.String())
	return err
}

// writeNoTargetsReport writes the table with only headline metrics and the
// "no targets configured" notice (no target columns, no passing-rate line).
func writeNoTargetsReport(w io.Writer, sr SweepResult) error {
	headlines := headlineMetrics()

	rateWidth := 4
	for _, rr := range sr.Rates {
		if n := len(fmt.Sprintf("%d", rr.Rate)); n > rateWidth {
			rateWidth = n
		}
	}
	widths := make([]int, len(headlines))
	for j, h := range headlines {
		widths[j] = utf8.RuneCountInString(h)
		if widths[j] < 4 {
			widths[j] = 4
		}
	}
	rows := make([][]string, len(sr.Rates))
	for i, rr := range sr.Rates {
		row := make([]string, len(headlines))
		for j, m := range headlines {
			v, ok := rr.Result.Summary[m]
			if !ok {
				row[j] = "-"
			} else {
				row[j] = formatFloat(v)
			}
			if n := utf8.RuneCountInString(row[j]); n > widths[j] {
				widths[j] = n
			}
		}
		rows[i] = row
	}

	var sb strings.Builder

	sb.WriteString(padLeft("rate", rateWidth))
	for j, h := range headlines {
		sb.WriteString(" | ")
		sb.WriteString(padLeft(h, widths[j]))
	}
	sb.WriteByte('\n')

	sb.WriteString(strings.Repeat("-", rateWidth))
	for j := range headlines {
		sb.WriteString("-+-")
		sb.WriteString(strings.Repeat("-", widths[j]))
	}
	sb.WriteByte('\n')

	for i, rr := range sr.Rates {
		sb.WriteString(padLeft(fmt.Sprintf("%d", rr.Rate), rateWidth))
		for j := range headlines {
			sb.WriteString(" | ")
			sb.WriteString(padLeft(rows[i][j], widths[j]))
		}
		sb.WriteByte('\n')
	}

	sb.WriteByte('\n')
	sb.WriteString("no targets configured\n")

	_, err := io.WriteString(w, sb.String())
	return err
}

// passingRateLine formats the passing-rate summary line.
func passingRateLine(sr SweepResult) string {
	if len(sr.PassingRates) == 0 {
		return "Passing rates: none"
	}
	parts := make([]string, len(sr.PassingRates))
	for i, r := range sr.PassingRates {
		parts[i] = fmt.Sprintf("%d", r)
	}
	list := strings.Join(parts, ", ")

	rangeStr := fmt.Sprintf("range %d–%d", sr.RangeMin, sr.RangeMax)
	contiguityStr := "contiguous"
	if !sr.Contiguous {
		contiguityStr = "gap"
	}
	return fmt.Sprintf("Passing rates: %s (%s, %s)", list, rangeStr, contiguityStr)
}

// boundLabel returns the bound annotation for a target column header.
func boundLabel(t config.CalibrationTarget) string {
	switch t.Comparison {
	case config.ComparisonAtLeast:
		if t.Min != nil {
			return "≥" + formatFloat(*t.Min)
		}
	case config.ComparisonAtMost:
		if t.Max != nil {
			return "≤" + formatFloat(*t.Max)
		}
	case config.ComparisonBetween:
		if t.Min != nil && t.Max != nil {
			return formatFloat(*t.Min) + "–" + formatFloat(*t.Max)
		}
	}
	return string(t.Comparison)
}

// formatFloat formats a float64 as an integer when it is a whole number,
// otherwise with the shortest accurate representation ('g' format).
func formatFloat(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// padLeft returns s right-aligned in a field of w runes.
// Width is measured in runes so that multi-byte characters (≤, ≥, –) in
// header strings do not cause misaligned table columns.
func padLeft(s string, w int) string {
	n := utf8.RuneCountInString(s)
	if n >= w {
		return s
	}
	return strings.Repeat(" ", w-n) + s
}
