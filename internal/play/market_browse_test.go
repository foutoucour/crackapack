package play_test

import (
	"errors"
	"testing"
	"time"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

func mustForSale(t *testing.T, g *play.Game) []play.ForSale {
	t.Helper()
	sales, err := g.ForSale()
	if err != nil {
		t.Fatalf("ForSale: %v", err)
	}
	return sales
}

func saleOf(t *testing.T, sales []play.ForSale, key domain.CardKey) play.ForSale {
	t.Helper()
	for _, s := range sales {
		if s.Card.ID == key.ID && s.Finish == key.Finish {
			return s
		}
	}
	t.Fatalf("%v is not for sale in %+v", key, sales)
	return play.ForSale{}
}

// The cards on the shelves of the NPCs that trade are for sale, each NPC with its name, its
// personality and its asking price, the cheapest first.
func TestForSale_listsTheShelvesWithWhoAsksWhat(t *testing.T) {
	t.Parallel()
	g := counterGame(t, &memStore{}, playtest.NewClock(), marketRules(), counterSet())

	sale := saleOf(t, mustForSale(t, g), cheap())

	if sale.SetCode != "S" || !sale.Missing || sale.Hunted {
		t.Errorf("sale = %+v, want a missing card, not hunted, of set S", sale)
	}
	if len(sale.Sellers) != 2 {
		t.Fatalf("sellers = %+v, want the fast and the slow NPC", sale.Sellers)
	}
	fast, slow := sale.Sellers[0], sale.Sellers[1]
	if fast.NPC != fastNPC || fast.Price != 100 || fast.Archetype != "a_fast" || fast.Name == "" {
		t.Errorf("first seller = %+v, want the fast NPC at 100 coins with its name", fast)
	}
	if slow.NPC != slowNPC || slow.Price != 110 || slow.Archetype != "b_slow" {
		t.Errorf("second seller = %+v, want the slow NPC at 110 coins", slow)
	}
}

func TestForSale_saysWhichCardsAreHuntedAndWhichAreHeld(t *testing.T) {
	t.Parallel()
	g := counterGame(t, &memStore{}, playtest.NewClock(), marketRules(), counterSet())
	mustHunt(t, g, dear())
	if _, err := g.BuyNow(fastNPC, cheap().ID, cheap().Finish); err != nil {
		t.Fatalf("BuyNow: %v", err)
	}

	sales := mustForSale(t, g)

	if s := saleOf(t, sales, dear()); !s.Hunted || !s.Missing {
		t.Errorf("hunted card = %+v, want hunted and missing", s)
	}
	if s := saleOf(t, sales, cheap()); s.Hunted || s.Missing {
		t.Errorf("held card = %+v, want neither hunted nor missing", s)
	}
}

func TestForSale_aGameWithoutAMarket(t *testing.T) {
	t.Parallel()
	g := counterGame(t, &memStore{}, playtest.NewClock(), counterRules(), counterSet())
	if _, err := g.ForSale(); !errors.Is(err, play.ErrNoMarket) {
		t.Errorf("ForSale error = %v, want ErrNoMarket", err)
	}
}

func TestBuyNow_paysTheAskingPriceAndSavesTheGame(t *testing.T) {
	t.Parallel()
	store := &memStore{}
	g := counterGame(t, store, playtest.NewClock(), marketRules(), counterSet())

	if _, err := g.BuyNow(slowNPC, cheap().ID, cheap().Finish); err != nil {
		t.Fatalf("BuyNow: %v", err)
	}

	if g.Balance() != 90 || held(g, "S", cheap()) != 1 {
		t.Errorf("balance %d, copies %d, want 90 and 1", g.Balance(), held(g, "S", cheap()))
	}
	sale := saleOf(t, mustForSale(t, g), cheap())
	before := saleOf(t, mustForSale(t, counterGame(t, &memStore{}, playtest.NewClock(), marketRules(), counterSet())), cheap())
	if got, want := sale.Sellers[1].Qty, before.Sellers[1].Qty-1; got != want {
		t.Errorf("the slow NPC holds %d copies after the purchase, want %d: the copy left its shelf", got, want)
	}
	if got := storedCopies(store.last(t), cheap()); got != 1 {
		t.Errorf("saved copies = %d, want 1", got)
	}
	if r := mustMarket(t, g).Results; len(r) != 1 || r[0].Outcome != domain.MarketAccepted {
		t.Errorf("results = %+v, want the purchase", r)
	}
}

func TestBuyNow_refusalsChangeNothing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		npc  int
		card domain.CardKey
		want error
	}{
		{name: "the NPC has no copy", npc: specNPC, card: cheap(), want: play.ErrGone},
		{name: "the coins are short", npc: fastNPC, card: dear(), want: play.ErrNotEnoughCoins},
		{name: "an unknown card", npc: slowNPC, card: domain.CardKey{ID: "nope", Finish: domain.FinishNonFoil}, want: play.ErrUnknownCard},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := counterGame(t, &memStore{}, playtest.NewClock(), marketRules(), counterSet())
			before := g.Balance()

			_, err := g.BuyNow(tc.npc, tc.card.ID, tc.card.Finish)

			if !errors.Is(err, tc.want) {
				t.Fatalf("BuyNow error = %v, want %v", err, tc.want)
			}
			if g.Balance() != before {
				t.Errorf("balance %d, want %d", g.Balance(), before)
			}
			if got := held(g, "S", tc.card); got > 0 {
				t.Errorf("copies %d, want none", got)
			}
		})
	}
}

