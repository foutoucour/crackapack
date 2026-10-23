package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
	"github.com/foutoucour/crackapack/internal/tune"
)

// Defaults of a search. The workers stay few on purpose: a search keeps every core busy for
// minutes, and a laptop runs hot.
const (
	defaultCandidates      = 200
	defaultKeepFraction    = 0.1
	defaultSeeds           = 3
	defaultValidationSeeds = 3
	defaultTop             = 3
	defaultWorkers         = 3
	defaultSearchSeed      = 1
)

// tuneArgs holds all dependencies for runTune, decoupled from the OS so that tests can substitute
// in-memory implementations.
type tuneArgs struct {
	// configReader is the base config; nil means all defaults.
	configReader  io.Reader
	spaceReader   io.Reader
	problemReader io.Reader

	catFS      fs.FS
	pricesFS   fs.FS
	pricesName string
	policies   map[string]func() sim.PlayerPolicy

	opts tune.Options

	// newOutput creates (or truncates) an output file by name.
	newOutput func(string) (io.WriteCloser, error)

	stdout io.Writer
}

// simRunner is the tune.Runner that simulates a candidate on the loaded catalogues. It only reads
// what it holds, so several goroutines may call Run at once.
type simRunner struct {
	sets          map[string]domain.SetCatalogue
	supplementary map[string]domain.SetCatalogue
	reference     []domain.SetCatalogue
	prices        domain.PriceSnapshot
	policies      map[string]func() sim.PlayerPolicy
}

// Run implements tune.Runner.
func (r simRunner) Run(cfg config.Config, run tune.Run) (sim.Summary, error) {
	cats := make([]domain.SetCatalogue, len(run.Sets))
	for i, code := range run.Sets {
		cats[i] = r.sets[code]
	}
	supplementary := make([]domain.SetCatalogue, len(run.Supplementary))
	for i, code := range run.Supplementary {
		supplementary[i] = r.supplementary[code]
	}
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed))
	engine, err := sim.NewEngine(cfg, cats, supplementary, r.prices, r.policies[run.Player](), rng,
		sim.WithReferenceCatalogues(r.reference))
	if err != nil {
		return nil, fmt.Errorf("engine: %w", err)
	}
	result, err := engine.Run()
	if err != nil {
		return nil, fmt.Errorf("run: %w", err)
	}
	return result.Summary, nil
}

// newSimRunner loads every catalogue the problem names, once, and checks its players.
func newSimRunner(a tuneArgs, runs []tune.Run) (simRunner, error) {
	reference, err := loadReferenceCatalogues(a.catFS, nil, nil)
	if err != nil {
		return simRunner{}, err
	}
	r := simRunner{
		sets:          make(map[string]domain.SetCatalogue, len(reference)),
		supplementary: make(map[string]domain.SetCatalogue),
		reference:     reference,
		policies:      a.policies,
	}
	for _, cat := range reference {
		r.sets[cat.Code] = cat
	}
	for _, run := range runs {
		if _, ok := a.policies[run.Player]; !ok {
			return simRunner{}, fmt.Errorf("run %s: unknown player %q", run.Name, run.Player)
		}
		for _, code := range run.Sets {
			if _, ok := r.sets[code]; !ok {
				return simRunner{}, fmt.Errorf("run %s: set %q is not an openable set of the catalogue directory", run.Name, code)
			}
		}
		for _, code := range run.Supplementary {
			if _, ok := r.supplementary[code]; ok {
				continue
			}
			name := code + ".json"
			pool, err := domain.LoadCardPool(a.catFS, name)
			if err != nil {
				return simRunner{}, fileError("supplementary", name, err)
			}
			r.supplementary[code] = pool
		}
	}
	if r.prices, err = domain.LoadPriceSnapshot(a.pricesFS, a.pricesName); err != nil {
		return simRunner{}, fmt.Errorf("prices %q: %w", a.pricesName, err)
	}
	return r, nil
}

// runTune is the testable core of the tune subcommand. It loads the inputs, runs the search,
// writes candidates.json, best.yaml and report.txt, and prints the report to stdout.
func runTune(ctx context.Context, a tuneArgs) error {
	base, err := loadBaseConfig(a.configReader)
	if err != nil {
		return err
	}
	space, err := tune.LoadSpace(a.spaceReader)
	if err != nil {
		return err
	}
	problem, err := tune.LoadProblem(a.problemReader)
	if err != nil {
		return err
	}
	runner, err := newSimRunner(a, problem.Runs())
	if err != nil {
		return err
	}

	report, err := tune.Search(ctx, base, space, problem, runner, a.opts)
	if err != nil {
		return err
	}

	outputs := []struct {
		name  string
		write func(io.Writer, tune.Report) error
	}{
		{"candidates.json", tune.WriteCandidatesJSON},
		{"best.yaml", tune.WriteBestYAML},
		{"report.txt", tune.WriteText},
	}
	for _, o := range outputs {
		if err := writeOutput(a.newOutput, o.name, func(w io.Writer) error { return o.write(w, report) }); err != nil {
			return err
		}
	}
	return tune.WriteText(a.stdout, report)
}

func loadBaseConfig(r io.Reader) (config.Config, error) {
	if r == nil {
		r = strings.NewReader("")
	}
	return config.Load(r)
}

