package study

import (
	_ "embed" // the report template
	"fmt"
	"html/template"
	"io"
	"math"
	"strings"
)

//go:embed report.tmpl
var reportTemplate string //nolint:gochecknoglobals // go:embed needs a package variable

// Report is everything the HTML report shows: the way the study was run, the targets, and what each
// variant did.
type Report struct {
	// Scenario names the pace scenario the sets and the visit rate come from.
	Scenario       string
	SessionsPerDay int
	// Horizon is the days each run lasted, and Seeds the seeds of the runs, which the study pools.
	Horizon int
	Seeds   []uint64
	Targets Targets
	// Verdicts are the variants, best first.
	Verdicts []Verdict
	// Outcomes are the variants as they ran, in the order of the variants file. The first is the
	// baseline the others are compared with.
	Outcomes []Outcome
}

// WriteReport writes the report as one self-contained HTML page.
func WriteReport(w io.Writer, r Report) error {
	t, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return fmt.Errorf("report: template: %w", err)
	}
	if err := t.Execute(w, newPage(r)); err != nil {
		return fmt.Errorf("report: %w", err)
	}
	return nil
}

// The geometry of the charts, in pixels.
const (
	chartLabelWidth = 330.0
	chartPlotWidth  = 400.0
	chartRightPad   = 56.0
	chartTopPad     = 28.0
	chartBottomPad  = 8.0
	barThickness    = 12.0
	barGap          = 2.0
	barRowPad       = 16.0
	barRadius       = 4.0
	dotRowHeight    = 30.0
	dotRadius       = 5.0
	valueOffset     = 6.0
	tickTarget      = 5
	percent         = 100
)

// players lists the three players in the order the charts draw them, with the series slot each wears.
var players = [...]struct{ Name, Label, Slot string }{ //nolint:gochecknoglobals // read-only table
	{BoostersOnly, "Boosters only", "1"},
	{Counter, "Boosters and the Counter", "2"},
	{Market, "Boosters, Counter and market", "3"},
}

type page struct {
	Report
	// SeedText names the seeds, such as "seed 42" or "seeds 42, 43, 44".
	SeedText string
	Headline string
	// LowerBound is true when most sets never finished with boosters alone in some variant.
	LowerBound bool
	Baseline   string
	Players    []legendItem
	Rows       []row
	Bars       barChart
	Speeds     speedChart
}

type legendItem struct{ Label, Slot string }

type row struct {
	Rank                      int
	Name, Note                string
	Overrides                 []string
	Boosters, Counter, Market Stats
	CounterSpeed, MarketSpeed string
	CounterDelta, MarketDelta string
	Checks                    []Check
	Pass                      bool
	Room                      string
	Missed                    string
	LowerBound                bool
	Asks, Listings, Deals     int
	UnfinishedBoosters        int
	UnfinishedCounter         int
	UnfinishedMarket          int
	IsBaseline                bool
}

type tick struct {
	X     float64
	Label string
}

type bar struct {
	Path, Slot, Title, Value string
	ValueX, ValueY           float64
}

type barRow struct {
	Label  string
	LabelY float64
	Bars   []bar
}

type barChart struct {
	Width, Height, Baseline, GridTop, GridBottom float64
	Rows                                         []barRow
	Ticks                                        []tick
}

type dot struct {
	X, Y  float64
	Title string
}

type speedRow struct {
	Label           string
	LabelY, Y       float64
	Counter, Market dot
}

type speedChart struct {
	Width, Height, GridTop, GridBottom float64
	BandX, BandWidth, BandHeight, MinX float64
	Rows                               []speedRow
	Ticks                              []tick
}

func newPage(r Report) page {
	p := page{Report: r, SeedText: seedText(r.Seeds)}
	for _, pl := range players {
		p.Players = append(p.Players, legendItem{Label: pl.Label, Slot: pl.Slot})
	}
	byName := map[string]Outcome{}
	for _, o := range r.Outcomes {
		byName[o.Name] = o
	}
	var base *Verdict
	if len(r.Outcomes) > 0 {
		p.Baseline = r.Outcomes[0].Name
		for i := range r.Verdicts {
			if r.Verdicts[i].Name == p.Baseline {
				base = &r.Verdicts[i]
			}
		}
	}
	for i, v := range r.Verdicts {
		rw := rowOf(i+1, v, byName[v.Name], base, r.Horizon)
		p.LowerBound = p.LowerBound || v.BoostersLowerBound
		p.Rows = append(p.Rows, rw)
	}
	p.Headline = headline(r.Verdicts)
	p.Bars = newBarChart(r)
	p.Speeds = newSpeedChart(r)
	return p
}

