package tune_test

import (
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/tune"
)

func loadSpace(t *testing.T, yaml string) tune.Space {
	t.Helper()
	space, err := tune.LoadSpace(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("LoadSpace: %v", err)
	}
	return space
}

func TestLoadSpace_defaultsTheScaleToLinear(t *testing.T) {
	t.Parallel()
	space := loadSpace(t, "params:\n  - {path: counter_buy_pct, type: int, min: 100, max: 200}\n")
	if got := space.Params[0].Scale; got != tune.ScaleLinear {
		t.Errorf("scale = %q, want linear", got)
	}
}

func TestLoadSpace_rejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"empty", "", "params must not be empty"},
		{"unknown key", "params:\n  - {path: a, type: int, min: 1, max: 2, step: 3}\n", "step"},
		{"missing path", "params:\n  - {type: int, min: 1, max: 2}\n", "no path"},
		{"unknown type", "params:\n  - {path: a, type: bool, min: 1, max: 2}\n", "type"},
		{"unknown scale", "params:\n  - {path: a, type: int, min: 1, max: 2, scale: exp}\n", "scale"},
		{"inverted range", "params:\n  - {path: a, type: int, min: 5, max: 5}\n", "below max"},
		{"log from zero", "params:\n  - {path: a, type: float, min: 0, max: 5, scale: log}\n", "positive min"},
		{"fractional int bound", "params:\n  - {path: a, type: int, min: 1.5, max: 5}\n", "whole"},
		{"duplicate path", "params:\n  - {path: a, type: int, min: 1, max: 5}\n  - {path: a, type: int, min: 1, max: 5}\n", "twice"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := tune.LoadSpace(strings.NewReader(tc.yaml))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("LoadSpace error = %v, want one naming %q", err, tc.want)
			}
		})
	}
}

func TestSample_staysInRangeAndHonoursTheType(t *testing.T) {
	t.Parallel()

	space := loadSpace(t, `params:
  - {path: whole, type: int, min: 3, max: 7}
  - {path: fraction, type: float, min: 0.5, max: 2.5}
  - {path: decades, type: float, min: 1, max: 1000, scale: log}
  - {path: whole_log, type: int, min: 10, max: 1000, scale: log}
`)
	rng := rand.New(rand.NewPCG(1, 1))
	seenWhole := map[float64]bool{}
	var decades []float64
	for range 2000 {
		got := space.Sample(rng)
		for i, o := range got {
			p := space.Params[i]
			if o.Path != p.Path || o.Value < p.Min || o.Value > p.Max {
				t.Fatalf("sample %+v is outside %+v", o, p)
			}
			if p.Type == tune.ParamInt && o.Value != math.Trunc(o.Value) {
				t.Fatalf("int param %s drew %v", p.Path, o.Value)
			}
		}
		seenWhole[got[0].Value] = true
		decades = append(decades, got[2].Value)
	}
	for v := 3.0; v <= 7; v++ {
		if !seenWhole[v] {
			t.Errorf("int range never drew %v: both ends must be reachable", v)
		}
	}
	// A log scale spreads the draws over the decades: about a third land below 10.
	below := 0
	for _, v := range decades {
		if v < 10 {
			below++
		}
	}
	if share := float64(below) / float64(len(decades)); share < 0.28 || share > 0.38 {
		t.Errorf("share of the log range below 10 = %.2f, want about 0.33", share)
	}
}

func TestSample_isReproducibleFromTheSeed(t *testing.T) {
	t.Parallel()
	space := loadSpace(t, "params:\n  - {path: a, type: float, min: 0, max: 1}\n")
	draw := func() []float64 {
		rng := rand.New(rand.NewPCG(7, 7))
		return []float64{space.Sample(rng)[0].Value, space.Sample(rng)[0].Value}
	}
	if a, b := draw(), draw(); !slices.Equal(a, b) {
		t.Errorf("two draws from one seed differ: %v and %v", a, b)
	}
}
