package config

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// Override sets one numeric key of a [Config], addressed by its path in the YAML document: dotted
// mapping keys and bracketed sequence indexes, such as "counter_buy_pct",
// "npcs.archetypes.casual.keep_copies" or "energy.cost_table[1].cost".
type Override struct {
	Path  string  `json:"path"`
	Value float64 `json:"value"`
}

// LoadOverrides reads a YAML list of overrides, each a path and a value, strictly: an unknown field
// is an error. An empty input is no override.
func LoadOverrides(r io.Reader) ([]Override, error) {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	var overrides []Override
	if err := dec.Decode(&overrides); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("overrides: %w", err)
	}
	return overrides, nil
}

// ApplyOverrides returns base with every override set, run through the same loading and validation
// as a config file, so a combination that Load would refuse is an error here too. base is not
// modified.
//
// A path must name an existing numeric key. A value with a fractional part is refused for a key
// that holds a whole number. An archetype is overridden field by field: its other fields keep the
// values they have in base.
func ApplyOverrides(base Config, overrides []Override) (Config, error) {
	var root yaml.Node
	if err := root.Encode(base); err != nil {
		return Config{}, fmt.Errorf("apply overrides: encode config: %w", err)
	}
	for _, o := range overrides {
		if err := setNumber(&root, o); err != nil {
			return Config{}, fmt.Errorf("apply overrides: %w", err)
		}
	}
	data, err := yaml.Marshal(&root)
	if err != nil {
		return Config{}, fmt.Errorf("apply overrides: encode config: %w", err)
	}
	cfg, err := Load(strings.NewReader(string(data)))
	if err != nil {
		return Config{}, fmt.Errorf("apply overrides: %w", err)
	}
	return cfg, nil
}

// setNumber sets the scalar that o.Path names in the YAML tree under root.
func setNumber(root *yaml.Node, o Override) error {
	node := root
	for _, step := range strings.Split(o.Path, ".") {
		var err error
		if node, err = descend(node, step, o.Path); err != nil {
			return err
		}
	}
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("%s is not a number", o.Path)
	}
	switch node.Tag {
	case "!!int":
		if o.Value != math.Trunc(o.Value) {
			return fmt.Errorf("%s holds a whole number, got %v", o.Path, o.Value)
		}
		node.Value = strconv.FormatInt(int64(o.Value), 10)
	case "!!float":
		node.Value = strconv.FormatFloat(o.Value, 'g', -1, 64)
	default:
		return fmt.Errorf("%s is not a number", o.Path)
	}
	return nil
}

// descend follows one step of a path: a mapping key, optionally followed by sequence indexes.
func descend(node *yaml.Node, step, path string) (*yaml.Node, error) {
	key, indexes, err := splitStep(step)
	if err != nil {
		return nil, fmt.Errorf("path %q: %w", path, err)
	}
	node, err = child(node, key)
	if err != nil {
		return nil, fmt.Errorf("path %q: %w", path, err)
	}
	for _, i := range indexes {
		if node.Kind != yaml.SequenceNode || i >= len(node.Content) {
			return nil, fmt.Errorf("path %q: %s[%d] does not exist", path, key, i)
		}
		node = node.Content[i]
	}
	return node, nil
}

// child returns the value under key in the mapping node.
func child(node *yaml.Node, key string) (*yaml.Node, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: parent is not a mapping", key)
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1], nil
		}
	}
	return nil, fmt.Errorf("unknown key %s", key)
}

// splitStep splits "cost_table[1][2]" into the key and its sequence indexes.
func splitStep(step string) (string, []int, error) {
	key, rest, _ := strings.Cut(step, "[")
	if key == "" {
		return "", nil, fmt.Errorf("empty key in %q", step)
	}
	var indexes []int
	for rest != "" {
		digits, tail, ok := strings.Cut(rest, "]")
		if !ok {
			return "", nil, fmt.Errorf("unclosed index in %q", step)
		}
		i, err := strconv.Atoi(digits)
		if err != nil || i < 0 {
			return "", nil, fmt.Errorf("bad index %q in %q", digits, step)
		}
		indexes = append(indexes, i)
		if tail != "" && !strings.HasPrefix(tail, "[") {
			return "", nil, fmt.Errorf("unexpected %q in %q", tail, step)
		}
		rest = strings.TrimPrefix(tail, "[")
	}
	return key, indexes, nil
}