// writeOutput creates the named output, fills it and closes it, reporting the first error.
func writeOutput(newOutput func(string) (io.WriteCloser, error), name string, write func(io.Writer) error) error {
	w, err := newOutput(name)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if err := write(w); err != nil {
		_ = w.Close()
		return fmt.Errorf("%s: %w", name, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("%s: close: %w", name, err)
	}
	return nil
}

// openTuneInputs opens the three YAML files of a search. The returned function closes them.
func openTuneInputs(configPath, spacePath, problemPath string) (tuneArgs, func(), error) {
	var a tuneArgs
	var files []*os.File
	closeAll := func() {
		for _, f := range files {
			_ = f.Close()
		}
	}
	open := func(path string) (*os.File, error) {
		f, err := os.Open(path)
		if err != nil {
			closeAll()
			return nil, err
		}
		files = append(files, f)
		return f, nil
	}
	if configPath != "" {
		f, err := open(configPath)
		if err != nil {
			return tuneArgs{}, nil, fmt.Errorf("config %q: %w", configPath, err)
		}
		a.configReader = f
	}
	space, err := open(spacePath)
	if err != nil {
		return tuneArgs{}, nil, fmt.Errorf("--space %q: %w", spacePath, err)
	}
	problem, err := open(problemPath)
	if err != nil {
		return tuneArgs{}, nil, fmt.Errorf("--goals %q: %w", problemPath, err)
	}
	a.spaceReader, a.problemReader = space, problem
	return a, closeAll, nil
}

// progressTo returns a tune.Options.Progress that prints a line to w when a stage finishes.
func progressTo(w io.Writer) func(stage string, done, total int) {
	return func(stage string, done, total int) {
		if done == total {
			fmt.Fprintf(w, "tune: %s: %d simulations done\n", stage, total)
		}
	}
}

// newTuneCmd returns the cobra command for the tune subcommand.
// stdout receives the report; stderr receives progress, errors, usage, and help.
func newTuneCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		configPath  string
		spacePath   string
		problemPath string
		catDir      string
		pricesPath  string
		outDir      string
		opts        tune.Options
	)

	cmd := &cobra.Command{
		Use:   "tune",
		Short: "search the config space for a config that meets the goals",
		Long: `Search many config keys at once for a config whose runs fall inside the bands
of the goals. A search space file names the keys and a range for each. A goals file names
the scenarios to simulate and a band on a metric for each goal.

The search draws candidate configs and scores each by the distance its runs fall outside the
bands, normalised by the size of each band. It screens every candidate on one seed, scores the
best fraction on all the seeds, then re-scores the best on seeds it never used.

The base config is the one given by --config, or the defaults. Each candidate is the base config
with the keys of the space replaced. The same --seed gives the same search whatever --workers is.

It writes candidates.json (every candidate), best.yaml (the best config, which simulate and sweep
accept) and report.txt to --out, and prints the report.`,
		Example: `  crackapack tune \
    --config base.yaml \
    --space space.yaml \
    --goals goals.yaml \
    --catalogue-dir ./catalogues \
    --prices ./prices.json \
    --out ./tune-out`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var missing []string
			for flag, value := range map[string]string{
				"--space": spacePath, "--goals": problemPath, "--catalogue-dir": catDir,
				"--prices": pricesPath, "--out": outDir,
			} {
				if value == "" {
					missing = append(missing, flag)
				}
			}
			if len(missing) > 0 {
				slices.Sort(missing)
				if err := cmd.Usage(); err != nil {
					return err
				}
				return fmt.Errorf("crackapack tune: required flags not set: %s", strings.Join(missing, ", "))
			}

			a, closeAll, err := openTuneInputs(configPath, spacePath, problemPath)
			if err != nil {
				return fmt.Errorf("crackapack tune: %w", err)
			}
			defer closeAll()

			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("crackapack tune: --out %q: %w", outDir, err)
			}
			opts.Progress = progressTo(stderr)
			a.opts = opts
			a.catFS = os.DirFS(catDir)
			a.pricesFS = os.DirFS(filepath.Dir(pricesPath))
			a.pricesName = filepath.Base(pricesPath)
			a.policies = buildPolicies()
			a.newOutput = func(name string) (io.WriteCloser, error) { return os.Create(filepath.Join(outDir, name)) }
			a.stdout = stdout

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			if err := runTune(ctx, a); err != nil {
				return fmt.Errorf("crackapack tune: %w", err)
			}
			return nil
		},
	}
	// cobra help and usage go to stderr to match existing test assertions.
	cmd.SetOut(stderr)
	cmd.SetErr(stderr)

	f := cmd.Flags()
	f.StringVar(&configPath, "config", "", "base YAML config file (optional; omit for defaults)")
	f.StringVar(&spacePath, "space", "", "search space YAML file (required)")
	f.StringVar(&problemPath, "goals", "", "scenarios and goals YAML file (required)")
	f.StringVar(&catDir, "catalogue-dir", "", "catalogue directory of <CODE>.json files (required)")
	f.StringVar(&pricesPath, "prices", "", "JSON price snapshot file (required)")
	f.StringVar(&outDir, "out", "", "output directory (required)")
	f.Uint64Var(&opts.Seed, "seed", defaultSearchSeed, "search seed: the same seed draws the same candidates and simulation seeds")
	f.IntVar(&opts.Candidates, "candidates", defaultCandidates, "configs drawn and screened on one seed")
	f.Float64Var(&opts.KeepFraction, "keep", defaultKeepFraction, "share of the valid candidates scored on all the seeds")
	f.IntVar(&opts.Seeds, "seeds", defaultSeeds, "simulation seeds that score the survivors")
	f.IntVar(&opts.ValidationSeeds, "validation-seeds", defaultValidationSeeds, "fresh seeds that re-score the best survivors")
	f.IntVar(&opts.Top, "top", defaultTop, "best survivors that are validated")
	f.IntVar(&opts.Workers, "workers", defaultWorkers, "simulations run at once")

	return cmd
}
