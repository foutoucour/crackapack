package demo

import (
	"io/fs"
	"math/rand/v2"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

// Dataset holds the demo inputs produced by Build.
// Files is an fs.FS containing DEM.json, DSP.json, DST.json, and prices.json.
// These files are generated in-memory; no data is committed to the repository.
type Dataset struct {
	// Files is a virtual filesystem with DEM.json, DSP.json, DST.json and prices.json.
	Files fs.FS

	// ConfigYAML is the YAML configuration string for the demo sweep, including
	// the default sweep grid and calibration targets.
	ConfigYAML string

	// SetCodes lists the openable set codes in the demo (DEM, DSP).
	SetCodes []string

	// SupplementaryCodes lists the card-pool-only codes (DST).
	SupplementaryCodes []string
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

	cats := []domain.SetCatalogue{dem, dsp}
	supps := []domain.SetCatalogue{dst}

	mapFS, err := domaintest.NewMapFS(cats, supps, prices)
	if err != nil {
		// NewMapFS serialises well-formed synthetic data; failure is a code error.
		panic("demo.Build: " + err.Error())
	}

	return Dataset{
		Files:              mapFS,
		ConfigYAML:         demoConfigYAML,
		SetCodes:           []string{"DEM", "DSP"},
		SupplementaryCodes: []string{"DST"},
	}
}
