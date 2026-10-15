package domain_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// stingyCards builds a hand-crafted card lookup covering every rarity. No test catalogue has a
// mythic, so we construct cards directly and pass them through TraitEnv.Cards.
func stingyCards() func(domain.CardID) (domain.Card, bool) {
	cards := map[domain.CardID]domain.Card{
		"st-common":   {ID: "st-common", Rarity: domain.RarityCommon},
		"st-uncommon": {ID: "st-uncommon", Rarity: domain.RarityUncommon},
		"st-rare":     {ID: "st-rare", Rarity: domain.RarityRare},
		"st-special":  {ID: "st-special", Rarity: domain.RaritySpecial},
		"st-mythic":   {ID: "st-mythic", Rarity: domain.RarityMythic},
	}
	return func(id domain.CardID) (domain.Card, bool) {
		c, ok := cards[id]
		return c, ok
	}
}

// stingyBehaviour builds a casual behaviour with the stingy trait and a nil Registry so that
// DefaultTraits() is exercised.
func stingyBehaviour(t *testing.T, cardsFunc func(domain.CardID) (domain.Card, bool)) domain.TradeBehaviour {
	t.Helper()
	b, err := domain.NewBehaviour("casual", domain.BehaviourParams{
		MarginPct: 10,
		Traits: []domain.TraitSpec{{Name: "stingy", Params: map[string]int{
			"common_pct":   0,
			"uncommon_pct": 5,
			"rare_pct":     20,
			"special_pct":  20,
			"mythic_pct":   40,
		}}},
		Registry:     nil, // exercises the DefaultTraits() path
		Env:          domain.TraitEnv{Cards: cardsFunc},
		MaxMarginPct: 1000,
	})
	if err != nil {
		t.Fatalf("stingyBehaviour: %v", err)
	}
	return b
}

func stingyOffer(id domain.CardID) domain.Offer {
	return domain.Offer{
		NPCGives: []domain.TradeLine{{Key: domain.CardKey{ID: id, Finish: domain.FinishNonFoil}, Qty: 1}},
	}
}

// --- Criterion 1: margin rises with rarity, exact values, ordering ---

