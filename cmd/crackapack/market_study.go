package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/foutoucour/crackapack/demo"
	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/sim"
	"github.com/foutoucour/crackapack/internal/study"
	"github.com/foutoucour/crackapack/internal/tune"
)

const (
	defaultStudyScenario = "pace-3-visits"
	defaultStudySeed     = 42
	// defaultStudyRunDays lets a player who only opens boosters finish most sets: the pace scenarios
	// stop at 40 days, which is the Counter's pace, not theirs.
	defaultStudyRunDays = 120
	// studyStopFile is the name of the file whose appearance in the output directory stops a study.
	studyStopFile     = "STOP"
	studyStopPollTime = 2 * time.Second

	// The speeds the maintainers want: the Counter a third faster than boosters alone, the market at
	// least twice and 1.5 times the Counter, and no set done in under five days. The Counter's band
	// is 1.33 with a tolerance of about a tenth either way.
	studyCounterSpeedMin      = 1.2
	studyCounterSpeedMax      = 1.5
	studyMarketSpeedMin       = 2.0
	studyMarketOverCounterMin = 1.5
	studyMarketFloorDays      = 5.0
	studyOutputDirMode        = 0o755
)

// studyPlayers are the players compared, by registered policy name: the slow road, the country road
// and the highway.
func studyPlayers() []string { return []string{study.BoostersOnly, study.Counter, study.Market} }

// studyTargets are the speeds a variant is judged on.
func studyTargets() study.Targets {
	return study.Targets{
		CounterSpeed:         study.Band{Min: studyCounterSpeedMin, Max: studyCounterSpeedMax},
		MarketSpeedMin:       studyMarketSpeedMin,
		MarketOverCounterMin: studyMarketOverCounterMin,
		FloorDays:            studyMarketFloorDays,
	}
}

// studyArgs holds all dependencies for runStudy, decoupled from the OS so that tests can substitute
// in-memory implementations.
type studyArgs struct {
	// baseConfig is the world the variants tweak.
	baseConfig config.Config
	// problemReader is a goals file, whose per-set scenario gives the sets, the pools and the visit
	// rate.
	problemReader  io.Reader
	scenario       string
	variantsReader io.Reader

	catFS      fs.FS
	pricesFS   fs.FS
	pricesName string
	policies   map[string]func() sim.PlayerPolicy

	runDays int
	seeds   []uint64
	workers int
	targets study.Targets

	// newOutput creates (or truncates) an output file by name.
	newOutput func(string) (io.WriteCloser, error)
	stdout    io.Writer
}

// studyJob is one simulation: a variant, a player, a seed and a set.
type studyJob struct{ variant, player, seed, run int }

// runStudy simulates every variant with every player on every set of the scenario, judges the
// variants against the targets and writes study.json and report.html. It writes nothing when ctx
// ends first.
func runStudy(ctx context.Context, a studyArgs) error {
	problem, err := tune.LoadProblem(a.problemReader)
	if err != nil {
		return fmt.Errorf("goals: %w", err)
	}
	runs, err := scenarioRuns(problem, a.scenario)
	if err != nil {
		return err
	}
	variants, err := study.LoadVariants(a.variantsReader)
	if err != nil {
		return err
	}
	configs := make([]config.Config, len(variants))
	for i, v := range variants {
		if configs[i], err = config.ApplyOverrides(a.baseConfig, v.Overrides); err != nil {
			return fmt.Errorf("variant %q: %w", v.Name, err)
		}
	}
	players := studyPlayers()
	for _, p := range players {
		if _, ok := a.policies[p]; !ok {
			return fmt.Errorf("player %q is not a registered policy", p)
		}
	}
	runner, err := newStudyRunner(tuneArgs{catFS: a.catFS, pricesFS: a.pricesFS, pricesName: a.pricesName, policies: a.policies}, runs)
	if err != nil {
		return err
	}

	var jobs []studyJob
	for v := range variants {
		for p := range players {
			for sd := range a.seeds {
				for r := range runs {
					jobs = append(jobs, studyJob{variant: v, player: p, seed: sd, run: r})
				}
			}
		}
	}
	setRuns := make([]study.SetRun, len(jobs))
	err = tune.Parallel(ctx, a.workers, len(jobs), func(n int) error {
		j := jobs[n]
		run := runs[j.run]
		run.Player = players[j.player]
		cfg := configs[j.variant]
		cfg.Seed, cfg.RunDays = a.seeds[j.seed], a.runDays
		if run.SessionsPerDay > 0 {
			cfg.SessionsPerDay = run.SessionsPerDay
		}
		summary, err := runner.Run(cfg, run)
		if err != nil {
			return fmt.Errorf("variant %q, player %q, %s: %w", variants[j.variant].Name, run.Player, run.Name, err)
		}
		setRuns[n] = setRunOf(run.Sets[0], cfg.Seed, summary)
		return nil
	})
	if err != nil {
		return fmt.Errorf("study: %w", err)
	}

	outcomes := make([]study.Outcome, len(variants))
	for v, variant := range variants {
		outcomes[v] = study.Outcome{
			Name: variant.Name, Note: variant.Note, Overrides: variant.Overrides,
			Runs: make(map[string][]study.SetRun, len(players)),
		}
	}
	for n, j := range jobs {
		o := outcomes[j.variant]
		o.Runs[players[j.player]] = append(o.Runs[players[j.player]], setRuns[n])
	}
	f := studyFile{
		Scenario:       a.scenario,
		Horizon:        a.runDays,
		SessionsPerDay: studySessions(runs, a.baseConfig),
		Seeds:          a.seeds,
		Outcomes:       outcomes,
	}
	return writeStudy(a.newOutput, a.stdout, f.report(a.targets))
}

