package domain_test

import (
	"maps"
	"math/rand/v2"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// newTestSupply builds a Counter that starts empty and a fixed supply that stocks it from the
// crowd test sets, on a random stream of its own.
func newTestSupply(t *testing.T, boostersPerDay int, shelf domain.ShelfLife, seed uint64) (*domain.CounterSupply, *domain.Counter) {
	t.Helper()
	sets := crowdSets()
	counter, err := domain.NewCounter(sets, domain.PriceSnapshot{}, domain.NewJournal(), domain.CounterParams{
		SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: 10, ShelfLife: shelf,
	})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	rng := rand.New(rand.NewPCG(seed, seed))
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
	supply, err := domain.NewFixedCounterSupply(counter, sets, opener, rng, boostersPerDay)
	if err != nil {
		t.Fatalf("NewFixedCounterSupply: %v", err)
	}
	return supply, counter
}

// runSupplyDays ticks the Counter and its supply for days days and returns what the supply opened.
func runSupplyDays(t *testing.T, supply *domain.CounterSupply, counter *domain.Counter, days int) domain.SupplyTick {
	t.Helper()
	var total domain.SupplyTick
	for range days * domain.TicksPerDay {
		counter.Tick()
		tick, err := supply.Tick()
		if err != nil {
			t.Fatalf("supply Tick: %v", err)
		}
		total.BoostersOpened += tick.BoostersOpened
		total.CardsOpened += tick.CardsOpened
	}
	return total
}

func TestFixedCounterSupplyOpensItsDailyRhythm(t *testing.T) {
	t.Parallel()
	supply, counter := newTestSupply(t, 3, domain.ForeverOnShelf(), 1)

	opened := runSupplyDays(t, supply, counter, 2)

	if opened.BoostersOpened != 6 {
		t.Errorf("opened %d boosters in 2 days, want 6", opened.BoostersOpened)
	}
	if opened.CardsOpened == 0 {
		t.Fatal("no card was opened")
	}
	if got := totalStock(counter); got != opened.CardsOpened {
		t.Errorf("stock = %d, want the %d cards opened: they go on the shelf", got, opened.CardsOpened)
	}
}

func TestFixedCounterSupplyStocksLikeSoldCards(t *testing.T) {
	t.Parallel()
	supply, counter := newTestSupply(t, 3, domain.VanishAfterDays(0), 1)

	opened := runSupplyDays(t, supply, counter, 2)

	if opened.CardsOpened == 0 {
		t.Fatal("no card was opened")
	}
	if got := totalStock(counter); got != 0 {
		t.Errorf("stock = %d, want 0: opened cards share the shelf life of sold cards", got)
	}
}

func TestFixedCounterSupplyIsReproducibleFromTheSeed(t *testing.T) {
	t.Parallel()
	a, ca := newTestSupply(t, 4, domain.ForeverOnShelf(), 7)
	b, cb := newTestSupply(t, 4, domain.ForeverOnShelf(), 7)

	runSupplyDays(t, a, ca, 2)
	runSupplyDays(t, b, cb, 2)

	if !maps.Equal(ca.StockByRarity(), cb.StockByRarity()) {
		t.Errorf("stock %v and %v differ for the same seed", ca.StockByRarity(), cb.StockByRarity())
	}
}

func TestNewFixedCounterSupplyRejectsBadInput(t *testing.T) {
	t.Parallel()
	sets := crowdSets()
	rng := rand.New(rand.NewPCG(1, 1))
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
	counter, err := domain.NewCounter(sets, domain.PriceSnapshot{}, domain.NewJournal(), domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	tests := []struct {
		name    string
		counter *domain.Counter
		sets    []domain.SetCatalogue
		opener  *domain.Opener
		perDay  int
	}{
		{name: "no counter", counter: nil, sets: sets, opener: opener, perDay: 1},
		{name: "no sets", counter: counter, sets: nil, opener: opener, perDay: 1},
		{name: "no opener", counter: counter, sets: sets, opener: nil, perDay: 1},
		{name: "zero boosters a day", counter: counter, sets: sets, opener: opener, perDay: 0},
		{name: "negative boosters a day", counter: counter, sets: sets, opener: opener, perDay: -3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := domain.NewFixedCounterSupply(tt.counter, tt.sets, tt.opener, rng, tt.perDay); err == nil {
				t.Error("want an error, got nil")
			}
		})
	}
}
