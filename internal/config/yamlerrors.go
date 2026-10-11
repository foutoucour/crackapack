package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// buildLineKeyMap parses data into a yaml.Node tree and returns a map from
// each node's line number to its dotted key path
// (e.g. "energy.cost_table[0].threshold", "targets[1].min").
// This enriches TypeError messages with the key name rather than just a line number.
func buildLineKeyMap(data []byte) map[int]string {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil
	}
	m := make(map[int]string)
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		walkNode(root.Content[0], "", m)
	}
	return m
}

// walkNode recursively records line -> dotted-key-path for every key and
// value node in mappings and sequences.
// Mapping entries use dotted notation: "energy.reserve_cap".
// Sequence items are indexed: "targets[0]", "energy.cost_table[1]".
// When a key and the first field of its block value share a line, the more
// specific sub-key path overwrites the broader one, so errors land on the
// most precise key available.
func walkNode(n *yaml.Node, prefix string, m map[int]string) {
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			keyNode := n.Content[i]
			valNode := n.Content[i+1]
			key := keyNode.Value
			if prefix != "" {
				key = prefix + "." + key
			}
			m[keyNode.Line] = key
			m[valNode.Line] = key
			walkNode(valNode, key, m)
		}
	case yaml.SequenceNode:
		for i, item := range n.Content {
			itemKey := fmt.Sprintf("%s[%d]", prefix, i)
			m[item.Line] = itemKey
			walkNode(item, itemKey, m)
		}
	case yaml.DocumentNode, yaml.ScalarNode, yaml.AliasNode:
		// No key paths to record for these node kinds.
	}
}

// enrichYAMLError converts a yaml decode error to one that names the offending key.
// Unknown-field errors from KnownFields(true) already carry the field name.
// Type errors ([yaml.TypeError]) have their "line N:" prefix replaced with the
// dotted key path when the line is found in lineKey.
func enrichYAMLError(err error, lineKey map[int]string) error {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return fmt.Errorf("load config: %w", err)
	}
	parts := make([]string, 0, len(typeErr.Errors))
	for _, msg := range typeErr.Errors {
		parts = append(parts, enrichTypeLine(msg, lineKey))
	}
	return fmt.Errorf("load config: %s", strings.Join(parts, "; "))
}

// enrichTypeLine replaces the "line N: <problem>" prefix with "key: <problem>"
// when the line number maps to a known YAML key path, leaving the message
// unchanged otherwise.
func enrichTypeLine(msg string, lineKey map[int]string) string {
	const linePrefix = "line "
	if !strings.HasPrefix(msg, linePrefix) {
		return msg
	}
	rest := msg[len(linePrefix):]
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return msg
	}
	lineNum, err := strconv.Atoi(rest[:colon])
	if err != nil {
		return msg
	}
	detail := strings.TrimSpace(rest[colon+1:])
	if key, ok := lineKey[lineNum]; ok {
		return key + ": " + detail
	}
	return msg
}
