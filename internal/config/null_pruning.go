package config

import yaml "go.yaml.in/yaml/v3"

// pruneNulls removes mapping entries whose value is a YAML null scalar from n
// and all its descendants. This implements the rule "a key whose value is null
// is treated as absent (the default applies)" for every mapping key in the
// config tree (energy and its sub-keys, cost_table, sweep_rates, targets,
// initial_stock and each rarity).
//
// Null items inside sequences are not pruned: Load rejects them before pruning, with a path-named
// error, see ADR-0017.
func pruneNulls(n *yaml.Node) {
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			pruneNulls(c)
		}
	case yaml.MappingNode:
		kept := n.Content[:0]
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i]
			val := n.Content[i+1]
			if isYAMLNull(val) {
				continue // treat null value as absent; default applies
			}
			pruneNulls(val)
			kept = append(kept, key, val)
		}
		n.Content = kept
	case yaml.SequenceNode:
		for _, item := range n.Content {
			pruneNulls(item)
		}
	case yaml.ScalarNode, yaml.AliasNode:
		// Scalars and aliases have no children to inspect or prune.
	}
}

// isYAMLNull reports whether n is a null scalar.
// YAML null, ~, and bare missing values all produce a ScalarNode tagged !!null.
func isYAMLNull(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && n.Tag == "!!null"
}

// isNullDoc reports whether root is an empty or null YAML document.
// When true the Load function skips decoding entirely so the pre-populated
// defaults are preserved.
func isNullDoc(root *yaml.Node) bool {
	if root.Kind == 0 {
		return true
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return true
	}
	c := root.Content[0]
	return isYAMLNull(c)
}
