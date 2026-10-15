package config

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// archetypeNameRe is the allowed pattern for an archetype file's base name (without the .yaml
// extension). It matches lowercase letters, digits and underscores, starting with a letter.
var archetypeNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

//go:embed archetypes
var embeddedArchetypes embed.FS //nolint:gochecknoglobals // go:embed needs a package variable

// LoadArchetypes reads every *.yaml file from fsys (rooted at the archetype directory), decodes
// each one strictly, validates it, and returns a fresh map keyed by file base name. It is the
// single source of archetype defaults: the caller must not modify the returned map.
//
// fsys must be rooted at the archetypes directory itself, so file names are bare (e.g.
// "casual.yaml"). Tests pass an [testing/fstest.MapFS]; production code receives the sub-FS of
// the embedded binary.
//
// Rules enforced per file:
//   - No directories.
//   - Extension must be .yaml (not .yml or bare).
//   - Base name (without .yaml) must match ^[a-z][a-z0-9_]*$.
//   - Body is decoded with strict (KnownFields) YAML; unknown fields are rejected.
//   - An empty file is rejected.
//   - Supply and trade fields are validated with the same rules as npcs.archetypes.
//
// An empty directory is rejected with "no archetype files".
func LoadArchetypes(fsys fs.FS) (map[string]ArchetypeConfig, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read archetype directory: %w", err)
	}

	result := make(map[string]ArchetypeConfig, len(entries))
	for _, entry := range entries {
		fileName := entry.Name()

		if entry.IsDir() {
			return nil, fmt.Errorf("archetype file %s: directories are not allowed", fileName)
		}
		if !strings.HasSuffix(fileName, ".yaml") {
			return nil, fmt.Errorf("archetype file %s: only .yaml files are accepted", fileName)
		}

		name := strings.TrimSuffix(fileName, ".yaml")
		if !archetypeNameRe.MatchString(name) {
			return nil, fmt.Errorf("archetype file %s: name %q does not match ^[a-z][a-z0-9_]*$", fileName, name)
		}

		data, err := fs.ReadFile(fsys, fileName)
		if err != nil {
			return nil, fmt.Errorf("archetype file %s: %w", fileName, err)
		}

		if err := checkStrictInput(data); err != nil {
			return nil, fmt.Errorf("archetype file %s: %w", fileName, err)
		}

		var a ArchetypeConfig
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(&a); err != nil {
			if errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("archetype file %s: file is empty", fileName)
			}
			return nil, fmt.Errorf("archetype file %s: %w", fileName, err)
		}

		key := "archetypes." + name
		if err := validateArchetype(key, a); err != nil {
			return nil, fmt.Errorf("archetype file %s: %w", fileName, err)
		}

		result[name] = a
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no archetype files")
	}

	return result, nil
}

// validateArchetype validates both the supply and the trade settings of one archetype.
// key is the dotted config path prefix, such as "archetypes.casual".
func validateArchetype(key string, a ArchetypeConfig) error {
	if err := validateArchetypeSupply(key, a); err != nil {
		return err
	}
	return validateArchetypeTrade(key, a)
}

// LoadWithArchetypes reads a YAML configuration from r, loads archetypes from the given FS,
// overlays the user YAML on the defaults, and validates the result. It is the testable core of
// [Load]: tests pass an [testing/fstest.MapFS] for archetypes instead of the embedded binary files.
//
// The user's npcs.archetypes map is decoded over the loaded archetype map. Because yaml.v3
// decodes each map entry into a fresh zero value, a user entry replaces the embedded entry of
// the same name as a whole; a new name is added. Embedded archetypes not mentioned in the user
// YAML are kept unchanged.
func LoadWithArchetypes(r io.Reader, archetypes fs.FS) (Config, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Config{}, fmt.Errorf("load config: read: %w", err)
	}

	if err := checkStrictInput(data); err != nil {
		return Config{}, err
	}

	// Prune null-valued mapping entries so they are treated as absent.
	if pruned, pruneErr := nullPruneAndEncode(data); pruneErr != nil {
		return Config{}, fmt.Errorf("load config: %w", pruneErr)
	} else if pruned != nil {
		data = pruned
	}

	lineKey := buildLineKeyMap(data)

	loaded, err := LoadArchetypes(archetypes)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}

	// Start from the non-archetype defaults and populate archetypes from the embedded files.
	cfg := defaults()
	cfg.NPCs.Archetypes = loaded

	// Overlay the user YAML. yaml.v3 decodes each map entry into a fresh zero value, so the
	// replace-the-whole-entry rule for npcs.archetypes is preserved automatically.
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return Config{}, enrichYAMLError(err, lineKey)
	}

	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Load reads a YAML configuration from r, overlays it on the defaults and the embedded
// archetypes, and validates the result. It returns an error naming the offending key for unknown
// keys, type mismatches, and invalid values. A malformed embedded file causes Load to return an
// error; the simulation does not start.
//
// A mapping key whose YAML value is null is treated as absent: the pre-populated default applies.
// This covers every key including nested ones (energy and its sub-keys, cost_table, sweep_rates,
// targets, initial_stock and each rarity). A null item inside a sequence is rejected with a
// path-named error, see ADR-0017.
//
// The caller is responsible for opening the file; Load accepts an [io.Reader] so tests can use
// [strings.NewReader] without touching the filesystem.
func Load(r io.Reader) (Config, error) {
	sub, err := fs.Sub(embeddedArchetypes, "archetypes")
	if err != nil {
		return Config{}, fmt.Errorf("load config: embedded archetypes: %w", err)
	}
	return LoadWithArchetypes(r, sub)
}
