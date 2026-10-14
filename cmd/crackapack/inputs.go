package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand/v2"
	"regexp"
	"sort"
	"strings"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// setFilePattern matches the file name of a set in the catalogue directory: an upper-case MTGJSON
// code and ".json". It leaves out the other files that may share the directory, such as
// prices.json.
var setFilePattern = regexp.MustCompile(`^[A-Z0-9_]+\.json$`)

// draftSection is the field of a CatalogueError for a set file without a draft booster.
const draftSection = "data.booster.draft"

// simulateArgs holds all dependencies for runSimulate (and loadInputs), decoupled
// from the OS so that tests can substitute in-memory implementations.
type simulateArgs struct {
	// configReader is an open reader for the YAML config file.
	// nil means "use all defaults" (equivalent to an empty YAML document).
	configReader io.Reader

	// catFS is the catalogue directory; set files are loaded as "<CODE>.json".
	catFS     fs.FS
	setCodes  []string // openable set codes (must have a booster.draft section)
	suppCodes []string // supplementary card-pool codes (no booster section required)

	// pricesFS is the FS rooted at the directory that contains the price file.
	// pricesName is the relative file name within that FS.
	pricesFS   fs.FS
	pricesName string

	player   string
	policies map[string]func() sim.PlayerPolicy

	// newOutput creates (or truncates) an output file by name. The caller is
	// responsible for calling Close and checking its error.
	newOutput func(string) (io.WriteCloser, error)

	stdout io.Writer
}

// sweepArgs holds all dependencies for runSweep, decoupled from the OS so that
// tests can substitute in-memory implementations.
type sweepArgs struct {
	// configReader is an open reader for the YAML config file.
	// nil means "use all defaults".
	configReader io.Reader

	// catFS is the catalogue directory.
	catFS     fs.FS
	setCodes  []string
	suppCodes []string

	// pricesFS is the FS rooted at the prices file's directory.
	pricesFS   fs.FS
	pricesName string

	player   string
	policies map[string]func() sim.PlayerPolicy

	// rates overrides cfg.SweepRates when non-nil.
	rates []domain.CoinRate

	// newOutput creates (or truncates) an output file by relative path.
	// The path may contain a subdirectory, e.g. "rate-10/result.json".
	// The OS wrapper creates parent directories; the in-memory fake stores by path.
	newOutput func(string) (io.WriteCloser, error)

	stdout io.Writer
}

// inputs holds the loaded simulation inputs produced by loadInputs.
type inputs struct {
	cfg           config.Config
	catalogues    []domain.SetCatalogue
	reference     []domain.SetCatalogue
	supplementary []domain.SetCatalogue
	prices        domain.PriceSnapshot
	// policyFn returns a fresh PlayerPolicy on each call. Sweep calls it once per rate.
	policyFn func() sim.PlayerPolicy
}

// newEngine builds an engine over the loaded inputs with a fresh player policy.
func (in inputs) newEngine(cfg config.Config, rng *rand.Rand) (*sim.Engine, error) {
	return sim.NewEngine(cfg, in.catalogues, in.supplementary, in.prices, in.policyFn(), rng,
		sim.WithReferenceCatalogues(in.reference))
}

// loadInputs performs the shared loading steps for the simulate and sweep subcommands:
// config, catalogues, supplementary pools, prices, and player policy lookup.
// All error messages match what simulate_test.go pins; do not change them.
func loadInputs(a simulateArgs) (inputs, error) {
	// 1. Load config. nil reader means "all defaults".
	reader := a.configReader
	if reader == nil {
		reader = strings.NewReader("")
	}
	cfg, err := config.Load(reader)
	if err != nil {
		return inputs{}, err
	}

	// 2. Load openable catalogues (booster.draft section required).
	catalogues := make([]domain.SetCatalogue, 0, len(a.setCodes))
	for _, code := range a.setCodes {
		name := code + ".json"
		cat, err := domain.LoadSetCatalogue(a.catFS, name)
		if err != nil {
			return inputs{}, fileError("catalogue", name, err)
		}
		catalogues = append(catalogues, cat)
	}

	// 2b. Load the reference catalogues of the energy cost tiers.
	reference, err := loadReferenceCatalogues(a.catFS, a.setCodes, catalogues)
	if err != nil {
		return inputs{}, err
	}

	// 3. Load supplementary card pools (no booster section required).
	supplementary := make([]domain.SetCatalogue, 0, len(a.suppCodes))
	for _, code := range a.suppCodes {
		name := code + ".json"
		pool, err := domain.LoadCardPool(a.catFS, name)
		if err != nil {
			return inputs{}, fileError("supplementary", name, err)
		}
		supplementary = append(supplementary, pool)
	}

	// 4. Load price snapshot.
	prices, err := domain.LoadPriceSnapshot(a.pricesFS, a.pricesName)
	if err != nil {
		return inputs{}, fmt.Errorf("prices %q: %w", a.pricesName, err)
	}

	// 5. Pick player policy.
	policyFn, ok := a.policies[a.player]
	if !ok {
		names := make([]string, 0, len(a.policies))
		for k := range a.policies {
			names = append(names, k)
		}
		sort.Strings(names)
		return inputs{}, fmt.Errorf("unknown player %q; valid names: %s",
			a.player, strings.Join(names, ", "))
	}

	return inputs{
		cfg:           cfg,
		catalogues:    catalogues,
		reference:     reference,
		supplementary: supplementary,
		prices:        prices,
		policyFn:      policyFn,
	}, nil
}

// fileError names the file in a load error. A CatalogueError already does, so it is returned as is.
func fileError(kind, name string, err error) error {
	var ce *domain.CatalogueError
	if errors.As(err, &ce) {
		return err
	}
	return fmt.Errorf("%s %s: %w", kind, name, err)
}

// loadReferenceCatalogues returns every openable set of the catalogue directory: the requested
// ones, already loaded, plus each other set file that has a draft booster. The energy cost of a
// booster is relative to their median value, so it does not depend on the sets of the run.
//
// A file without a draft booster, such as the TSB card pool, is skipped. Any other defect in a
// file that was not requested is an error: skipping it would shift the median unnoticed.
func loadReferenceCatalogues(catFS fs.FS, setCodes []string, requested []domain.SetCatalogue) ([]domain.SetCatalogue, error) {
	entries, err := fs.ReadDir(catFS, ".")
	if err != nil {
		return nil, fmt.Errorf("catalogue directory: %w", err)
	}

	isRequested := make(map[string]bool, len(setCodes))
	for _, code := range setCodes {
		isRequested[code+".json"] = true
	}

	reference := append([]domain.SetCatalogue(nil), requested...)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !setFilePattern.MatchString(name) || isRequested[name] {
			continue
		}
		cat, err := domain.LoadSetCatalogue(catFS, name)
		var ce *domain.CatalogueError
		if errors.As(err, &ce) && ce.Field == draftSection {
			continue
		}
		if err != nil {
			return nil, fileError("catalogue", name, err)
		}
		reference = append(reference, cat)
	}
	return reference, nil
}

// splitCodes splits a comma-separated string of set codes, trims whitespace,
// and removes empty entries.
func splitCodes(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