func seedText(seeds []uint64) string {
	names := make([]string, len(seeds))
	for i, s := range seeds {
		names[i] = fmt.Sprint(s)
	}
	if len(seeds) == 1 {
		return "seed " + names[0]
	}
	return "seeds " + strings.Join(names, ", ")
}

func rowOf(rank int, v Verdict, o Outcome, base *Verdict, horizon int) row {
	rw := row{
		Rank: rank, Name: v.Name, Note: v.Note,
		Boosters: v.Stats[BoostersOnly], Counter: v.Stats[Counter], Market: v.Stats[Market],
		CounterSpeed: format(v.CounterSpeed), MarketSpeed: format(v.MarketSpeed),
		Checks: v.Checks, Pass: v.Pass, LowerBound: v.BoostersLowerBound, IsBaseline: base != nil && base.Name == v.Name,
		Room: fmt.Sprintf("%.0f%%", v.Slack*percent),
	}
	for _, ov := range o.Overrides {
		rw.Overrides = append(rw.Overrides, fmt.Sprintf("%s = %g", ov.Path, ov.Value))
	}
	if base != nil && !rw.IsBaseline {
		rw.CounterDelta = signed(v.CounterSpeed - base.CounterSpeed)
		rw.MarketDelta = signed(v.MarketSpeed - base.MarketSpeed)
	}
	var missed []string
	for _, c := range v.Checks {
		if !c.Pass {
			missed = append(missed, c.Name)
		}
	}
	rw.Missed = strings.Join(missed, ", ")
	for _, r := range o.Runs[Market] {
		rw.Asks += int(r.Asks)
		rw.Listings += int(r.Listings)
		rw.Deals += int(r.Deals)
	}
	rw.UnfinishedBoosters = unfinishedSets(o.Runs[BoostersOnly], horizon)
	rw.UnfinishedCounter = unfinishedSets(o.Runs[Counter], horizon)
	rw.UnfinishedMarket = unfinishedSets(o.Runs[Market], horizon)
	return rw
}

func unfinishedSets(runs []SetRun, horizon int) int {
	n := 0
	for _, r := range runs {
		if r.Days > float64(horizon) {
			n++
		}
	}
	return n
}

// signed formats a change with an explicit sign, using a true minus sign.
func signed(v float64) string {
	s := format(math.Abs(v))
	switch {
	case s == "0.00":
		return "0.00"
	case v < 0:
		return "−" + s
	}
	return "+" + s
}

// headline says in a sentence what the study found.
func headline(verdicts []Verdict) string {
	if len(verdicts) == 0 {
		return "No variant ran."
	}
	passing := 0
	for _, v := range verdicts {
		if v.Pass {
			passing++
		}
	}
	best := verdicts[0]
	if passing > 0 {
		return fmt.Sprintf("%d of %d variants meet every target. The most robust is %s: its tightest speed target has %.0f%% of room.",
			passing, len(verdicts), best.Name, best.Slack*percent)
	}
	var missed []string
	for _, c := range best.Checks {
		if !c.Pass {
			missed = append(missed, c.Name)
		}
	}
	return fmt.Sprintf("No variant meets every target. The closest is %s, which misses: %s.", best.Name, strings.Join(missed, ", "))
}

// barPath is a horizontal bar from x, y of width w and thickness h, square at its left end and
// rounded at its right end.
func barPath(x, y, w, h float64) string {
	r := math.Min(barRadius, w)
	return fmt.Sprintf("M%.1f,%.1f h%.1f a%.1f,%.1f 0 0 1 %.1f,%.1f v%.1f a%.1f,%.1f 0 0 1 -%.1f,%.1f h-%.1f z",
		x, y, w-r, r, r, r, r, h-2*r, r, r, r, r, w-r)
}

// niceStep returns a round step, 1, 2 or 5 times a power of ten, that puts about target ticks under
// max.
func niceStep(maxValue float64, target int) float64 {
	if maxValue <= 0 {
		return 1
	}
	raw := maxValue / float64(target)
	pow := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, m := range []float64{1, 2, 5, 10} {
		if m*pow >= raw {
			return m * pow
		}
	}
	return 10 * pow
}

