package domain_test

import (
	"errors"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestProposeAcceptsAFairSwapAtZeroMargin(t *testing.T) {
	t.Parallel()
	fx := newExchangeFixture(t, 1, tradingSpec(0), domain.ExchangeParams{})
	mine, theirs := fx.keys[0], fx.keys[1]
	fx.setPrice(mine, 100)
	fx.setPrice(theirs, 100)
	fx.give(t, mine, 1)
	fx.shelve(0, theirs, 1)

	d, err := fx.exchange.Propose(0, lineOf(mine, 1), lineOf(theirs, 1), 0)

	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if d.Outcome != domain.OutcomeAccepted {
		t.Fatalf("Outcome = %v (reason %v), want accepted", d.Outcome, d.Reason)
	}
	if got := fx.player.Quantity(theirs); got != 1 {
		t.Errorf("player holds %d of the NPC's card, want 1", got)
	}
	if got := fx.player.Quantity(mine); got != 0 {
		t.Errorf("player still holds %d of the card it gave", got)
	}
	if got := fx.npcs[0].ShelfQuantity(theirs); got != 0 {
		t.Errorf("the NPC's shelf still holds %d of the card it gave", got)
	}
	if got := fx.npcs[0].Collection().Quantity(mine); got != 1 {
		t.Errorf("the NPC holds %d of the card it received, want 1", got)
	}
	stats := fx.exchange.TakeDayStats()
	if stats.Accepted != 1 || stats.PlayerProposals != 1 || stats.VolumeCoins != 200 {
		t.Errorf("stats = %+v, want 1 accepted of 1 proposal and 200 coins of volume", stats)
	}
}

func TestProposeCountersWithTheSmallestTopUpWhenTheMarginIsShort(t *testing.T) {
	t.Parallel()
	fx := newExchangeFixture(t, 1, tradingSpec(10), domain.ExchangeParams{})
	mine, theirs := fx.keys[0], fx.keys[1]
	fx.setPrice(mine, 100)
	fx.setPrice(theirs, 100)
	fx.give(t, mine, 1)
	fx.shelve(0, theirs, 1)
	before := fx.state(0)

	d, err := fx.exchange.Propose(0, lineOf(mine, 1), lineOf(theirs, 1), 0)

	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if d.Outcome != domain.OutcomeCountered {
		t.Fatalf("Outcome = %v (reason %v), want countered", d.Outcome, d.Reason)
	}
	if d.Offer.TopUp != 10 || d.Offer.Origin != domain.OriginNPCCounter {
		t.Errorf("counter = %+v, want a top-up of 10 coins from the NPC", d.Offer)
	}
	if offers := fx.exchange.Offers(); len(offers) != 1 || offers[0].ID != d.Offer.ID {
		t.Errorf("Offers = %+v, want the counter-offer open", offers)
	}
	fx.requireUnchanged(t, 0, before)
}

func TestAcceptWithoutCoinsKeepsTheOfferOpenAndMovesNothing(t *testing.T) {
	t.Parallel()
	fx := newExchangeFixture(t, 1, tradingSpec(10), domain.ExchangeParams{})
	mine, theirs := fx.keys[0], fx.keys[1]
	fx.setPrice(mine, 100)
	fx.setPrice(theirs, 100)
	fx.give(t, mine, 1)
	fx.shelve(0, theirs, 1)
	counter, err := fx.exchange.Propose(0, lineOf(mine, 1), lineOf(theirs, 1), 0)
	if err != nil || counter.Outcome != domain.OutcomeCountered {
		t.Fatalf("setup: %+v, %v", counter, err)
	}
	before := fx.state(0)

	_, err = fx.exchange.Accept(counter.Offer.ID)

	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Fatalf("Accept = %v, want ErrInsufficientBalance", err)
	}
	fx.requireUnchanged(t, 0, before)
	if len(fx.exchange.Offers()) != 1 {
		t.Error("the offer closed although the player could not pay: it should stay open")
	}

	fx.fund(t, 10)
	d, err := fx.exchange.Accept(counter.Offer.ID)
	if err != nil || d.Outcome != domain.OutcomeAccepted {
		t.Fatalf("Accept after funding = %+v, %v, want accepted", d, err)
	}
	if got := fx.journal.Balance(domain.AccountPlayer); got != 0 {
		t.Errorf("player balance = %d, want 0 after paying 10", got)
	}
	if got := fx.journal.Balance(domain.AccountTradeTopUp); got != 10 {
		t.Errorf("top-up account = %d, want 10", got)
	}
	if len(fx.exchange.Offers()) != 0 {
		t.Error("the offer is still open after it was accepted")
	}
}