// studyFile is study.json: how a study ran and every run it made, from which the report is judged
// again. Verdicts are the judgement at the time, written for the reader of the file and not read
// back.
type studyFile struct {
	Scenario       string          `json:"scenario"`
	Horizon        int             `json:"horizon_days"`
	SessionsPerDay int             `json:"sessions_per_day"`
	Seeds          []uint64        `json:"seeds"`
	Outcomes       []study.Outcome `json:"outcomes"`
	Verdicts       []study.Verdict `json:"verdicts,omitempty"`
}

// report judges the outcomes of f against targets.
func (f studyFile) report(targets study.Targets) study.Report {
	return study.Report{
		Scenario:       f.Scenario,
		SessionsPerDay: f.SessionsPerDay,
		Horizon:        f.Horizon,
		Seeds:          f.Seeds,
		Targets:        targets,
		Verdicts:       study.Analyze(f.Outcomes, targets, f.Horizon),
		Outcomes:       f.Outcomes,
	}
}

// renderStudy writes the report of a study.json that a study wrote, judged against targets, without
// simulating anything: it is how a change to the targets or to the report reaches a study that took
// an hour.
func renderStudy(in io.Reader, targets study.Targets, newOutput func(string) (io.WriteCloser, error), stdout io.Writer) error {
	var f studyFile
	if err := json.NewDecoder(in).Decode(&f); err != nil {
		return fmt.Errorf("study.json: %w", err)
	}
	if len(f.Outcomes) == 0 {
		return errors.New("study.json: no variant in the file")
	}
	return writeStudy(newOutput, stdout, f.report(targets))
}

// newStudyRunner is a simRunner whose players all play in the crowd of the game, which keeps its
// surplus on a shelf. Without it only the player that trades would, and the Counter of the others
// would be fed a week sooner: the players would not play in one world.
func newStudyRunner(a tuneArgs, runs []tune.Run) (simRunner, error) {
	r, err := newSimRunner(a, runs)
	r.tradingCrowd = true
	return r, err
}

// scenarioRuns returns the runs of a per-set scenario, one per set, in file order.
func scenarioRuns(p tune.Problem, scenario string) ([]tune.Run, error) {
	var runs []tune.Run
	for _, run := range p.Runs() {
		if strings.HasPrefix(run.Name, scenario+"/") {
			runs = append(runs, run)
		}
	}
	if len(runs) == 0 {
		return nil, fmt.Errorf("scenario %q: not a per-set scenario of the goals file", scenario)
	}
	return runs, nil
}

// setRunOf reads what a run of set reported.
func setRunOf(set string, seed uint64, s sim.Summary) study.SetRun {
	code := strings.ToLower(set)
	return study.SetRun{
		Set:      set,
		Seed:     seed,
		Days:     s["days_to_100pct."+code],
		CUDays:   s["days_to_100pct_cu."+code],
		Asks:     s["market_asks"],
		Listings: s["market_listings"],
		Deals:    s["market_deals"],
	}
}

