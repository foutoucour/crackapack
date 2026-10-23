// Package tune searches the configuration space for a config that meets the economy's goals.
//
// A search space names the config keys to vary and a range for each. A problem names the
// scenarios to simulate and the goals, each a band on a metric. The search draws candidate
// configs, scores each by the distance its runs fall outside the bands, and keeps the best,
// see ADR-0045.
package tune

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"

	yaml "go.yaml.in/yaml/v3"

	"github.com/foutoucour/crackapack/internal/config"
)

// ParamType is the kind of number a parameter holds.
type ParamType string

// The two kinds of parameter.
const (
	ParamInt   ParamType = "int"
	ParamFloat ParamType = "float"
)

// Scale is how a parameter's range is sampled.
type Scale string

// The two scales. A log scale suits a parameter that spans orders of magnitude.
const (
	ScaleLinear Scale = "linear"
	ScaleLog    Scale = "log"
)

// Param is one tunable config key and the range a candidate draws it from.
type Param struct {
	// Path addresses the key, see [config.Override].
	Path string `yaml:"path" json:"path"`
	// Type is ParamInt or ParamFloat.
	Type ParamType `yaml:"type" json:"type"`
	// Min and Max bound the range, both included for an int.
	Min float64 `yaml:"min" json:"min"`
	Max float64 `yaml:"max" json:"max"`
	// Scale is ScaleLinear, the default, or ScaleLog, which needs a positive Min.
	Scale Scale `yaml:"scale" json:"scale"`
}

// Space is the set of parameters a search varies.
type Space struct {
	Params []Param `yaml:"params"`
}

// LoadSpace reads a search space from YAML and validates it. An unknown key is an error.
func LoadSpace(r io.Reader) (Space, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Space{}, fmt.Errorf("load search space: %w", err)
	}
	var s Space
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil && !errors.Is(err, io.EOF) {
		return Space{}, fmt.Errorf("load search space: %w", err)
	}
	for i := range s.Params {
		if s.Params[i].Scale == "" {
			s.Params[i].Scale = ScaleLinear
		}
	}
	if err := s.validate(); err != nil {
		return Space{}, fmt.Errorf("load search space: %w", err)
	}
	return s, nil
}

func (s Space) validate() error {
	if len(s.Params) == 0 {
		return errors.New("params must not be empty")
	}
	seen := make(map[string]bool, len(s.Params))
	for _, p := range s.Params {
		if err := p.validate(); err != nil {
			return err
		}
		if seen[p.Path] {
			return fmt.Errorf("param %s is listed twice", p.Path)
		}
		seen[p.Path] = true
	}
	return nil
}

func (p Param) validate() error {
	switch {
	case p.Path == "":
		return errors.New("a param has no path")
	case p.Type != ParamInt && p.Type != ParamFloat:
		return fmt.Errorf("param %s: type must be %q or %q, got %q", p.Path, ParamInt, ParamFloat, p.Type)
	case p.Scale != ScaleLinear && p.Scale != ScaleLog:
		return fmt.Errorf("param %s: scale must be %q or %q, got %q", p.Path, ScaleLinear, ScaleLog, p.Scale)
	case !(p.Min < p.Max):
		return fmt.Errorf("param %s: min (%v) must be below max (%v)", p.Path, p.Min, p.Max)
	case p.Scale == ScaleLog && p.Min <= 0:
		return fmt.Errorf("param %s: a log scale needs a positive min, got %v", p.Path, p.Min)
	case p.Type == ParamInt && (p.Min != math.Trunc(p.Min) || p.Max != math.Trunc(p.Max)):
		return fmt.Errorf("param %s: an int param needs whole min and max, got %v and %v", p.Path, p.Min, p.Max)
	}
	return nil
}

// Sample draws one candidate: a value for every parameter, in the order of the space.
func (s Space) Sample(rng *rand.Rand) []config.Override {
	overrides := make([]config.Override, len(s.Params))
	for i, p := range s.Params {
		overrides[i] = config.Override{Path: p.Path, Value: p.sample(rng)}
	}
	return overrides
}

func (p Param) sample(rng *rand.Rand) float64 {
	if p.Type == ParamInt && p.Scale == ScaleLinear {
		return p.Min + float64(rng.IntN(int(p.Max-p.Min)+1))
	}
	var v float64
	if p.Scale == ScaleLog {
		v = math.Exp(math.Log(p.Min) + rng.Float64()*(math.Log(p.Max)-math.Log(p.Min)))
	} else {
		v = p.Min + rng.Float64()*(p.Max-p.Min)
	}
	if p.Type == ParamInt {
		return math.Min(math.Max(math.Round(v), p.Min), p.Max)
	}
	return v
}
