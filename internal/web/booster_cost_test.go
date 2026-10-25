package web_test

import (
	"encoding/json"
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/play"
	"github.com/foutoucour/crackapack/internal/play/playtest"
)

// In these tests S asks a fee on top of its energy and T does not, a booster of either costs
// playtest.Cost energy, and the player starts with startingCoins.
const (
	boosterFee    = domain.Coins(150)
	energyPrice   = domain.Coins(40)
	startingCoins = domain.Coins(1000)
)

// newCostShop serves a game on the sets S and T whose welcome boosters are opened, so that the next
// opening costs. topUp offers the coin top-up.
func newCostShop(t *testing.T, topUp bool, coins domain.Coins) counterShop {
	t.Helper()
	sets := []domain.SetCatalogue{playtest.TinySet("S", 3, 3), playtest.TinySet("T", 3, 3)}
	rules := playtest.Rules(sets...)
	rules.Fees = map[string]domain.Coins{"S": boosterFee}
	if topUp {
		rules.TopUp = &play.TopUpRules{Price: energyPrice, MinEnergy: 1}
	}
	clock := playtest.NewClock()
	game, err := play.NewGame(sets, nil, newOpener(), rules, clock.Now)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	counter := counterRules()
	counter.WelcomeBonus = coins
	counter.Prices = domain.PriceSnapshot{}
	if err := game.OpenCounter(counter, play.NewStream(7), play.NewStream(8)); err != nil {
		t.Fatalf("OpenCounter: %v", err)
	}
	for _, pack := range game.Packs() {
		for range pack.Count {
			if _, err := game.Open(pack.SetCode); err != nil {
				t.Fatalf("Open: %v", err)
			}
		}
	}
	// A booster left unturned takes the place of its tile's price: turn the last one.
	if last, ok := game.LastOpening(); ok {
		if err := game.Reveal(len(last.Cards)); err != nil {
			t.Fatalf("Reveal: %v", err)
		}
	}
	srv, c, _ := serve(t, game, fakeImages{})
	return counterShop{srv: srv, c: c, game: game, clock: clock}
}

// drain opens two boosters of T, which empties the reserve of ten.
func (s counterShop) drain(t *testing.T) {
	t.Helper()
	for range 2 {
		opening, err := s.game.Open("T")
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		// A booster left unturned takes the place of its tile's price.
		if err := s.game.Reveal(len(opening.Cards)); err != nil {
			t.Fatalf("Reveal: %v", err)
		}
	}
}

func TestHome_aBoosterWithAFeeSaysSo(t *testing.T) {
	t.Parallel()
	shop := newCostShop(t, false, startingCoins)

	body := shop.get(t, "/")

	mustContain(t, "the home page", body, "◎150 fee")
}

func TestOpen_aBoosterWithAFeeTakesTheFeeAndTheEnergy(t *testing.T) {
	t.Parallel()
	shop := newCostShop(t, false, startingCoins)

	resp, _ := shop.post(t, "/open", url.Values{"set": {"S"}})

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /open = %d, want 303", resp.StatusCode)
	}
	if got, want := shop.game.Balance(), startingCoins-boosterFee; got != want {
		t.Errorf("balance %d, want %d", got, want)
	}
}

func TestOpen_aFeeThatTheCoinsCannotPayIsRefusedWithWhatItCosts(t *testing.T) {
	t.Parallel()
	shop := newCostShop(t, false, boosterFee-1)

	resp, body := shop.post(t, "/open", url.Values{"set": {"S"}})

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("POST /open = %d, want 409", resp.StatusCode)
	}
	mustContain(t, "the refusal", body, "Not enough coins", "◎150", "◎149")
}

func TestHome_aTopUpIsOfferedOnlyToAPlayerShortOfEnergyWithSomeLeft(t *testing.T) {
	t.Parallel()
	shop := newCostShop(t, true, startingCoins)
	mustNotContain(t, "the page with a full reserve", shop.get(t, "/"), `name="topup" value="yes"`)

	shop.drain(t)
	mustNotContain(t, "the page with no energy", shop.get(t, "/"), `name="topup" value="yes"`)

	shop.clock.Advance(3 * playtest.Tick)
	mustContain(t, "the page with 3 energy", shop.get(t, "/"), `name="topup" value="yes"`)
}

