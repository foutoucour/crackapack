package sim_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/sim"
)

// BenchmarkRun measures one 90-day run with 500 NPCs, without and with trades, on the synthetic
// catalogues. It is the reference for the cost of a run in a sweep or a search: run it before
// and after a change to the crowd or the Exchange.
//
//	go test ./internal/sim -run '^$' -bench BenchmarkRun -benchmem
func BenchmarkRun(b *testing.B) {
	for _, trades := range []bool{false, true} {
		b.Run(fmt.Sprintf("trades=%t", trades), func(b *testing.B) {
			yaml := "run_days: 90\nnpcs: {population: 500}\n"
			if trades {
				yaml += "trades: {enabled: true}\n"
			}
			cfg, err := config.Load(strings.NewReader(yaml))
			if err != nil {
				b.Fatalf("Load: %v", err)
			}
			cats := testCatalogues()
			prices := mergedPrices(cats, cfg.Seed)

			for b.Loop() {
				var policy sim.PlayerPolicy = sim.NewCollectorPolicy()
				if trades {
					policy = sim.NewTraderPolicy()
				}
				engine, err := sim.NewEngine(cfg, cats, nil, prices, policy, newRNG(cfg.Seed))
				if err != nil {
					b.Fatalf("NewEngine: %v", err)
				}
				if _, err := engine.Run(); err != nil {
					b.Fatalf("Run: %v", err)
				}
			}
		})
	}
}