func TestProposeRefusesAndMovesNothing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		params  domain.ExchangeParams
		setup   func(t *testing.T, fx *exchangeFixture)
		npc     int
		give    func(fx *exchangeFixture) []domain.TradeLine
		ask     func(fx *exchangeFixture) []domain.TradeLine
		topUp   domain.Coins
		wantErr error
	}{
		{
			name: "an unknown NPC", npc: 9,
			give:    func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[0], 1) },
			ask:     func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[1], 1) },
			wantErr: domain.ErrUnknownNPC,
		},
		{
			name:    "a card the NPC does not hold on its shelf",
			setup:   func(t *testing.T, fx *exchangeFixture) { fx.give(t, fx.keys[0], 1) },
			give:    func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[0], 1) },
			ask:     func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[1], 1) },
			wantErr: domain.ErrNPCLacksCard,
		},
		{
			name:    "a card the player does not hold",
			setup:   func(t *testing.T, fx *exchangeFixture) { fx.shelve(0, fx.keys[1], 1) },
			give:    func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[0], 1) },
			ask:     func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[1], 1) },
			wantErr: domain.ErrPlayerLacksCard,
		},
		{
			name:    "a zero quantity",
			setup:   func(t *testing.T, fx *exchangeFixture) { fx.give(t, fx.keys[0], 1); fx.shelve(0, fx.keys[1], 1) },
			give:    func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[0], 0) },
			ask:     func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[1], 1) },
			wantErr: domain.ErrInvalidLine,
		},
		{
			name:    "a negative quantity",
			setup:   func(t *testing.T, fx *exchangeFixture) { fx.give(t, fx.keys[0], 1); fx.shelve(0, fx.keys[1], 1) },
			give:    func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[0], 1) },
			ask:     func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[1], -2) },
			wantErr: domain.ErrInvalidLine,
		},
		{
			name:  "a negative top-up",
			setup: func(t *testing.T, fx *exchangeFixture) { fx.give(t, fx.keys[0], 1); fx.shelve(0, fx.keys[1], 1) },
			give:  func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[0], 1) },
			ask:   func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[1], 1) },
			topUp: -5, wantErr: domain.ErrInvalidLine,
		},
		{
			name:    "nothing asked of the NPC",
			setup:   func(t *testing.T, fx *exchangeFixture) { fx.give(t, fx.keys[0], 1) },
			give:    func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[0], 1) },
			ask:     func(fx *exchangeFixture) []domain.TradeLine { return nil },
			wantErr: domain.ErrEmptyOffer,
		},
		{
			name:    "a priced card asked for with nothing offered",
			setup:   func(t *testing.T, fx *exchangeFixture) { fx.shelve(0, fx.keys[1], 1) },
			give:    func(fx *exchangeFixture) []domain.TradeLine { return nil },
			ask:     func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[1], 1) },
			wantErr: domain.ErrNothingOffered,
		},
		{
			name: "more cards than one side may hold", params: domain.ExchangeParams{MaxCardsPerSide: 2},
			setup:   func(t *testing.T, fx *exchangeFixture) { fx.give(t, fx.keys[0], 3); fx.shelve(0, fx.keys[1], 1) },
			give:    func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[0], 3) },
			ask:     func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[1], 1) },
			wantErr: domain.ErrTooManyCards,
		},
		{
			name:  "a top-up the player cannot pay",
			setup: func(t *testing.T, fx *exchangeFixture) { fx.give(t, fx.keys[0], 1); fx.shelve(0, fx.keys[1], 1) },
			give:  func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[0], 1) },
			ask:   func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[1], 1) },
			topUp: 5, wantErr: domain.ErrInsufficientBalance,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fx := newExchangeFixture(t, 1, tradingSpec(0), tt.params)
			fx.setPrice(fx.keys[0], 100)
			fx.setPrice(fx.keys[1], 100)
			if tt.setup != nil {
				tt.setup(t, fx)
			}
			before := fx.state(0)

			d, err := fx.exchange.Propose(tt.npc, tt.give(fx), tt.ask(fx), tt.topUp)

			if err != nil {
				t.Fatalf("Propose: %v", err)
			}
			if d.Outcome != domain.OutcomeRefused || !errors.Is(d.Reason, tt.wantErr) {
				t.Errorf("Decision = outcome %v, reason %v, want a refusal with %v", d.Outcome, d.Reason, tt.wantErr)
			}
			if tt.npc == 0 {
				fx.requireUnchanged(t, 0, before)
			}
			if got := fx.exchange.TakeDayStats().Refused; got != 1 {
				t.Errorf("Refused = %d, want 1", got)
			}
		})
	}
}

