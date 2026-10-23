package demo

import (
	"bytes"
	"io/fs"
	"math/rand/v2"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
	"github.com/foutoucour/crackapack/internal/images"
)

// Dataset holds the demo inputs produced by Build.
// Files is an fs.FS containing DEM.json, DSP.json, DEX.json, DST.json, and prices.json.
// These files are generated in-memory; no data is committed to the repository. DSP.json holds its
// booster as "default", like the MTGJSON sets before Mirage.
type Dataset struct {
	// Files is a virtual filesystem with DEM.json, DSP.json, DEX.json, DST.json and prices.json.
	Files fs.FS

	// ConfigYAML is the YAML configuration string for the demo sweep, including
	// the default sweep grid and calibration targets.
	ConfigYAML string

	// PlayConfigYAML is the configuration of crackapack play --demo: stages small enough that a
	// player unlocks the second set in a few boosters, and a Counter that stocks itself, with its
	// mythics out of stock at the start.
	PlayConfigYAML string

	// SetCodes lists the openable set codes in the demo (DEM, DSP). DEX is in the directory but is
	// an expensive set, so it is not played.
	SetCodes []string

	// SupplementaryCodes lists the card-pool-only codes (DST).
	SupplementaryCodes []string

	// Images draws a picture for each demo card, by its Scryfall ID, so the demo shows card
	// pictures without the network.
	Images images.Source
	// TuneConfigYAML, TuneSpaceYAML and TuneGoalsYAML are the base config, the search space and
	// the goals of the demo search.
	TuneConfigYAML string
	TuneSpaceYAML  string
	TuneGoalsYAML  string
}

// Build constructs the demo dataset from a fixed seed. The result is
// deterministic: the same binary produces byte-identical output on every run.
// Build panics only on internal programming errors (JSON serialisation failures),
// which cannot occur with well-formed synthetic data.
func Build() Dataset {
	rng := rand.New(rand.NewPCG(demSeed, demSeed))

	dem := newDEMCatalogue()
	dsp, dst := newDSPCatalogues()

	prices := newDemoPrices([]domain.SetCatalogue{dem, dsp, dst}, rng)

	// DEX is priced apart from the others, so the draws of the played sets do not move.
	dex := newDEXCatalogue()
	prices = domaintest.MergePrices(prices, newDEXPrices(dex))

	cats := []domain.SetCatalogue{dem, dsp, dex}
	supps := []domain.SetCatalogue{dst}

	assignScryfallIDs(dem, dsp, dst)

	mapFS, err := domaintest.NewMapFS(cats, supps, prices)
	if err != nil {
		// NewMapFS serialises well-formed synthetic data; failure is a code error.
		panic("demo.Build: " + err.Error())
	}

	// MTGJSON serves the sets before Mirage with a "default" booster and no "draft" one. DSP
	// is served that way, so the demo exercises the loader's fallback.
	mapFS["DSP.json"].Data = bytes.Replace(mapFS["DSP.json"].Data, []byte(`"draft"`), []byte(`"default"`), 1)

	return Dataset{
		Files:              mapFS,
		ConfigYAML:         demoConfigYAML,
		PlayConfigYAML:     playConfigYAML,
		SetCodes:           []string{"DEM", "DSP"},
		SupplementaryCodes: []string{"DST"},
		Images:             newCardArt(dem, dsp, dst),
		TuneConfigYAML:     TuneConfigYAML,
		TuneSpaceYAML:      TuneSpaceYAML,
		TuneGoalsYAML:      TuneGoalsYAML,
	}
}