// studySessions is the visit rate of the scenario's runs, or the config's own when it sets none.
func studySessions(runs []tune.Run, base config.Config) int {
	if runs[0].SessionsPerDay > 0 {
		return runs[0].SessionsPerDay
	}
	return base.SessionsPerDay
}

// writeStudy writes the outputs and prints the ranking of the variants.
func writeStudy(newOutput func(string) (io.WriteCloser, error), stdout io.Writer, report study.Report) error {
	if err := writeOutput(newOutput, "study.json", func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(studyFile{
			Scenario: report.Scenario, Horizon: report.Horizon, SessionsPerDay: report.SessionsPerDay,
			Seeds: report.Seeds, Outcomes: report.Outcomes, Verdicts: report.Verdicts,
		})
	}); err != nil {
		return err
	}
	if err := writeOutput(newOutput, "report.html", func(w io.Writer) error { return study.WriteReport(w, report) }); err != nil {
		return err
	}
	return printStudy(stdout, report)
}

// printStudy prints one line per variant, best first.
func printStudy(w io.Writer, r study.Report) error {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "VARIANT\tCOUNTER SPEED\tMARKET SPEED\tFASTEST SET (DAYS)\tMEETS TARGETS")
	for _, v := range r.Verdicts {
		fmt.Fprintf(tw, "%s\t%.2f\t%.2f\t%.0f\t%t\n", v.Name, v.CounterSpeed, v.MarketSpeed, v.Stats[study.Market].Min, v.Pass)
	}
	return tw.Flush()
}

// stopOnFile returns a context that ends when the file at path appears, which it looks for every
// poll, and when the deadline passes, unless the deadline is zero.
func stopOnFile(parent context.Context, path string, poll, deadline time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	stop := cancel
	if deadline > 0 {
		timer := time.AfterFunc(deadline, cancel)
		stop = func() {
			timer.Stop()
			cancel()
		}
	}
	go func() {
		ticker := time.NewTicker(poll)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := os.Stat(path); err == nil {
					cancel()
					return
				}
			}
		}
	}()
	return ctx, stop
}

func newMarketStudyCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		configPath   string
		goalsPath    string
		variantsPath string
		catDir       string
		pricesPath   string
		outDir       string
		scenario     string
		runDays      int
		seeds        []uint
		workers      int
		deadline     time.Duration
		fromPath     string
	)

	cmd := &cobra.Command{
		Use:   "market-study",
		Short: "compare boosters alone, the Counter and the market, under tweaks of the economy",
		Long: `Run three players on each set of a per-set scenario, under each variant of a variants
file, and judge how much faster the Counter and the market make a binder than opening boosters
alone: the Counter should be about a third faster, the market at least twice, and no set done in
under five days.

The players are boosters-only, collector (the Counter) and market-trader. The base world is the
game's own, tune/base.yaml with the play overlay, unless --config gives another. The sets, the
pools and the visit rate come from --scenario of the goals file. A variant is a name and a list of
config overrides.

It writes study.json and report.html to --out and prints the variants, best first. Create a file
named STOP in --out to stop it, which writes nothing.`,
		Example: `  crackapack market-study \
    --goals tune/goals.yaml \
    --variants tune/market-study.yaml \
    --catalogue-dir ./catalogues \
    --prices ./prices.json \
    --out ./study-out`,
		RunE: func(cmd *cobra.Command, args []string) error {
			required := map[string]string{"--out": outDir}
			if fromPath == "" {
				required["--goals"], required["--variants"] = goalsPath, variantsPath
				required["--catalogue-dir"], required["--prices"] = catDir, pricesPath
			}
			for flag, value := range required {
				if value == "" {
					return fmt.Errorf("crackapack market-study: required flag not set: %s", flag)
				}
			}
			if fromPath != "" {
				return renderStudyFile(fromPath, outDir, stdout)
			}
			base, err := studyBaseConfig(configPath)
			if err != nil {
				return fmt.Errorf("crackapack market-study: %w", err)
			}
			goals, err := os.Open(goalsPath)
			if err != nil {
				return fmt.Errorf("crackapack market-study: --goals %q: %w", goalsPath, err)
			}
			defer func() { _ = goals.Close() }()
			variants, err := os.Open(variantsPath)
			if err != nil {
				return fmt.Errorf("crackapack market-study: --variants %q: %w", variantsPath, err)
			}
			defer func() { _ = variants.Close() }()
			if err := os.MkdirAll(outDir, studyOutputDirMode); err != nil {
				return fmt.Errorf("crackapack market-study: --out %q: %w", outDir, err)
			}

			parent := cmd.Context()
			if parent == nil {
				parent = context.Background()
			}
			ctx, cancel := stopOnFile(parent, filepath.Join(outDir, studyStopFile), studyStopPollTime, deadline)
			defer cancel()
			err = runStudy(ctx, studyArgs{
				baseConfig:     base,
				problemReader:  goals,
				scenario:       scenario,
				variantsReader: variants,
				catFS:          os.DirFS(catDir),
				pricesFS:       os.DirFS(filepath.Dir(pricesPath)),
				pricesName:     filepath.Base(pricesPath),
				policies:       buildPolicies(),
				runDays:        runDays,
				seeds:          studySeeds(seeds),
				workers:        workers,
				targets:        studyTargets(),
				newOutput:      func(name string) (io.WriteCloser, error) { return os.Create(filepath.Join(outDir, name)) },
				stdout:         stdout,
			})
			if err != nil {
				return fmt.Errorf("crackapack market-study: %w", err)
			}
			return nil
		},
	}
	cmd.SetOut(stderr)
	cmd.SetErr(stderr)

	f := cmd.Flags()
	f.StringVar(&configPath, "config", "", "YAML config file replacing the game's own world (optional)")
	f.StringVar(&goalsPath, "goals", "", "goals YAML file whose per-set scenario gives the sets (required)")
	f.StringVar(&scenario, "scenario", defaultStudyScenario, "per-set scenario of the goals file")
	f.StringVar(&variantsPath, "variants", "", "variants YAML file (required)")
	f.StringVar(&catDir, "catalogue-dir", "", "catalogue directory of <CODE>.json files (required)")
	f.StringVar(&pricesPath, "prices", "", "JSON price snapshot file (required)")
	f.StringVar(&outDir, "out", "", "output directory (required)")
	f.IntVar(&runDays, "run-days", defaultStudyRunDays, "days each run lasts")
	f.UintSliceVar(&seeds, "seeds", []uint{defaultStudySeed}, "seeds of the runs, comma-separated: the study pools the sets of all of them")
	f.IntVar(&workers, "workers", defaultWorkers, "simulations run at once")
	f.StringVar(&fromPath, "from", "", "study.json of an earlier study: write its report again, with no simulation, judged against today's targets")
	f.DurationVar(&deadline, "deadline", 0, "stop after this long, writing nothing (0 for no deadline)")

	return cmd
}

// renderStudyFile rewrites the outputs in outDir from the study.json at fromPath.
func renderStudyFile(fromPath, outDir string, stdout io.Writer) error {
	in, err := os.Open(fromPath)
	if err != nil {
		return fmt.Errorf("crackapack market-study: --from %q: %w", fromPath, err)
	}
	defer func() { _ = in.Close() }()
	if err := os.MkdirAll(outDir, studyOutputDirMode); err != nil {
		return fmt.Errorf("crackapack market-study: --out %q: %w", outDir, err)
	}
	newOutput := func(name string) (io.WriteCloser, error) { return os.Create(filepath.Join(outDir, name)) }
	if err := renderStudy(in, studyTargets(), newOutput, stdout); err != nil {
		return fmt.Errorf("crackapack market-study: %w", err)
	}
	return nil
}

// studySeeds converts the seeds of the flag, which pflag reads as uint, to the seeds of the engine.
func studySeeds(flag []uint) []uint64 {
	seeds := make([]uint64, len(flag))
	for i, s := range flag {
		seeds[i] = uint64(s)
	}
	return seeds
}

// studyBaseConfig is the game's world, or the config file at path when it is not empty.
func studyBaseConfig(path string) (config.Config, error) {
	if path == "" {
		return playConfig(false, demo.Dataset{})
	}
	f, err := os.Open(path)
	if err != nil {
		return config.Config{}, fmt.Errorf("--config %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return config.Load(f)
}
