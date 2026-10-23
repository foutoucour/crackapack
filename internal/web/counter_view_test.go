package web

import (
	"errors"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
)

func TestHumanAgo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "less than a minute ago"},
		{59 * time.Second, "less than a minute ago"},
		{time.Minute, "1 minute ago"},
		{5*time.Minute + 59*time.Second, "5 minutes ago"},
		{59 * time.Minute, "59 minutes ago"},
		{time.Hour, "1 hour ago"},
		{23*time.Hour + 59*time.Minute, "23 hours ago"},
		{24 * time.Hour, "1 day ago"},
		{72 * time.Hour, "3 days ago"},
	}
	for _, tt := range tests {
		if got := humanAgo(tt.d); got != tt.want {
			t.Errorf("humanAgo(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestCoinsText(t *testing.T) {
	t.Parallel()
	for coins, want := range map[domain.Coins]string{0: "0 coins", 1: "1 coin", 60: "60 coins"} {
		if got := coinsText(coins); got != want {
			t.Errorf("coinsText(%d) = %q, want %q", coins, got, want)
		}
	}
}

// fakeQuotes prices every card by finish, and refuses the cards it does not list.
type fakeQuotes map[domain.CardKey]play.Quote

func (f fakeQuotes) CounterQuote(id domain.CardID, finish domain.Finish) (play.Quote, error) {
	q, ok := f[domain.CardKey{ID: id, Finish: finish}]
	if !ok {
		return play.Quote{}, play.ErrUnknownCard
	}
	return q, nil
}

func card(id string, finishes ...domain.Finish) domain.Card {
	return domain.Card{ID: domain.CardID(id), Name: id, Number: "1", Rarity: domain.RarityCommon, Finishes: finishes}
}

func TestShelfApply(t *testing.T) {
	t.Parallel()
	both := []domain.Finish{domain.FinishNonFoil, domain.FinishFoil}
	quotes := fakeQuotes{
		{ID: "a", Finish: domain.FinishNonFoil}: {BuyPrice: 120, SellPrice: 60, InStock: true},
		{ID: "a", Finish: domain.FinishFoil}:    {BuyPrice: 600, SellPrice: 300},
		{ID: "f", Finish: domain.FinishFoil}:    {BuyPrice: 500, SellPrice: 250, InStock: true},
		{ID: "z", Finish: domain.FinishNonFoil}: {},
		{ID: "o", Finish: domain.FinishNonFoil}: {BuyPrice: 120, SellPrice: 60, LastInStock: time.Unix(1, 0), LastInStockAgo: 5 * time.Minute},
		{ID: "n", Finish: domain.FinishNonFoil}: {BuyPrice: 120, SellPrice: 60},
	}
	tests := []struct {
		name string
		// locked is a binder of a locked set, and noCounter a game without a Counter.
		locked, noCounter bool
		card              play.BinderCard
		sales             []saleView
		buy               buyView
	}{
		{
			name: "a card held in both finishes is sold by finish",
			card: play.BinderCard{Card: card("a", both...), Regular: 2, Foil: 1},
			sales: []saleView{
				{cardRef: cardRef{Set: "S", ID: "a", Finish: "nonfoil"}, Copies: 2, Price: 60, Traded: true},
				{cardRef: cardRef{Set: "S", ID: "a", Finish: "foil"}, Foil: true, Copies: 1, Price: 300, Traded: true},
			},
		},
		{
			name: "a card the Counter pays nothing for is not for sale",
			card: play.BinderCard{Card: card("z", domain.FinishNonFoil), Regular: 1},
			sales: []saleView{
				{cardRef: cardRef{Set: "S", ID: "z", Finish: "nonfoil"}, Copies: 1},
			},
		},
		{
			name: "an empty slot of a card in stock is bought",
			card: play.BinderCard{Card: card("a", both...)},
			buy:  buyView{cardRef: cardRef{Set: "S", ID: "a", Finish: "nonfoil"}, Price: 120, Buyable: true},
		},
		{
			name: "an empty slot of a card that only comes foil is bought foil",
			card: play.BinderCard{Card: card("f", domain.FinishFoil)},
			buy:  buyView{cardRef: cardRef{Set: "S", ID: "f", Finish: "foil"}, Price: 500, Buyable: true},
		},
		{
			name: "an empty slot of a card out of stock says when it was last in stock",
			card: play.BinderCard{Card: card("o", domain.FinishNonFoil)},
			buy:  buyView{cardRef: cardRef{Set: "S", ID: "o", Finish: "nonfoil"}, Price: 120, Note: "Out of stock, last in stock 5 minutes ago"},
		},
		{
			name: "an empty slot of a card never in stock says so",
			card: play.BinderCard{Card: card("n", domain.FinishNonFoil)},
			buy:  buyView{cardRef: cardRef{Set: "S", ID: "n", Finish: "nonfoil"}, Price: 120, Note: "Out of stock, never seen at the Counter"},
		},
		{
			name: "an empty slot of a card worth nothing is not traded",
			card: play.BinderCard{Card: card("z", domain.FinishNonFoil)},
			buy:  buyView{cardRef: cardRef{Set: "S", ID: "z", Finish: "nonfoil"}, Note: "Not traded"},
		},
		{
			name:   "an empty slot of a locked set offers nothing",
			locked: true,
			card:   play.BinderCard{Card: card("a", both...)},
		},
		{
			name:      "a game without a Counter offers nothing",
			noCounter: true,
			card:      play.BinderCard{Card: card("a", both...), Regular: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sh := shelf{set: "S", locked: tt.locked}
			if !tt.noCounter {
				sh.quotes = quotes
			}
			slot := slotView{Regular: tt.card.Regular, Foil: tt.card.Foil}

			if err := sh.apply(&slot, tt.card); err != nil {
				t.Fatalf("apply: %v", err)
			}

			if !equalSales(slot.Sales, tt.sales) || slot.Buy != tt.buy {
				t.Errorf("sales = %+v, buy = %+v, want %+v and %+v", slot.Sales, slot.Buy, tt.sales, tt.buy)
			}
		})
	}
}

func TestShelfApply_aQuoteThatFailsFails(t *testing.T) {
	t.Parallel()
	sh := shelf{set: "S", quotes: fakeQuotes{}}
	err := sh.apply(&slotView{}, play.BinderCard{Card: card("a", domain.FinishNonFoil)})
	if !errors.Is(err, play.ErrUnknownCard) {
		t.Errorf("apply error = %v, want ErrUnknownCard", err)
	}
}

func equalSales(a, b []saleView) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
