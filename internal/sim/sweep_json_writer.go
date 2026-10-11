package sim

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
)

// sweepJSONRate is the per-rate entry serialised to sweep.json.
// It carries the summary map and outcomes; the per-rate Days arrays are
// already written to rate-<N>/result.json and are not duplicated here.
type sweepJSONRate struct {
	Rate     domain.CoinRate    `json:"rate"`
	Pass     bool               `json:"pass"`
	Summary  Summary            `json:"summary"`
	Outcomes []sweepJSONOutcome `json:"outcomes"`
}

type sweepJSONOutcome struct {
	Metric     string            `json:"metric"`
	Comparison config.Comparison `json:"comparison"`
	Min        *float64          `json:"min,omitempty"`
	Max        *float64          `json:"max,omitempty"`
	Value      float64           `json:"value"`
	Pass       bool              `json:"pass"`
}

// sweepJSONReport is the top-level structure of sweep.json.
type sweepJSONReport struct {
	NoTargets    bool              `json:"no_targets"`
	PassingRates []domain.CoinRate `json:"passing_rates"`
	RangeMin     domain.CoinRate   `json:"range_min,omitempty"`
	RangeMax     domain.CoinRate   `json:"range_max,omitempty"`
	Contiguous   bool              `json:"contiguous,omitempty"`
	Rates        []sweepJSONRate   `json:"rates"`
}

// WriteSweepJSON writes a machine-readable sweep.json to w.
// It includes per-rate summary maps and calibration outcomes.
// The per-rate day arrays are not included (they are in rate-<N>/result.json).
func WriteSweepJSON(w io.Writer, sr SweepResult) error {
	passing := sr.PassingRates
	if passing == nil {
		passing = []domain.CoinRate{} // never null in JSON
	}

	rates := make([]sweepJSONRate, len(sr.Rates))
	for i, rr := range sr.Rates {
		outcomes := make([]sweepJSONOutcome, len(rr.Outcomes))
		for j, o := range rr.Outcomes {
			outcomes[j] = sweepJSONOutcome(o)
		}
		rates[i] = sweepJSONRate{
			Rate:     rr.Rate,
			Pass:     rr.Pass,
			Summary:  rr.Result.Summary,
			Outcomes: outcomes,
		}
	}

	report := sweepJSONReport{
		NoTargets:    sr.NoTargets,
		PassingRates: passing,
		RangeMin:     sr.RangeMin,
		RangeMax:     sr.RangeMax,
		Contiguous:   sr.Contiguous,
		Rates:        rates,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return fmt.Errorf("write sweep json: %w", err)
	}
	return nil
}
