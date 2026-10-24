package study

import (
	"errors"
	"fmt"
	"io"

	yaml "go.yaml.in/yaml/v3"

	"github.com/foutoucour/crackapack/internal/config"
)

// Variant is a tweak to try: a name and the config keys it changes, such as the number of NPCs or
// what the Counter pays. A variant with no override is the base itself.
type Variant struct {
	Name      string            `yaml:"name"`
	Note      string            `yaml:"note"`
	Overrides []config.Override `yaml:"overrides"`
}

// LoadVariants reads a YAML file of variants, strictly: an unknown field is an error, and so are a
// missing name and a name used twice.
func LoadVariants(r io.Reader) ([]Variant, error) {
	var doc struct {
		Variants []Variant `yaml:"variants"`
	}
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("variants: %w", err)
	}
	if len(doc.Variants) == 0 {
		return nil, errors.New("variants: no variant to run")
	}
	seen := map[string]bool{}
	for i, v := range doc.Variants {
		switch {
		case v.Name == "":
			return nil, fmt.Errorf("variants: variant %d has no name", i+1)
		case seen[v.Name]:
			return nil, fmt.Errorf("variants: %q is named twice", v.Name)
		}
		seen[v.Name] = true
	}
	return doc.Variants, nil
}