func TestOfferTo_theNPCRepliesOnceAfterItsDelay(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	store := &memStore{}
	g := counterGame(t, store, clock, marketRules(), counterSet())

	if _, err := g.OfferTo(slowNPC, cheap().ID, cheap().Finish, 105); err != nil {
		t.Fatalf("OfferTo: %v", err)
	}

	v := mustMarket(t, g)
	if len(v.Requests) != 1 || !v.Requests[0].Direct || v.Requests[0].Stage != play.StageWaiting {
		t.Fatalf("requests = %+v, want one direct request that waits on an NPC", v.Requests)
	}
	a := answerOf(t, v, slowNPC)
	if a.Coins != 110 || a.Counter == nil || a.Counter.Coins != 105 || !a.Counter.ReplyAt.Equal(clock.Now().Add(slowDelay)) {
		t.Errorf("answer = %+v, want 110 coins countered with 105, replying after the delay", a)
	}
	if !v.Requests[0].NextAt.IsZero() {
		t.Errorf("NextAt = %v, want none: no NPC is to answer, one is to reply", v.Requests[0].NextAt)
	}

	clock.Advance(slowDelay)
	v = mustMarket(t, g)
	if len(v.Requests) != 0 || len(v.Results) != 1 || v.Results[0].Outcome != domain.MarketRefused {
		t.Errorf("market after the reply = %+v, want the refusal as the only result", v)
	}
	if g.Balance() != 200 {
		t.Errorf("balance %d, want 200", g.Balance())
	}
}

func TestOfferTo_isSavedWithTheGame(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	store := &memStore{}
	g := counterGame(t, store, clock, marketRules(), counterSet())
	if _, err := g.OfferTo(slowNPC, cheap().ID, cheap().Finish, 105); err != nil {
		t.Fatalf("OfferTo: %v", err)
	}

	again := counterGame(t, store.copied(), clock, marketRules(), counterSet())

	if v := mustMarket(t, again); len(v.Requests) != 1 || !v.Requests[0].Direct {
		t.Errorf("requests after a reload = %+v, want the direct request", v.Requests)
	}
}

func TestOfferTo_refusals(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		npc   int
		coins domain.Coins
		want  error
	}{
		{name: "no coin", npc: slowNPC, coins: 0, want: play.ErrInvalidPrice},
		{name: "not below the asking price", npc: slowNPC, coins: 110, want: play.ErrOfferNotBelowAsk},
		{name: "the NPC has no copy", npc: specNPC, coins: 50, want: play.ErrGone},
		{name: "more than the player has", npc: fastNPC, coins: 249, want: play.ErrNotEnoughCoins},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := counterGame(t, &memStore{}, playtest.NewClock(), marketRules(), counterSet())

			card := cheap()
			if tc.want == play.ErrNotEnoughCoins {
				card = dear()
			}

			_, err := g.OfferTo(tc.npc, card.ID, card.Finish, tc.coins)

			if !errors.Is(err, tc.want) {
				t.Errorf("OfferTo error = %v, want %v", err, tc.want)
			}
			if v := mustMarket(t, g); len(v.Requests) != 0 {
				t.Errorf("requests = %+v, want none", v.Requests)
			}
		})
	}
}

// A trade says whom it waits on: the player, when an answer is open, and an NPC otherwise, with the
// next instant an NPC answers and the first an answer lapses.
func TestMarket_eachTradeSaysWhoItWaitsOn(t *testing.T) {
	t.Parallel()
	clock := playtest.NewClock()
	g := counterGame(t, &memStore{}, clock, marketRules(), counterSet())
	start := clock.Now()
	mustAsk(t, g, cheap())

	// The fast NPC has answered, the slow one has not.
	r := mustMarket(t, g).Requests[0]
	if r.Stage != play.StageNeedsYou || !r.NextAt.Equal(start.Add(slowDelay)) || !r.LapsesAt.Equal(start.Add(3*day)) {
		t.Errorf("request = %+v, want it to need the player, next at +%v, lapsing in 3 days", r, slowDelay)
	}

	// Once the player declines the answer there is only the slow NPC left to wait for.
	if err := g.DeclineAnswer(r.Answers[0].ID); err != nil {
		t.Fatalf("DeclineAnswer: %v", err)
	}
	r = mustMarket(t, g).Requests[0]
	if r.Stage != play.StageWaiting || !r.LapsesAt.IsZero() {
		t.Errorf("request = %+v, want it to wait on an NPC with nothing lapsing", r)
	}

	// A listing waits for the offers of the NPCs that want the card: here the speculator, in an hour.
	clock.Advance(time.Hour)
	if _, err := g.BuyNow(fastNPC, cheap().ID, cheap().Finish); err != nil {
		t.Fatalf("BuyNow: %v", err)
	}
	if _, err := g.List(cheap().ID, cheap().Finish, 500); err != nil {
		t.Fatalf("List: %v", err)
	}
	if l := mustMarket(t, g).Listings[0]; l.Stage != play.StageWaiting || !l.NextAt.Equal(clock.Now().Add(time.Hour)) {
		t.Errorf("listing = %+v, want it to wait on an NPC, who answers in an hour", l)
	}
}