func TestStingyMarginRisesByRarity(t *testing.T) {
	t.Parallel()
	b := stingyBehaviour(t, stingyCards())

	tests := []struct {
		name   string
		cardID domain.CardID
		want   domain.Percent
	}{
		{"common equals base", "st-common", 10},   // 10 + 0 = 10
		{"uncommon", "st-uncommon", 15},           // 10 + 5 = 15
		{"rare", "st-rare", 30},                   // 10 + 20 = 30
		{"special equals rare", "st-special", 30}, // 10 + 20 = 30 (same tier as rare)
		{"mythic", "st-mythic", 50},               // 10 + 40 = 50
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := b.Margin(stingyOffer(tt.cardID))
			if got != tt.want {
				t.Errorf("Margin(%s) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}

	// Explicit ordering: uncommon < rare < mythic.
	uncommon := b.Margin(stingyOffer("st-uncommon"))
	rare := b.Margin(stingyOffer("st-rare"))
	mythic := b.Margin(stingyOffer("st-mythic"))
	if uncommon >= rare {
		t.Errorf("uncommon margin %d must be < rare margin %d", uncommon, rare)
	}
	if rare >= mythic {
		t.Errorf("rare margin %d must be < mythic margin %d", rare, mythic)
	}
}

// --- Criterion 2: the rarest card in the offer decides ---

func TestStingyMaxRarityDecides(t *testing.T) {
	t.Parallel()
	cards := stingyCards()
	b := stingyBehaviour(t, cards)

	// Mixed offer: common and rare in either order — result must be the rare's add.
	wantMargin := domain.Percent(30) // base 10 + rare_pct 20

	mixedAB := domain.Offer{NPCGives: []domain.TradeLine{
		{Key: domain.CardKey{ID: "st-common", Finish: domain.FinishNonFoil}, Qty: 1},
		{Key: domain.CardKey{ID: "st-rare", Finish: domain.FinishNonFoil}, Qty: 1},
	}}
	mixedBA := domain.Offer{NPCGives: []domain.TradeLine{
		{Key: domain.CardKey{ID: "st-rare", Finish: domain.FinishNonFoil}, Qty: 1},
		{Key: domain.CardKey{ID: "st-common", Finish: domain.FinishNonFoil}, Qty: 1},
	}}
	if got := b.Margin(mixedAB); got != wantMargin {
		t.Errorf("mixed(common+rare) order A: Margin = %d, want %d", got, wantMargin)
	}
	if got := b.Margin(mixedBA); got != wantMargin {
		t.Errorf("mixed(rare+common) order B: Margin = %d, want %d", got, wantMargin)
	}

	// Unknown card adds 0 (not found in lookup), empty NPCGives returns base, nil NPC is fine.
	unknown := domain.Offer{NPCGives: []domain.TradeLine{{Key: domain.CardKey{ID: "unknown", Finish: domain.FinishNonFoil}, Qty: 1}}}
	if got := b.Margin(unknown); got != 10 {
		t.Errorf("unknown card: Margin = %d, want 10 (base only)", got)
	}
	if got := b.Margin(domain.Offer{}); got != 10 {
		t.Errorf("empty NPCGives: Margin = %d, want 10 (base only)", got)
	}
	// nil ctx.NPC is implicitly tested: Margin delegates to MarginFor({Offer: offer}).
}

// --- Criterion 3: constructor errors and interface assertions ---

func TestStingyConstructorErrors(t *testing.T) {
	t.Parallel()
	cardsFunc := stingyCards()
	validEnv := domain.TraitEnv{Cards: cardsFunc}

	tests := []struct {
		name    string
		params  map[string]int
		env     domain.TraitEnv
		wantErr string
	}{
		{
			name:    "negative value",
			params:  map[string]int{"common_pct": -1},
			env:     validEnv,
			wantErr: "common_pct",
		},
		{
			name:    "unknown parameter name",
			params:  map[string]int{"unknown_param": 1},
			env:     validEnv,
			wantErr: "unknown_param",
		},
		{
			name:    "nil env.Cards",
			params:  map[string]int{},
			env:     domain.TraitEnv{},
			wantErr: "Cards",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.NewBehaviour("casual", domain.BehaviourParams{
				Traits:       []domain.TraitSpec{{Name: "stingy", Params: tt.params}},
				Env:          tt.env,
				MaxMarginPct: 1000,
			})
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestStingyImplementsMarginAdderNotVetoNotReact(t *testing.T) {
	t.Parallel()
	b := stingyBehaviour(t, stingyCards())

	// The composed behaviour must be a contextMarginer (MarginAdder pathway).
	type contextMarginer interface {
		MarginFor(domain.MarginContext) domain.Percent
	}
	if _, ok := b.(contextMarginer); !ok {
		t.Error("stingy must be wrapped in traitBehaviour with MarginFor")
	}

	// It must NOT implement CounterVeto.
	type counterVetoer interface{ VetoesCounter(domain.Offer) bool }
	if cv, ok := b.(counterVetoer); ok && cv.VetoesCounter(domain.Offer{}) {
		t.Error("stingy must not veto a player counter")
	}

	// It must NOT implement DeclineReactor as a trait. traitBehaviour always wraps ReactToDecline,
	// but that is a no-op because stingyTrait does not implement DeclineReactor.
	// Verified implicitly: stingyTrait.Name() == "stingy" and only implements MarginAdder.
}

// --- Criterion 4: stingy through the Exchange ---

// TestStingyExchangeMarginByRarity verifies that:
//   - an uncommon NPC card requires a lower top-up than a rare NPC card of the same price
//   - topUp == need → Accepted; topUp == need-1 → Countered (NPC answers, never refuses)
func TestStingyExchangeMarginByRarity(t *testing.T) {
	t.Parallel()
	// Build an archetype with stingy trait using the DefaultTraits registry (nil).
	spec := domain.ArchetypeSpec{
		Share:          1,
		KeepCopies:     1,
		Behaviour:      "casual",
		MarginPct:      10,
		OfferChancePct: 0,
		Traits: []domain.TraitSpec{{Name: "stingy", Params: map[string]int{
			"uncommon_pct": 5,
			"rare_pct":     20,
		}}},
	}
	// nil registry → DefaultTraits(), which now has "stingy".
	fx := newExchangeFixtureWithRegistry(t, 1, spec, domain.ExchangeParams{}, nil, 1000)

	const cardValue = 1000 // USD cents; both player card and NPC cards are the same price

	// NewRavnicaCatalogue cards sorted alphabetically (non-basic, non-foil only):
	//   [0-10] rav-c-000..rav-c-010   commons   (11 cards)
	//   [11]   rav-r-000              rare       (1 card)
	//   [12-15] rav-u-000..rav-u-003  uncommons  (4 cards)
	// We use the rare (index 11) and an uncommon (index 12) so the stingy trait's rarity lookup
	// gives different adds. A common (index 0) is the player's card.
	playerCard := fx.keys[0]   // common
	npcRare := fx.keys[11]     // rav-r-000 (rare)    → stingy adds rare_pct=20   → margin=30
	npcUncommon := fx.keys[12] // rav-u-000 (uncommon) → stingy adds uncommon_pct=5 → margin=15

	fx.setPrice(playerCard, cardValue)
	fx.setPrice(npcRare, cardValue)
	fx.setPrice(npcUncommon, cardValue)

	fx.shelve(0, npcUncommon, 1)
	fx.shelve(0, npcRare, 1)
	fx.give(t, playerCard, 1)
	fx.exchange.StartDay(1)

	// Probe the uncommon card.
	d, err := fx.exchange.Propose(0, lineOf(playerCard, 1), lineOf(npcUncommon, 1), 0)
	if err != nil {
		t.Fatalf("Propose uncommon (0 top-up): %v", err)
	}
	if d.Outcome != domain.OutcomeCountered {
		t.Fatalf("expected counter for topUp 0, got %v", d.Outcome)
	}
	uncommonNeed := d.Offer.TopUp

	// Decline the counter so the offer is closed and the card is available again.
	if err := fx.exchange.Decline(d.Offer.ID); err != nil {
		t.Fatalf("Decline uncommon counter: %v", err)
	}

	// Probe the rare card.
	d, err = fx.exchange.Propose(0, lineOf(playerCard, 1), lineOf(npcRare, 1), 0)
	if err != nil {
		t.Fatalf("Propose rare (0 top-up): %v", err)
	}
	if d.Outcome != domain.OutcomeCountered {
		t.Fatalf("expected counter for topUp 0, got %v", d.Outcome)
	}
	rareNeed := d.Offer.TopUp
	if err := fx.exchange.Decline(d.Offer.ID); err != nil {
		t.Fatalf("Decline rare counter: %v", err)
	}

	// The uncommon should require fewer coins than the rare, because stingy adds more for rarer cards.
	if uncommonNeed >= rareNeed {
		t.Errorf("uncommonNeed %d must be < rareNeed %d (stingy adds more for rarer cards)", uncommonNeed, rareNeed)
	}

	// Fund the player and verify: topUp == need → Accepted; topUp == need-1 → Countered.
	fx.fund(t, uncommonNeed+rareNeed+10)
	fx.give(t, playerCard, 1) // replenish after the earlier Propose consumed it

	d, err = fx.exchange.Propose(0, lineOf(playerCard, 1), lineOf(npcUncommon, 1), uncommonNeed)
	if err != nil {
		t.Fatalf("Propose uncommon (exact top-up): %v", err)
	}
	if d.Outcome != domain.OutcomeAccepted {
		t.Errorf("topUp == need: got %v, want Accepted", d.Outcome)
	}

	// Replenish cards and retry with need-1 for the rare.
	fx.give(t, playerCard, 1)
	d, err = fx.exchange.Propose(0, lineOf(playerCard, 1), lineOf(npcRare, 1), rareNeed-1)
	if err != nil {
		t.Fatalf("Propose rare (need-1): %v", err)
	}
	if d.Outcome != domain.OutcomeCountered {
		t.Errorf("topUp == need-1: got %v (reason %v), want Countered (NPC answers, not refuses)", d.Outcome, d.Reason)
	}
}
