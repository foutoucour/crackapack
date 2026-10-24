package sim_test

import (
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// fakeMarketSession is a fakeSession with a market that records what the policy asks, lists and
// accepts, and answers with the answers it is given.
type fakeMarketSession struct {
	fakeSession
	answers  []domain.MarketAnswer
	asked    []domain.CardKey
	listed   map[domain.CardKey]domain.Coins
	accepted []domain.MarketID
}

func (f *fakeMarketSession) MarketAsk(card domain.CardKey) (domain.MarketID, error) {
	f.asked = append(f.asked, card)
	return domain.MarketID(len(f.asked)), nil
}

func (f *fakeMarketSession) MarketList(card domain.CardKey, price domain.Coins) (domain.MarketID, error) {
	if f.listed == nil {
		f.listed = map[domain.CardKey]domain.Coins{}
	}
	f.listed[card] = price
	return domain.MarketID(len(f.listed)), nil
}

func (f *fakeMarketSession) MarketAnswers() []domain.MarketAnswer { return f.answers }

func (f *fakeMarketSession) MarketAccept(id domain.MarketID) error {
	f.accepted = append(f.accepted, id)
	return nil
}

func nonFoil(id domain.CardID) domain.CardKey {
	return domain.CardKey{ID: id, Finish: domain.FinishNonFoil}
}

// marketTraderWorld is a set of two rares, a mythic, a common and a basic land, with the player
// holding the rare "owned-rare" twice and the common "common" once, and a balance far above the coin
// floor. Every card is worth $5, which is 500 coins at a rate of 100.
func marketTraderWorld(balance domain.Coins) (sim.WorldView, domain.PriceSnapshot) {
	finishes := []domain.Finish{domain.FinishNonFoil}
	cards := map[domain.CardID]domain.Card{
		"missing-rare":   {ID: "missing-rare", Rarity: domain.RarityRare, Finishes: finishes},
		"missing-mythic": {ID: "missing-mythic", Rarity: domain.RarityMythic, Finishes: finishes},
		"owned-rare":     {ID: "owned-rare", Rarity: domain.RarityRare, Finishes: finishes},
		"common":         {ID: "common", Rarity: domain.RarityCommon, Finishes: finishes},
		"missing-common": {ID: "missing-common", Rarity: domain.RarityCommon, Finishes: finishes},
		"forest":         {ID: "forest", Rarity: domain.RarityCommon, Finishes: finishes, IsBasicLand: true},
	}
	prices := domain.PriceSnapshot{}
	for id := range cards {
		prices[id] = 500
	}
	owned := map[domain.CardKey]int{nonFoil("owned-rare"): 2, nonFoil("common"): 1}
	cats := []domain.SetCatalogue{{Code: "SET", Cards: cards}}
	// No progress and no energy: the policy opens no booster.
	v := makeView(balance, 100, 0, map[string]domain.Progress{}, map[string]int{"SET": 5}, cats, prices, owned)
	return v, prices
}

func newFakeMarketSession(v sim.WorldView) *fakeMarketSession {
	return &fakeMarketSession{fakeSession: fakeSession{views: []sim.WorldView{v}}}
}

func TestMarketTrader_usesTheMarket(t *testing.T) {
	t.Parallel()

	if !sim.NewMarketTraderPolicy().UsesMarket() {
		t.Error("UsesMarket = false, want true")
	}
}

func TestMarketTrader_asksAroundForMissingRaresAndMythicsOnly(t *testing.T) {
	t.Parallel()
	v, _ := marketTraderWorld(5000)
	fs := newFakeMarketSession(v)

	sim.NewMarketTraderPolicy().RunSession(fs)

	want := []domain.CardKey{nonFoil("missing-mythic"), nonFoil("missing-rare")}
	got := slices.Clone(fs.asked)
	slices.SortFunc(got, func(a, b domain.CardKey) int { return compareIDs(a.ID, b.ID) })
	if !slices.Equal(got, want) {
		t.Errorf("asked around for %v, want the missing rare and mythic %v: not owned cards, commons or basic lands", got, want)
	}
}

func compareIDs(a, b domain.CardID) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func TestMarketTrader_listsDuplicatesAtTheirReferenceValue(t *testing.T) {
	t.Parallel()
	v, _ := marketTraderWorld(5000)
	fs := newFakeMarketSession(v)

	sim.NewMarketTraderPolicy().RunSession(fs)

	want := map[domain.CardKey]domain.Coins{nonFoil("owned-rare"): 500}
	if len(fs.listed) != len(want) || fs.listed[nonFoil("owned-rare")] != 500 {
		t.Errorf("listed %v, want %v: the card held twice at $5, nothing for a single copy", fs.listed, want)
	}
}

func ask(id, to domain.MarketID, card domain.CardID, npc int, coins domain.Coins) domain.MarketAnswer {
	return domain.MarketAnswer{ID: id, To: to, Kind: domain.AnswerAsk, Card: nonFoil(card), NPC: npc, Coins: coins}
}

func TestMarketTrader_acceptsTheCheapestAffordableAskOfARequest(t *testing.T) {
	t.Parallel()
	v, _ := marketTraderWorld(1000)
	fs := newFakeMarketSession(v)
	fs.answers = []domain.MarketAnswer{
		ask(10, 1, "missing-rare", 0, 600),
		ask(11, 1, "missing-rare", 1, 450),
		ask(12, 1, "missing-rare", 2, 100000),
	}

	sim.NewMarketTraderPolicy().RunSession(fs)

	if !slices.Equal(fs.accepted, []domain.MarketID{11}) {
		t.Errorf("accepted %v, want only the cheapest answer 11", fs.accepted)
	}
}

func TestMarketTrader_keepsTheCoinFloorWhenAccepting(t *testing.T) {
	t.Parallel()
	// A balance of 500 and a floor of one daily reward, 100, leave 400 to spend.
	v, _ := marketTraderWorld(500)
	fs := newFakeMarketSession(v)
	fs.answers = []domain.MarketAnswer{ask(10, 1, "missing-rare", 0, 450)}

	sim.NewMarketTraderPolicy().RunSession(fs)

	if len(fs.accepted) != 0 {
		t.Errorf("accepted %v, want none: paying 450 would leave under the floor", fs.accepted)
	}
}

func TestMarketTrader_doesNotBuyACardItOwns(t *testing.T) {
	t.Parallel()
	v, _ := marketTraderWorld(5000)
	fs := newFakeMarketSession(v)
	// An answer can outlive the purchase of its card by another route, such as a booster.
	fs.answers = []domain.MarketAnswer{ask(10, 1, "owned-rare", 0, 100)}

	sim.NewMarketTraderPolicy().RunSession(fs)

	if len(fs.accepted) != 0 {
		t.Errorf("accepted %v, want none: the player owns the card", fs.accepted)
	}
}

func offer(id, to domain.MarketID, coins domain.Coins, cards ...domain.CardID) domain.MarketAnswer {
	a := domain.MarketAnswer{ID: id, To: to, Kind: domain.AnswerOffer, Card: nonFoil("owned-rare"), Coins: coins}
	for _, c := range cards {
		a.Cards = append(a.Cards, domain.TradeLine{Key: nonFoil(c), Qty: 1})
	}
	return a
}

func TestMarketTrader_prefersAnOfferOfAMissingCardOverCoins(t *testing.T) {
	t.Parallel()
	v, _ := marketTraderWorld(5000)
	fs := newFakeMarketSession(v)
	fs.answers = []domain.MarketAnswer{
		offer(20, 1, 450),
		offer(21, 1, 0, "missing-common"),
	}

	sim.NewMarketTraderPolicy().RunSession(fs)

	if len(fs.accepted) == 0 || fs.accepted[0] != 21 {
		t.Errorf("accepted %v, want the offer of a missing card 21 first", fs.accepted)
	}
}

func TestMarketTrader_acceptsTheHighestCoinOfferWhenNoOfferGivesAMissingCard(t *testing.T) {
	t.Parallel()
	v, _ := marketTraderWorld(5000)
	fs := newFakeMarketSession(v)
	fs.answers = []domain.MarketAnswer{
		offer(20, 1, 300),
		offer(21, 1, 450),
		offer(22, 1, 0, "common"),
	}

	sim.NewMarketTraderPolicy().RunSession(fs)

	if len(fs.accepted) == 0 || fs.accepted[0] != 21 {
		t.Errorf("accepted %v, want the highest coin offer 21 first: the card offered is owned already", fs.accepted)
	}
}

func TestMarketTrader_sellsDuplicatesAndBuysLikeTheCollector(t *testing.T) {
	t.Parallel()
	v, _ := marketTraderWorld(5000)
	fs := newFakeMarketSession(v)

	sim.NewMarketTraderPolicy().RunSession(fs)

	if !slices.Equal(fs.sellCalls, []int{domain.PlaysetSize}) {
		t.Errorf("SellDuplicates calls = %v, want one call keeping a playset", fs.sellCalls)
	}
	if len(fs.buyCalls) == 0 {
		t.Error("BuyCard never called, want the collector's Counter purchases of missing cards")
	}
}