func ticksTo(maxValue float64, xOf func(float64) float64, label func(float64) string) []tick {
	step := niceStep(maxValue, tickTarget)
	var ticks []tick
	for v := 0.0; v <= maxValue+1e-9; v += step {
		ticks = append(ticks, tick{X: xOf(v), Label: label(v)})
	}
	return ticks
}

func newBarChart(r Report) barChart {
	maxDays := 1.0
	for _, v := range r.Verdicts {
		for _, pl := range players {
			maxDays = math.Max(maxDays, v.Stats[pl.Name].Median)
		}
	}
	step := niceStep(maxDays, tickTarget)
	maxDays = math.Ceil(maxDays/step) * step
	scale := chartPlotWidth / maxDays
	xOf := func(days float64) float64 { return chartLabelWidth + days*scale }
	rowHeight := float64(len(players))*barThickness + float64(len(players)-1)*barGap + barRowPad

	c := barChart{
		Width:      chartLabelWidth + chartPlotWidth + chartRightPad,
		Height:     chartTopPad + float64(len(r.Verdicts))*rowHeight + chartBottomPad,
		Baseline:   chartLabelWidth,
		GridTop:    chartTopPad - 6,
		GridBottom: chartTopPad + float64(len(r.Verdicts))*rowHeight - barRowPad/2,
		Ticks:      ticksTo(maxDays, xOf, func(v float64) string { return fmt.Sprintf("%.0f", v) }),
	}
	for i, v := range r.Verdicts {
		top := chartTopPad + float64(i)*rowHeight
		br := barRow{Label: v.Name, LabelY: top + (rowHeight-barRowPad)/2 + 4}
		for k, pl := range players {
			median := v.Stats[pl.Name].Median
			y := top + float64(k)*(barThickness+barGap)
			value := fmt.Sprintf("%.0f", median)
			if median > float64(r.Horizon) {
				value = fmt.Sprintf(">%d", r.Horizon)
			}
			br.Bars = append(br.Bars, bar{
				Path: barPath(chartLabelWidth, y, median*scale, barThickness), Slot: pl.Slot,
				Title:  fmt.Sprintf("%s, %s: %s days (median over %d sets)", v.Name, pl.Label, value, v.Stats[pl.Name].N),
				Value:  value,
				ValueX: xOf(median) + valueOffset, ValueY: y + barThickness - 2,
			})
		}
		c.Rows = append(c.Rows, br)
	}
	return c
}

func newSpeedChart(r Report) speedChart {
	maxSpeed := math.Max(r.Targets.MarketSpeedMin*1.25, r.Targets.CounterSpeed.Max)
	for _, v := range r.Verdicts {
		maxSpeed = math.Max(maxSpeed, math.Max(v.CounterSpeed, v.MarketSpeed))
	}
	step := niceStep(maxSpeed, tickTarget)
	maxSpeed = math.Ceil(maxSpeed/step) * step
	scale := chartPlotWidth / maxSpeed
	xOf := func(speed float64) float64 { return chartLabelWidth + speed*scale }

	c := speedChart{
		Width:      chartLabelWidth + chartPlotWidth + chartRightPad,
		Height:     chartTopPad + float64(len(r.Verdicts))*dotRowHeight + chartBottomPad,
		GridTop:    chartTopPad - 6,
		GridBottom: chartTopPad + float64(len(r.Verdicts))*dotRowHeight,
		BandHeight: float64(len(r.Verdicts))*dotRowHeight + 6,
		BandX:      xOf(r.Targets.CounterSpeed.Min),
		BandWidth:  (r.Targets.CounterSpeed.Max - r.Targets.CounterSpeed.Min) * scale,
		MinX:       xOf(r.Targets.MarketSpeedMin),
		Ticks:      ticksTo(maxSpeed, xOf, func(v float64) string { return fmt.Sprintf("%.0f", v) }),
	}
	for i, v := range r.Verdicts {
		y := chartTopPad + float64(i)*dotRowHeight + dotRowHeight/2
		c.Rows = append(c.Rows, speedRow{
			Label: v.Name, LabelY: y + 4, Y: y,
			Counter: dot{X: xOf(v.CounterSpeed), Y: y, Title: fmt.Sprintf("%s, the Counter: speed %s", v.Name, format(v.CounterSpeed))},
			Market:  dot{X: xOf(v.MarketSpeed), Y: y, Title: fmt.Sprintf("%s, the market: speed %s", v.Name, format(v.MarketSpeed))},
		})
	}
	return c
}