func TestProposeRefusesAnNPCThatDoesNotTrade(t *testing.T) {
	t.Parallel()
	fx := newExchangeFixture(t, 1, domain.ArchetypeSpec{Share: 1, KeepCopies: 1}, domain.ExchangeParams{})
	fx.give(t, fx.keys[0], 1)
	fx.shelve(0, fx.keys[1], 1)

	d, err := fx.exchange.Propose(0, lineOf(fx.keys[0], 1), lineOf(fx.keys[1], 1), 0)

	if err != nil || d.Outcome != domain.OutcomeRefused || !errors.Is(d.Reason, domain.ErrNPCNotTrading) {
		t.Errorf("Propose = %+v, %v, want a refusal with ErrNPCNotTrading", d, err)
	}
}

func TestProposeMergesTheSameCardListedTwice(t *testing.T) {
	t.Parallel()
	fx := newExchangeFixture(t, 1, tradingSpec(0), domain.ExchangeParams{})
	mine, theirs := fx.keys[0], fx.keys[1]
	fx.setPrice(mine, 100)
	fx.setPrice(theirs, 200)
	fx.give(t, mine, 2)
	fx.shelve(0, theirs, 1)

	d, err := fx.exchange.Propose(0, []domain.TradeLine{{Key: mine, Qty: 1}, {Key: mine, Qty: 1}}, lineOf(theirs, 1), 0)

	if err != nil || d.Outcome != domain.OutcomeAccepted {
		t.Fatalf("Propose = %+v, %v, want accepted: the two lines are one line of 2", d, err)
	}
	if got := fx.player.Quantity(mine); got != 0 {
		t.Errorf("player holds %d, want both copies gone", got)
	}
}

func TestHistoryKeepsTheLastEntriesAndStatsResetEachDay(t *testing.T) {
	t.Parallel()
	fx := newExchangeFixture(t, 1, tradingSpec(0), domain.ExchangeParams{HistorySize: 2})
	for range 3 {
		if _, err := fx.exchange.Propose(0, lineOf(fx.keys[0], 1), lineOf(fx.keys[1], 1), 0); err != nil {
			t.Fatalf("Propose: %v", err)
		}
	}

	if got := len(fx.exchange.History()); got != 2 {
		t.Errorf("history has %d entries, want the last 2", got)
	}
	if got := fx.exchange.TakeDayStats(); got.Refused != 3 || got.PlayerProposals != 3 {
		t.Errorf("stats = %+v, want 3 refused of 3 proposals", got)
	}
	if got := fx.exchange.TakeDayStats(); got != (domain.ExchangeStats{}) {
		t.Errorf("stats after a reset = %+v, want zero", got)
	}
}

func TestNewExchangeRejectsInvalidSetups(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(p *domain.ExchangeParams)
	}{
		{"no expiry", func(p *domain.ExchangeParams) { p.ExpiryDays = -1 }},
		{"no card cap", func(p *domain.ExchangeParams) { p.MaxCardsPerSide = -1 }},
		{"duplicate keep above a playset", func(p *domain.ExchangeParams) { p.DuplicateKeep = 5 }},
		{"negative offers per day", func(p *domain.ExchangeParams) { p.MaxOffersPerDay = -1 }},
		{"negative history", func(p *domain.ExchangeParams) { p.HistorySize = -1 }},
		{"no coin rate", func(p *domain.ExchangeParams) { p.CoinRate = -1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fx := newExchangeFixture(t, 1, tradingSpec(0), domain.ExchangeParams{})
			params := withExchangeDefaults(domain.ExchangeParams{})
			tt.mutate(&params)

			_, err := domain.NewExchange(params, fx.crowd, fx.player, fx.journal, domain.NewValuer(fx.prices, 1))

			if err == nil {
				t.Error("want error, got nil")
			}
		})
	}
	t.Run("a crowd that holds no surplus has nothing to trade", func(t *testing.T) {
		t.Parallel()
		crowd, _, _ := newTestCrowd(t, 1, fixedSpec(1, 1), crowdSets(), 1)
		_, err := domain.NewExchange(withExchangeDefaults(domain.ExchangeParams{}), crowd, domain.NewCollection(), domain.NewJournal(), domain.Valuer{})
		if err == nil {
			t.Error("want error, got nil")
		}
	})
}
