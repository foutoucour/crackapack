package config

import (
	"fmt"
	"regexp"

	yaml "go.yaml.in/yaml/v3"
)

// floatKeys matches the only keys whose value may be a float: the bounds of a calibration target
// and the share of a binder that unlocks sets.
var floatKeys = regexp.MustCompile(`^(targets\[\d+\]\.(min|max)|unlock\.binder_threshold)$`)

// checkStrictInput walks the YAML tree of the raw document and rejects what the decoder would
// otherwise accept silently, see ADR-0017: a null item in a sequence, which is dropped, and a float
// where an integer is expected, which is truncated. It returns an error naming the key. A document
// that does not parse is left to the decoder, which reports the syntax error.
func checkStrictInput(data []byte) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil //nolint:nilerr // the decoder reports the syntax error
	}
	return walkStrict(&root, "")
}

func walkStrict(n *yaml.Node, path string) error {
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			if err := walkStrict(c, path); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i].Value
			if path != "" {
				key = path + "." + key
			}
			if err := walkStrict(n.Content[i+1], key); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for i, item := range n.Content {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			if isYAMLNull(item) {
				return fmt.Errorf("config: %s must not be null", itemPath)
			}
			if err := walkStrict(item, itemPath); err != nil {
				return err
			}
		}
	case yaml.ScalarNode:
		if n.Tag == "!!float" && !floatKeys.MatchString(path) {
			return fmt.Errorf("config: %s must be a whole number, got %s", path, n.Value)
		}
	case yaml.AliasNode:
		// An alias points at a node walked where it is defined.
	}
	return nil
}
