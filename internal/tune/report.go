package tune

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"text/tabwriter"

	yaml "go.yaml.in/yaml/v3"
)

// windowFraction is the share of the survivors a parameter window is read over when no candidate
// meets every band.
const windowFraction = 0.25

// maxFailingBands is how many of the best candidate's failing bands the text report lists.
const maxFailingBands = 10

// Window is the range a parameter takes among the candidates that do best.
type Window struct {
	Path string  `json:"path"`
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
}

// Influence is how strongly a parameter moves the loss: Rho is the Spearman rank correlation
// between the parameter's value and the screening loss over the valid candidates.
type Influence struct {
	Path string  `json:"path"`
	Rho  float64 `json:"rho"`
}

// analyse fills the windows and the influence of the parameters. survivors are the indexes of the
// candidates that were scored on the search seeds.
func (r *Report) analyse(survivors []int) {
	var pool, feasible []int
	for _, i := range r.Ranked {
		if *r.Candidates[i].Loss == 0 {
			feasible = append(feasible, i)
		}
	}
	r.Feasible = len(feasible) > 0
	if r.Feasible {
		pool = feasible
	} else {
		pool = r.Ranked[:max(1, int(math.Ceil(windowFraction*float64(len(survivors)))))]
	}

	r.Windows = make([]Window, len(r.Params))
	for p, param := range r.Params {
		w := Window{Path: param.Path, Min: math.Inf(1), Max: math.Inf(-1)}
		for _, i := range pool {
			v := r.Candidates[i].Overrides[p].Value
			w.Min, w.Max = math.Min(w.Min, v), math.Max(w.Max, v)
		}
		r.Windows[p] = w
	}

	var valid []Candidate
	for _, c := range r.Candidates {
		if c.Invalid == "" {
			valid = append(valid, c)
		}
	}
	losses := make([]float64, len(valid))
	for i, c := range valid {
		losses[i] = c.ScreenLoss
	}
	r.Influence = make([]Influence, len(r.Params))
	for p, param := range r.Params {
		values := make([]float64, len(valid))
		for i, c := range valid {
			values[i] = c.Overrides[p].Value
		}
		r.Influence[p] = Influence{Path: param.Path, Rho: spearman(values, losses)}
	}
	slices.SortStableFunc(r.Influence, func(a, b Influence) int { return cmp.Compare(math.Abs(b.Rho), math.Abs(a.Rho)) })
}

// spearman is the rank correlation of x and y, 0 when either has no spread or there are fewer
// than three points.
func spearman(x, y []float64) float64 {
	if len(x) < 3 {
		return 0
	}
	rx, ry := ranks(x), ranks(y)
	mx, my := average(rx), average(ry)
	var cov, vx, vy float64
	for i := range rx {
		cov += (rx[i] - mx) * (ry[i] - my)
		vx += (rx[i] - mx) * (rx[i] - mx)
		vy += (ry[i] - my) * (ry[i] - my)
	}
	if vx == 0 || vy == 0 {
		return 0
	}
	return cov / math.Sqrt(vx*vy)
}

// ranks returns the 1-based rank of each value, tied values sharing the average of their ranks.
func ranks(values []float64) []float64 {
	order := make([]int, len(values))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(values[a], values[b]) })
	out := make([]float64, len(values))
	for start := 0; start < len(order); {
		end := start
		for end+1 < len(order) && values[order[end+1]] == values[order[start]] {
			end++
		}
		rank := float64(start+end)/2 + 1
		for k := start; k <= end; k++ {
			out[order[k]] = rank
		}
		start = end + 1
	}
	return out
}

func average(values []float64) float64 {
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total / float64(len(values))
}

// WriteCandidatesJSON writes the whole report, every candidate included, as indented JSON.
func WriteCandidatesJSON(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return fmt.Errorf("write candidates: %w", err)
	}
	return nil
}

// WriteBestYAML writes the best config as a full config file that simulate and sweep accept.
func WriteBestYAML(w io.Writer, r Report) error {
	data, err := yaml.Marshal(r.BestConfig)
	if err != nil {
		return fmt.Errorf("write best config: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write best config: %w", err)
	}
	return nil
}

// WriteText writes the report a person reads: the best candidates with their validation gap, the
// bands the best one still misses, the window of each parameter and what moves the loss.
func WriteText(w io.Writer, r Report) error {
	var b strings.Builder
	survivors := len(r.Ranked)
	fmt.Fprintf(&b, "%d candidates drawn, %d invalid, %d scored on %d seeds, %d validated on %d fresh seeds\n",
		len(r.Candidates), r.Invalid, survivors, len(r.SearchSeeds), min(r.Options.Top, survivors), len(r.ValidationSeeds))
	fmt.Fprintf(&b, "base config loss: %.4f\n\n", *r.Baseline.Loss)

	fmt.Fprintln(&b, "Best candidates")
	tw := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "rank\tid\tloss\tvalidation\tgap")
	for rank, i := range r.Ranked[:min(r.Options.Top, survivors)] {
		c := r.Candidates[i]
		fmt.Fprintf(tw, "%d\t%d\t%.4f\t%.4f\t%+.4f\n", rank+1, c.ID, *c.Loss, *c.ValidationLoss, *c.ValidationLoss-*c.Loss)
	}
	_ = tw.Flush() // a strings.Builder does not fail

	writeFailingBands(&b, r.Best())
	writeWindows(&b, r)
	writeInfluence(&b, r)
	_, err := io.WriteString(w, b.String())
	return err
}

func writeFailingBands(b *strings.Builder, best Candidate) {
	type band struct {
		name string
		loss float64
	}
	var failing []band
	for name, loss := range best.Goals {
		if loss > 0 {
			failing = append(failing, band{name, loss})
		}
	}
	slices.SortFunc(failing, func(x, y band) int {
		return cmp.Or(cmp.Compare(y.loss, x.loss), cmp.Compare(x.name, y.name))
	})
	if len(failing) == 0 {
		fmt.Fprintln(b, "\nThe best candidate meets every band.")
		return
	}
	fmt.Fprintf(b, "\nBands the best candidate still misses (%d, worst first)\n", len(failing))
	tw := tabwriter.NewWriter(b, 0, 2, 2, ' ', 0)
	for _, f := range failing[:min(maxFailingBands, len(failing))] {
		fmt.Fprintf(tw, "%s\t%.4f\n", f.name, f.loss)
	}
	_ = tw.Flush() // a strings.Builder does not fail
}

func writeWindows(b *strings.Builder, r Report) {
	if r.Feasible {
		fmt.Fprintln(b, "\nFeasible window of each parameter (candidates with loss 0)")
	} else {
		fmt.Fprintln(b, "\nNo candidate met every band. Window of each parameter over the best quarter of the scored candidates")
	}
	tw := tabwriter.NewWriter(b, 0, 2, 2, ' ', 0)
	for _, win := range r.Windows {
		fmt.Fprintf(tw, "%s\t%g\t%g\n", win.Path, win.Min, win.Max)
	}
	_ = tw.Flush() // a strings.Builder does not fail
}

func writeInfluence(b *strings.Builder, r Report) {
	fmt.Fprintln(b, "\nWhat moves the loss (Spearman correlation of each parameter with the screening loss)")
	tw := tabwriter.NewWriter(b, 0, 2, 2, ' ', 0)
	for _, inf := range r.Influence {
		fmt.Fprintf(tw, "%s\t%+.2f\n", inf.Path, inf.Rho)
	}
	_ = tw.Flush() // a strings.Builder does not fail
}