func TestHome_aTopUpIsTheBoostersOwnPriceInEnergyAndCoinsNotAButtonOfItsOwn(t *testing.T) {
	t.Parallel()
	shop := newCostShop(t, true, startingCoins)
	shop.drain(t)
	shop.clock.Advance(3 * playtest.Tick)

	body := html.UnescapeString(shop.get(t, "/"))

	// The 3 energy the player has go into the booster, T needs 2 more: 80 coins. S adds its fee: 230.
	mustContain(t, "the page with 3 energy", body,
		`<span data-pack-price>3 + ◎80</span>`, `<span data-pack-price>3 + ◎230</span>`,
		`, 3 energy and ◎80"`, "Short of 2 energy")
	mustNotContain(t, "the page with 3 energy", body, "Top up", " disabled")
}

func TestHome_aBoosterPaidInEnergyAloneNamesNoCoins(t *testing.T) {
	t.Parallel()
	shop := newCostShop(t, true, startingCoins)

	body := html.UnescapeString(shop.get(t, "/"))

	mustContain(t, "the page with a full reserve", body,
		`<span data-pack-price>5</span>`, `<span data-pack-price>5 + ◎150</span>`)
}

func TestEnergy_theEndpointSaysTheWholePriceOfEachSetSoTheButtonsFollowTheGauge(t *testing.T) {
	t.Parallel()
	shop := newCostShop(t, true, startingCoins)
	shop.drain(t)
	shop.clock.Advance(3 * playtest.Tick)

	var got struct {
		Packs []struct {
			Set, Wait, Price, Label, Hint string
			TopUp                         bool
		}
	}
	if err := json.Unmarshal([]byte(shop.get(t, "/energy")), &got); err != nil {
		t.Fatalf("the body is not JSON: %v", err)
	}

	for _, p := range got.Packs {
		if p.Set != "T" {
			continue
		}
		if p.Price != "3 + ◎80" || p.Label != "3 energy and ◎80" || !p.TopUp || p.Wait == "" || !strings.Contains(p.Hint, "Short of 2 energy") {
			t.Errorf("T = %+v, want the price 3 + ◎80 as a top-up that still has a wait to offer instead", p)
		}
		return
	}
	t.Fatalf("no pack T in %+v", got.Packs)
}

func TestOpen_withATopUpPaysForTheMissingEnergyInCoins(t *testing.T) {
	t.Parallel()
	shop := newCostShop(t, true, startingCoins)
	shop.drain(t)
	shop.clock.Advance(3 * playtest.Tick)

	plain, body := shop.post(t, "/open", url.Values{"set": {"T"}})
	if plain.StatusCode != http.StatusConflict {
		t.Fatalf("POST /open without the top-up = %d, want 409", plain.StatusCode)
	}
	mustContain(t, "the refusal", body, "Not enough energy")

	resp, _ := shop.post(t, "/open", url.Values{"set": {"T"}, "topup": {"yes"}})

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /open with the top-up = %d, want 303", resp.StatusCode)
	}
	if got, want := shop.game.Balance(), startingCoins-2*energyPrice; got != want {
		t.Errorf("balance %d, want %d", got, want)
	}
}

func TestOpen_aTopUpWithNoEnergyAtAllSaysWhatItNeeds(t *testing.T) {
	t.Parallel()
	shop := newCostShop(t, true, startingCoins)
	shop.drain(t)

	resp, body := shop.post(t, "/open", url.Values{"set": {"T"}, "topup": {"yes"}})

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status %d, want 409", resp.StatusCode)
	}
	mustContain(t, "the refusal", body, "at least 1 energy")
}

func TestOpen_aTopUpInAGameThatOffersNoneIsRefused(t *testing.T) {
	t.Parallel()
	shop := newCostShop(t, false, startingCoins)
	shop.drain(t)
	shop.clock.Advance(3 * playtest.Tick)

	resp, body := shop.post(t, "/open", url.Values{"set": {"T"}, "topup": {"yes"}})

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status %d, want 409", resp.StatusCode)
	}
	mustContain(t, "the refusal", body, "does not offer")
	if got := shop.game.Balance(); got != startingCoins {
		t.Errorf("balance %d, want %d", got, startingCoins)
	}
}

func TestPack_theOpenAnotherButtonSaysTheFee(t *testing.T) {
	t.Parallel()
	shop := newCostShop(t, false, startingCoins)
	shop.post(t, "/open", url.Values{"set": {"S"}})

	body := shop.get(t, "/pack")

	mustContain(t, "the pack page", body, "◎150")
}
