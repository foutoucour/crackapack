# Trades with NPCs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the simulation trade with NPCs, so long runs across many scenarios and parameter sets can measure the economy with trades.

**Architecture:** A new `domain.Exchange` owns offers, the acceptance rule, one counter-offer, expiry and the atomic transfer. A `Valuer` extracted from the Counter gives both the same reference value. NPCs gain a wish list and a shelf of surplus, and a per-archetype `TradeBehaviour` strategy picks wishes, the margin and proposals. `internal/sim` wires it behind `trades.enabled` and adds `trader` and `exploiter` player policies.

**Tech Stack:** Go 1.26, `go.yaml.in/yaml/v3`, cobra. Standard library only for new code.

**Spec:** [docs/trades-with-npcs-design.md](trades-with-npcs-design.md)

## Global Constraints

- Every number that shapes trade behaviour is a flat config key with a default, validated by `config.Load`. No tunable is a constant in the domain.
- Trades are off by default (`trades.enabled: false`). With trades disabled, the output of a run is byte-identical to today's.
- The NPC accepts when `received × 100 ≥ given × (100 + margin)`. A negative margin is rejected, at config load and in the domain.
- Money is integers, never floats. Card values are USD cents, coins are `Coins`, and a coin top-up counts at the coin rate.
- The domain imports no infrastructure: no database, HTTP, filesystem, clock or global randomness.
- No package-level variables (a lint rule): registries are returned by functions. Sentinel `Err...` variables are the existing exception.
- Each run owns its `Exchange`, `Crowd` and `Journal`, and draws only from streams derived from its seed.
- A run's memory must not grow with its length: offers are dropped when answered or expired, and the history is a bounded ring.
- Code, comments, docs and commit messages are in English. Tests are TDD, with no mocks of internal code. Never weaken an assertion to make a test pass.
- Commands: `go test -race ./...`, `golangci-lint run`, `go mod tidy -diff`, `gofmt -l .` (no output). The code blocks in this plan are not guaranteed gofmt-aligned: each commit step runs `gofmt -w` first.
- Every commit message ends with the trailer `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`, passed as a second `-m`.

## Review Focus

Failure modes the spec implies and no task's happy path would exercise. Each line names the task whose tests pin it.

1. A stale offer: the NPC's shelf card was traded or released, or the player sold the offered card, before the player accepts. Expected: the offer is refused and closed, nothing moves, no error. (Task 7)
2. Accepting a counter-offer without enough coins. Expected: `ErrInsufficientBalance`, the offer stays open, no card or coin moves. (Task 6)
3. Trade lines with a zero or negative quantity, or the same card listed twice. Expected: refused as `ErrInvalidLine`, or merged into one line. (Tasks 3 and 6)
4. A priced card asked for with nothing offered, no cards and no coins. Expected: refused as `ErrNothingOffered`, never a free card. (Task 6)
5. The expiry boundary. Expected: an offer made on day `d` stays open through day `d + offer_expiry_days - 1` and lapses on the next. (Task 7)

## File Structure

Create in `internal/domain/`:

| File | Responsibility |
|------|----------------|
| `valuer.go` | `Valuer`: the reference value of a card |
| `exchange_rules.go` | `TradeLine`, line helpers, `requiredTopUp`: the pure acceptance maths |
| `behaviour.go` | `Wish`, the contexts, `TradeBehaviour`, the registry and shared helpers |
| `behaviour_archetypes.go` | The five behaviours |
| `npc_shelf.go` | NPC shelf and wish state, and the crowd's index, hold and wish methods |
| `exchange.go` | `Exchange` types, errors, constructor and queries |
| `exchange_trade.go` | `Propose`, `Accept`, settlement, history and stats |
| `exchange_offers.go` | NPC-initiated offers, expiry and `Counter` |
| `export_trades_test.go` | Test hooks in `package domain` |

Modify in `internal/domain/`: `counter.go`, `ledger.go`, `collection.go`, `npc.go`, `npc_crowd.go`.

Create in `internal/config/`: `validate_trades.go`. Modify `config.go`.

Create in `internal/sim/`: `trades.go`, `trading.go` (the `Trading` interface and port methods), `trader.go`, `exploiter.go`. Modify `engine.go`, `crowd.go`, `metrics.go`, `writer.go`, `compare.go`, `policy.go`.

Modify `cmd/crackapack/policies.go`, `cmd/crackapack/demo.go`, `demo/config.go`.

Docs: `docs/adr/0009…` (status only), new `docs/adr/0010…0012`, `docs/simulation.md`, `docs/core-features.md`, `docs/architecture.md`, `docs/todo.md`, `CHANGELOG.md`.

---

### Task 1: Valuer shared by the Counter and trades

**Files:**
- Create: `internal/domain/valuer.go`
- Test: `internal/domain/valuer_test.go`
- Modify: `internal/domain/counter.go`

**Interfaces:**
- Produces: `type Valuer`, `func NewValuer(prices PriceSnapshot, fallback USDCents) Valuer`, `func (v Valuer) Price(id CardID) USDCents`.

- [ ] **Step 1: Write the failing test**

```go
package domain_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestValuerPrice(t *testing.T) {
	t.Parallel()
	valuer := domain.NewValuer(domain.PriceSnapshot{"priced": 250, "free": 0}, 7)

	tests := []struct {
		name string
		id   domain.CardID
		want domain.USDCents
	}{
		{"a card in the snapshot", "priced", 250},
		{"a card priced at zero stays at zero", "free", 0},
		{"a card absent from the snapshot falls back", "missing", 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := valuer.Price(tt.id); got != tt.want {
				t.Errorf("Price(%q) = %d, want %d", tt.id, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/domain/ -run TestValuerPrice`
Expected: FAIL to compile, `undefined: domain.NewValuer`.

- [ ] **Step 3: Implement `Valuer` and make the Counter use it**

`internal/domain/valuer.go`:

```go
package domain

// Valuer gives the reference value of a card: its price in the run's snapshot, or the fallback
// price when the card has none. The Counter and trades value cards through it, so both agree.
//
// A foil and its non-foil share one CardID and therefore one value, a documented simplification
// pending a per-finish price source.
type Valuer struct {
	prices   PriceSnapshot
	fallback USDCents
}

// NewValuer returns a Valuer over prices. fallback values the cards absent from prices.
func NewValuer(prices PriceSnapshot, fallback USDCents) Valuer {
	return Valuer{prices: prices, fallback: fallback}
}

// Price returns the reference value of the printing id in USD cents.
func (v Valuer) Price(id CardID) USDCents {
	if p, ok := v.prices[id]; ok {
		return p
	}
	return v.fallback
}
```

In `internal/domain/counter.go`, replace the struct field:

```go
	params    CounterParams
	prices    PriceSnapshot
	journal   *Journal
```
with
```go
	params    CounterParams
	valuer    Valuer
	journal   *Journal
```
In `NewCounter`'s returned literal replace `prices:    prices,` with `valuer:    NewValuer(prices, params.FallbackPrice),`. Replace the body of `referencePrice`:

```go
	if p, ok := c.prices[id]; ok {
		return p
	}
	return c.params.FallbackPrice
```
with `return c.valuer.Price(id)`.

- [ ] **Step 4: Run the domain tests**

Run: `go test ./internal/domain/...`
Expected: PASS, including every existing Counter test untouched.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal
git add internal/domain/valuer.go internal/domain/valuer_test.go internal/domain/counter.go
git commit -m "Extract the reference value of a card into a Valuer" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Trade top-up ledger account and per-finish quantity

**Files:**
- Modify: `internal/domain/ledger.go`, `internal/domain/collection.go`
- Test: `internal/domain/ledger_topup_test.go`, `internal/domain/collection_quantity_test.go`

**Interfaces:**
- Produces: `AccountTradeTopUp Account = "trade_topup"`; `func (c *Collection) Quantity(key CardKey) int`.

- [ ] **Step 1: Write the failing tests**

`internal/domain/ledger_topup_test.go`:

```go
package domain_test

import (
	"errors"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestJournalTradeTopUpIsASinkOfThePlayer(t *testing.T) {
	t.Parallel()
	journal := domain.NewJournal()
	if err := journal.Post(domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: 50}); err != nil {
		t.Fatalf("Post welcome bonus: %v", err)
	}

	if err := journal.Post(domain.Entry{From: domain.AccountPlayer, To: domain.AccountTradeTopUp, Amount: 20}); err != nil {
		t.Fatalf("Post top-up: %v", err)
	}

	if got := journal.Balance(domain.AccountPlayer); got != 30 {
		t.Errorf("player balance = %d, want 30", got)
	}
	if got := journal.Balance(domain.AccountTradeTopUp); got != 20 {
		t.Errorf("top-up balance = %d, want 20: the total the player paid", got)
	}
	if got := journal.SumAllBalances(); got != 0 {
		t.Errorf("sum of balances = %d, want 0", got)
	}
}

func TestJournalRefusesATopUpTheWalletCannotPay(t *testing.T) {
	t.Parallel()
	journal := domain.NewJournal()

	err := journal.Post(domain.Entry{From: domain.AccountPlayer, To: domain.AccountTradeTopUp, Amount: 1})

	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Errorf("Post = %v, want ErrInsufficientBalance", err)
	}
}
```

`internal/domain/collection_quantity_test.go`:

```go
package domain_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestCollectionQuantityCountsOnlyTheGivenFinish(t *testing.T) {
	t.Parallel()
	collection := domain.NewCollection()
	nonFoil := domain.CardKey{ID: "a", Finish: domain.FinishNonFoil}
	foil := domain.CardKey{ID: "a", Finish: domain.FinishFoil}
	if err := collection.Add(nonFoil, 2); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := collection.Add(foil, 1); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if got := collection.Quantity(nonFoil); got != 2 {
		t.Errorf("Quantity(non-foil) = %d, want 2", got)
	}
	if got := collection.Quantity(foil); got != 1 {
		t.Errorf("Quantity(foil) = %d, want 1", got)
	}
	if got := collection.Quantity(domain.CardKey{ID: "b", Finish: domain.FinishNonFoil}); got != 0 {
		t.Errorf("Quantity(absent) = %d, want 0", got)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/domain/ -run 'TestJournalTradeTopUp|TestJournalRefusesATopUp|TestCollectionQuantity'`
Expected: FAIL to compile, `undefined: domain.AccountTradeTopUp` and `collection.Quantity undefined`.

- [ ] **Step 3: Implement**

In `internal/domain/ledger.go`, add after `AccountCounterPurchase` inside the `const` block:

```go

	// AccountTradeTopUp is the sink account for the coins the player adds to a trade with an NPC:
	// the top-up posts from AccountPlayer to AccountTradeTopUp. NPCs hold no coins and have no
	// account, so the coins leave the economy. It is unguarded, and its balance is the total of
	// top-ups paid.
	AccountTradeTopUp Account = "trade_topup"
```

In `internal/domain/collection.go`, add after `Copies`:

```go
// Quantity returns the number of copies of key held, in that finish only. Copies sums both
// finishes of a printing.
func (c *Collection) Quantity(key CardKey) int {
	return c.qty[key]
}
```

- [ ] **Step 4: Run the domain tests**

Run: `go test ./internal/domain/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal
git add internal/domain/ledger.go internal/domain/collection.go internal/domain/ledger_topup_test.go internal/domain/collection_quantity_test.go
git commit -m "Add the trade top-up ledger account and a per-finish quantity" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Acceptance maths and trade lines

**Files:**
- Create: `internal/domain/exchange_rules.go`
- Test: `internal/domain/exchange_rules_test.go` (`package domain`, so it can reach the unexported helpers)

**Interfaces:**
- Consumes: `Valuer.Price` (Task 1).
- Produces:
  - `type TradeLine struct{ Key CardKey; Qty int }`
  - `var ErrInvalidLine error`
  - `func normalizeLines(lines []TradeLine) ([]TradeLine, error)`: merges equal keys, sorts by (ID, finish), rejects a quantity ≤ 0 with `ErrInvalidLine`
  - `func cmpKey(a, b CardKey) int`
  - `func cardCount(lines []TradeLine) int`
  - `func linesValue(v Valuer, lines []TradeLine) USDCents`
  - `func requiredTopUp(playerCents, npcCents USDCents, margin Percent, rate CoinRate) Coins`

- [ ] **Step 1: Write the failing tests**

```go
package domain

import (
	"errors"
	"slices"
	"testing"
)

func TestRequiredTopUp(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		player USDCents
		npc    USDCents
		margin Percent
		rate   CoinRate
		want   Coins
	}{
		{"equal value, no margin", 100, 100, 0, 100, 0},
		{"the player gives more than needed", 200, 100, 0, 100, 0},
		{"one cent short costs one coin at rate 100", 100, 101, 0, 100, 1},
		{"the margin sits on top of the value", 100, 150, 10, 100, 65},
		{"a fractional coin rounds up in the NPC's favour", 100, 150, 1, 100, 52},
		{"a margin on equal value", 100, 100, 1, 100, 1},
		{"a lower rate makes a coin worth more dollars", 100, 150, 0, 10, 5},
		{"a priced card asked for with nothing offered", 0, 100, 0, 100, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := requiredTopUp(tt.player, tt.npc, tt.margin, tt.rate); got != tt.want {
				t.Errorf("requiredTopUp(%d, %d, %d%%, rate %d) = %d, want %d", tt.player, tt.npc, tt.margin, tt.rate, got, tt.want)
			}
		})
	}
}

func TestNormalizeLinesMergesAndSorts(t *testing.T) {
	t.Parallel()
	b := CardKey{ID: "b", Finish: FinishNonFoil}
	a := CardKey{ID: "a", Finish: FinishNonFoil}
	aFoil := CardKey{ID: "a", Finish: FinishFoil}

	got, err := normalizeLines([]TradeLine{{b, 1}, {a, 1}, {aFoil, 2}, {a, 2}})
	if err != nil {
		t.Fatalf("normalizeLines: %v", err)
	}

	want := []TradeLine{{a, 3}, {aFoil, 2}, {b, 1}}
	if !slices.Equal(got, want) {
		t.Errorf("normalizeLines = %v, want %v", got, want)
	}
}

func TestNormalizeLinesRejectsNonPositiveQuantities(t *testing.T) {
	t.Parallel()
	key := CardKey{ID: "a", Finish: FinishNonFoil}
	for _, qty := range []int{0, -1} {
		if _, err := normalizeLines([]TradeLine{{key, qty}}); !errors.Is(err, ErrInvalidLine) {
			t.Errorf("quantity %d: err = %v, want ErrInvalidLine", qty, err)
		}
	}
}

func TestLinesValueAndCardCount(t *testing.T) {
	t.Parallel()
	valuer := NewValuer(PriceSnapshot{"a": 100, "b": 250}, 7)
	lines := []TradeLine{
		{CardKey{ID: "a", Finish: FinishNonFoil}, 2},
		{CardKey{ID: "b", Finish: FinishNonFoil}, 1},
		{CardKey{ID: "c", Finish: FinishNonFoil}, 1},
	}

	if got := linesValue(valuer, lines); got != 2*100+250+7 {
		t.Errorf("linesValue = %d, want 457", got)
	}
	if got := cardCount(lines); got != 4 {
		t.Errorf("cardCount = %d, want 4", got)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/domain/ -run 'TestRequiredTopUp|TestNormalizeLines|TestLinesValue'`
Expected: FAIL to compile, `undefined: requiredTopUp`.

- [ ] **Step 3: Implement**

`internal/domain/exchange_rules.go`:

```go
package domain

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
)

// ErrInvalidLine is returned for a trade line with a quantity that is not positive, or for a
// negative coin top-up.
var ErrInvalidLine = errors.New("exchange: a trade line needs a positive quantity")

// TradeLine is a quantity of one card on one side of a trade.
type TradeLine struct {
	Key CardKey
	Qty int
}

// normalizeLines merges the lines of equal keys and sorts the result by key, so an offer has one
// line per card whatever the caller sent. It rejects a quantity that is not positive.
func normalizeLines(lines []TradeLine) ([]TradeLine, error) {
	merged := make(map[CardKey]int, len(lines))
	for _, l := range lines {
		if l.Qty <= 0 {
			return nil, fmt.Errorf("%w: %s x%d", ErrInvalidLine, l.Key.ID, l.Qty)
		}
		merged[l.Key] += l.Qty
	}
	out := make([]TradeLine, 0, len(merged))
	for key, qty := range merged {
		out = append(out, TradeLine{Key: key, Qty: qty})
	}
	slices.SortFunc(out, func(a, b TradeLine) int { return cmpKey(a.Key, b.Key) })
	return out, nil
}

// cmpKey orders card keys by printing, then finish.
func cmpKey(a, b CardKey) int {
	if c := cmp.Compare(a.ID, b.ID); c != 0 {
		return c
	}
	return cmp.Compare(a.Finish, b.Finish)
}

// cardCount returns the number of cards in lines, counting quantities.
func cardCount(lines []TradeLine) int {
	var total int
	for _, l := range lines {
		total += l.Qty
	}
	return total
}

// linesValue returns the reference value of lines in USD cents.
func linesValue(v Valuer, lines []TradeLine) USDCents {
	var total USDCents
	for _, l := range lines {
		total += v.Price(l.Key.ID) * USDCents(l.Qty)
	}
	return total
}

// requiredTopUp returns the smallest number of coins the player must add so that the NPC accepts:
// received × 100 ≥ given × (100 + margin), where received is the value of the player's cards plus
// the top-up and given is the value of the NPC's cards.
//
// Values are compared in hundredths of a coin: a card worth c cents is c × rate of them and a coin
// is 100. The gap is rounded up to a whole coin, which is always in the NPC's favour. A negative
// margin is the caller's error, and it is rejected where behaviours are built.
func requiredTopUp(playerCents, npcCents USDCents, margin Percent, rate CoinRate) Coins {
	gap := int64(rate) * (int64(npcCents)*(100+int64(margin)) - int64(playerCents)*100)
	if gap <= 0 {
		return 0
	}
	// gap is scaled by 100 (percent) and by 100 (hundredths of a coin per coin).
	const gapScale = 10_000
	return Coins((gap + gapScale - 1) / gapScale)
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/domain/ -run 'TestRequiredTopUp|TestNormalizeLines|TestLinesValue' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal
git add internal/domain/exchange_rules.go internal/domain/exchange_rules_test.go
git commit -m "Add the acceptance maths of a trade" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 4: TradeBehaviour, the five archetypes and the archetype settings

**Files:**
- Create: `internal/domain/offer.go`, `internal/domain/behaviour.go`, `internal/domain/behaviour_archetypes.go`
- Modify: `internal/domain/npc.go`
- Test: `internal/domain/behaviour_test.go`, `internal/domain/npc_trade_settings_test.go`

**Interfaces:**
- Consumes: `Valuer`, `TradeLine`, `linesValue`, `capLines` helpers defined here.
- Produces:
  - `type Wish struct{ ID CardID; Want int }`
  - `type WishContext struct{ NPC *NPC; Scope []Card; Wished map[CardID]struct{} }`
  - `type ProposalContext struct{ NPC *NPC; Wanted, Shelf []TradeLine; PlayerCopies func(CardID) int; MaxCards int }`
  - `type TradeBehaviour interface { PickWishes(ctx WishContext, n int, rng *rand.Rand) []Wish; Margin(offer Offer) Percent; Propose(ctx ProposalContext) (want, give []TradeLine, ok bool) }`
  - `type Offer`, `type OfferOrigin` with `OriginPlayer`, `OriginNPC`, `OriginNPCCounter` (new file `internal/domain/offer.go`, used by the Exchange from Task 6)
  - `type BehaviourParams struct{ MarginPct Percent; Valuer Valuer }`
  - `func NewBehaviour(name string, p BehaviourParams) (TradeBehaviour, error)`, `func KnownBehaviour(name string) bool`, `func BehaviourNames() []string`
  - `ArchetypeSpec` gains `Behaviour string`, `WishSize int`, `MarginPct Percent`, `OfferChancePct int`; `PopulationParams` gains `Valuer Valuer`.
  - `NPC` gains `Behaviour TradeBehaviour`, `WishSize int`, `OfferChancePct int`, and unexported `wishes []Wish`, `shelf map[CardKey][]int`.

- [ ] **Step 1: Write the failing tests**

`internal/domain/behaviour_test.go`:

```go
package domain_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

func behaviourScope(cat domain.SetCatalogue) []domain.Card {
	var scope []domain.Card
	for _, card := range cat.Cards {
		if !card.IsBasicLand {
			scope = append(scope, card)
		}
	}
	slices.SortFunc(scope, func(a, b domain.Card) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
	return scope
}

func behaviourNPC(t *testing.T, cat domain.SetCatalogue) *domain.NPC {
	t.Helper()
	npcs, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: 1, Archetypes: map[string]domain.ArchetypeSpec{"npc": {Share: 1, KeepCopies: 1}}},
		[]domain.SetCatalogue{cat}, rand.New(rand.NewPCG(1, 1)))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	return npcs[0]
}

func behaviourValuer(cat domain.SetCatalogue) domain.Valuer {
	return domain.NewValuer(domaintest.NewPrices(cat, 3), 10)
}

func TestBehavioursPickDistinctWishesTheNPCLacks(t *testing.T) {
	t.Parallel()
	for _, name := range domain.BehaviourNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cat := domaintest.NewRavnicaCatalogue()
			npc := behaviourNPC(t, cat)
			scope := behaviourScope(cat)
			held := scope[0]
			if err := npc.Collection().Add(domain.CardKey{ID: held.ID, Finish: domain.FinishNonFoil}, 4); err != nil {
				t.Fatalf("Add: %v", err)
			}
			b, err := domain.NewBehaviour(name, domain.BehaviourParams{Valuer: behaviourValuer(cat)})
			if err != nil {
				t.Fatalf("NewBehaviour: %v", err)
			}

			wishes := b.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 5, rand.New(rand.NewPCG(2, 2)))

			if len(wishes) == 0 || len(wishes) > 5 {
				t.Fatalf("got %d wishes, want between 1 and 5", len(wishes))
			}
			seen := map[domain.CardID]bool{}
			for _, w := range wishes {
				if seen[w.ID] {
					t.Errorf("card %q wished twice", w.ID)
				}
				seen[w.ID] = true
				if w.ID == held.ID {
					t.Errorf("wished %q, which the NPC already holds in full", w.ID)
				}
				if w.Want < 1 {
					t.Errorf("wish %q wants %d copies, want at least 1", w.ID, w.Want)
				}
			}
		})
	}
}

func TestBehavioursSkipWishesAlreadyOnTheList(t *testing.T) {
	t.Parallel()
	for _, name := range domain.BehaviourNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cat := domaintest.NewRavnicaCatalogue()
			npc := behaviourNPC(t, cat)
			scope := behaviourScope(cat)
			b, err := domain.NewBehaviour(name, domain.BehaviourParams{Valuer: behaviourValuer(cat)})
			if err != nil {
				t.Fatalf("NewBehaviour: %v", err)
			}
			wished := map[domain.CardID]struct{}{}
			for _, card := range scope {
				wished[card.ID] = struct{}{}
			}

			if got := b.PickWishes(domain.WishContext{NPC: npc, Scope: scope, Wished: wished}, 5, rand.New(rand.NewPCG(2, 2))); len(got) != 0 {
				t.Errorf("picked %v with every card already wished, want none", got)
			}
			if got := b.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 0, rand.New(rand.NewPCG(2, 2))); len(got) != 0 {
				t.Errorf("picked %v for n = 0, want none", got)
			}
		})
	}
}

func TestBehavioursPickTheSameWishesFromTheSameSeed(t *testing.T) {
	t.Parallel()
	for _, name := range domain.BehaviourNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cat := domaintest.NewRavnicaCatalogue()
			npc := behaviourNPC(t, cat)
			scope := behaviourScope(cat)
			b, err := domain.NewBehaviour(name, domain.BehaviourParams{Valuer: behaviourValuer(cat)})
			if err != nil {
				t.Fatalf("NewBehaviour: %v", err)
			}

			first := b.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 4, rand.New(rand.NewPCG(9, 9)))
			second := b.PickWishes(domain.WishContext{NPC: npc, Scope: scope}, 4, rand.New(rand.NewPCG(9, 9)))

			if !slices.Equal(first, second) {
				t.Errorf("same seed gave %v then %v", first, second)
			}
		})
	}
}

func TestPlaysetHunterWishesCommonsAndUncommonsByFour(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewRavnicaCatalogue()
	npc := behaviourNPC(t, cat)
	b, err := domain.NewBehaviour("playset_hunter", domain.BehaviourParams{})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}

	wishes := b.PickWishes(domain.WishContext{NPC: npc, Scope: behaviourScope(cat)}, 15, rand.New(rand.NewPCG(2, 2)))

	if len(wishes) != 15 {
		t.Fatalf("got %d wishes, want all 15 commons and uncommons", len(wishes))
	}
	for _, w := range wishes {
		card := cat.Cards[w.ID]
		if card.Rarity != domain.RarityCommon && card.Rarity != domain.RarityUncommon {
			t.Errorf("wished %q, a %s", w.ID, card.Rarity)
		}
		if w.Want != domain.PlaysetSize {
			t.Errorf("wish %q wants %d, want a playset", w.ID, w.Want)
		}
	}
}

func TestWhaleWishesOnlyRaresAndSpecials(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewRavnicaCatalogue()
	npc := behaviourNPC(t, cat)
	b, err := domain.NewBehaviour("whale", domain.BehaviourParams{})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}

	wishes := b.PickWishes(domain.WishContext{NPC: npc, Scope: behaviourScope(cat)}, 5, rand.New(rand.NewPCG(2, 2)))

	if len(wishes) == 0 {
		t.Fatal("no wish: the catalogue has a rare")
	}
	for _, w := range wishes {
		if r := cat.Cards[w.ID].Rarity; r != domain.RarityRare && r != domain.RaritySpecial {
			t.Errorf("wished %q, a %s", w.ID, r)
		}
	}
}

func TestSpeculatorWishesTheMostValuableCardsFirst(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewRavnicaCatalogue()
	npc := behaviourNPC(t, cat)
	valuer := behaviourValuer(cat)
	b, err := domain.NewBehaviour("speculator", domain.BehaviourParams{MarginPct: 15, Valuer: valuer})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}

	wishes := b.PickWishes(domain.WishContext{NPC: npc, Scope: behaviourScope(cat)}, 6, rand.New(rand.NewPCG(2, 2)))

	for i := 1; i < len(wishes); i++ {
		if valuer.Price(wishes[i-1].ID) < valuer.Price(wishes[i].ID) {
			t.Errorf("wish %d (%q, %d) is worth more than wish %d (%q, %d)", i, wishes[i].ID, valuer.Price(wishes[i].ID), i-1, wishes[i-1].ID, valuer.Price(wishes[i-1].ID))
		}
	}
}

func TestTargetedCollectorWishesRarerCardsFirst(t *testing.T) {
	t.Parallel()
	cat := domaintest.NewRavnicaCatalogue()
	npc := behaviourNPC(t, cat)
	b, err := domain.NewBehaviour("targeted_collector", domain.BehaviourParams{})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}

	wishes := b.PickWishes(domain.WishContext{NPC: npc, Scope: behaviourScope(cat)}, 3, rand.New(rand.NewPCG(2, 2)))

	if len(wishes) != 3 {
		t.Fatalf("got %d wishes, want 3", len(wishes))
	}
	if r := cat.Cards[wishes[0].ID].Rarity; r != domain.RarityRare {
		t.Errorf("first wish is a %s, want the rare", r)
	}
	if r := cat.Cards[wishes[1].ID].Rarity; r != domain.RarityUncommon {
		t.Errorf("second wish is a %s, want an uncommon", r)
	}
}

func TestNewBehaviourRejectsAnUnknownNameAndANegativeMargin(t *testing.T) {
	t.Parallel()
	if _, err := domain.NewBehaviour("gambler", domain.BehaviourParams{}); err == nil {
		t.Error("unknown behaviour accepted")
	}
	if _, err := domain.NewBehaviour("casual", domain.BehaviourParams{MarginPct: -1}); err == nil {
		t.Error("negative margin accepted: an NPC must never give more value than it gets")
	}
	if domain.KnownBehaviour("gambler") || !domain.KnownBehaviour("casual") {
		t.Error("KnownBehaviour disagrees with the registry")
	}
}

func TestBehaviourMarginIsTheConfiguredPercentage(t *testing.T) {
	t.Parallel()
	b, err := domain.NewBehaviour("speculator", domain.BehaviourParams{MarginPct: 15})
	if err != nil {
		t.Fatalf("NewBehaviour: %v", err)
	}
	if got := b.Margin(domain.Offer{}); got != 15 {
		t.Errorf("Margin = %d, want 15", got)
	}
}

func TestProposeBuildsAnOfferTheNPCWouldAccept(t *testing.T) {
	t.Parallel()
	wanted := domain.CardKey{ID: "wanted", Finish: domain.FinishNonFoil}
	cheap := domain.CardKey{ID: "cheap", Finish: domain.FinishNonFoil}
	pricey := domain.CardKey{ID: "pricey", Finish: domain.FinishNonFoil}
	valuer := domain.NewValuer(domain.PriceSnapshot{"wanted": 1000, "cheap": 400, "pricey": 700}, 10)
	ctx := domain.ProposalContext{
		Wanted:       []domain.TradeLine{{Key: wanted, Qty: 1}},
		Shelf:        []domain.TradeLine{{Key: cheap, Qty: 2}, {Key: pricey, Qty: 1}},
		PlayerCopies: func(id domain.CardID) int { return map[domain.CardID]int{"pricey": 1}[id] },
		MaxCards:     10,
	}

	tests := []struct {
		name     string
		margin   domain.Percent
		maxCards int
		wantGive []domain.TradeLine
		wantOK   bool
	}{
		{"fits at zero margin, the cards the player lacks first", 0, 10, []domain.TradeLine{{Key: cheap, Qty: 2}}, true},
		{"a margin shrinks what the NPC gives", 50, 10, []domain.TradeLine{{Key: cheap, Qty: 1}}, true},
		{"the card cap bounds the give side", 0, 1, []domain.TradeLine{{Key: cheap, Qty: 1}}, true},
		{"nothing fits a huge margin", 400, 10, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b, err := domain.NewBehaviour("casual", domain.BehaviourParams{MarginPct: tt.margin, Valuer: valuer})
			if err != nil {
				t.Fatalf("NewBehaviour: %v", err)
			}
			c := ctx
			c.MaxCards = tt.maxCards

			want, give, ok := b.Propose(c)

			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !slices.Equal(give, tt.wantGive) {
				t.Errorf("give = %v, want %v", give, tt.wantGive)
			}
			if ok && !slices.Equal(want, ctx.Wanted) {
				t.Errorf("want = %v, want the wanted lines %v", want, ctx.Wanted)
			}
		})
	}
}

func TestSpeculatorAsksForTheMostValuableDuplicateFirst(t *testing.T) {
	t.Parallel()
	low := domain.CardKey{ID: "low", Finish: domain.FinishNonFoil}
	high := domain.CardKey{ID: "high", Finish: domain.FinishNonFoil}
	shelf := domain.CardKey{ID: "shelf", Finish: domain.FinishNonFoil}
	valuer := domain.NewValuer(domain.PriceSnapshot{"low": 100, "high": 900, "shelf": 50}, 10)
	ctx := domain.ProposalContext{
		Wanted:       []domain.TradeLine{{Key: low, Qty: 1}, {Key: high, Qty: 1}},
		Shelf:        []domain.TradeLine{{Key: shelf, Qty: 3}},
		PlayerCopies: func(domain.CardID) int { return 0 },
		MaxCards:     1,
	}
	for name, wantFirst := range map[string]domain.CardKey{"casual": low, "speculator": high} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, err := domain.NewBehaviour(name, domain.BehaviourParams{Valuer: valuer})
			if err != nil {
				t.Fatalf("NewBehaviour: %v", err)
			}

			want, _, ok := b.Propose(ctx)

			if !ok || len(want) != 1 || want[0].Key != wantFirst {
				t.Errorf("want = %v (ok %v), want the single line %v", want, ok, wantFirst)
			}
		})
	}
}
```

`internal/domain/npc_trade_settings_test.go`:

```go
package domain_test

import (
	"math/rand/v2"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func TestGeneratePopulationGivesEachNPCItsArchetypeBehaviour(t *testing.T) {
	t.Parallel()
	spec := domain.ArchetypeSpec{Share: 1, KeepCopies: 1, Behaviour: "speculator", WishSize: 3, MarginPct: 15, OfferChancePct: 7}

	npcs, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: 2, Archetypes: map[string]domain.ArchetypeSpec{"spec": spec}},
		crowdSets(), rand.New(rand.NewPCG(1, 1)))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}

	for _, npc := range npcs {
		if npc.Behaviour == nil {
			t.Fatalf("NPC %d has no behaviour", npc.ID)
		}
		if npc.WishSize != 3 || npc.OfferChancePct != 7 {
			t.Errorf("NPC %d: WishSize %d, OfferChancePct %d, want 3 and 7", npc.ID, npc.WishSize, npc.OfferChancePct)
		}
		if got := npc.Behaviour.Margin(domain.Offer{}); got != 15 {
			t.Errorf("NPC %d margin = %d, want 15", npc.ID, got)
		}
	}
}

func TestGeneratePopulationLeavesAnArchetypeWithoutBehaviourNotTrading(t *testing.T) {
	t.Parallel()
	npcs, err := domain.GeneratePopulation(
		domain.PopulationParams{Size: 1, Archetypes: map[string]domain.ArchetypeSpec{"plain": {Share: 1, KeepCopies: 1}}},
		crowdSets(), rand.New(rand.NewPCG(1, 1)))
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	if npcs[0].Behaviour != nil {
		t.Error("an archetype with no behaviour name got one")
	}
}

func TestGeneratePopulationRejectsInvalidTradeSettings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		spec domain.ArchetypeSpec
	}{
		{"unknown behaviour", domain.ArchetypeSpec{Share: 1, KeepCopies: 1, Behaviour: "gambler"}},
		{"negative wish size", domain.ArchetypeSpec{Share: 1, KeepCopies: 1, Behaviour: "casual", WishSize: -1}},
		{"negative margin", domain.ArchetypeSpec{Share: 1, KeepCopies: 1, Behaviour: "casual", MarginPct: -1}},
		{"offer chance above 100", domain.ArchetypeSpec{Share: 1, KeepCopies: 1, Behaviour: "casual", OfferChancePct: 101}},
		{"negative offer chance", domain.ArchetypeSpec{Share: 1, KeepCopies: 1, Behaviour: "casual", OfferChancePct: -1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := domain.GeneratePopulation(
				domain.PopulationParams{Size: 1, Archetypes: map[string]domain.ArchetypeSpec{"a": tt.spec}},
				crowdSets(), rand.New(rand.NewPCG(1, 1)))
			if err == nil {
				t.Error("want error, got nil")
			}
		})
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/domain/ -run 'Behaviour|Propose|Speculator|PlaysetHunter|Whale|TargetedCollector|TradeSettings|GeneratePopulation'`
Expected: FAIL to compile, `undefined: domain.BehaviourNames`, `domain.Offer`, and the new `ArchetypeSpec` fields.

- [ ] **Step 3: Implement**

`Behaviour.Margin` takes an `Offer`, so the offer type comes first. Create it in `internal/domain/offer.go`:

```go
package domain

// OfferOrigin says who made an offer.
type OfferOrigin int

const (
	// OriginPlayer is an offer the player proposed. The Exchange never stores one: it is accepted,
	// countered or refused at once.
	OriginPlayer OfferOrigin = iota + 1
	// OriginNPC is an offer an NPC proposed because its wish list matches the player's duplicates.
	OriginNPC
	// OriginNPCCounter is the NPC's answer to a player offer that fell short: the same cards with
	// the smallest top-up that satisfies the rule.
	OriginNPCCounter
)

// Offer is a proposed swap between the player and one NPC. The player gives PlayerGives and
// TopUp coins, and the NPC gives NPCGives.
type Offer struct {
	// ID is unique within an Exchange and starts at 1. Zero means not yet stored.
	ID          int
	NPC         int
	Origin      OfferOrigin
	PlayerGives []TradeLine
	NPCGives    []TradeLine
	// TopUp is the coins the player adds. NPCs hold no coins, so it never goes the other way.
	TopUp Coins
	// CreatedDay is the day the offer was made, and LastDay the last day it can be answered.
	CreatedDay int
	LastDay    int
}
```

`internal/domain/behaviour.go`:

```go
package domain

import (
	"cmp"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
)

// Wish is a card an NPC wants, and how many copies in total it wants to hold.
type Wish struct {
	ID   CardID
	Want int
}

// WishContext is what a behaviour sees when it picks wishes.
type WishContext struct {
	NPC *NPC
	// Scope is every card the NPC may wish for, sorted by ID.
	Scope []Card
	// Wished holds the cards already on the NPC's wish list.
	Wished map[CardID]struct{}
}

// ProposalContext is what a behaviour sees when it builds an offer to the player.
type ProposalContext struct {
	NPC *NPC
	// Wanted is what the NPC asks of the player: duplicates the player holds that are on the NPC's
	// wish list, capped by what the NPC still wants.
	Wanted []TradeLine
	// Shelf is what the NPC can give.
	Shelf []TradeLine
	// PlayerCopies returns how many copies of a printing the player holds.
	PlayerCopies func(CardID) int
	// MaxCards bounds each side of the offer.
	MaxCards int
}

// TradeBehaviour is how an archetype trades. An archetype names a registered behaviour in the
// configuration, see ADR-0010. A new personality is a type, one registry entry and a config entry.
type TradeBehaviour interface {
	// PickWishes returns up to n wishes for the NPC, none of them already in ctx.Wished.
	PickWishes(ctx WishContext, n int, rng *rand.Rand) []Wish
	// Margin returns the percentage the NPC demands on top of the reference value it gives. It is
	// never negative.
	Margin(offer Offer) Percent
	// Propose builds an offer to the player: the cards the NPC asks for and the cards it gives.
	// The cards it gives are worth no more than the cards it asks for, once the margin is paid.
	// ok is false when no such offer exists.
	Propose(ctx ProposalContext) (want, give []TradeLine, ok bool)
}

// BehaviourParams are the numbers a behaviour reads. They come from the archetype's configuration.
type BehaviourParams struct {
	MarginPct Percent
	Valuer    Valuer
}

type behaviourConstructor func(BehaviourParams) TradeBehaviour

// behaviourConstructors returns the registry of behaviours by name. It is a function, not a
// package-level variable, to avoid the global-variable lint rule.
func behaviourConstructors() map[string]behaviourConstructor {
	return map[string]behaviourConstructor{
		"casual":             func(p BehaviourParams) TradeBehaviour { return casualBehaviour{base{p}} },
		"playset_hunter":     func(p BehaviourParams) TradeBehaviour { return playsetHunterBehaviour{base{p}} },
		"targeted_collector": func(p BehaviourParams) TradeBehaviour { return targetedCollectorBehaviour{base{p}} },
		"speculator":         func(p BehaviourParams) TradeBehaviour { return speculatorBehaviour{base{p}} },
		"whale":              func(p BehaviourParams) TradeBehaviour { return whaleBehaviour{base{p}} },
	}
}

// BehaviourNames returns the registered behaviour names, sorted.
func BehaviourNames() []string {
	return slices.Sorted(maps.Keys(behaviourConstructors()))
}

// KnownBehaviour reports whether name is a registered behaviour.
func KnownBehaviour(name string) bool {
	_, ok := behaviourConstructors()[name]
	return ok
}

// NewBehaviour builds the behaviour registered under name. It rejects an unknown name and a
// negative margin: a negative margin would let the player gain value by trading through a chain
// of NPCs and selling the result to the Counter.
func NewBehaviour(name string, p BehaviourParams) (TradeBehaviour, error) {
	if p.MarginPct < 0 {
		return nil, fmt.Errorf("behaviour %q: margin must be >= 0, got %d", name, p.MarginPct)
	}
	build, ok := behaviourConstructors()[name]
	if !ok {
		return nil, fmt.Errorf("behaviour %q is not registered, known behaviours: %v", name, BehaviourNames())
	}
	return build(p), nil
}

// base holds what every behaviour shares: the margin is the configured percentage, and the
// proposal is a swap of wanted duplicates for shelf cards.
type base struct{ params BehaviourParams }

func (b base) Margin(Offer) Percent { return b.params.MarginPct }

func (b base) Propose(ctx ProposalContext) (want, give []TradeLine, ok bool) {
	return proposeSwap(ctx, b.params)
}

// proposeSwap asks for the wanted lines, up to ctx.MaxCards cards, and gives the most it can from
// the shelf without giving more than it asks for, once the margin is paid. Cards the player lacks
// come first, so the offer is worth taking.
func proposeSwap(ctx ProposalContext, p BehaviourParams) (want, give []TradeLine, ok bool) {
	want = capLines(ctx.Wanted, ctx.MaxCards)
	if len(want) == 0 {
		return nil, nil, false
	}
	budget := int64(linesValue(p.Valuer, want)) * 100

	shelf := slices.Clone(ctx.Shelf)
	slices.SortStableFunc(shelf, func(a, b TradeLine) int {
		return cmp.Compare(playerHolds(ctx, a), playerHolds(ctx, b))
	})

	var cards int
	var cents int64
	for _, line := range shelf {
		price := int64(p.Valuer.Price(line.Key.ID))
		qty := 0
		for qty < line.Qty && cards < ctx.MaxCards && (cents+price)*(100+int64(p.MarginPct)) <= budget {
			qty++
			cards++
			cents += price
		}
		if qty > 0 {
			give = append(give, TradeLine{Key: line.Key, Qty: qty})
		}
	}
	if len(give) == 0 {
		return nil, nil, false
	}
	return want, give, true
}

// playerHolds is 1 when the player already holds the printing of line, else 0.
func playerHolds(ctx ProposalContext, line TradeLine) int {
	if ctx.PlayerCopies(line.Key.ID) > 0 {
		return 1
	}
	return 0
}

// capLines keeps lines in order until maxCards cards are reached, shortening the last one.
func capLines(lines []TradeLine, maxCards int) []TradeLine {
	var out []TradeLine
	left := maxCards
	for _, l := range lines {
		if left <= 0 {
			break
		}
		qty := min(l.Qty, left)
		out = append(out, TradeLine{Key: l.Key, Qty: qty})
		left -= qty
	}
	return out
}

// lacking returns the cards of ctx.Scope the NPC holds no copy of and has not wished yet.
func lacking(ctx WishContext) []Card {
	out := make([]Card, 0, len(ctx.Scope))
	for _, card := range ctx.Scope {
		if _, wished := ctx.Wished[card.ID]; wished {
			continue
		}
		if ctx.NPC.collection.Copies(card.ID) > 0 {
			continue
		}
		out = append(out, card)
	}
	return out
}

// pickRandom returns n cards of pool drawn without replacement, or all of them when there are
// fewer. pool is not modified.
func pickRandom(pool []Card, n int, rng *rand.Rand) []Card {
	if n <= 0 {
		return nil
	}
	shuffled := slices.Clone(pool)
	n = min(n, len(shuffled))
	for i := range n {
		j := i + rng.IntN(len(shuffled)-i)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	return shuffled[:n]
}

// firstN returns the first n cards of pool.
func firstN(pool []Card, n int) []Card {
	if n <= 0 {
		return nil
	}
	return pool[:min(n, len(pool))]
}

// wishesOf turns cards into wishes that want copies of each.
func wishesOf(cards []Card, copies int) []Wish {
	wishes := make([]Wish, 0, len(cards))
	for _, card := range cards {
		wishes = append(wishes, Wish{ID: card.ID, Want: copies})
	}
	return wishes
}
```

`internal/domain/behaviour_archetypes.go`:

```go
package domain

import (
	"cmp"
	"math/rand/v2"
	"slices"
)

// casualBehaviour wishes for any card it lacks, at random.
type casualBehaviour struct{ base }

func (b casualBehaviour) PickWishes(ctx WishContext, n int, rng *rand.Rand) []Wish {
	return wishesOf(pickRandom(lacking(ctx), n, rng), 1)
}

// playsetHunterBehaviour wishes for commons and uncommons it holds fewer than a playset of.
type playsetHunterBehaviour struct{ base }

func (b playsetHunterBehaviour) PickWishes(ctx WishContext, n int, rng *rand.Rand) []Wish {
	var pool []Card
	for _, card := range ctx.Scope {
		if card.Rarity != RarityCommon && card.Rarity != RarityUncommon {
			continue
		}
		if _, wished := ctx.Wished[card.ID]; wished {
			continue
		}
		if ctx.NPC.collection.Copies(card.ID) >= PlaysetSize {
			continue
		}
		pool = append(pool, card)
	}
	return wishesOf(pickRandom(pool, n, rng), PlaysetSize)
}

// targetedCollectorBehaviour completes its set from the rarest cards down.
type targetedCollectorBehaviour struct{ base }

func (b targetedCollectorBehaviour) PickWishes(ctx WishContext, n int, _ *rand.Rand) []Wish {
	pool := lacking(ctx)
	slices.SortStableFunc(pool, func(a, b Card) int { return cmp.Compare(rarityRank(b.Rarity), rarityRank(a.Rarity)) })
	return wishesOf(firstN(pool, n), 1)
}

// rarityRank orders rarities for a collector: a rare or special card comes before an uncommon,
// which comes before a common.
func rarityRank(r Rarity) int {
	switch r {
	case RarityRare, RaritySpecial:
		return 2
	case RarityUncommon:
		return 1
	default:
		return 0
	}
}

// speculatorBehaviour wishes for the most valuable cards it lacks, and asks for the most valuable
// duplicates first.
type speculatorBehaviour struct{ base }

func (b speculatorBehaviour) PickWishes(ctx WishContext, n int, _ *rand.Rand) []Wish {
	pool := lacking(ctx)
	slices.SortStableFunc(pool, func(x, y Card) int {
		return cmp.Compare(b.params.Valuer.Price(y.ID), b.params.Valuer.Price(x.ID))
	})
	return wishesOf(firstN(pool, n), 1)
}

func (b speculatorBehaviour) Propose(ctx ProposalContext) (want, give []TradeLine, ok bool) {
	wanted := slices.Clone(ctx.Wanted)
	slices.SortStableFunc(wanted, func(x, y TradeLine) int {
		return cmp.Compare(b.params.Valuer.Price(y.Key.ID), b.params.Valuer.Price(x.Key.ID))
	})
	ctx.Wanted = wanted
	return proposeSwap(ctx, b.params)
}

// whaleBehaviour goes for rares and specials, at random.
type whaleBehaviour struct{ base }

func (b whaleBehaviour) PickWishes(ctx WishContext, n int, rng *rand.Rand) []Wish {
	var pool []Card
	for _, card := range lacking(ctx) {
		if card.Rarity == RarityRare || card.Rarity == RaritySpecial {
			pool = append(pool, card)
		}
	}
	return wishesOf(pickRandom(pool, n, rng), 1)
}
```

In `internal/domain/npc.go`:

1. In `ArchetypeSpec`, after the `FocusSet bool` field, add:

```go
	// Behaviour names the registered TradeBehaviour of the archetype, see BehaviourNames. Empty
	// means the archetype does not trade.
	Behaviour string
	// WishSize is how many cards an NPC wishes for at once.
	WishSize int
	// MarginPct is the margin an NPC demands on top of the reference value it gives. It must be
	// >= 0, so an NPC never gives more value than it gets.
	MarginPct Percent
	// OfferChancePct is the chance, per tick, that an NPC with a match proposes a trade to the
	// player. It must be in [0, 100].
	OfferChancePct int
```

2. In `PopulationParams`, after `Archetypes`, add:

```go
	// Valuer values cards for the archetypes that have a behaviour. It may be the zero Valuer
	// when none does.
	Valuer Valuer
```

3. In `NPC`, after `FocusSet string`, add:

```go
	// Behaviour is how the NPC trades, or nil when its archetype does not trade.
	Behaviour TradeBehaviour
	// WishSize is how many cards the NPC wishes for at once.
	WishSize int
	// OfferChancePct is the chance, per tick, that the NPC proposes a trade when it has a match.
	OfferChancePct int
```
and, among the unexported fields after `collection *Collection`, add:

```go
	// wishes is the wish list, and shelf holds the surplus copies the NPC can trade away, per
	// card, with the tick each copy arrived on, oldest first. See npc_shelf.go.
	wishes []Wish
	shelf  map[CardKey][]int
```

4. In `GeneratePopulation`, after `totalShare, err := validateArchetypes(...)` and its error check, add:

```go
	behaviours := make(map[string]TradeBehaviour, len(names))
	for _, name := range names {
		spec := p.Archetypes[name]
		if spec.Behaviour == "" {
			continue
		}
		behaviour, err := NewBehaviour(spec.Behaviour, BehaviourParams{MarginPct: spec.MarginPct, Valuer: p.Valuer})
		if err != nil {
			return nil, fmt.Errorf("population: archetype %q: %w", name, err)
		}
		behaviours[name] = behaviour
	}
```
and extend the `&NPC{...}` literal with:

```go
				Behaviour:      behaviours[name],
				WishSize:       spec.WishSize,
				OfferChancePct: spec.OfferChancePct,
				shelf:          make(map[CardKey][]int),
```

5. In `validateArchetypes`, add to the `switch`, after the `KeepCopies` case:

```go
		case spec.Behaviour != "" && !KnownBehaviour(spec.Behaviour):
			return 0, fmt.Errorf("population: archetype %q: unknown behaviour %q, known behaviours: %v", name, spec.Behaviour, BehaviourNames())
		case spec.WishSize < 0:
			return 0, fmt.Errorf("population: archetype %q: wish size must be >= 0, got %d", name, spec.WishSize)
		case spec.MarginPct < 0:
			return 0, fmt.Errorf("population: archetype %q: margin must be >= 0, got %d", name, spec.MarginPct)
		case spec.OfferChancePct < 0 || spec.OfferChancePct > 100:
			return 0, fmt.Errorf("population: archetype %q: offer chance must be in [0, 100], got %d", name, spec.OfferChancePct)
```

- [ ] **Step 4: Run the domain tests**

Run: `go test ./internal/domain/...`
Expected: PASS. Existing population and crowd tests are untouched because the new fields default to "no behaviour".

- [ ] **Step 5: Refactor**

Look at `behaviour_test.go` for tests that say the same thing: merge any that assert the same property across behaviours into the existing table-driven ones, and drop a test that a broader one already covers. Re-run `go test ./internal/domain/...`.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal
git add internal/domain
git commit -m "Add the TradeBehaviour strategy and the five archetype behaviours" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Shelf, wish lists, indexes and the surplus hold in the crowd

**Files:**
- Create: `internal/domain/npc_shelf.go`, `internal/domain/export_trades_test.go`
- Modify: `internal/domain/npc_crowd.go` (replace the whole file), `internal/domain/npc_crowd_test.go` (helper only)
- Test: `internal/domain/crowd_shelf_test.go`

**Interfaces:**
- Consumes: `TradeBehaviour`, `Wish`, `WishContext` (Task 4); `TradeLine`, `cmpKey` (Task 3).
- Produces:
  - `type CrowdOption func(*Crowd)`; `func WithSurplusHold(days int) CrowdOption`
  - `NewCrowd(npcs []*NPC, sets []SetCatalogue, opener *Opener, counter *Counter, rng *rand.Rand, opts ...CrowdOption) (*Crowd, error)`
  - `func (c *Crowd) CardsHeld() int`, `func (c *Crowd) HoldersOf(key CardKey) []int`
  - `func (n *NPC) Shelf() []TradeLine`, `func (n *NPC) ShelfQuantity(key CardKey) int`, `func (n *NPC) Wishes() []Wish`
  - unexported, used by the Exchange: `c.byID map[int]*NPC`, `c.wishers map[CardID]map[int]struct{}`, `c.holdTicks int`, `c.rng *rand.Rand`, `c.applyTrade(npc *NPC, gives, gets []TradeLine) error`
  - Test hooks in `package domain`: `ShelveForTest`, `UnshelveForTest`, `SetWishesForTest`, `WishersForTest`.

- [ ] **Step 1: Write the failing tests**

First make the existing helper accept options. In `internal/domain/npc_crowd_test.go`, change the signature and the `NewCrowd` call:

```go
func newTestCrowd(t *testing.T, size int, spec domain.ArchetypeSpec, sets []domain.SetCatalogue, seed uint64, opts ...domain.CrowdOption) (*domain.Crowd, []*domain.NPC, *domain.Counter) {
```
```go
	crowd, err := domain.NewCrowd(npcs, sets, opener, counter, rng, opts...)
```

`internal/domain/crowd_shelf_test.go`:

```go
package domain_test

import (
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

func shelfTotal(npc *domain.NPC) int {
	var total int
	for _, line := range npc.Shelf() {
		total += line.Qty
	}
	return total
}

func TestCrowdKeepsSurplusOnTheShelfForTheHold(t *testing.T) {
	t.Parallel()
	crowd, npcs, counter := newTestCrowd(t, 1, fixedSpec(10, 1), crowdSets(), 1, domain.WithSurplusHold(2))

	for day := 1; day <= 2; day++ {
		runDays(t, crowd, 1)
		if got := totalStock(counter); got != 0 {
			t.Fatalf("day %d: the Counter holds %d cards, want none: surplus is held for 2 days", day, got)
		}
	}
	if shelfTotal(npcs[0]) == 0 {
		t.Fatal("no surplus on the shelf after two days")
	}

	runDays(t, crowd, 1)
	if totalStock(counter) == 0 {
		t.Error("day 3: nothing reached the Counter, but the day-1 surplus is past its hold")
	}
}

func TestCrowdCardsHeldCountsCollectionsAndShelves(t *testing.T) {
	t.Parallel()
	for _, hold := range []int{0, 2} {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			crowd, npcs, _ := newTestCrowd(t, 2, fixedSpec(8, 1), crowdSets(), 3, domain.WithSurplusHold(hold))
			var drawn, sold int
			for range 4 {
				tick := runDays(t, crowd, 1)
				drawn += tick.CardsDrawn
				sold += tick.CardsSold
			}

			var held int
			for _, npc := range npcs {
				held += totalCopies(npc.Collection()) + shelfTotal(npc)
			}
			if got := crowd.CardsHeld(); got != held {
				t.Errorf("hold %d: CardsHeld = %d, want %d (collections plus shelves)", hold, got, held)
			}
			if drawn-sold != held {
				t.Errorf("hold %d: drawn %d - sold %d = %d, want %d held", hold, drawn, sold, drawn-sold, held)
			}
		})
	}
}

func TestCrowdHoldersIndexMatchesTheShelves(t *testing.T) {
	t.Parallel()
	crowd, npcs, _ := newTestCrowd(t, 5, fixedSpec(10, 1), crowdSets(), 4, domain.WithSurplusHold(1))
	runDays(t, crowd, 4)

	for _, cat := range crowdSets() {
		for id := range cat.Cards {
			for _, finish := range []domain.Finish{domain.FinishNonFoil, domain.FinishFoil} {
				key := domain.CardKey{ID: id, Finish: finish}
				var want []int
				for _, npc := range npcs {
					if npc.ShelfQuantity(key) > 0 {
						want = append(want, npc.ID)
					}
				}
				if got := crowd.HoldersOf(key); !slices.Equal(got, want) {
					t.Errorf("HoldersOf(%v) = %v, want %v", key, got, want)
				}
			}
		}
	}
}

func TestCrowdFillsAndRefillsWishLists(t *testing.T) {
	t.Parallel()
	spec := fixedSpec(10, 1)
	spec.Behaviour = "casual"
	spec.WishSize = 3
	crowd, npcs, _ := newTestCrowd(t, 4, spec, crowdSets(), 5, domain.WithSurplusHold(2))

	for _, npc := range npcs {
		if got := len(npc.Wishes()); got != 3 {
			t.Fatalf("NPC %d starts with %d wishes, want 3", npc.ID, got)
		}
	}
	runDays(t, crowd, 5)

	wishedBy := map[domain.CardID][]int{}
	for _, npc := range npcs {
		wishes := npc.Wishes()
		if len(wishes) > 3 {
			t.Errorf("NPC %d has %d wishes, want at most 3", npc.ID, len(wishes))
		}
		for _, w := range wishes {
			if npc.Collection().Copies(w.ID) >= w.Want {
				t.Errorf("NPC %d still wishes %q, which it holds %d times", npc.ID, w.ID, npc.Collection().Copies(w.ID))
			}
			wishedBy[w.ID] = append(wishedBy[w.ID], npc.ID)
		}
	}
	for _, cat := range crowdSets() {
		for id := range cat.Cards {
			if got := crowd.WishersForTest(id); !slices.Equal(got, wishedBy[id]) {
				t.Errorf("wishers index of %q = %v, want %v", id, got, wishedBy[id])
			}
		}
	}
}

func TestCrowdWithoutBehavioursHasNoWishesAndNoShelf(t *testing.T) {
	t.Parallel()
	plainCrowd, plainNPCs, _ := newTestCrowd(t, 3, fixedSpec(10, 1), crowdSets(), 8)
	runDays(t, plainCrowd, 3)

	for _, npc := range plainNPCs {
		if len(npc.Wishes()) != 0 {
			t.Errorf("NPC %d has wishes without a behaviour", npc.ID)
		}
		if len(npc.Shelf()) != 0 {
			t.Errorf("NPC %d has a shelf without a surplus hold", npc.ID)
		}
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/domain/ -run 'TestCrowd'`
Expected: FAIL to compile, `undefined: domain.WithSurplusHold`, `npc.Shelf undefined`, `crowd.CardsHeld undefined`.

- [ ] **Step 3: Implement**

Replace `internal/domain/npc_crowd.go` with:

```go
package domain

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"slices"
)

// CrowdTick reports what the crowd did during one tick.
type CrowdTick struct {
	BoostersOpened int
	// CardsDrawn is the number of cards the opened boosters contained.
	CardsDrawn int
	// CardsSold is the number of cards sent to the Counter as surplus: right after the draw, or
	// when they leave a shelf, see WithSurplusHold.
	CardsSold int
}

// CrowdOption tunes NewCrowd.
type CrowdOption func(*Crowd)

// WithSurplusHold makes NPCs keep the copies above their keep threshold on a shelf for days
// before selling them to the Counter. While on a shelf the copies can be traded, see ADR-0011.
// Without the option, or with no days, an NPC sells its surplus the moment it draws it.
func WithSurplusHold(days int) CrowdOption {
	return func(c *Crowd) { c.holdTicks = days * TicksPerDay }
}

// Crowd is the NPC population and what it needs to act: the sets it opens, the opener that
// draws the boosters and the Counter that receives its surplus. It is not safe for concurrent
// use.
type Crowd struct {
	npcs    []*NPC
	byID    map[int]*NPC
	sets    []SetCatalogue
	setByID map[string]SetCatalogue
	opener  *Opener
	counter *Counter
	rng     *rand.Rand

	tick      int // ticks elapsed since the crowd was built
	holdTicks int // how long surplus waits on a shelf; zero sells it at once
	held      int // cards in all NPC collections and on all shelves

	// scopeAll and scopeBySet are the cards an NPC may wish for, sorted by ID: every card of every
	// set, or the cards of one set for an NPC with a focus set.
	scopeAll   []Card
	scopeBySet map[string][]Card
	// wishers and holders index the NPCs by the card they wish for and by the card they hold on
	// their shelf, so a proposal looks a card up instead of scanning every NPC.
	wishers map[CardID]map[int]struct{}
	holders map[CardKey]map[int]struct{}
}

// NewCrowd builds a crowd from npcs generated by GeneratePopulation.
//
// opener and rng must come from the same seeded stream, dedicated to the crowd, so that one seed
// reproduces the crowd and the crowd never moves the player's draws. rng chooses the set an NPC
// opens, opener draws the cards, and the behaviours of the NPCs draw their wishes from it.
func NewCrowd(npcs []*NPC, sets []SetCatalogue, opener *Opener, counter *Counter, rng *rand.Rand, opts ...CrowdOption) (*Crowd, error) {
	switch {
	case opener == nil:
		return nil, fmt.Errorf("crowd: opener must not be nil")
	case counter == nil:
		return nil, fmt.Errorf("crowd: counter must not be nil")
	case rng == nil:
		return nil, fmt.Errorf("crowd: rng must not be nil")
	case len(npcs) > 0 && len(sets) == 0:
		return nil, fmt.Errorf("crowd: at least one set is required to open boosters from")
	}
	setByID := make(map[string]SetCatalogue, len(sets))
	for _, cat := range sets {
		setByID[cat.Code] = cat
	}
	for _, npc := range npcs {
		if _, ok := setByID[npc.FocusSet]; npc.FocusSet != "" && !ok {
			return nil, fmt.Errorf("crowd: NPC %d: focus set %q is not among the sets", npc.ID, npc.FocusSet)
		}
	}
	c := &Crowd{
		npcs: npcs, byID: make(map[int]*NPC, len(npcs)), sets: sets, setByID: setByID,
		opener: opener, counter: counter, rng: rng,
		scopeBySet: make(map[string][]Card, len(sets)),
		wishers:    make(map[CardID]map[int]struct{}),
		holders:    make(map[CardKey]map[int]struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}
	for _, npc := range npcs {
		c.byID[npc.ID] = npc
	}
	for _, cat := range sets {
		scope := make([]Card, 0, len(cat.Cards))
		for _, card := range cat.Cards {
			if !card.IsBasicLand {
				scope = append(scope, card)
			}
		}
		slices.SortFunc(scope, func(a, b Card) int { return cmp.Compare(a.ID, b.ID) })
		c.scopeBySet[cat.Code] = scope
		c.scopeAll = append(c.scopeAll, scope...)
	}
	for _, npc := range npcs {
		c.refreshWishes(npc)
	}
	return c, nil
}

// Tick advances every NPC by one tick, in ID order. An NPC opens a booster each time it has
// accumulated a day's worth of credit, so over TicksPerDay ticks it opens exactly its
// BoostersPerDay. At the end of each day, shelf copies past their hold go to the Counter.
func (c *Crowd) Tick() (CrowdTick, error) {
	c.tick++
	var total CrowdTick
	for _, npc := range c.npcs {
		npc.credit += npc.BoostersPerDay
		for npc.credit >= TicksPerDay {
			npc.credit -= TicksPerDay
			drawn, sold, err := c.openBooster(npc)
			if err != nil {
				return total, fmt.Errorf("crowd: NPC %d: %w", npc.ID, err)
			}
			total.BoostersOpened++
			total.CardsDrawn += drawn
			total.CardsSold += sold
		}
	}
	if c.holdTicks > 0 && c.tick%TicksPerDay == 0 {
		released, err := c.releaseAged()
		total.CardsSold += released
		if err != nil {
			return total, fmt.Errorf("crowd: release surplus: %w", err)
		}
	}
	return total, nil
}

// CardsHeld returns the number of cards in all NPC collections and on all shelves.
func (c *Crowd) CardsHeld() int { return c.held }

// openBooster opens one booster for npc. Each drawn copy that takes the NPC above its keep
// threshold is surplus: it goes to the Counter at once, or onto the shelf when the crowd holds
// surplus. The copy moved is the one just drawn, so no scan of the whole collection is needed.
func (c *Crowd) openBooster(npc *NPC) (drawn, sold int, err error) {
	cat := c.pickSet(npc)
	booster, err := c.opener.Open(BoosterLayoutClassic, cat)
	if err != nil {
		return 0, 0, fmt.Errorf("open booster of %q: %w", cat.Code, err)
	}
	for _, key := range booster.Cards {
		if err := npc.collection.Add(key, 1); err != nil {
			return drawn, sold, err
		}
		drawn++
		c.held++
		if npc.collection.Copies(key.ID) <= npc.KeepCopies {
			continue
		}
		if err := npc.collection.Remove(key, 1); err != nil {
			return drawn, sold, err
		}
		if c.holdTicks > 0 {
			c.shelfPut(npc, key)
			continue
		}
		if err := c.counter.AcceptSurplus(key, 1); err != nil {
			return drawn, sold, fmt.Errorf("sell surplus of %q: %w", key.ID, err)
		}
		c.held--
		sold++
	}
	c.refreshWishes(npc)
	return drawn, sold, nil
}

func (c *Crowd) pickSet(npc *NPC) SetCatalogue {
	if npc.FocusSet != "" {
		return c.setByID[npc.FocusSet]
	}
	return c.sets[c.rng.IntN(len(c.sets))]
}
```

`internal/domain/npc_shelf.go`:

```go
package domain

import (
	"fmt"
	"maps"
	"slices"
)

// Shelf returns what the NPC can trade away: the surplus it holds before selling it to the
// Counter, sorted by card.
func (n *NPC) Shelf() []TradeLine {
	lines := make([]TradeLine, 0, len(n.shelf))
	for key, ticks := range n.shelf {
		lines = append(lines, TradeLine{Key: key, Qty: len(ticks)})
	}
	slices.SortFunc(lines, func(a, b TradeLine) int { return cmpKey(a.Key, b.Key) })
	return lines
}

// ShelfQuantity returns how many copies of key the NPC holds on its shelf.
func (n *NPC) ShelfQuantity(key CardKey) int { return len(n.shelf[key]) }

// Wishes returns a copy of the NPC's wish list.
func (n *NPC) Wishes() []Wish { return slices.Clone(n.wishes) }

// HoldersOf returns the IDs of the NPCs holding key on their shelf, ascending.
func (c *Crowd) HoldersOf(key CardKey) []int {
	return slices.Sorted(maps.Keys(c.holders[key]))
}

// indexAdd records that NPC npc relates to key in idx.
func indexAdd[K comparable](idx map[K]map[int]struct{}, key K, npc int) {
	if idx[key] == nil {
		idx[key] = make(map[int]struct{})
	}
	idx[key][npc] = struct{}{}
}

// indexRemove drops the link between NPC npc and key in idx, and the key when no NPC is left.
func indexRemove[K comparable](idx map[K]map[int]struct{}, key K, npc int) {
	delete(idx[key], npc)
	if len(idx[key]) == 0 {
		delete(idx, key)
	}
}

// shelfPut puts one copy of key on npc's shelf, stamped with the current tick.
func (c *Crowd) shelfPut(npc *NPC, key CardKey) {
	if len(npc.shelf[key]) == 0 {
		indexAdd(c.holders, key, npc.ID)
	}
	npc.shelf[key] = append(npc.shelf[key], c.tick)
}

// shelfTake removes the n oldest copies of key from npc's shelf.
func (c *Crowd) shelfTake(npc *NPC, key CardKey, n int) error {
	if n <= 0 {
		return &InvalidQuantityError{N: n}
	}
	ticks := npc.shelf[key]
	if len(ticks) < n {
		return &InsufficientQuantityError{Key: key, Have: len(ticks), Want: n}
	}
	if len(ticks) == n {
		delete(npc.shelf, key)
		indexRemove(c.holders, key, npc.ID)
		return nil
	}
	npc.shelf[key] = ticks[n:]
	return nil
}

// releaseAged sells to the Counter every shelf copy that has waited the hold, and returns how many.
func (c *Crowd) releaseAged() (int, error) {
	var released int
	for _, npc := range c.npcs {
		if len(npc.shelf) == 0 {
			continue
		}
		for _, key := range slices.SortedFunc(maps.Keys(npc.shelf), cmpKey) {
			ticks := npc.shelf[key]
			aged := 0
			for aged < len(ticks) && c.tick-ticks[aged] >= c.holdTicks {
				aged++
			}
			if aged == 0 {
				continue
			}
			if err := c.counter.AcceptSurplus(key, aged); err != nil {
				return released, fmt.Errorf("NPC %d: sell surplus of %q: %w", npc.ID, key.ID, err)
			}
			if err := c.shelfTake(npc, key, aged); err != nil {
				return released, fmt.Errorf("NPC %d: %w", npc.ID, err)
			}
			c.held -= aged
			released += aged
		}
	}
	return released, nil
}

// applyTrade moves the cards of a trade on the NPC's side: it gives the lines in gives from its
// shelf and receives the lines in gets. A received copy above the keep threshold goes onto the
// shelf. The NPC's wishes are refreshed afterwards. The caller has checked the NPC holds
// everything it gives.
func (c *Crowd) applyTrade(npc *NPC, gives, gets []TradeLine) error {
	for _, l := range gives {
		if err := c.shelfTake(npc, l.Key, l.Qty); err != nil {
			return err
		}
		c.held -= l.Qty
	}
	for _, l := range gets {
		if err := npc.collection.Add(l.Key, l.Qty); err != nil {
			return err
		}
		c.held += l.Qty
		for range l.Qty {
			if npc.collection.Copies(l.Key.ID) <= npc.KeepCopies {
				break
			}
			if err := npc.collection.Remove(l.Key, 1); err != nil {
				return err
			}
			c.shelfPut(npc, l.Key)
		}
	}
	c.refreshWishes(npc)
	return nil
}

// refreshWishes drops the wishes the NPC has fulfilled and, when the list is below the NPC's
// wish size, asks its behaviour for more.
func (c *Crowd) refreshWishes(npc *NPC) {
	if npc.Behaviour == nil || npc.WishSize <= 0 {
		return
	}
	kept := npc.wishes[:0]
	for _, w := range npc.wishes {
		if npc.collection.Copies(w.ID) >= w.Want {
			indexRemove(c.wishers, w.ID, npc.ID)
			continue
		}
		kept = append(kept, w)
	}
	npc.wishes = kept
	if len(kept) >= npc.WishSize {
		return
	}
	wished := make(map[CardID]struct{}, len(kept))
	for _, w := range kept {
		wished[w.ID] = struct{}{}
	}
	ctx := WishContext{NPC: npc, Scope: c.scopeOf(npc), Wished: wished}
	for _, w := range npc.Behaviour.PickWishes(ctx, npc.WishSize-len(kept), c.rng) {
		npc.wishes = append(npc.wishes, w)
		indexAdd(c.wishers, w.ID, npc.ID)
	}
}

// scopeOf returns the cards npc may wish for.
func (c *Crowd) scopeOf(npc *NPC) []Card {
	if npc.FocusSet != "" {
		return c.scopeBySet[npc.FocusSet]
	}
	return c.scopeAll
}
```

`internal/domain/export_trades_test.go`:

```go
package domain

import (
	"maps"
	"slices"
)

// ShelveForTest puts n copies of key on the shelf of NPC npcID, stamped with the current tick.
func (c *Crowd) ShelveForTest(npcID int, key CardKey, n int) {
	npc := c.byID[npcID]
	for range n {
		c.shelfPut(npc, key)
	}
}

// UnshelveForTest removes n copies of key from the shelf of NPC npcID.
func (c *Crowd) UnshelveForTest(npcID int, key CardKey, n int) error {
	npc := c.byID[npcID]
	if err := c.shelfTake(npc, key, n); err != nil {
		return err
	}
	c.held -= n
	return nil
}

// SetWishesForTest replaces the wish list of NPC npcID and keeps the wishers index in step.
func (c *Crowd) SetWishesForTest(npcID int, wishes []Wish) {
	npc := c.byID[npcID]
	for _, w := range npc.wishes {
		indexRemove(c.wishers, w.ID, npcID)
	}
	npc.wishes = slices.Clone(wishes)
	for _, w := range wishes {
		indexAdd(c.wishers, w.ID, npcID)
	}
}

// WishersForTest returns the IDs of the NPCs that wish for id, ascending.
func (c *Crowd) WishersForTest(id CardID) []int {
	return slices.Sorted(maps.Keys(c.wishers[id]))
}
```

- [ ] **Step 4: Run the domain tests**

Run: `go test -race ./internal/domain/...`
Expected: PASS. The existing crowd tests (rhythm, keep threshold, determinism) still pass: with no `WithSurplusHold` the crowd behaves as before.

- [ ] **Step 5: Refactor**

Re-read `crowd_shelf_test.go` and the existing crowd tests for overlap, for example the held-card arithmetic. Merge or drop tests a broader one already covers. Re-run the domain tests.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal
git add internal/domain
git commit -m "Give NPCs a shelf of surplus, wish lists and card indexes" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Exchange: proposals, acceptance, atomic transfer, history and stats

**Files:**
- Create: `internal/domain/exchange.go`, `internal/domain/exchange_trade.go`
- Test: `internal/domain/exchange_fixture_test.go`, `internal/domain/exchange_test.go`

**Interfaces:**
- Consumes: everything from Tasks 1 to 5.
- Produces:
  - `type Outcome int` with `OutcomeAccepted`, `OutcomeCountered`, `OutcomeRefused`, `OutcomeExpired`
  - `type Decision struct{ Outcome Outcome; Offer Offer; Reason error }`
  - `type ExchangeStats struct{ PlayerProposals, NPCOffers, Accepted, Countered, Refused, Expired int; VolumeCoins, TopUpCoins Coins }`
  - `type HistoryEntry struct{ Day int; Offer Offer; Outcome Outcome; Reason error }`
  - `type ExchangeParams struct{ ExpiryDays, MaxCardsPerSide, DuplicateKeep, MaxOffersPerDay, HistorySize int; CoinRate CoinRate }`
  - `func NewExchange(p ExchangeParams, crowd *Crowd, player *Collection, journal *Journal, valuer Valuer) (*Exchange, error)`
  - `func (x *Exchange) Propose(npcID int, playerGives, npcGives []TradeLine, topUp Coins) (Decision, error)`
  - `func (x *Exchange) Accept(offerID int) (Decision, error)`
  - `func (x *Exchange) Offers() []Offer`, `NPCCount() int`, `Shelf(npcID int) []TradeLine`, `HoldersOf(key CardKey) []int`, `TakeDayStats() ExchangeStats`, `History() []HistoryEntry`
  - Sentinels: `ErrUnknownNPC`, `ErrNPCNotTrading`, `ErrEmptyOffer`, `ErrNothingOffered`, `ErrTooManyCards`, `ErrPlayerLacksCard`, `ErrNPCLacksCard`, `ErrBelowMargin`, `ErrOfferNotFound`, `ErrNotCounterable`
  - unexported: `x.evaluate`, `x.prepare`, `x.execute`, `x.refuse`, `x.accepted`, `x.counterWith`, `x.store`, `x.close`, `x.record`, `x.newID`, fields `x.day`, `x.dayOffers`, `x.open`, `x.openCount`.

- [ ] **Step 1: Write the failing tests**

`internal/domain/exchange_fixture_test.go`:

```go
package domain_test

import (
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/domain/domaintest"
)

const exchangeFallbackPrice domain.USDCents = 10

// exchangeFixture is an Exchange over a real crowd, Counter and journal. NPCs open no boosters
// (their rhythm is 0), so a test puts exactly the cards it wants on their shelves.
type exchangeFixture struct {
	exchange *domain.Exchange
	crowd    *domain.Crowd
	npcs     []*domain.NPC
	player   *domain.Collection
	journal  *domain.Journal
	prices   domain.PriceSnapshot
	// keys are the non-basic non-foil printings of the first set, sorted by ID.
	keys []domain.CardKey
}

// tradingSpec is an archetype of NPCs that trade with the given margin and never open a booster.
func tradingSpec(margin domain.Percent) domain.ArchetypeSpec {
	return domain.ArchetypeSpec{Share: 1, KeepCopies: 1, Behaviour: "casual", MarginPct: margin}
}

func withExchangeDefaults(p domain.ExchangeParams) domain.ExchangeParams {
	if p.ExpiryDays == 0 {
		p.ExpiryDays = 3
	}
	if p.MaxCardsPerSide == 0 {
		p.MaxCardsPerSide = 10
	}
	if p.DuplicateKeep == 0 {
		p.DuplicateKeep = 1
	}
	if p.MaxOffersPerDay == 0 {
		p.MaxOffersPerDay = 20
	}
	if p.HistorySize == 0 {
		p.HistorySize = 50
	}
	if p.CoinRate == 0 {
		p.CoinRate = 100
	}
	return p
}

func newExchangeFixture(t *testing.T, size int, spec domain.ArchetypeSpec, params domain.ExchangeParams) *exchangeFixture {
	t.Helper()
	sets := crowdSets()
	prices := domain.PriceSnapshot{}
	for i, cat := range sets {
		for id, price := range domaintest.NewPrices(cat, uint64(i+1)) {
			prices[id] = price
		}
	}
	valuer := domain.NewValuer(prices, exchangeFallbackPrice)
	rng := rand.New(rand.NewPCG(1, 1))
	npcs, err := domain.GeneratePopulation(domain.PopulationParams{Size: size, Archetypes: map[string]domain.ArchetypeSpec{"npc": spec}, Valuer: valuer}, sets, rng)
	if err != nil {
		t.Fatalf("GeneratePopulation: %v", err)
	}
	journal := domain.NewJournal()
	counter, err := domain.NewCounter(sets, prices, journal, domain.CounterParams{SellPct: 60, BuyPct: 120, CoinRate: 100, FallbackPrice: exchangeFallbackPrice})
	if err != nil {
		t.Fatalf("NewCounter: %v", err)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
	crowd, err := domain.NewCrowd(npcs, sets, opener, counter, rng, domain.WithSurplusHold(2))
	if err != nil {
		t.Fatalf("NewCrowd: %v", err)
	}
	player := domain.NewCollection()
	exchange, err := domain.NewExchange(withExchangeDefaults(params), crowd, player, journal, valuer)
	if err != nil {
		t.Fatalf("NewExchange: %v", err)
	}

	fx := &exchangeFixture{exchange: exchange, crowd: crowd, npcs: npcs, player: player, journal: journal, prices: prices}
	for id, card := range sets[0].Cards {
		if !card.IsBasicLand {
			fx.keys = append(fx.keys, domain.CardKey{ID: id, Finish: domain.FinishNonFoil})
		}
	}
	slices.SortFunc(fx.keys, func(a, b domain.CardKey) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
	return fx
}

// setPrice sets the reference value of key's printing, for the Counter and the Exchange alike.
func (fx *exchangeFixture) setPrice(key domain.CardKey, cents domain.USDCents) { fx.prices[key.ID] = cents }

func (fx *exchangeFixture) give(t *testing.T, key domain.CardKey, n int) {
	t.Helper()
	if err := fx.player.Add(key, n); err != nil {
		t.Fatalf("give: %v", err)
	}
}

func (fx *exchangeFixture) shelve(npcID int, key domain.CardKey, n int) { fx.crowd.ShelveForTest(npcID, key, n) }

func (fx *exchangeFixture) fund(t *testing.T, coins domain.Coins) {
	t.Helper()
	if err := fx.journal.Post(domain.Entry{From: domain.AccountWelcomeBonus, To: domain.AccountPlayer, Amount: coins}); err != nil {
		t.Fatalf("fund: %v", err)
	}
}

// fixtureState is everything a refused or failed trade must leave untouched.
type fixtureState struct {
	Player  map[domain.CardKey]int
	NPCHeld map[domain.CardKey]int
	Shelf   []domain.TradeLine
	Balance domain.Coins
	TopUp   domain.Coins
}

func (fx *exchangeFixture) state(npcID int) fixtureState {
	npc := fx.npcs[npcID]
	return fixtureState{
		Player:  fx.player.Snapshot(),
		NPCHeld: npc.Collection().Snapshot(),
		Shelf:   npc.Shelf(),
		Balance: fx.journal.Balance(domain.AccountPlayer),
		TopUp:   fx.journal.Balance(domain.AccountTradeTopUp),
	}
}

func (fx *exchangeFixture) requireUnchanged(t *testing.T, npcID int, before fixtureState) {
	t.Helper()
	if after := fx.state(npcID); !reflect.DeepEqual(before, after) {
		t.Errorf("state changed:\n before %+v\n after  %+v", before, after)
	}
}

func lineOf(key domain.CardKey, qty int) []domain.TradeLine {
	return []domain.TradeLine{{Key: key, Qty: qty}}
}
```

`internal/domain/exchange_test.go`:

```go
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
			give: func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[0], 1) },
			ask:  func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[1], 1) },
			wantErr: domain.ErrUnknownNPC,
		},
		{
			name: "a card the NPC does not hold on its shelf",
			setup: func(t *testing.T, fx *exchangeFixture) { fx.give(t, fx.keys[0], 1) },
			give:  func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[0], 1) },
			ask:   func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[1], 1) },
			wantErr: domain.ErrNPCLacksCard,
		},
		{
			name: "a card the player does not hold",
			setup: func(t *testing.T, fx *exchangeFixture) { fx.shelve(0, fx.keys[1], 1) },
			give:  func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[0], 1) },
			ask:   func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[1], 1) },
			wantErr: domain.ErrPlayerLacksCard,
		},
		{
			name: "a zero quantity",
			setup: func(t *testing.T, fx *exchangeFixture) { fx.give(t, fx.keys[0], 1); fx.shelve(0, fx.keys[1], 1) },
			give:  func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[0], 0) },
			ask:   func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[1], 1) },
			wantErr: domain.ErrInvalidLine,
		},
		{
			name: "a negative quantity",
			setup: func(t *testing.T, fx *exchangeFixture) { fx.give(t, fx.keys[0], 1); fx.shelve(0, fx.keys[1], 1) },
			give:  func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[0], 1) },
			ask:   func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[1], -2) },
			wantErr: domain.ErrInvalidLine,
		},
		{
			name: "a negative top-up",
			setup: func(t *testing.T, fx *exchangeFixture) { fx.give(t, fx.keys[0], 1); fx.shelve(0, fx.keys[1], 1) },
			give:  func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[0], 1) },
			ask:   func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[1], 1) },
			topUp: -5, wantErr: domain.ErrInvalidLine,
		},
		{
			name: "nothing asked of the NPC",
			setup: func(t *testing.T, fx *exchangeFixture) { fx.give(t, fx.keys[0], 1) },
			give:  func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[0], 1) },
			ask:   func(fx *exchangeFixture) []domain.TradeLine { return nil },
			wantErr: domain.ErrEmptyOffer,
		},
		{
			name: "a priced card asked for with nothing offered",
			setup: func(t *testing.T, fx *exchangeFixture) { fx.shelve(0, fx.keys[1], 1) },
			give:  func(fx *exchangeFixture) []domain.TradeLine { return nil },
			ask:   func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[1], 1) },
			wantErr: domain.ErrNothingOffered,
		},
		{
			name: "more cards than one side may hold", params: domain.ExchangeParams{MaxCardsPerSide: 2},
			setup: func(t *testing.T, fx *exchangeFixture) { fx.give(t, fx.keys[0], 3); fx.shelve(0, fx.keys[1], 1) },
			give:  func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[0], 3) },
			ask:   func(fx *exchangeFixture) []domain.TradeLine { return lineOf(fx.keys[1], 1) },
			wantErr: domain.ErrTooManyCards,
		},
		{
			name: "a top-up the player cannot pay",
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
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/domain/ -run 'TestPropose|TestAccept|TestHistory|TestNewExchange'`
Expected: FAIL to compile, `undefined: domain.NewExchange`.

- [ ] **Step 3: Implement**

`internal/domain/exchange.go`:

```go
package domain

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

var (
	// ErrUnknownNPC is the reason of a refusal for an NPC ID the crowd does not have.
	ErrUnknownNPC = errors.New("exchange: unknown NPC")
	// ErrNPCNotTrading is the reason of a refusal for an NPC whose archetype has no behaviour.
	ErrNPCNotTrading = errors.New("exchange: the NPC does not trade")
	// ErrEmptyOffer is the reason of a refusal when the NPC is asked to give no card.
	ErrEmptyOffer = errors.New("exchange: the NPC must give at least one card")
	// ErrNothingOffered is the reason of a refusal when the player gives no card and no coin.
	ErrNothingOffered = errors.New("exchange: the player must give a card or coins")
	// ErrTooManyCards is the reason of a refusal when a side holds more cards than allowed.
	ErrTooManyCards = errors.New("exchange: too many cards on one side")
	// ErrPlayerLacksCard is the reason of a refusal when the player does not hold what it offers.
	ErrPlayerLacksCard = errors.New("exchange: the player does not hold the offered cards")
	// ErrNPCLacksCard is the reason of a refusal when the NPC's shelf lacks what it is asked for.
	ErrNPCLacksCard = errors.New("exchange: the NPC does not hold the requested cards")
	// ErrBelowMargin is the reason of a refusal when the NPC would receive less than it gives plus
	// its margin and may not counter.
	ErrBelowMargin = errors.New("exchange: the NPC would receive less than it gives plus its margin")
	// ErrOfferNotFound is returned for an offer ID that is not open: it never existed, was
	// answered, or has lapsed.
	ErrOfferNotFound = errors.New("exchange: no such open offer")
	// ErrNotCounterable is returned when the player counters an offer that is not an NPC's own
	// proposal. An NPC counters once, see ADR-0012.
	ErrNotCounterable = errors.New("exchange: only an offer proposed by an NPC can be countered")
)

// Outcome is how an offer ended or was answered.
type Outcome int

const (
	// OutcomeAccepted means the trade was executed.
	OutcomeAccepted Outcome = iota + 1
	// OutcomeCountered means the NPC answered with a counter-offer, now open.
	OutcomeCountered
	// OutcomeRefused means nothing happened: see Decision.Reason.
	OutcomeRefused
	// OutcomeExpired means an open offer lapsed unanswered.
	OutcomeExpired
)

// Decision is the Exchange's answer to a proposal or an acceptance.
type Decision struct {
	Outcome Outcome
	// Offer is the trade executed when accepted, or the NPC's counter-offer, open until it lapses,
	// when countered. It is the zero Offer when refused.
	Offer Offer
	// Reason says why an offer was refused, and is nil otherwise.
	Reason error
}

// ExchangeStats are the trade counters of a period, see Exchange.TakeDayStats.
type ExchangeStats struct {
	// PlayerProposals counts the offers the player made, counters to NPC offers included.
	PlayerProposals int
	// NPCOffers counts the offers NPCs made to the player.
	NPCOffers int
	Accepted  int
	// Countered counts the counter-offers NPCs made.
	Countered int
	Refused   int
	Expired   int
	// VolumeCoins is the reference value of the cards moved by accepted trades, in coins.
	VolumeCoins Coins
	// TopUpCoins is the coins the player paid on top of cards.
	TopUpCoins Coins
}

// HistoryEntry records how one offer was answered.
type HistoryEntry struct {
	Day     int
	Offer   Offer
	Outcome Outcome
	Reason  error
}

// ExchangeParams configures an Exchange. Every field comes from the configuration.
type ExchangeParams struct {
	// ExpiryDays is how many days an offer stays open, the day it is made included.
	ExpiryDays int
	// MaxCardsPerSide bounds the cards each side of an offer gives, counting quantities.
	MaxCardsPerSide int
	// DuplicateKeep is how many copies of a printing the player keeps out of NPC proposals: the
	// copies above it are the player's duplicates. It must be in [1, PlaysetSize].
	DuplicateKeep int
	// MaxOffersPerDay bounds the offers the crowd makes the player each day.
	MaxOffersPerDay int
	// HistorySize is how many answered offers History keeps. Zero keeps none.
	HistorySize int
	// CoinRate is the coins per US dollar, to put a coin top-up and a card on one scale.
	CoinRate CoinRate
}

// Exchange runs the trades between the player and the NPCs of a crowd: it prices offers, applies
// the acceptance rule, counters once and moves the cards atomically. It is not safe for
// concurrent use.
type Exchange struct {
	params  ExchangeParams
	crowd   *Crowd
	player  *Collection
	journal *Journal
	valuer  Valuer

	day       int
	dayOffers int // offers NPCs made today
	nextID    int
	open      map[int]Offer
	openCount map[int]int // open offers per NPC
	stats     ExchangeStats
	history   []HistoryEntry
}

// NewExchange builds an Exchange between player and the NPCs of crowd. Top-ups are posted to
// journal.
//
// The crowd must hold surplus on a shelf, see WithSurplusHold: an NPC with no shelf has nothing to
// give, so every trade would be refused.
func NewExchange(p ExchangeParams, crowd *Crowd, player *Collection, journal *Journal, valuer Valuer) (*Exchange, error) {
	switch {
	case crowd == nil:
		return nil, fmt.Errorf("exchange: crowd must not be nil")
	case player == nil:
		return nil, fmt.Errorf("exchange: player collection must not be nil")
	case journal == nil:
		return nil, fmt.Errorf("exchange: journal must not be nil")
	case crowd.holdTicks <= 0:
		return nil, fmt.Errorf("exchange: the crowd holds no surplus, so NPCs have nothing to trade: build it with a surplus hold above zero")
	case p.CoinRate <= 0:
		return nil, fmt.Errorf("exchange: coin rate must be > 0, got %d", p.CoinRate)
	case p.ExpiryDays < 1:
		return nil, fmt.Errorf("exchange: expiry days must be >= 1, got %d", p.ExpiryDays)
	case p.MaxCardsPerSide < 1:
		return nil, fmt.Errorf("exchange: max cards per side must be >= 1, got %d", p.MaxCardsPerSide)
	case p.DuplicateKeep < 1 || p.DuplicateKeep > PlaysetSize:
		return nil, fmt.Errorf("exchange: duplicate keep must be in [1, %d], got %d", PlaysetSize, p.DuplicateKeep)
	case p.MaxOffersPerDay < 0:
		return nil, fmt.Errorf("exchange: max offers per day must be >= 0, got %d", p.MaxOffersPerDay)
	case p.HistorySize < 0:
		return nil, fmt.Errorf("exchange: history size must be >= 0, got %d", p.HistorySize)
	}
	return &Exchange{
		params: p, crowd: crowd, player: player, journal: journal, valuer: valuer,
		open: make(map[int]Offer), openCount: make(map[int]int),
	}, nil
}

// Offers returns the open offers, oldest first.
func (x *Exchange) Offers() []Offer {
	offers := make([]Offer, 0, len(x.open))
	for _, id := range slices.Sorted(maps.Keys(x.open)) {
		offers = append(offers, x.open[id])
	}
	return offers
}

// NPCCount returns the number of NPCs. Their IDs run from 0 to NPCCount()-1.
func (x *Exchange) NPCCount() int { return len(x.crowd.npcs) }

// Shelf returns what NPC npcID can trade away, sorted by card, or nil for an unknown NPC.
func (x *Exchange) Shelf(npcID int) []TradeLine {
	npc, ok := x.crowd.byID[npcID]
	if !ok {
		return nil
	}
	return npc.Shelf()
}

// HoldersOf returns the IDs of the NPCs holding key on their shelf, ascending.
func (x *Exchange) HoldersOf(key CardKey) []int { return x.crowd.HoldersOf(key) }

// TakeDayStats returns the counters since the last call and resets them.
func (x *Exchange) TakeDayStats() ExchangeStats {
	s := x.stats
	x.stats = ExchangeStats{}
	return s
}

// History returns the last HistorySize answered offers, oldest first.
func (x *Exchange) History() []HistoryEntry { return slices.Clone(x.history) }
```

`internal/domain/exchange_trade.go`:

```go
package domain

import (
	"errors"
	"fmt"
)

// Propose offers the NPC npcID a swap: the player gives playerGives and topUp coins, the NPC gives
// npcGives from its shelf.
//
// The NPC accepts when it receives at least the reference value it gives plus its margin, and the
// trade executes at once. Otherwise it counters once, with the same cards and the smallest top-up
// that satisfies it: the counter is an open offer the player can Accept. A malformed or
// impossible offer is refused, with the reason in the Decision. The error is for failures of the
// Exchange itself, which abort a run.
func (x *Exchange) Propose(npcID int, playerGives, npcGives []TradeLine, topUp Coins) (Decision, error) {
	x.stats.PlayerProposals++
	return x.evaluate(npcID, playerGives, npcGives, topUp, true)
}

// Accept executes the open offer offerID, as the player's answer to an NPC's offer or counter.
//
// It returns ErrOfferNotFound for an offer that is not open, and ErrInsufficientBalance, leaving
// the offer open, when the player cannot pay the top-up. When what the offer needs is gone, the NPC
// traded or sold the card, or the player sold its own, the offer is closed and refused.
func (x *Exchange) Accept(offerID int) (Decision, error) {
	offer, ok := x.open[offerID]
	if !ok {
		return Decision{}, fmt.Errorf("%w: %d", ErrOfferNotFound, offerID)
	}
	err := x.execute(x.crowd.byID[offer.NPC], offer)
	switch {
	case err == nil:
		x.close(offerID)
		return x.accepted(offer), nil
	case errors.Is(err, ErrInsufficientBalance):
		return Decision{}, err
	case errors.Is(err, ErrPlayerLacksCard), errors.Is(err, ErrNPCLacksCard):
		x.close(offerID)
		return x.refuse(offer, err), nil
	}
	return Decision{}, fmt.Errorf("exchange: accept offer %d: %w", offerID, err)
}

// evaluate prices an offer and either executes it, counters it or refuses it. When canCounter is
// false a short offer is refused instead of countered.
func (x *Exchange) evaluate(npcID int, playerGives, npcGives []TradeLine, topUp Coins, canCounter bool) (Decision, error) {
	npc, offer, err := x.prepare(npcID, playerGives, npcGives, topUp)
	if err != nil {
		return x.refuse(offer, err), nil
	}
	need := requiredTopUp(linesValue(x.valuer, offer.PlayerGives), linesValue(x.valuer, offer.NPCGives),
		npc.Behaviour.Margin(offer), x.params.CoinRate)
	if topUp < need {
		if !canCounter {
			return x.refuse(offer, ErrBelowMargin), nil
		}
		return x.counterWith(offer, need), nil
	}
	err = x.execute(npc, offer)
	switch {
	case errors.Is(err, ErrInsufficientBalance):
		return x.refuse(offer, err), nil
	case err != nil:
		return Decision{}, fmt.Errorf("exchange: trade with NPC %d: %w", npcID, err)
	}
	return x.accepted(offer), nil
}

// prepare validates an offer and returns the NPC and the offer with its lines normalised. On an
// error the returned Offer is still filled in as far as it got, for the history.
func (x *Exchange) prepare(npcID int, playerGives, npcGives []TradeLine, topUp Coins) (*NPC, Offer, error) {
	offer := Offer{NPC: npcID, Origin: OriginPlayer, TopUp: topUp, CreatedDay: x.day, LastDay: x.day}
	npc, ok := x.crowd.byID[npcID]
	if !ok {
		return nil, offer, fmt.Errorf("%w: %d", ErrUnknownNPC, npcID)
	}
	if npc.Behaviour == nil {
		return nil, offer, fmt.Errorf("%w: NPC %d", ErrNPCNotTrading, npcID)
	}
	var err error
	if offer.PlayerGives, err = normalizeLines(playerGives); err != nil {
		return nil, offer, err
	}
	if offer.NPCGives, err = normalizeLines(npcGives); err != nil {
		return nil, offer, err
	}
	switch {
	case topUp < 0:
		return nil, offer, fmt.Errorf("%w: top-up is %d", ErrInvalidLine, topUp)
	case len(offer.NPCGives) == 0:
		return nil, offer, ErrEmptyOffer
	case len(offer.PlayerGives) == 0 && topUp == 0:
		return nil, offer, ErrNothingOffered
	case cardCount(offer.PlayerGives) > x.params.MaxCardsPerSide || cardCount(offer.NPCGives) > x.params.MaxCardsPerSide:
		return nil, offer, fmt.Errorf("%w: at most %d", ErrTooManyCards, x.params.MaxCardsPerSide)
	}
	if err := x.holdingsError(npc, offer); err != nil {
		return nil, offer, err
	}
	return npc, offer, nil
}

// holdingsError checks that the player holds what it gives and the NPC's shelf holds what it gives.
func (x *Exchange) holdingsError(npc *NPC, o Offer) error {
	for _, l := range o.PlayerGives {
		if x.player.Quantity(l.Key) < l.Qty {
			return fmt.Errorf("%w: %s x%d", ErrPlayerLacksCard, l.Key.ID, l.Qty)
		}
	}
	for _, l := range o.NPCGives {
		if npc.ShelfQuantity(l.Key) < l.Qty {
			return fmt.Errorf("%w: %s x%d", ErrNPCLacksCard, l.Key.ID, l.Qty)
		}
	}
	return nil
}

// execute moves the cards and the top-up of o atomically: it checks both sides hold what they
// give, posts the top-up, which is the only step that can still fail, and then moves the cards.
func (x *Exchange) execute(npc *NPC, o Offer) error {
	if err := x.holdingsError(npc, o); err != nil {
		return err
	}
	if o.TopUp > 0 {
		if err := x.journal.Post(Entry{
			From: AccountPlayer, To: AccountTradeTopUp, Amount: o.TopUp,
			Memo: fmt.Sprintf("trade top-up: NPC %d", npc.ID),
		}); err != nil {
			return err
		}
	}
	for _, l := range o.PlayerGives {
		if err := x.player.Remove(l.Key, l.Qty); err != nil {
			return err
		}
	}
	for _, l := range o.NPCGives {
		if err := x.player.Add(l.Key, l.Qty); err != nil {
			return err
		}
	}
	return x.crowd.applyTrade(npc, o.NPCGives, o.PlayerGives)
}

// accepted books an executed offer and returns the Decision.
func (x *Exchange) accepted(o Offer) Decision {
	if o.ID == 0 {
		o.ID = x.newID()
	}
	x.stats.Accepted++
	x.stats.TopUpCoins += o.TopUp
	x.stats.VolumeCoins += x.coinsOf(linesValue(x.valuer, o.PlayerGives) + linesValue(x.valuer, o.NPCGives))
	x.record(o, OutcomeAccepted, nil)
	return Decision{Outcome: OutcomeAccepted, Offer: o}
}

// refuse books a refusal and returns the Decision.
func (x *Exchange) refuse(o Offer, reason error) Decision {
	x.stats.Refused++
	x.record(o, OutcomeRefused, reason)
	return Decision{Outcome: OutcomeRefused, Reason: reason}
}

// counterWith stores the NPC's counter-offer to o: the same cards with the top-up need.
func (x *Exchange) counterWith(o Offer, need Coins) Decision {
	o.ID = x.newID()
	o.Origin = OriginNPCCounter
	o.TopUp = need
	o.CreatedDay = x.day
	o.LastDay = x.day + x.params.ExpiryDays - 1
	x.store(o)
	x.stats.Countered++
	x.record(o, OutcomeCountered, nil)
	return Decision{Outcome: OutcomeCountered, Offer: o}
}

func (x *Exchange) newID() int {
	x.nextID++
	return x.nextID
}

func (x *Exchange) store(o Offer) {
	x.open[o.ID] = o
	x.openCount[o.NPC]++
}

func (x *Exchange) close(id int) {
	o, ok := x.open[id]
	if !ok {
		return
	}
	delete(x.open, id)
	x.openCount[o.NPC]--
	if x.openCount[o.NPC] <= 0 {
		delete(x.openCount, o.NPC)
	}
}

// record appends to the bounded history, dropping the oldest entries beyond HistorySize.
func (x *Exchange) record(o Offer, outcome Outcome, reason error) {
	if x.params.HistorySize == 0 {
		return
	}
	x.history = append(x.history, HistoryEntry{Day: x.day, Offer: o, Outcome: outcome, Reason: reason})
	if over := len(x.history) - x.params.HistorySize; over > 0 {
		x.history = append(x.history[:0:0], x.history[over:]...)
	}
}

// coinsOf converts a value in USD cents to whole coins at the coin rate.
func (x *Exchange) coinsOf(cents USDCents) Coins {
	return Coins(int64(cents) * int64(x.params.CoinRate) / 100)
}
```

- [ ] **Step 4: Run the domain tests**

Run: `go test -race ./internal/domain/...`
Expected: PASS.

- [ ] **Step 5: Refactor**

Check the 11-case refusal table and the setup closures for repetition: if three cases share a setup, hoist it into a named helper on the fixture. Do not weaken an assertion. Re-run the domain tests.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal
git add internal/domain
git commit -m "Add the Exchange: proposals, the acceptance rule and the atomic transfer" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Exchange: NPC offers, counters from the player, expiry

**Files:**
- Create: `internal/domain/exchange_offers.go`
- Test: `internal/domain/exchange_offers_test.go`

**Interfaces:**
- Consumes: Task 6 (`store`, `close`, `record`, `newID`, `evaluate`, `x.day`, `x.dayOffers`, `x.openCount`) and Task 5 (`crowd.wishers`, `crowd.byID`, `crowd.rng`).
- Produces: `func (x *Exchange) StartDay(day int)`, `func (x *Exchange) ProposeFromNPCs() error`, `func (x *Exchange) Counter(offerID int, playerGives []TradeLine, topUp Coins) (Decision, error)`.

- [ ] **Step 1: Write the failing tests**

```go
package domain_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// offerFixture is a fixture where NPC 0 wishes for keys[0] (worth 500), holds 3 copies of
// keys[1] (worth 100 each) on its shelf, and always proposes. The player holds two copies of
// keys[0], so one is a duplicate.
func offerFixture(t *testing.T, size int, margin domain.Percent, params domain.ExchangeParams) *exchangeFixture {
	t.Helper()
	spec := tradingSpec(margin)
	spec.OfferChancePct = 100
	fx := newExchangeFixture(t, size, spec, params)
	fx.setPrice(fx.keys[0], 500)
	fx.setPrice(fx.keys[1], 100)
	fx.give(t, fx.keys[0], 2)
	for id := range size {
		fx.crowd.SetWishesForTest(id, []domain.Wish{{ID: fx.keys[0].ID, Want: 1}})
		fx.shelve(id, fx.keys[1], 3)
	}
	fx.exchange.StartDay(1)
	return fx
}

func TestProposeFromNPCsMakesAnOfferForAMatchingDuplicate(t *testing.T) {
	t.Parallel()
	fx := offerFixture(t, 1, 0, domain.ExchangeParams{})

	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}

	offers := fx.exchange.Offers()
	if len(offers) != 1 {
		t.Fatalf("got %d offers, want 1", len(offers))
	}
	o := offers[0]
	if o.Origin != domain.OriginNPC || o.NPC != 0 || o.TopUp != 0 || o.CreatedDay != 1 || o.LastDay != 3 {
		t.Errorf("offer = %+v, want an NPC offer from NPC 0 with no top-up, made on day 1, open through day 3", o)
	}
	if want := lineOf(fx.keys[0], 1); !slices.Equal(o.PlayerGives, want) {
		t.Errorf("PlayerGives = %v, want %v: only the duplicate", o.PlayerGives, want)
	}
	if want := lineOf(fx.keys[1], 3); !slices.Equal(o.NPCGives, want) {
		t.Errorf("NPCGives = %v, want %v: three 100-cent cards for one 500-cent card", o.NPCGives, want)
	}
	if got := fx.exchange.TakeDayStats().NPCOffers; got != 1 {
		t.Errorf("NPCOffers = %d, want 1", got)
	}
}

func TestAcceptingAnNPCOfferTradesAndRefillsItsWishList(t *testing.T) {
	t.Parallel()
	spec := tradingSpec(0)
	spec.OfferChancePct = 100
	spec.WishSize = 3
	fx := newExchangeFixture(t, 1, spec, domain.ExchangeParams{})
	fx.setPrice(fx.keys[0], 500)
	fx.setPrice(fx.keys[1], 100)
	fx.give(t, fx.keys[0], 2)
	fx.crowd.SetWishesForTest(0, []domain.Wish{{ID: fx.keys[0].ID, Want: 1}})
	fx.shelve(0, fx.keys[1], 3)
	fx.exchange.StartDay(1)
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	offer := fx.exchange.Offers()[0]

	d, err := fx.exchange.Accept(offer.ID)

	if err != nil || d.Outcome != domain.OutcomeAccepted {
		t.Fatalf("Accept = %+v, %v, want accepted", d, err)
	}
	if got := fx.player.Quantity(fx.keys[1]); got != 3 {
		t.Errorf("player holds %d of the NPC's cards, want 3", got)
	}
	if got := fx.player.Quantity(fx.keys[0]); got != 1 {
		t.Errorf("player holds %d of the wished card, want 1 left", got)
	}
	if got := fx.npcs[0].Collection().Quantity(fx.keys[0]); got != 1 {
		t.Errorf("the NPC holds %d of its wished card, want 1", got)
	}
	wishes := fx.npcs[0].Wishes()
	if len(wishes) != 3 {
		t.Errorf("the NPC has %d wishes, want its list refilled to 3", len(wishes))
	}
	for _, w := range wishes {
		if w.ID == fx.keys[0].ID {
			t.Errorf("the NPC still wishes %q, which it now holds", w.ID)
		}
	}
	if len(fx.exchange.Offers()) != 0 {
		t.Error("the accepted offer is still open")
	}
}

func TestProposeFromNPCsMakesOneOpenOfferPerNPC(t *testing.T) {
	t.Parallel()
	fx := offerFixture(t, 1, 0, domain.ExchangeParams{})

	for range 3 {
		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
	}

	if got := len(fx.exchange.Offers()); got != 1 {
		t.Errorf("got %d open offers, want 1: an NPC waits for an answer", got)
	}
}

func TestProposeFromNPCsStopsAtTheDailyCap(t *testing.T) {
	t.Parallel()
	fx := offerFixture(t, 3, 0, domain.ExchangeParams{MaxOffersPerDay: 2})

	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	if got := len(fx.exchange.Offers()); got != 2 {
		t.Fatalf("day 1: got %d offers, want the cap of 2", got)
	}

	fx.exchange.StartDay(2)
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	if got := len(fx.exchange.Offers()); got != 3 {
		t.Errorf("day 2: got %d offers, want the third NPC to propose once the cap resets", got)
	}
}

func TestProposeFromNPCsNeedsAMatchAndAChance(t *testing.T) {
	t.Parallel()
	t.Run("no duplicate above the keep", func(t *testing.T) {
		t.Parallel()
		fx := offerFixture(t, 1, 0, domain.ExchangeParams{})
		if err := fx.player.Remove(fx.keys[0], 1); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		if got := len(fx.exchange.Offers()); got != 0 {
			t.Errorf("got %d offers for a card the player holds once, want 0", got)
		}
	})
	t.Run("a zero chance never proposes", func(t *testing.T) {
		t.Parallel()
		spec := tradingSpec(0)
		fx := newExchangeFixture(t, 1, spec, domain.ExchangeParams{})
		fx.give(t, fx.keys[0], 2)
		fx.crowd.SetWishesForTest(0, []domain.Wish{{ID: fx.keys[0].ID, Want: 1}})
		fx.shelve(0, fx.keys[1], 3)
		fx.exchange.StartDay(1)
		for range 50 {
			if err := fx.exchange.ProposeFromNPCs(); err != nil {
				t.Fatalf("ProposeFromNPCs: %v", err)
			}
		}
		if got := len(fx.exchange.Offers()); got != 0 {
			t.Errorf("got %d offers with a 0%% chance, want 0", got)
		}
	})
	t.Run("an empty shelf has nothing to give", func(t *testing.T) {
		t.Parallel()
		fx := offerFixture(t, 1, 0, domain.ExchangeParams{})
		if err := fx.crowd.UnshelveForTest(0, fx.keys[1], 3); err != nil {
			t.Fatalf("UnshelveForTest: %v", err)
		}
		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		if got := len(fx.exchange.Offers()); got != 0 {
			t.Errorf("got %d offers from an NPC with an empty shelf, want 0", got)
		}
	})
}

func TestOffersLapseAfterTheirLastDay(t *testing.T) {
	t.Parallel()
	fx := offerFixture(t, 1, 0, domain.ExchangeParams{ExpiryDays: 3})
	if err := fx.exchange.ProposeFromNPCs(); err != nil {
		t.Fatalf("ProposeFromNPCs: %v", err)
	}
	fx.exchange.TakeDayStats()

	fx.exchange.StartDay(3)
	if got := len(fx.exchange.Offers()); got != 1 {
		t.Fatalf("day 3: got %d offers, want the offer of day 1 still open: it lasts 3 days", got)
	}

	fx.exchange.StartDay(4)
	if got := len(fx.exchange.Offers()); got != 0 {
		t.Errorf("day 4: got %d offers, want the offer lapsed", got)
	}
	if got := fx.exchange.TakeDayStats().Expired; got != 1 {
		t.Errorf("Expired = %d, want 1", got)
	}
	history := fx.exchange.History()
	if len(history) == 0 || history[len(history)-1].Outcome != domain.OutcomeExpired {
		t.Errorf("history = %+v, want a last entry with the expired outcome", history)
	}
	if _, err := fx.exchange.Accept(1); !errors.Is(err, domain.ErrOfferNotFound) {
		t.Errorf("Accept of a lapsed offer = %v, want ErrOfferNotFound", err)
	}
}

func TestAcceptRefusesAndClosesAnOfferWhoseCardsAreGone(t *testing.T) {
	t.Parallel()
	t.Run("the NPC no longer holds what it gives", func(t *testing.T) {
		t.Parallel()
		fx := offerFixture(t, 1, 0, domain.ExchangeParams{})
		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		offer := fx.exchange.Offers()[0]
		if err := fx.crowd.UnshelveForTest(0, fx.keys[1], 1); err != nil {
			t.Fatalf("UnshelveForTest: %v", err)
		}
		before := fx.state(0)

		d, err := fx.exchange.Accept(offer.ID)

		if err != nil {
			t.Fatalf("Accept: %v, want no error for a stale offer", err)
		}
		if d.Outcome != domain.OutcomeRefused || !errors.Is(d.Reason, domain.ErrNPCLacksCard) {
			t.Errorf("Decision = %+v, want a refusal with ErrNPCLacksCard", d)
		}
		fx.requireUnchanged(t, 0, before)
		if len(fx.exchange.Offers()) != 0 {
			t.Error("the stale offer is still open")
		}
	})
	t.Run("the player sold what it offers", func(t *testing.T) {
		t.Parallel()
		fx := offerFixture(t, 1, 0, domain.ExchangeParams{})
		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		offer := fx.exchange.Offers()[0]
		if err := fx.player.Remove(fx.keys[0], 2); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		before := fx.state(0)

		d, err := fx.exchange.Accept(offer.ID)

		if err != nil || d.Outcome != domain.OutcomeRefused || !errors.Is(d.Reason, domain.ErrPlayerLacksCard) {
			t.Errorf("Accept = %+v, %v, want a refusal with ErrPlayerLacksCard", d, err)
		}
		fx.requireUnchanged(t, 0, before)
		if len(fx.exchange.Offers()) != 0 {
			t.Error("the stale offer is still open")
		}
	})
}

func TestCounterReplacesWhatThePlayerGives(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T, replacementPrice domain.USDCents) (*exchangeFixture, domain.Offer, domain.CardKey) {
		t.Helper()
		fx := offerFixture(t, 1, 0, domain.ExchangeParams{})
		replacement := fx.keys[2]
		fx.setPrice(replacement, replacementPrice)
		fx.give(t, replacement, 1)
		if err := fx.exchange.ProposeFromNPCs(); err != nil {
			t.Fatalf("ProposeFromNPCs: %v", err)
		}
		return fx, fx.exchange.Offers()[0], replacement
	}

	t.Run("a replacement worth enough is accepted", func(t *testing.T) {
		t.Parallel()
		fx, offer, replacement := setup(t, 300)

		d, err := fx.exchange.Counter(offer.ID, lineOf(replacement, 1), 0)

		if err != nil || d.Outcome != domain.OutcomeAccepted {
			t.Fatalf("Counter = %+v, %v, want accepted: 300 received for 300 given", d, err)
		}
		if got := fx.player.Quantity(fx.keys[1]); got != 3 {
			t.Errorf("player holds %d of the NPC's cards, want 3", got)
		}
		if len(fx.exchange.Offers()) != 0 {
			t.Error("the countered offer is still open")
		}
	})
	t.Run("a replacement worth too little is refused, the NPC does not counter again", func(t *testing.T) {
		t.Parallel()
		fx, offer, replacement := setup(t, 200)
		before := fx.state(0)

		d, err := fx.exchange.Counter(offer.ID, lineOf(replacement, 1), 0)

		if err != nil || d.Outcome != domain.OutcomeRefused || !errors.Is(d.Reason, domain.ErrBelowMargin) {
			t.Fatalf("Counter = %+v, %v, want a refusal with ErrBelowMargin", d, err)
		}
		fx.requireUnchanged(t, 0, before)
		if len(fx.exchange.Offers()) != 0 {
			t.Error("the original offer is still open after the player countered it")
		}
	})
}

func TestCounterRejectsUnknownAndNPCCounterOffers(t *testing.T) {
	t.Parallel()
	fx := newExchangeFixture(t, 1, tradingSpec(10), domain.ExchangeParams{})
	fx.setPrice(fx.keys[0], 100)
	fx.setPrice(fx.keys[1], 100)
	fx.give(t, fx.keys[0], 1)
	fx.shelve(0, fx.keys[1], 1)
	counter, err := fx.exchange.Propose(0, lineOf(fx.keys[0], 1), lineOf(fx.keys[1], 1), 0)
	if err != nil || counter.Outcome != domain.OutcomeCountered {
		t.Fatalf("setup: %+v, %v", counter, err)
	}

	if _, err := fx.exchange.Counter(counter.Offer.ID, lineOf(fx.keys[0], 1), 0); !errors.Is(err, domain.ErrNotCounterable) {
		t.Errorf("Counter of an NPC counter-offer = %v, want ErrNotCounterable: an NPC counters once", err)
	}
	if _, err := fx.exchange.Counter(999, lineOf(fx.keys[0], 1), 0); !errors.Is(err, domain.ErrOfferNotFound) {
		t.Errorf("Counter of an unknown offer = %v, want ErrOfferNotFound", err)
	}
}

func TestHoldersOfFollowsTheShelves(t *testing.T) {
	t.Parallel()
	fx := newExchangeFixture(t, 3, tradingSpec(0), domain.ExchangeParams{})
	fx.shelve(0, fx.keys[3], 1)
	fx.shelve(2, fx.keys[3], 2)

	if got, want := fx.exchange.HoldersOf(fx.keys[3]), []int{0, 2}; !slices.Equal(got, want) {
		t.Errorf("HoldersOf = %v, want %v", got, want)
	}
	if got, want := fx.exchange.Shelf(2), lineOf(fx.keys[3], 2); !slices.Equal(got, want) {
		t.Errorf("Shelf(2) = %v, want %v", got, want)
	}
	if fx.exchange.NPCCount() != 3 {
		t.Errorf("NPCCount = %d, want 3", fx.exchange.NPCCount())
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/domain/ -run 'TestProposeFromNPCs|TestAccepting|TestOffersLapse|TestAcceptRefuses|TestCounter|TestHoldersOf'`
Expected: FAIL to compile, `fx.exchange.StartDay undefined`.

- [ ] **Step 3: Implement**

`internal/domain/exchange_offers.go`:

```go
package domain

import (
	"fmt"
	"maps"
	"slices"
)

// StartDay opens day: it lapses the offers whose last day has passed and resets the daily cap on
// the offers NPCs make. Call it before the first tick of each day.
func (x *Exchange) StartDay(day int) {
	x.day = day
	x.dayOffers = 0
	for _, id := range slices.Sorted(maps.Keys(x.open)) {
		o := x.open[id]
		if o.LastDay >= day {
			continue
		}
		x.close(id)
		x.stats.Expired++
		x.record(o, OutcomeExpired, nil)
	}
}

// ProposeFromNPCs lets NPCs whose wish list matches a duplicate of the player propose a trade.
// Candidates are the NPCs that wish for a printing the player holds above DuplicateKeep copies,
// visited in ID order. An NPC with an open offer waits, and the rest propose with their
// OfferChancePct. Call it once per tick, after the crowd's tick.
func (x *Exchange) ProposeFromNPCs() error {
	if x.dayOffers >= x.params.MaxOffersPerDay {
		return nil
	}
	duplicates, err := x.player.DuplicatesAbove(x.params.DuplicateKeep)
	if err != nil {
		return fmt.Errorf("exchange: player duplicates: %w", err)
	}
	if len(duplicates) == 0 {
		return nil
	}
	candidates := make(map[int]struct{})
	for _, s := range duplicates {
		for id := range x.crowd.wishers[s.Key.ID] {
			candidates[id] = struct{}{}
		}
	}
	for _, id := range slices.Sorted(maps.Keys(candidates)) {
		if x.dayOffers >= x.params.MaxOffersPerDay {
			return nil
		}
		if x.openCount[id] > 0 {
			continue
		}
		npc := x.crowd.byID[id]
		if x.crowd.rng.IntN(100) >= npc.OfferChancePct {
			continue
		}
		x.offerFrom(npc, duplicates)
	}
	return nil
}

// offerFrom asks npc's behaviour for an offer to the player and stores it when there is one.
func (x *Exchange) offerFrom(npc *NPC, duplicates []Surplus) {
	shelf := npc.Shelf()
	if len(shelf) == 0 {
		return
	}
	want, give, ok := npc.Behaviour.Propose(ProposalContext{
		NPC: npc, Wanted: wantedLines(npc, duplicates), Shelf: shelf,
		PlayerCopies: x.player.Copies, MaxCards: x.params.MaxCardsPerSide,
	})
	if !ok {
		return
	}
	x.store(Offer{
		ID: x.newID(), NPC: npc.ID, Origin: OriginNPC, PlayerGives: want, NPCGives: give,
		CreatedDay: x.day, LastDay: x.day + x.params.ExpiryDays - 1,
	})
	x.dayOffers++
	x.stats.NPCOffers++
}

// wantedLines returns what npc asks of the player: for each wish, the player's duplicates of that
// printing, up to the copies the NPC still lacks.
func wantedLines(npc *NPC, duplicates []Surplus) []TradeLine {
	var lines []TradeLine
	for _, w := range npc.wishes {
		need := w.Want - npc.collection.Copies(w.ID)
		for _, s := range duplicates {
			if need <= 0 {
				break
			}
			if s.Key.ID != w.ID {
				continue
			}
			qty := min(s.Quantity, need)
			lines = append(lines, TradeLine{Key: s.Key, Qty: qty})
			need -= qty
		}
	}
	return lines
}

// Counter answers the open offer offerID, proposed by an NPC, with other cards: the player gives
// playerGives and topUp coins instead of what the offer asked, for the same cards from the NPC.
// The NPC accepts or refuses and does not counter again, see ADR-0012. The original offer is
// closed either way.
//
// It returns ErrOfferNotFound for an offer that is not open and ErrNotCounterable for an offer
// that is not an NPC's own proposal.
func (x *Exchange) Counter(offerID int, playerGives []TradeLine, topUp Coins) (Decision, error) {
	o, ok := x.open[offerID]
	if !ok {
		return Decision{}, fmt.Errorf("%w: %d", ErrOfferNotFound, offerID)
	}
	if o.Origin != OriginNPC {
		return Decision{}, fmt.Errorf("%w: offer %d", ErrNotCounterable, offerID)
	}
	x.close(offerID)
	x.record(o, OutcomeCountered, nil)
	x.stats.PlayerProposals++
	return x.evaluate(o.NPC, playerGives, o.NPCGives, topUp, false)
}
```

- [ ] **Step 4: Run the domain tests**

Run: `go test -race ./internal/domain/...`
Expected: PASS.

- [ ] **Step 5: Refactor**

Merge repeated setup in `exchange_offers_test.go` into `offerFixture` where it reads better, and drop a test that another covers. Re-run the domain tests.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal
git add internal/domain
git commit -m "Let NPCs propose trades, let the player counter, and lapse old offers" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 8: No chain of trades gains value (property test)

**Files:**
- Test: `internal/domain/exchange_invariant_test.go`

**Interfaces:**
- Consumes: the fixture (Task 6), `Exchange.Propose`, `Exchange.Accept`.

- [ ] **Step 1: Write the test**

```go
package domain_test

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
)

// playerValue is what the player owns, in hundredths of a coin at rate 100: coins × 100 plus the
// reference value of the cards in cents × the rate. A trade must never raise it.
func (fx *exchangeFixture) playerValue() int64 {
	const rate = 100
	coins := int64(fx.journal.Balance(domain.AccountPlayer)) * 100
	cards := int64(fx.player.Value(fx.prices, exchangeFallbackPrice)) * rate
	return coins + cards
}

func randomLines(rng *rand.Rand, pool []domain.TradeLine, maxLines, maxQty int) []domain.TradeLine {
	if len(pool) == 0 {
		return nil
	}
	var lines []domain.TradeLine
	for range 1 + rng.IntN(maxLines) {
		src := pool[rng.IntN(len(pool))]
		lines = append(lines, domain.TradeLine{Key: src.Key, Qty: 1 + rng.IntN(min(maxQty, src.Qty))})
	}
	return lines
}

func ownedLines(fx *exchangeFixture) []domain.TradeLine {
	var lines []domain.TradeLine
	for key, qty := range fx.player.Snapshot() {
		lines = append(lines, domain.TradeLine{Key: key, Qty: qty})
	}
	slices.SortFunc(lines, func(a, b domain.TradeLine) int {
		switch {
		case a.Key.ID < b.Key.ID:
			return -1
		case a.Key.ID > b.Key.ID:
			return 1
		}
		return int(a.Key.Finish[0]) - int(b.Key.Finish[0])
	})
	return lines
}

func TestExchangeNeverLetsThePlayerGainValue(t *testing.T) {
	t.Parallel()
	for _, margin := range []domain.Percent{0, 5, 25} {
		t.Run(fmt.Sprintf("margin %d%%", margin), func(t *testing.T) {
			t.Parallel()
			const npcCount = 5
			fx := newExchangeFixture(t, npcCount, tradingSpec(margin), domain.ExchangeParams{})
			rng := rand.New(rand.NewPCG(99, 99))
			var all []domain.CardKey
			for _, cat := range crowdSets() {
				for id, card := range cat.Cards {
					if !card.IsBasicLand {
						all = append(all, domain.CardKey{ID: id, Finish: domain.FinishNonFoil})
					}
				}
			}
			slices.SortFunc(all, func(a, b domain.CardKey) int {
				switch {
				case a.ID < b.ID:
					return -1
				case a.ID > b.ID:
					return 1
				}
				return 0
			})
			for range 60 {
				fx.give(t, all[rng.IntN(len(all))], 1+rng.IntN(3))
			}
			for id := range npcCount {
				for range 25 {
					fx.shelve(id, all[rng.IntN(len(all))], 1+rng.IntN(2))
				}
			}
			fx.fund(t, 5000)

			for step := range 2000 {
				before := fx.playerValue()
				npcID := rng.IntN(npcCount)
				d, err := fx.exchange.Propose(npcID,
					randomLines(rng, ownedLines(fx), 3, 2),
					randomLines(rng, fx.npcs[npcID].Shelf(), 3, 2),
					domain.Coins(rng.IntN(60)))
				if err != nil {
					t.Fatalf("step %d: Propose: %v", step, err)
				}
				if d.Outcome == domain.OutcomeCountered && rng.IntN(2) == 0 {
					if _, err := fx.exchange.Accept(d.Offer.ID); err != nil && !errors.Is(err, domain.ErrInsufficientBalance) {
						t.Fatalf("step %d: Accept: %v", step, err)
					}
				}
				if after := fx.playerValue(); after > before {
					t.Fatalf("step %d: the player's value rose from %d to %d through a trade", step, before, after)
				}
				if sum := fx.journal.SumAllBalances(); sum != 0 {
					t.Fatalf("step %d: the journal sums to %d, want 0", step, sum)
				}
			}
			if stats := fx.exchange.TakeDayStats(); stats.Accepted == 0 {
				t.Errorf("no trade was accepted in 2000 attempts, the test proves nothing: %+v", stats)
			}
		})
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test -race ./internal/domain/ -run TestExchangeNeverLetsThePlayerGainValue -v`
Expected: PASS. If it fails on `Accepted == 0`, make the random offers likelier to fit (a larger top-up range or fewer cards per line), without touching the value assertions. If it fails on a value rise, that is a real bug in the acceptance rule: stop and fix `requiredTopUp` or `execute`, never the assertion.

- [ ] **Step 3: Commit**

```bash
gofmt -w internal
git add internal/domain/exchange_invariant_test.go
git commit -m "Test that no chain of trades gains value for the player" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Configuration keys and validation

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/config/validate_trades.go`
- Test: `internal/config/trades_test.go`

**Interfaces:**
- Consumes: `domain.KnownBehaviour`, `domain.BehaviourNames`, `domain.PlaysetSize`.
- Produces:
  - `type TradesConfig struct{ Enabled bool; OfferExpiryDays, MaxCardsPerSide, DuplicateKeep, HistorySize, MaxOpenOffersPerDay int }`, YAML keys `enabled`, `offer_expiry_days`, `max_cards_per_side`, `duplicate_keep`, `history_size`, `max_open_offers_per_day`
  - `Config.Trades TradesConfig` (`yaml:"trades"`), `NPCConfig.SurplusHoldDays int` (`yaml:"surplus_hold_days"`)
  - `ArchetypeConfig` gains `Behaviour string` (`behaviour`), `WishSize int` (`wish_size`), `MarginPct domain.Percent` (`margin_pct`), `OfferChancePct int` (`offer_chance_pct`)

- [ ] **Step 1: Write the failing tests**

`internal/config/trades_test.go`:

```go
package config_test

import (
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
)

func TestLoad_tradesDefaultToOffWithDocumentedValues(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := config.TradesConfig{Enabled: false, OfferExpiryDays: 3, MaxCardsPerSide: 10, DuplicateKeep: 1, HistorySize: 0, MaxOpenOffersPerDay: 20}
	if cfg.Trades != want {
		t.Errorf("Trades = %+v, want %+v", cfg.Trades, want)
	}
	if cfg.NPCs.SurplusHoldDays != 2 {
		t.Errorf("NPCs.SurplusHoldDays = %d, want 2", cfg.NPCs.SurplusHoldDays)
	}
	for _, name := range defaultArchetypeNames() {
		if got := cfg.NPCs.Archetypes[name].Behaviour; got != name {
			t.Errorf("archetype %q has behaviour %q, want one of the same name", name, got)
		}
	}
	if got := cfg.NPCs.Archetypes["casual"].MarginPct; got != 0 {
		t.Errorf("casual margin = %d, want 0", got)
	}
	if got := cfg.NPCs.Archetypes["speculator"].MarginPct; got != 15 {
		t.Errorf("speculator margin = %d, want 15", got)
	}
}

func TestLoad_tradesOverrideMergesWithTheDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(strings.NewReader("npcs: {population: 10}\ntrades: {enabled: true, offer_expiry_days: 5}\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !cfg.Trades.Enabled || cfg.Trades.OfferExpiryDays != 5 {
		t.Errorf("Trades = %+v, want enabled with 5 days of expiry", cfg.Trades)
	}
	if cfg.Trades.MaxCardsPerSide != 10 {
		t.Errorf("MaxCardsPerSide = %d, want the default 10 kept", cfg.Trades.MaxCardsPerSide)
	}
}

func TestLoad_tradesRejectsInvalidValues(t *testing.T) {
	t.Parallel()
	const archetype = "npcs: {archetypes: {casual: {share: 1, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1, %s}}}"
	tests := []struct {
		name    string
		yaml    string
		wantKey string
	}{
		{"zero expiry", "trades: {offer_expiry_days: 0}", "trades.offer_expiry_days"},
		{"zero card cap", "trades: {max_cards_per_side: 0}", "trades.max_cards_per_side"},
		{"duplicate keep above a playset", "trades: {duplicate_keep: 5}", "trades.duplicate_keep"},
		{"duplicate keep of zero", "trades: {duplicate_keep: 0}", "trades.duplicate_keep"},
		{"negative history", "trades: {history_size: -1}", "trades.history_size"},
		{"negative daily offers", "trades: {max_open_offers_per_day: -1}", "trades.max_open_offers_per_day"},
		{"negative hold", "npcs: {surplus_hold_days: -1}", "npcs.surplus_hold_days"},
		{"unknown behaviour", strings.Replace(archetype, "%s", "behaviour: gambler", 1), "npcs.archetypes.casual.behaviour"},
		{"negative margin", strings.Replace(archetype, "%s", "behaviour: casual, margin_pct: -1", 1), "npcs.archetypes.casual.margin_pct"},
		{"offer chance above 100", strings.Replace(archetype, "%s", "behaviour: casual, offer_chance_pct: 101", 1), "npcs.archetypes.casual.offer_chance_pct"},
		{"negative offer chance", strings.Replace(archetype, "%s", "behaviour: casual, offer_chance_pct: -1", 1), "npcs.archetypes.casual.offer_chance_pct"},
		{"negative wish size", strings.Replace(archetype, "%s", "behaviour: casual, wish_size: -1", 1), "npcs.archetypes.casual.wish_size"},
		{"trades without a crowd", "trades: {enabled: true}", "trades.enabled"},
		{"trades without a hold", "npcs: {population: 5, surplus_hold_days: 0}\ntrades: {enabled: true}", "npcs.surplus_hold_days"},
		{
			name:    "trades with an archetype that has no behaviour",
			yaml:    "npcs: {population: 5, archetypes: {casual: {share: 1, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}}}\ntrades: {enabled: true}",
			wantKey: "npcs.archetypes.casual.behaviour",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.Load(strings.NewReader(tt.yaml))
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantKey) {
				t.Errorf("error %q does not name %q", err, tt.wantKey)
			}
		})
	}
}

func TestLoad_tradesAcceptsAnArchetypeWithoutBehaviourWhileTradesAreOff(t *testing.T) {
	t.Parallel()
	const yaml = "npcs: {archetypes: {casual: {share: 1, min_boosters_per_day: 1, max_boosters_per_day: 1, keep_copies: 1}}}"
	if _, err := config.Load(strings.NewReader(yaml)); err != nil {
		t.Fatalf("Load: %v", err)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/config/ -run TestLoad_trades`
Expected: FAIL to compile, `undefined: config.TradesConfig`.

- [ ] **Step 3: Implement**

In `internal/config/config.go`:

1. Extend `ArchetypeConfig`, after the `FocusSet` field:

```go

	// Behaviour names the registered trade behaviour of the archetype, see
	// domain.BehaviourNames. It may be empty while trades are off, and is required for an
	// archetype with a share when they are on.
	Behaviour string `yaml:"behaviour"`

	// WishSize is how many cards an NPC of the archetype wishes for at once.
	WishSize int `yaml:"wish_size"`

	// MarginPct is the margin an NPC demands on top of the reference value it gives. Never
	// negative: an NPC never gives more value than it gets.
	MarginPct domain.Percent `yaml:"margin_pct"`

	// OfferChancePct is the chance, per tick, that an NPC with a match proposes a trade. In
	// [0, 100].
	OfferChancePct int `yaml:"offer_chance_pct"`
```

2. In `NPCConfig`, before `Archetypes`, add:

```go
	// SurplusHoldDays is how many days an NPC keeps the copies above its keep threshold on a
	// shelf, where they can be traded, before selling them to the Counter. It applies only when
	// trades are enabled, and trades need at least 1, see ADR-0011. Default 2.
	SurplusHoldDays int `yaml:"surplus_hold_days"`

```

3. Add the `TradesConfig` type before `Config`:

```go
// TradesConfig holds the parameters of the trades between the player and the NPCs, see
// docs/core-features.md. Trades are off by default: a run without them is unchanged.
type TradesConfig struct {
	// Enabled turns trades on. It needs an NPC crowd and a surplus hold of at least one day.
	Enabled bool `yaml:"enabled"`

	// OfferExpiryDays is how many days an offer stays open, the day it is made included
	// (default 3).
	OfferExpiryDays int `yaml:"offer_expiry_days"`

	// MaxCardsPerSide bounds the cards each side of an offer gives (default 10).
	MaxCardsPerSide int `yaml:"max_cards_per_side"`

	// DuplicateKeep is how many copies of a printing the player keeps out of NPC proposals; the
	// copies above it are its duplicates. In [1, 4] (default 1).
	DuplicateKeep int `yaml:"duplicate_keep"`

	// HistorySize is how many answered offers the Exchange remembers. 0 keeps none, which is
	// enough for the metrics.
	HistorySize int `yaml:"history_size"`

	// MaxOpenOffersPerDay bounds the offers NPCs make the player each day (default 20).
	// ASSUMPTION: guessed, pending calibration.
	MaxOpenOffersPerDay int `yaml:"max_open_offers_per_day"`
}

```

4. In `Config`, after the `NPCs` field, add:

```go
	// Trades holds the trade parameters.
	Trades TradesConfig `yaml:"trades"`

```

5. In `defaults()`, replace the whole `NPCs: NPCConfig{...},` block (comment included) with:

```go
		// ASSUMPTION: archetype shares, rhythms, keep thresholds and the trade settings are starting
		// guesses pending calibration. A casual is the main source of supply, a whale opens far
		// more than the rest. The casual and speculator margins come from docs/core-features.md.
		NPCs: NPCConfig{
			SurplusHoldDays: 2,
			Archetypes: map[string]ArchetypeConfig{
				"casual":             {Share: 50, MinBoostersPerDay: 4, MaxBoostersPerDay: 8, KeepCopies: 1, Behaviour: "casual", WishSize: 5, MarginPct: 0, OfferChancePct: 2},
				"playset_hunter":     {Share: 15, MinBoostersPerDay: 3, MaxBoostersPerDay: 6, KeepCopies: 4, Behaviour: "playset_hunter", WishSize: 5, MarginPct: 5, OfferChancePct: 2},
				"targeted_collector": {Share: 20, MinBoostersPerDay: 2, MaxBoostersPerDay: 5, KeepCopies: 1, FocusSet: true, Behaviour: "targeted_collector", WishSize: 5, MarginPct: 5, OfferChancePct: 2},
				"speculator":         {Share: 10, MinBoostersPerDay: 2, MaxBoostersPerDay: 4, KeepCopies: 2, Behaviour: "speculator", WishSize: 5, MarginPct: 15, OfferChancePct: 1},
				"whale":              {Share: 5, MinBoostersPerDay: 30, MaxBoostersPerDay: 50, KeepCopies: 1, Behaviour: "whale", WishSize: 3, MarginPct: 10, OfferChancePct: 1},
			},
		},
		Trades: TradesConfig{OfferExpiryDays: 3, MaxCardsPerSide: 10, DuplicateKeep: 1, MaxOpenOffersPerDay: 20},
```

6. In `validate`, after the `validateNPCs` block, add:

```go
	if err := validateTrades(cfg); err != nil {
		return err
	}
```

`internal/config/validate_trades.go`:

```go
package config

import (
	"fmt"
	"maps"
	"slices"

	"github.com/foutoucour/crackapack/internal/domain"
)

// validateTrades checks the trade block and the trade settings of the archetypes. It returns an
// error naming the offending key on the first failure. It runs after validateNPCs, which checks
// the supply settings of the same archetypes.
func validateTrades(cfg Config) error {
	t := cfg.Trades
	switch {
	case t.OfferExpiryDays < 1:
		return fmt.Errorf("config: trades.offer_expiry_days must be >= 1, got %d", t.OfferExpiryDays)
	case t.MaxCardsPerSide < 1:
		return fmt.Errorf("config: trades.max_cards_per_side must be >= 1, got %d", t.MaxCardsPerSide)
	case t.DuplicateKeep < 1 || t.DuplicateKeep > domain.PlaysetSize:
		return fmt.Errorf("config: trades.duplicate_keep must be in [1, %d], got %d", domain.PlaysetSize, t.DuplicateKeep)
	case t.HistorySize < 0:
		return fmt.Errorf("config: trades.history_size must be >= 0, got %d", t.HistorySize)
	case t.MaxOpenOffersPerDay < 0:
		return fmt.Errorf("config: trades.max_open_offers_per_day must be >= 0, got %d", t.MaxOpenOffersPerDay)
	case cfg.NPCs.SurplusHoldDays < 0:
		return fmt.Errorf("config: npcs.surplus_hold_days must be >= 0, got %d", cfg.NPCs.SurplusHoldDays)
	}
	if err := validateTradeArchetypes(cfg.NPCs); err != nil {
		return err
	}
	if !t.Enabled {
		return nil
	}
	if cfg.NPCs.Population == 0 {
		return fmt.Errorf("config: trades.enabled needs npcs.population > 0: NPCs are who the player trades with")
	}
	if cfg.NPCs.SurplusHoldDays < 1 {
		return fmt.Errorf("config: npcs.surplus_hold_days must be >= 1 when trades.enabled is true, got %d: an NPC with no shelf has nothing to give", cfg.NPCs.SurplusHoldDays)
	}
	for _, name := range slices.Sorted(maps.Keys(cfg.NPCs.Archetypes)) {
		a := cfg.NPCs.Archetypes[name]
		if a.Share > 0 && a.Behaviour == "" {
			return fmt.Errorf("config: npcs.archetypes.%s.behaviour is required when trades.enabled is true, known behaviours: %v", name, domain.BehaviourNames())
		}
	}
	return nil
}

// validateTradeArchetypes checks the trade settings of every archetype, in name order so the
// reported key is stable.
func validateTradeArchetypes(n NPCConfig) error {
	for _, name := range slices.Sorted(maps.Keys(n.Archetypes)) {
		a := n.Archetypes[name]
		key := "npcs.archetypes." + name
		switch {
		case a.Behaviour != "" && !domain.KnownBehaviour(a.Behaviour):
			return fmt.Errorf("config: %s.behaviour %q is not registered, known behaviours: %v", key, a.Behaviour, domain.BehaviourNames())
		case a.WishSize < 0:
			return fmt.Errorf("config: %s.wish_size must be >= 0, got %d", key, a.WishSize)
		case a.MarginPct < 0:
			return fmt.Errorf("config: %s.margin_pct must be >= 0, got %d", key, a.MarginPct)
		case a.OfferChancePct < 0 || a.OfferChancePct > 100:
			return fmt.Errorf("config: %s.offer_chance_pct must be in [0, 100], got %d", key, a.OfferChancePct)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run the config tests and the whole suite**

Run: `go test -race ./...`
Expected: PASS. If an existing test compares a whole default archetype or config and now fails, read it before touching it: a test that asserts the old default values for a field this task did not intend to change is a real regression, while a test that lists every field of a struct literal needs the new fields added to its expectation, which is a contract change, not a weakened assertion.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal
git add internal/config
git commit -m "Add the trades configuration and its validation" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 10: Wire trades into the engine, the metrics and the outputs

**Files:**
- Create: `internal/sim/trades.go`
- Modify: `internal/sim/crowd.go` (replace), `internal/sim/engine.go`, `internal/sim/metrics.go`, `internal/sim/writer.go`, `internal/sim/compare.go`
- Test: `internal/sim/trades_engine_test.go`, `internal/sim/trades_output_test.go`

**Interfaces:**
- Consumes: Tasks 1 to 9.
- Produces:
  - `type TradeDayMetrics struct{ PlayerProposals, NPCOffers, Accepted, Countered, Refused, Expired int; VolumeCoins, TopUpCoins domain.Coins }`
  - `DayMetrics.Trade *TradeDayMetrics` (`json:",omitempty"`)
  - Summary keys, present only with trades: `trade_player_proposals`, `trade_npc_offers`, `trade_accepted`, `trade_countered`, `trade_refused`, `trade_expired`, `trade_volume_coins`, `trade_topup_coins`
  - CSV columns, present only with trades, after the NPC columns: `trade_topup`, then the eight `trade_*` per-day columns in the order of the summary keys
  - `Engine.exchange *domain.Exchange`, nil unless trades are enabled

- [ ] **Step 1: Write the failing tests**

`internal/sim/trades_engine_test.go`:

```go
package sim_test

import (
	"reflect"
	"testing"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// tradesConfig is npcConfig with trades on: every NPC is one archetype that trades, always
// proposes when it has a match and wishes for many cards, so a short run sees offers.
func tradesConfig(population int) config.Config {
	cfg := npcConfig(population, 6, 1)
	cfg.NPCs.SurplusHoldDays = 2
	a := cfg.NPCs.Archetypes["npc"]
	a.Behaviour = "casual"
	a.WishSize = 10
	a.OfferChancePct = 50
	cfg.NPCs.Archetypes["npc"] = a
	cfg.Trades = config.TradesConfig{Enabled: true, OfferExpiryDays: 3, MaxCardsPerSide: 10, DuplicateKeep: 1, MaxOpenOffersPerDay: 20}
	cfg.RunDays = 15
	return cfg
}

func TestRun_tradesOffLeaveACrowdRunUnchanged(t *testing.T) {
	t.Parallel()
	plain := npcConfig(4, 10, 1)
	configured := plain
	configured.NPCs = config.NPCConfig{
		Population:      4,
		SurplusHoldDays: 2,
		Archetypes: map[string]config.ArchetypeConfig{
			"npc": {Share: 1, MinBoostersPerDay: 10, MaxBoostersPerDay: 10, KeepCopies: 1, Behaviour: "casual", WishSize: 10, OfferChancePct: 100},
		},
	}

	want := runResult(t, plain, sim.NewCollectorPolicy())
	got := runResult(t, configured, sim.NewCollectorPolicy())

	if !reflect.DeepEqual(got, want) {
		t.Error("a run with trades.enabled false differs from the same run without trade settings")
	}
}

func TestRun_tradesReportTradeMetrics(t *testing.T) {
	t.Parallel()

	result := runResult(t, tradesConfig(20), openOnlyPolicy{})

	for _, d := range result.Days {
		if d.Trade == nil {
			t.Fatalf("day %d has no trade metrics", d.Day)
		}
		if _, ok := d.MoneySupply[domain.AccountTradeTopUp]; !ok {
			t.Errorf("day %d: the money supply lacks the %s account", d.Day, domain.AccountTradeTopUp)
		}
	}
	if got := result.Summary["trade_npc_offers"]; got == 0 {
		t.Error("trade_npc_offers = 0: with a crowd that always wishes and a player that collects duplicates, NPCs should have proposed")
	}
	if _, ok := result.Summary["trade_accepted"]; !ok {
		t.Error("the summary lacks trade_accepted")
	}
}

func TestRun_withoutTradesReportsNoTradeMetrics(t *testing.T) {
	t.Parallel()

	result := runResult(t, npcConfig(4, 10, 1), sim.NewCollectorPolicy())

	for _, d := range result.Days {
		if d.Trade != nil {
			t.Fatalf("day %d has trade metrics with trades off", d.Day)
		}
		if _, ok := d.MoneySupply[domain.AccountTradeTopUp]; ok {
			t.Fatalf("day %d lists the top-up account with trades off", d.Day)
		}
	}
}

func TestRun_tradesDoNotMoveThePlayersBoosterDraws(t *testing.T) {
	t.Parallel()
	off := npcConfig(20, 6, 1)
	on := tradesConfig(20)
	on.RunDays = off.RunDays

	without := runResult(t, off, openOnlyPolicy{})
	with := runResult(t, on, openOnlyPolicy{})

	for i := range without.Days {
		if without.Days[i].BoostersOpened != with.Days[i].BoostersOpened || without.Days[i].CollectionUSD != with.Days[i].CollectionUSD {
			t.Fatalf("day %d: the player's draws changed when trades were enabled: %d boosters / %d cents without, %d / %d with",
				i+1, without.Days[i].BoostersOpened, without.Days[i].CollectionUSD, with.Days[i].BoostersOpened, with.Days[i].CollectionUSD)
		}
	}
}

func TestNewEngine_tradesNeedACrowd(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()
	cfg.Trades = config.TradesConfig{Enabled: true, OfferExpiryDays: 3, MaxCardsPerSide: 10, DuplicateKeep: 1}

	cats := testCatalogues()
	_, err := sim.NewEngine(cfg, cats, nil, mergedPrices(cats, cfg.Seed), sim.NewIdlePolicy(), newRNG(cfg.Seed))

	if err == nil {
		t.Error("NewEngine accepted trades without an NPC crowd")
	}
}
```

`internal/sim/trades_output_test.go`:

```go
package sim_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/sim"
)

func TestOutputs_tradesAddTheirColumnsOnlyWhenEnabled(t *testing.T) {
	t.Parallel()
	with := runResult(t, tradesConfig(10), openOnlyPolicy{})
	without := runResult(t, npcConfig(10, 6, 1), openOnlyPolicy{})

	var withJSON, withCSV, withoutJSON, withoutCSV bytes.Buffer
	for _, c := range []struct {
		buf    *bytes.Buffer
		result sim.Result
		csv    bool
	}{{&withJSON, with, false}, {&withCSV, with, true}, {&withoutJSON, without, false}, {&withoutCSV, without, true}} {
		var err error
		if c.csv {
			err = sim.WriteCSV(c.buf, c.result)
		} else {
			err = sim.WriteJSON(c.buf, c.result)
		}
		if err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	if !strings.Contains(withJSON.String(), `"Trade"`) {
		t.Error("result.json lacks the Trade block with trades on")
	}
	header, _, _ := strings.Cut(withCSV.String(), "\n")
	for _, col := range []string{"trade_topup", "trade_player_proposals", "trade_npc_offers", "trade_accepted", "trade_countered", "trade_refused", "trade_expired", "trade_volume_coins", "trade_topup_coins"} {
		if !strings.Contains(header, col) {
			t.Errorf("days.csv header lacks %q: %s", col, header)
		}
	}
	if strings.Contains(withoutJSON.String(), `"Trade"`) || strings.Contains(withoutCSV.String(), "trade_") {
		t.Error("output mentions trades without trades: runs without trades must not change")
	}
}

func TestCompare_refusesATradeRunAgainstARunWithoutTrades(t *testing.T) {
	t.Parallel()
	with := runResult(t, tradesConfig(10), openOnlyPolicy{})
	cfg := npcConfig(10, 6, 1)
	cfg.RunDays = with.Days[len(with.Days)-1].Day
	without := runResult(t, cfg, openOnlyPolicy{})

	if _, err := sim.Compare(with, without); err == nil {
		t.Error("Compare accepted a run with trades against a run without")
	}
}

func TestCompare_comparesTwoTradeRuns(t *testing.T) {
	t.Parallel()
	a := runResult(t, tradesConfig(10), openOnlyPolicy{})
	b := runResult(t, tradesConfig(10), openOnlyPolicy{})

	if _, err := sim.Compare(a, b); err != nil {
		t.Errorf("Compare: %v", err)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/sim/ -run 'TestRun_trades|TestRun_withoutTrades|TestNewEngine_trades|TestOutputs_trades|TestCompare_.*Trade'`
Expected: FAIL to compile, `d.Trade undefined (type sim.DayMetrics has no field or method Trade)`.

- [ ] **Step 3: Implement**

Replace `internal/sim/crowd.go` with:

```go
package sim

import (
	"fmt"
	"math/rand/v2"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
)

// npcStreamSalt separates the crowd's random stream from the player's. Both derive from
// cfg.Seed, so a seed still reproduces the whole run, but the crowd's draws never shift the
// player's: adding NPCs to a config leaves what the player opens unchanged.
const npcStreamSalt uint64 = 0x9e3779b97f4a7c15

// newCrowd builds the NPC crowd described by cfg.NPCs on its own random stream. It returns nil
// when the population is zero, which is a run without NPCs.
//
// The trade settings of the archetypes and the surplus hold apply only when trades are enabled, so
// a run without trades behaves as it did before they existed.
func newCrowd(cfg config.Config, openable []domain.SetCatalogue, counter *domain.Counter, valuer domain.Valuer) (*domain.Crowd, error) {
	if cfg.NPCs.Population == 0 {
		return nil, nil
	}
	archetypes := make(map[string]domain.ArchetypeSpec, len(cfg.NPCs.Archetypes))
	for name, a := range cfg.NPCs.Archetypes {
		spec := domain.ArchetypeSpec{
			Share:             a.Share,
			MinBoostersPerDay: a.MinBoostersPerDay,
			MaxBoostersPerDay: a.MaxBoostersPerDay,
			KeepCopies:        a.KeepCopies,
			FocusSet:          a.FocusSet,
		}
		if cfg.Trades.Enabled {
			spec.Behaviour = a.Behaviour
			spec.WishSize = a.WishSize
			spec.MarginPct = a.MarginPct
			spec.OfferChancePct = a.OfferChancePct
		}
		archetypes[name] = spec
	}

	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^npcStreamSalt))
	npcs, err := domain.GeneratePopulation(domain.PopulationParams{Size: cfg.NPCs.Population, Archetypes: archetypes, Valuer: valuer}, openable, rng)
	if err != nil {
		return nil, fmt.Errorf("sim: npcs: %w", err)
	}
	opener := domain.NewOpener(map[domain.BoosterLayout]domain.BoosterGenerator{domain.BoosterLayoutClassic: domain.NewClassicGenerator()}, rng)
	var opts []domain.CrowdOption
	if cfg.Trades.Enabled {
		opts = append(opts, domain.WithSurplusHold(cfg.NPCs.SurplusHoldDays))
	}
	crowd, err := domain.NewCrowd(npcs, openable, opener, counter, rng, opts...)
	if err != nil {
		return nil, fmt.Errorf("sim: npcs: %w", err)
	}
	return crowd, nil
}
```

`internal/sim/trades.go`:

```go
package sim

import (
	"fmt"

	"github.com/foutoucour/crackapack/internal/config"
	"github.com/foutoucour/crackapack/internal/domain"
)

// newExchange builds the Exchange between the player and the crowd, or returns nil when trades
// are not enabled.
func newExchange(cfg config.Config, crowd *domain.Crowd, player *domain.Collection, journal *domain.Journal, valuer domain.Valuer) (*domain.Exchange, error) {
	if !cfg.Trades.Enabled {
		return nil, nil
	}
	exchange, err := domain.NewExchange(domain.ExchangeParams{
		ExpiryDays:      cfg.Trades.OfferExpiryDays,
		MaxCardsPerSide: cfg.Trades.MaxCardsPerSide,
		DuplicateKeep:   cfg.Trades.DuplicateKeep,
		MaxOffersPerDay: cfg.Trades.MaxOpenOffersPerDay,
		HistorySize:     cfg.Trades.HistorySize,
		CoinRate:        cfg.CoinRate,
	}, crowd, player, journal, valuer)
	if err != nil {
		return nil, fmt.Errorf("sim: trades: %w", err)
	}
	return exchange, nil
}

// tickTrades lets NPCs propose trades for this tick. It does nothing in a run without trades.
func (e *Engine) tickTrades() error {
	if e.exchange == nil {
		return nil
	}
	return e.exchange.ProposeFromNPCs()
}

// tradeMetrics returns the trade counters of the day just ended, or nil without trades.
func (e *Engine) tradeMetrics() *TradeDayMetrics {
	if e.exchange == nil {
		return nil
	}
	s := e.exchange.TakeDayStats()
	return &TradeDayMetrics{
		PlayerProposals: s.PlayerProposals,
		NPCOffers:       s.NPCOffers,
		Accepted:        s.Accepted,
		Countered:       s.Countered,
		Refused:         s.Refused,
		Expired:         s.Expired,
		VolumeCoins:     s.VolumeCoins,
		TopUpCoins:      s.TopUpCoins,
	}
}

// addTradeSummary adds the trade totals to s. Every entry of days must carry trade metrics.
func addTradeSummary(s Summary, days []DayMetrics) {
	var total TradeDayMetrics
	for _, d := range days {
		total.PlayerProposals += d.Trade.PlayerProposals
		total.NPCOffers += d.Trade.NPCOffers
		total.Accepted += d.Trade.Accepted
		total.Countered += d.Trade.Countered
		total.Refused += d.Trade.Refused
		total.Expired += d.Trade.Expired
		total.VolumeCoins += d.Trade.VolumeCoins
		total.TopUpCoins += d.Trade.TopUpCoins
	}
	s["trade_player_proposals"] = float64(total.PlayerProposals)
	s["trade_npc_offers"] = float64(total.NPCOffers)
	s["trade_accepted"] = float64(total.Accepted)
	s["trade_countered"] = float64(total.Countered)
	s["trade_refused"] = float64(total.Refused)
	s["trade_expired"] = float64(total.Expired)
	s["trade_volume_coins"] = float64(total.VolumeCoins)
	s["trade_topup_coins"] = float64(total.TopUpCoins)
}
```

`internal/sim/engine.go`:

1. In the `Engine` struct, after the `crowd` field add `exchange      *domain.Exchange // nil unless trades are enabled`, and delete the line `npcCardsHeld int // cards in all NPC collections; carried across days` together with the blank line above it.

2. In `NewEngine`, replace
```go
	crowd, err := newCrowd(cfg, catalogues, counter)
	if err != nil {
		return nil, err
	}
```
with
```go
	valuer := domain.NewValuer(prices, cfg.FallbackPrice)
	crowd, err := newCrowd(cfg, catalogues, counter, valuer)
	if err != nil {
		return nil, err
	}
	collection := domain.NewCollection()
	exchange, err := newExchange(cfg, crowd, collection, journal, valuer)
	if err != nil {
		return nil, err
	}
```
and in the returned `&Engine{...}` literal replace `collection:    domain.NewCollection(),` with `collection:    collection,` and add `exchange:      exchange,` after `crowd:         crowd,`.

3. In `Run`, right after the `e.dayNPCSold = 0` reset line, add:
```go

		if e.exchange != nil {
			e.exchange.StartDay(day)
		}
```
and in the tick loop, right after the `tickCrowd` error check, add:
```go
			if err := e.tickTrades(); err != nil {
				return Result{}, fmt.Errorf("sim: day %d tick %d: %w", day, tick, err)
			}
```
and after
```go
	if e.crowd != nil {
		addNPCSummary(summary, days)
	}
```
add
```go
	if e.exchange != nil {
		addTradeSummary(summary, days)
	}
```

4. In `tickCrowd`, delete the line `e.npcCardsHeld += tick.CardsDrawn - tick.CardsSold`. In `npcMetrics`, replace `CardsHeld: e.npcCardsHeld` with `CardsHeld: e.crowd.CardsHeld()`.

5. In `snapshotDay`, between the declaration of the `accounts` slice and the line `supply := make(...)`, insert:
```go
	if e.exchange != nil {
		accounts = append(accounts, domain.AccountTradeTopUp)
	}
```
and in the returned `DayMetrics` literal add `Trade: e.tradeMetrics(),` after `NPC:             e.npcMetrics(),`.

`internal/sim/metrics.go`:

1. After `NPCDayMetrics` add:
```go
// TradeDayMetrics is what happened to the trades with NPCs on one day.
type TradeDayMetrics struct {
	// PlayerProposals counts the offers the player made, counters to NPC offers included.
	PlayerProposals int
	// NPCOffers counts the offers NPCs made to the player.
	NPCOffers int
	Accepted  int
	// Countered counts the counter-offers NPCs made.
	Countered int
	Refused   int
	// Expired counts the offers that lapsed unanswered.
	Expired int
	// VolumeCoins is the reference value of the cards moved by accepted trades, in coins.
	VolumeCoins domain.Coins
	// TopUpCoins is the coins the player paid on top of cards.
	TopUpCoins domain.Coins
}
```
2. In `DayMetrics`, after the `NPC` field add:
```go

	// Trade is nil when trades are not enabled, which keeps the output of such runs unchanged.
	Trade *TradeDayMetrics `json:",omitempty"`
```
3. At the end of the `Summary` doc comment (after the `npc_cards_held` bullet) add:
```go
//
// A run with trades adds eight more, absent from every other run, each summed over the run:
//   - trade_player_proposals, trade_npc_offers: offers the player and the NPCs made
//   - trade_accepted, trade_countered, trade_refused, trade_expired: how offers ended
//   - trade_volume_coins: reference value of the cards moved, in coins
//   - trade_topup_coins: coins the player paid on top of cards
```

`internal/sim/writer.go`:

1. Change the signature to `func dayColumns(d DayMetrics, setCodes []string, withNPC, withTrades bool) []column` and extend its doc comment with: `withTrades appends the trade account balance and the eight trade columns after the NPC columns.`

2. After the `if withNPC { ... }` block, before `return cols`, add:
```go
	if withTrades {
		var trade TradeDayMetrics
		if d.Trade != nil {
			trade = *d.Trade
		}
		cols = append(cols,
			column{name: string(domain.AccountTradeTopUp), value: int64(d.MoneySupply[domain.AccountTradeTopUp])},
			column{name: "trade_player_proposals", value: int64(trade.PlayerProposals)},
			column{name: "trade_npc_offers", value: int64(trade.NPCOffers)},
			column{name: "trade_accepted", value: int64(trade.Accepted)},
			column{name: "trade_countered", value: int64(trade.Countered)},
			column{name: "trade_refused", value: int64(trade.Refused)},
			column{name: "trade_expired", value: int64(trade.Expired)},
			column{name: "trade_volume_coins", value: int64(trade.VolumeCoins)},
			column{name: "trade_topup_coins", value: int64(trade.TopUpCoins)},
		)
	}
```
The balance column is named `trade_topup` (the account name), the per-day flow `trade_topup_coins`.

3. Add after `hasNPCMetrics`:
```go
// hasTradeMetrics reports whether the run behind days had trades. Every day of a run agrees, so
// the first one decides.
func hasTradeMetrics(days []DayMetrics) bool {
	return len(days) > 0 && days[0].Trade != nil
}
```

4. In `WriteCSV`, after `withNPC := hasNPCMetrics(r.Days)` add `withTrades := hasTradeMetrics(r.Days)`, and pass `withTrades` as the last argument of both `dayColumns(...)` calls there. Update the `WriteCSV` column-order doc comment with `8. trade_topup, then the eight trade_* columns, only when the run had trades`.

`internal/sim/compare.go`:

1. In the `Compare` doc comment's list of errors add `//   - only one of the results has trades (derived from day 0's trade metrics)`.
2. After the `2b` block add:
```go
	// 2c. Both runs have trades or neither: their columns would not line up otherwise.
	if hasTradeMetrics(a.Days) != hasTradeMetrics(b.Days) {
		return Comparison{}, fmt.Errorf("results differ in trades: only one of them has trade metrics")
	}
```
3. In step 5, after `withNPC := hasNPCMetrics(a.Days)` add `withTrades := hasTradeMetrics(a.Days)` and pass it: `computeDayComparison(a.Days[i], b.Days[i], setCodes, withNPC, withTrades)`.
4. Change `computeDayComparison` to take `withNPC, withTrades bool`, update its doc comment ("withNPC and withTrades must match both days"), and pass both to the two `dayColumns` calls. Run `go vet ./...` to find every other caller of `dayColumns` and `computeDayComparison`, tests included, and pass `false` there.

- [ ] **Step 4: Run the sim tests and the whole suite**

Run: `go test -race ./...`
Expected: PASS. In particular `TestRun_tradesOffLeaveACrowdRunUnchanged` proves a run with trades off is identical, and the existing `npc_*` tests are unchanged.

- [ ] **Step 5: Refactor**

Re-read the new engine tests next to `npc_engine_test.go` and `npc_output_test.go`. Merge assertions that repeat the same check, and drop a test a broader one covers. Re-run `go test ./internal/sim/...`.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal
git add internal/sim
git commit -m "Wire trades into the engine, the metrics and the outputs" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 11: The Trading interface on Session

**Files:**
- Create: `internal/sim/trading.go`
- Modify: `internal/sim/policy.go`, `internal/sim/collector_test.go` (the fake only)
- Test: `internal/sim/trading_test.go`

**Interfaces:**
- Consumes: `Engine.exchange` (Task 10), the Exchange API (Tasks 6 and 7).
- Produces:
  - `type Trading interface { Offers() []domain.Offer; NPCCount() int; NPCShelf(npcID int) []domain.TradeLine; HoldersOf(key domain.CardKey) []int; ProposeTrade(npcID int, give, ask []domain.TradeLine, topUp domain.Coins) domain.Decision; AcceptOffer(offerID int) (domain.Decision, error); CounterOffer(offerID int, give []domain.TradeLine, topUp domain.Coins) (domain.Decision, error) }`
  - `Session` embeds `Trading`.
  - `var ErrTradesDisabled error`
  - On a run without trades: `Offers` and `HoldersOf` and `NPCShelf` return nil, `NPCCount` returns 0, `ProposeTrade` returns a refusal with `ErrTradesDisabled`, `AcceptOffer` and `CounterOffer` return `ErrTradesDisabled`.

- [ ] **Step 1: Write the failing tests**

`internal/sim/trading_test.go`:

```go
package sim_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// scriptPolicy runs script once, in the first session, and does nothing afterwards.
type scriptPolicy struct {
	script func(s sim.Session)
	ran    *bool
}

func (p scriptPolicy) RunSession(s sim.Session) {
	if *p.ran {
		return
	}
	*p.ran = true
	p.script(s)
}

func TestSession_tradingIsDisabledWithoutTrades(t *testing.T) {
	t.Parallel()
	var ran bool
	var decision domain.Decision
	var acceptErr, counterErr error
	var offers []domain.Offer
	var npcCount int
	policy := scriptPolicy{ran: &ran, script: func(s sim.Session) {
		offers = s.Offers()
		npcCount = s.NPCCount()
		decision = s.ProposeTrade(0, nil, nil, 0)
		_, acceptErr = s.AcceptOffer(1)
		_, counterErr = s.CounterOffer(1, nil, 0)
	}}

	runResult(t, smallConfig(), policy)

	if !ran {
		t.Fatal("the policy never ran")
	}
	if len(offers) != 0 || npcCount != 0 {
		t.Errorf("Offers = %v, NPCCount = %d, want none without trades", offers, npcCount)
	}
	if decision.Outcome != domain.OutcomeRefused || !errors.Is(decision.Reason, sim.ErrTradesDisabled) {
		t.Errorf("ProposeTrade = %+v, want a refusal with ErrTradesDisabled", decision)
	}
	if !errors.Is(acceptErr, sim.ErrTradesDisabled) || !errors.Is(counterErr, sim.ErrTradesDisabled) {
		t.Errorf("AcceptOffer = %v, CounterOffer = %v, want ErrTradesDisabled", acceptErr, counterErr)
	}
}

func TestSession_badTradeRequestsAreRefusalsNotRunAborts(t *testing.T) {
	t.Parallel()
	var ran bool
	var decision domain.Decision
	var acceptErr error
	policy := scriptPolicy{ran: &ran, script: func(s sim.Session) {
		decision = s.ProposeTrade(9999, []domain.TradeLine{{Key: domain.CardKey{ID: "x", Finish: domain.FinishNonFoil}, Qty: 1}}, nil, 0)
		_, acceptErr = s.AcceptOffer(424242)
	}}

	runResult(t, tradesConfig(5), policy)

	if !ran {
		t.Fatal("the policy never ran")
	}
	if decision.Outcome != domain.OutcomeRefused || !errors.Is(decision.Reason, domain.ErrUnknownNPC) {
		t.Errorf("ProposeTrade of an unknown NPC = %+v, want a refusal with ErrUnknownNPC", decision)
	}
	if !errors.Is(acceptErr, domain.ErrOfferNotFound) {
		t.Errorf("AcceptOffer of an unknown offer = %v, want ErrOfferNotFound", acceptErr)
	}
}

// everySession runs f in every session of a run.
type everySession struct{ f func(sim.Session) }

func (p everySession) RunSession(s sim.Session) { p.f(s) }

func TestSession_exposesTheCrowdAndItsShelves(t *testing.T) {
	t.Parallel()
	cfg := tradesConfig(8)
	cfg.RunDays = 6
	var npcCount, shelvesSeen int
	holdersConsistent := true
	policy := everySession{f: func(s sim.Session) {
		npcCount = s.NPCCount()
		for id := range s.NPCCount() {
			shelf := s.NPCShelf(id)
			if len(shelf) > 0 {
				shelvesSeen++
			}
			for _, line := range shelf {
				holdersConsistent = holdersConsistent && slices.Contains(s.HoldersOf(line.Key), id)
			}
		}
	}}

	runResult(t, cfg, policy)

	if npcCount != 8 {
		t.Errorf("NPCCount = %d, want 8", npcCount)
	}
	if shelvesSeen == 0 {
		t.Error("no NPC ever had a shelf over 6 days, want some surplus waiting")
	}
	if !holdersConsistent {
		t.Error("HoldersOf does not list an NPC for a card on its shelf")
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/sim/ -run 'TestSession_'`
Expected: FAIL to compile, `s.Offers undefined (type sim.Session has no field or method Offers)`.

- [ ] **Step 3: Implement**

`internal/sim/trading.go`:

```go
package sim

import (
	"errors"
	"fmt"

	"github.com/foutoucour/crackapack/internal/domain"
)

// ErrTradesDisabled is returned, or given as the reason of a refusal, by the trade actions of a
// run whose configuration has trades off.
var ErrTradesDisabled = errors.New("sim: trades are not enabled in this run")

// Trading is the trade side of a Session: what the player sees of the NPCs and what it can do
// with them. An NPC counters a short offer once, and that counter is an open offer the player
// answers with AcceptOffer. See docs/simulation.md.
//
// Refusals, a missing offer and an insufficient balance are normal outcomes. Any other error
// aborts the run.
type Trading interface {
	// Offers returns the open offers, oldest first.
	Offers() []domain.Offer
	// NPCCount returns the number of NPCs. Their IDs run from 0 to NPCCount()-1.
	NPCCount() int
	// NPCShelf returns what the NPC can trade away, sorted by card.
	NPCShelf(npcID int) []domain.TradeLine
	// HoldersOf returns the IDs of the NPCs holding key on their shelf, ascending.
	HoldersOf(key domain.CardKey) []int
	// ProposeTrade offers the NPC a swap: the player gives give and topUp coins, the NPC gives ask
	// from its shelf. The NPC accepts, counters once or refuses.
	ProposeTrade(npcID int, give, ask []domain.TradeLine, topUp domain.Coins) domain.Decision
	// AcceptOffer accepts an open offer. It returns domain.ErrOfferNotFound for an offer that is
	// not open and domain.ErrInsufficientBalance, leaving the offer open, when the player cannot
	// pay its top-up.
	AcceptOffer(offerID int) (domain.Decision, error)
	// CounterOffer answers an offer an NPC proposed with other cards. The NPC accepts or refuses.
	CounterOffer(offerID int, give []domain.TradeLine, topUp domain.Coins) (domain.Decision, error)
}

func (p *port) Offers() []domain.Offer {
	if p.e.exchange == nil {
		return nil
	}
	return p.e.exchange.Offers()
}

func (p *port) NPCCount() int {
	if p.e.exchange == nil {
		return 0
	}
	return p.e.exchange.NPCCount()
}

func (p *port) NPCShelf(npcID int) []domain.TradeLine {
	if p.e.exchange == nil {
		return nil
	}
	return p.e.exchange.Shelf(npcID)
}

func (p *port) HoldersOf(key domain.CardKey) []int {
	if p.e.exchange == nil {
		return nil
	}
	return p.e.exchange.HoldersOf(key)
}

func (p *port) ProposeTrade(npcID int, give, ask []domain.TradeLine, topUp domain.Coins) domain.Decision {
	if p.e.exchange == nil {
		return domain.Decision{Outcome: domain.OutcomeRefused, Reason: ErrTradesDisabled}
	}
	d, err := p.e.exchange.Propose(npcID, give, ask, topUp)
	if err != nil {
		p.fatalErr = fmt.Errorf("propose trade: %w", err)
	}
	return d
}

func (p *port) AcceptOffer(offerID int) (domain.Decision, error) {
	if p.e.exchange == nil {
		return domain.Decision{}, ErrTradesDisabled
	}
	return p.tradeResult("accept offer", func() (domain.Decision, error) { return p.e.exchange.Accept(offerID) })
}

func (p *port) CounterOffer(offerID int, give []domain.TradeLine, topUp domain.Coins) (domain.Decision, error) {
	if p.e.exchange == nil {
		return domain.Decision{}, ErrTradesDisabled
	}
	return p.tradeResult("counter offer", func() (domain.Decision, error) {
		return p.e.exchange.Counter(offerID, give, topUp)
	})
}

// tradeResult runs a trade action and passes its error on to the policy when it is a normal
// outcome, and aborts the run on any other error.
func (p *port) tradeResult(action string, run func() (domain.Decision, error)) (domain.Decision, error) {
	d, err := run()
	switch {
	case err == nil:
		return d, nil
	case errors.Is(err, domain.ErrInsufficientBalance),
		errors.Is(err, domain.ErrOfferNotFound),
		errors.Is(err, domain.ErrNotCounterable):
		return d, err
	}
	p.fatalErr = fmt.Errorf("%s: %w", action, err)
	return d, err
}
```

In `internal/sim/policy.go`, change the `Session` interface declaration so it embeds `Trading` as its first line:

```go
type Session interface {
	Trading

```
(the existing `// View returns ...` method list follows unchanged), and add a sentence to the doc comment above it: `A run without trades still offers the Trading methods, which refuse or return nothing, so a policy needs no check.`

In `internal/sim/collector_test.go`, add the embedded interface to `fakeSession` so it still satisfies `Session`; the collector never trades, so a nil embedded interface is fine:

```go
type fakeSession struct {
	sim.Trading
	views       []sim.WorldView // views returned in sequence; last one repeated
```

- [ ] **Step 4: Run the sim tests and the whole suite**

Run: `go test -race ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal
git add internal/sim
git commit -m "Give policies the Trading actions through Session" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 12: The trader policy

**Files:**
- Create: `internal/sim/trader.go`
- Modify: `cmd/crackapack/policies.go`
- Test: `internal/sim/trader_test.go`, `cmd/crackapack/policies_trading_test.go`

**Interfaces:**
- Consumes: `Trading` (Task 11), `CollectorPolicy`, `buildBuyCandidates`, `collectorReserveDays`.
- Produces: `type TraderPolicy`, `func NewTraderPolicy() TraderPolicy`; registry names `trader`.

- [ ] **Step 1: Write the failing tests**

`internal/sim/trader_test.go`:

```go
package sim_test

import (
	"reflect"
	"testing"

	"github.com/foutoucour/crackapack/internal/sim"
)

func TestTraderPolicy_tradesWithNPCs(t *testing.T) {
	t.Parallel()
	cfg := tradesConfig(25)
	cfg.RunDays = 20

	result := runResult(t, cfg, sim.NewTraderPolicy())

	if got := result.Summary["trade_accepted"]; got == 0 {
		t.Errorf("trade_accepted = 0 over 20 days with 25 NPCs: the trader never completed a trade; summary: %v", result.Summary)
	}
	if got := result.Summary["trade_player_proposals"] + result.Summary["trade_npc_offers"]; got == 0 {
		t.Error("neither the trader nor the NPCs proposed a trade")
	}
}

func TestTraderPolicy_isDeterministic(t *testing.T) {
	t.Parallel()
	cfg := tradesConfig(25)

	first := runResult(t, cfg, sim.NewTraderPolicy())
	second := runResult(t, cfg, sim.NewTraderPolicy())

	if !reflect.DeepEqual(first, second) {
		t.Error("two runs of the trader with the same seed differ")
	}
}

func TestTraderPolicy_withoutTradesPlaysLikeTheCollector(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()

	trader := runResult(t, cfg, sim.NewTraderPolicy())
	collector := runResult(t, cfg, sim.NewCollectorPolicy())

	if !reflect.DeepEqual(trader, collector) {
		t.Error("the trader differs from the collector in a run where trades are off")
	}
}
```

`cmd/crackapack/policies_trading_test.go` (Task 13 extends it with the exploiter):

```go
package main

import "testing"

func TestBuildPolicies_includesTheTrader(t *testing.T) {
	t.Parallel()
	build, ok := buildPolicies()["trader"]
	if !ok {
		t.Fatal("policy \"trader\" is not registered")
	}
	if build() == nil {
		t.Error("policy \"trader\" builds nil")
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/sim/ ./cmd/... -run 'TestTraderPolicy|TestBuildPolicies_includesTheTrader'`
Expected: FAIL to compile, `undefined: sim.NewTraderPolicy`.

- [ ] **Step 3: Implement**

`internal/sim/trader.go`:

```go
package sim

import (
	"cmp"
	"slices"

	"github.com/foutoucour/crackapack/internal/domain"
)

const (
	// maxSwapProposals bounds the swaps a TraderPolicy proposes in one session, so a session's
	// cost does not depend on the size of the crowd.
	maxSwapProposals = 20

	// tradeCardCap is the most cards a TraderPolicy puts on its side of an offer. The Exchange
	// may allow fewer, and refuses an offer that goes over.
	tradeCardCap = 10
)

// TraderPolicy is a PlayerPolicy that collects like the CollectorPolicy and also trades with NPCs.
//
// Per session, in order:
//  1. Accept the offers NPCs made that bring a card the player lacks.
//  2. Swap: for each missing card an NPC holds on its shelf, offer the player's cheapest duplicates
//     for it, and accept the NPC's counter when it fits above the coin floor.
//  3. Play the collector's session: open boosters, sell duplicates above a playset, buy missing
//     cards from the Counter.
//
// In a run without trades the first two steps do nothing, and it plays as the collector.
type TraderPolicy struct{ collector CollectorPolicy }

// NewTraderPolicy returns a TraderPolicy ready to use.
func NewTraderPolicy() TraderPolicy { return TraderPolicy{collector: NewCollectorPolicy()} }

// RunSession implements PlayerPolicy.
func (p TraderPolicy) RunSession(s Session) {
	acceptUsefulOffers(s)
	proposeSwaps(s)
	p.collector.RunSession(s)
}

// acceptUsefulOffers accepts each open NPC offer that gives at least one card the player lacks.
func acceptUsefulOffers(s Session) {
	owned := ownedIDs(s.View())
	for _, o := range s.Offers() {
		if o.Origin != domain.OriginNPC || !givesMissingCard(o, owned) {
			continue
		}
		if d, err := s.AcceptOffer(o.ID); err == nil && d.Outcome == domain.OutcomeAccepted {
			owned = ownedIDs(s.View())
		}
	}
}

func ownedIDs(v WorldView) map[domain.CardID]bool {
	owned := make(map[domain.CardID]bool, len(v.OwnedCards))
	for key, qty := range v.OwnedCards {
		if qty > 0 {
			owned[key.ID] = true
		}
	}
	return owned
}

func givesMissingCard(o domain.Offer, owned map[domain.CardID]bool) bool {
	for _, l := range o.NPCGives {
		if !owned[l.Key.ID] {
			return true
		}
	}
	return false
}

// proposeSwaps offers duplicates for the missing cards NPCs hold, cheapest missing card first.
func proposeSwaps(s Session) {
	v := s.View()
	proposals := 0
	for _, want := range buildBuyCandidates(v) {
		for _, npcID := range s.HoldersOf(want.key) {
			if proposals >= maxSwapProposals {
				return
			}
			give := cheapestDuplicates(v, priceOf(v, want.key.ID))
			if len(give) == 0 {
				return
			}
			proposals++
			if trySwap(s, v, npcID, want.key, give) {
				v = s.View()
				break
			}
		}
	}
}

// trySwap proposes give for one copy of want and accepts the NPC's counter when its top-up fits
// above the coin floor. It reports whether a trade was made.
func trySwap(s Session, v WorldView, npcID int, want domain.CardKey, give []domain.TradeLine) bool {
	d := s.ProposeTrade(npcID, give, []domain.TradeLine{{Key: want, Qty: 1}}, 0)
	switch d.Outcome {
	case domain.OutcomeAccepted:
		return true
	case domain.OutcomeCountered:
		floor := v.DailyReward * domain.Coins(collectorReserveDays)
		if d.Offer.TopUp > v.Balance-floor {
			return false
		}
		accepted, err := s.AcceptOffer(d.Offer.ID)
		return err == nil && accepted.Outcome == domain.OutcomeAccepted
	}
	return false
}

// cheapestDuplicates returns non-foil copies the player holds above the first, cheapest first,
// until their value reaches targetCents or tradeCardCap cards are reached.
func cheapestDuplicates(v WorldView, targetCents domain.USDCents) []domain.TradeLine {
	totals := make(map[domain.CardID]int, len(v.OwnedCards))
	for key, qty := range v.OwnedCards {
		totals[key.ID] += qty
	}
	type duplicate struct {
		key   domain.CardKey
		spare int
		price domain.USDCents
	}
	var duplicates []duplicate
	for key, qty := range v.OwnedCards {
		if key.Finish != domain.FinishNonFoil {
			continue
		}
		if spare := min(qty, totals[key.ID]-1); spare > 0 {
			duplicates = append(duplicates, duplicate{key: key, spare: spare, price: priceOf(v, key.ID)})
		}
	}
	slices.SortFunc(duplicates, func(a, b duplicate) int {
		if c := cmp.Compare(a.price, b.price); c != 0 {
			return c
		}
		return cmp.Compare(a.key.ID, b.key.ID)
	})

	var lines []domain.TradeLine
	var cents domain.USDCents
	var cards int
	for _, d := range duplicates {
		qty := 0
		for qty < d.spare && cards < tradeCardCap && cents < targetCents {
			qty++
			cards++
			cents += d.price
		}
		if qty > 0 {
			lines = append(lines, domain.TradeLine{Key: d.key, Qty: qty})
		}
	}
	return lines
}

// priceOf returns the reference value of a card in USD cents.
func priceOf(v WorldView, id domain.CardID) domain.USDCents {
	if p, ok := v.Prices[id]; ok {
		return p
	}
	return v.FallbackPrice
}
```

In `cmd/crackapack/policies.go`, add to the map:

```go
		"trader":    func() sim.PlayerPolicy { return sim.NewTraderPolicy() },
```

- [ ] **Step 4: Run the tests**

Run: `go test -race ./...`
Expected: PASS. If `TestTraderPolicy_tradesWithNPCs` reports `trade_accepted = 0`, print the summary (the test message does) and find why before touching the config: check `trade_npc_offers` (NPCs never offered: raise the test's `OfferChancePct`, which is test data), `trade_player_proposals` (the trader never proposed: `HoldersOf` empty means NPCs have no shelf), and `trade_countered`. Adjust test data only, never the assertion.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal cmd demo
git add internal/sim cmd/crackapack/policies.go cmd/crackapack/policies_trading_test.go
git commit -m "Add the trader player policy" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 13: The exploiter policy and the whole-run value invariant

**Files:**
- Create: `internal/sim/exploiter.go`
- Modify: `cmd/crackapack/policies.go`, `cmd/crackapack/policies_trading_test.go`
- Test: `internal/sim/exploiter_test.go`

**Interfaces:**
- Consumes: Task 12 (`cheapestDuplicates`, `priceOf`, `NewTraderPolicy`), `Trading`.
- Produces: `type ExploiterPolicy`, `func NewExploiterPolicy() ExploiterPolicy`; registry name `exploiter`.

- [ ] **Step 1: Write the failing tests**

`internal/sim/exploiter_test.go`:

```go
package sim_test

import (
	"fmt"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

// playerValue is what the player owns in hundredths of a coin: coins × 100 plus the reference
// value of its cards in cents × the coin rate. A trade must never raise it.
func playerValue(v sim.WorldView) int64 {
	var cards int64
	for key, qty := range v.OwnedCards {
		price, ok := v.Prices[key.ID]
		if !ok {
			price = v.FallbackPrice
		}
		cards += int64(price) * int64(qty)
	}
	return int64(v.Balance)*100 + cards*int64(v.CoinRate)
}

// probeSession checks, around every trade action, that the player's value did not rise.
type probeSession struct {
	sim.Session
	t        *testing.T
	accepted *int
}

func (p *probeSession) check(before int64, action string) {
	p.t.Helper()
	if after := playerValue(p.Session.View()); after > before {
		p.t.Fatalf("%s raised the player's value from %d to %d: a chain of trades gained value", action, before, after)
	}
}

func (p *probeSession) ProposeTrade(npcID int, give, ask []domain.TradeLine, topUp domain.Coins) domain.Decision {
	before := playerValue(p.Session.View())
	d := p.Session.ProposeTrade(npcID, give, ask, topUp)
	p.check(before, "ProposeTrade")
	if d.Outcome == domain.OutcomeAccepted {
		*p.accepted++
	}
	return d
}

func (p *probeSession) AcceptOffer(offerID int) (domain.Decision, error) {
	before := playerValue(p.Session.View())
	d, err := p.Session.AcceptOffer(offerID)
	p.check(before, "AcceptOffer")
	if err == nil && d.Outcome == domain.OutcomeAccepted {
		*p.accepted++
	}
	return d, err
}

func (p *probeSession) CounterOffer(offerID int, give []domain.TradeLine, topUp domain.Coins) (domain.Decision, error) {
	before := playerValue(p.Session.View())
	d, err := p.Session.CounterOffer(offerID, give, topUp)
	p.check(before, "CounterOffer")
	if err == nil && d.Outcome == domain.OutcomeAccepted {
		*p.accepted++
	}
	return d, err
}

type probePolicy struct {
	inner    sim.PlayerPolicy
	t        *testing.T
	accepted *int
}

func (p probePolicy) RunSession(s sim.Session) {
	p.inner.RunSession(&probeSession{Session: s, t: p.t, accepted: p.accepted})
}

func TestNoChainOfTradesGainsValue(t *testing.T) {
	t.Parallel()
	policies := map[string]sim.PlayerPolicy{
		"exploiter": sim.NewExploiterPolicy(),
		"trader":    sim.NewTraderPolicy(),
	}
	for name, policy := range policies {
		for _, margin := range []domain.Percent{0, 10} {
			t.Run(fmt.Sprintf("%s margin %d%%", name, margin), func(t *testing.T) {
				t.Parallel()
				cfg := tradesConfig(30)
				cfg.RunDays = 30
				a := cfg.NPCs.Archetypes["npc"]
				a.MarginPct = margin
				a.OfferChancePct = 100
				cfg.NPCs.Archetypes["npc"] = a
				var accepted int

				runResult(t, cfg, probePolicy{inner: policy, t: t, accepted: &accepted})

				if accepted == 0 {
					t.Error("no trade was accepted in 30 days, so the invariant was never exercised")
				}
			})
		}
	}
}

func TestExploiterPolicy_isDeterministic(t *testing.T) {
	t.Parallel()
	cfg := tradesConfig(20)

	first := runResult(t, cfg, sim.NewExploiterPolicy())
	second := runResult(t, cfg, sim.NewExploiterPolicy())

	if fmt.Sprint(first.Summary) != fmt.Sprint(second.Summary) {
		t.Errorf("two runs of the exploiter with the same seed differ:\n%v\n%v", first.Summary, second.Summary)
	}
}
```

Extend `cmd/crackapack/policies_trading_test.go` to cover both names:

```go
package main

import "testing"

func TestBuildPolicies_includesTheTradingPolicies(t *testing.T) {
	t.Parallel()
	policies := buildPolicies()

	for _, name := range []string{"trader", "exploiter"} {
		build, ok := policies[name]
		if !ok {
			t.Errorf("policy %q is not registered", name)
			continue
		}
		if build() == nil {
			t.Errorf("policy %q builds nil", name)
		}
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/sim/ ./cmd/... -run 'TestNoChainOfTrades|TestExploiter|TestBuildPolicies'`
Expected: FAIL to compile, `undefined: sim.NewExploiterPolicy`.

- [ ] **Step 3: Implement**

`internal/sim/exploiter.go`:

```go
package sim

import "github.com/foutoucour/crackapack/internal/domain"

const (
	// exploiterShelfProbes is how many shelf cards of each NPC the ExploiterPolicy tries to get.
	exploiterShelfProbes = 3

	// exploiterGiveProbes is how many of its cheapest duplicates the ExploiterPolicy offers for
	// each shelf card.
	exploiterGiveProbes = 3

	// exploiterMaxTopUp is the most coins the ExploiterPolicy pays to close a countered offer.
	exploiterMaxTopUp domain.Coins = 5

	// allDuplicates is a target value no pile of duplicates reaches: it asks cheapestDuplicates
	// for as many as the card cap allows.
	allDuplicates domain.USDCents = 1 << 62
)

// ExploiterPolicy is a PlayerPolicy that hunts for a gain from trades: it offers its duplicates
// to every NPC for their shelf cards at no top-up, takes the NPC's counter when it costs next to
// nothing, then sells what it got to the Counter. No gain exists: the acceptance rule never lets an
// NPC give more value than it receives, which is what a run with this policy shows. See
// docs/simulation.md.
//
// Per session, in order: accept every NPC offer, chain trades across all NPCs, then play the
// collector's session.
type ExploiterPolicy struct{ collector CollectorPolicy }

// NewExploiterPolicy returns an ExploiterPolicy ready to use.
func NewExploiterPolicy() ExploiterPolicy { return ExploiterPolicy{collector: NewCollectorPolicy()} }

// RunSession implements PlayerPolicy.
func (p ExploiterPolicy) RunSession(s Session) {
	for _, o := range s.Offers() {
		if o.Origin == domain.OriginNPC {
			_, _ = s.AcceptOffer(o.ID) // a refusal or a lack of coins leaves the state as it was
		}
	}
	chainTrades(s)
	p.collector.RunSession(s)
}

// chainTrades offers each NPC a swap of its cheapest duplicates for each of its first shelf cards.
func chainTrades(s Session) {
	duplicates := cheapestDuplicates(s.View(), allDuplicates)
	for npcID := range s.NPCCount() {
		shelf := s.NPCShelf(npcID)
		for _, shelfLine := range shelf[:min(len(shelf), exploiterShelfProbes)] {
			for _, give := range duplicates[:min(len(duplicates), exploiterGiveProbes)] {
				d := s.ProposeTrade(npcID, []domain.TradeLine{{Key: give.Key, Qty: 1}}, []domain.TradeLine{{Key: shelfLine.Key, Qty: 1}}, 0)
				switch d.Outcome {
				case domain.OutcomeAccepted:
					duplicates = cheapestDuplicates(s.View(), allDuplicates)
				case domain.OutcomeCountered:
					if d.Offer.TopUp > exploiterMaxTopUp {
						continue
					}
					if accepted, err := s.AcceptOffer(d.Offer.ID); err == nil && accepted.Outcome == domain.OutcomeAccepted {
						duplicates = cheapestDuplicates(s.View(), allDuplicates)
					}
				}
			}
		}
	}
}
```

In `cmd/crackapack/policies.go`, add:

```go
		"exploiter": func() sim.PlayerPolicy { return sim.NewExploiterPolicy() },
```

- [ ] **Step 4: Run the tests**

Run: `go test -race ./...`
Expected: PASS. A failure of `TestNoChainOfTradesGainsValue` on a value rise is a real bug: stop, find the trade that raised it, and fix the Exchange. If the only failure is `no trade was accepted`, the test data is too thin: raise the crowd's wishes or the run length in the test, not the assertion.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal cmd demo
git add internal/sim cmd/crackapack
git commit -m "Add the exploiter policy and a whole-run test that trades never gain value" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 14: Benchmark of a deep run

**Files:**
- Create: `internal/sim/bench_test.go`

- [ ] **Step 1: Write the benchmark**

```go
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
```

- [ ] **Step 2: Run it and record the baseline**

Run: `go test ./internal/sim -run '^$' -bench BenchmarkRun -benchmem -benchtime 3x`
Expected: both sub-benchmarks complete. Write the two ns/op and B/op figures into the commit message body. If the `trades=true` run is more than about 5 times slower than `trades=false`, profile before going on (`-cpuprofile`): the likely cost is `DuplicatesAbove` or `View()` in a hot loop, and the fix belongs in the code, not in this benchmark.

- [ ] **Step 3: Check that memory does not grow with the run length**

Run the benchmark twice with `run_days: 30` and `run_days: 180` by temporarily editing the YAML string, and compare B/op: it should grow roughly linearly with days only through the result's per-day metrics, not through offers or history. If the bytes per day keep climbing, an offer or history slice is not being bounded: find it before committing. Restore `run_days: 90`.

- [ ] **Step 4: Commit**

```bash
gofmt -w internal
git add internal/sim/bench_test.go
git commit -m "Add a benchmark of a 90-day run with 500 NPCs" -m "Baseline on this machine: <paste the ns/op and B/op of both sub-benchmarks>." -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 15: Demo data, ADRs and documentation

**Files:**
- Modify: `demo/config.go`, `cmd/crackapack/demo.go`, `docs/adr/0009-npc-archetypes-are-data.md`, `docs/simulation.md`, `docs/core-features.md`, `docs/architecture.md`, `docs/todo.md`, `CHANGELOG.md`
- Create: `docs/adr/0010-npc-trade-behaviour-is-a-strategy.md`, `docs/adr/0011-npcs-hold-surplus-on-a-shelf.md`, `docs/adr/0012-an-npc-counters-once.md`
- Test: `cmd/crackapack/demo_trades_test.go`
- Delete at the end: `docs/trades-with-npcs-design.md`, `docs/trades-with-npcs-plan.md`

- [ ] **Step 1: Write the failing demo test**

`cmd/crackapack/demo_trades_test.go`:

```go
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdDemo_showsTradeMetrics(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer

	if err := run([]string{"demo", "--out", dir, "--rates", "100"}, &stdout, &stderr); err != nil {
		t.Fatalf("demo: %v\nstderr: %s", err, stderr.String())
	}

	csv, err := os.ReadFile(filepath.Join(dir, "rate-100", "days.csv"))
	if err != nil {
		t.Fatalf("read days.csv: %v", err)
	}
	header, _, _ := strings.Cut(string(csv), "\n")
	if !strings.Contains(header, "trade_accepted") {
		t.Errorf("days.csv header lacks the trade columns: the demo should run with trades on: %s", header)
	}
	result, err := os.ReadFile(filepath.Join(dir, "rate-100", "result.json"))
	if err != nil {
		t.Fatalf("read result.json: %v", err)
	}
	if !strings.Contains(string(result), "trade_accepted") {
		t.Error("result.json summary lacks trade_accepted")
	}
}
```

Run: `go test ./cmd/... -run TestCmdDemo_showsTradeMetrics`
Expected: FAIL, the header lacks `trade_accepted`.

- [ ] **Step 2: Turn trades on in the demo**

In `demo/config.go`, replace the `npcs:` block of `demoConfigYAML` with:

```
npcs:
  population: 60
  surplus_hold_days: 2
trades:
  enabled: true
```
In `cmd/crackapack/demo.go`, change `player:       "collector",` to `player:       "trader",`, and add to the `Long` text, after the seed paragraph: `The demo player is the trader, so the report shows trades with NPCs.`

Run: `go test -race ./...`
Expected: PASS, the new test included. The demo targets and its pass/fail windows may shift with the trader. If a demo or golden test now fails on a pass/fail window or a number, do not edit it: that test asserts a calibration the maintainers chose. Stop and report the failing assertion and the new values to the user, who decides between keeping the collector in the demo (and showing trades another way) and re-calibrating.

- [ ] **Step 3: Write the ADRs**

`docs/adr/0010-npc-trade-behaviour-is-a-strategy.md`:

```markdown
# 0010 – NPC trade behaviour is a strategy

## Status

Accepted — 2026-10-13. Supersedes [ADR-0009](0009-npc-archetypes-are-data.md).

## Context

[ADR-0009](0009-npc-archetypes-are-data.md) made archetypes data, because supply differs between
them by three numbers only. It expected a strategy to appear with trades, where behaviour may not
fit parameters.

It does not. The five archetypes pick what they wish for in different ways: a casual at random, a
playset hunter among commons and uncommons it holds fewer than four of, a targeted collector from
the rarest down, a speculator by value, a whale among rares. They also differ in what they ask of
the player. A parameter cannot say that.

## Decision

An archetype names a registered `TradeBehaviour` and carries the numbers it reads: `behaviour`,
`wish_size`, `margin_pct` and `offer_chance_pct`. The behaviour picks wishes, gives the margin and
builds a proposal.

Supply stays as ADR-0009 decided: `min_boosters_per_day`, `max_boosters_per_day`, `keep_copies`
and `focus_set` are data read by one step. A new personality is a type in `internal/domain`, one
registry entry and a config entry.

## Consequences

- The five default archetypes name the five behaviours of the same name, and a config can define
  others that reuse a registered behaviour with other numbers.
- Config rejects an unknown behaviour, and requires one for every archetype with a share when
  trades are enabled. A behaviour reads only config numbers, so a scenario is still a config file.
- A negative margin is rejected twice, at config load and when a behaviour is built, so an NPC
  never gives more value than it gets.
- ADR-0009 stays as written, apart from its status.
```

`docs/adr/0011-npcs-hold-surplus-on-a-shelf.md`:

```markdown
# 0011 – NPCs hold surplus on a shelf before selling it

## Status

Accepted — 2026-10-13

## Context

The NPC crowd sold every copy above an NPC's keep threshold to the Counter the moment it was drawn,
see [NPC crowd](../simulation.md#npc-crowd). An NPC therefore never held a duplicate, so it had
nothing to give in a trade, and "which NPCs have this card as a duplicate" would always be empty.

## Decision

With trades enabled, surplus waits on the NPC's shelf for `npcs.surplus_hold_days` days, 2 by
default, and can be traded meanwhile. At the end of each day the crowd sells to the Counter every
shelf copy that has waited the hold. Trades need a hold of at least one day.

With trades disabled the hold does not apply, and the crowd sells at once as before, so a run
without trades is unchanged.

## Consequences

- Supply to the Counter is delayed by the hold, so Counter stock differs between a run with trades
  and one without. The two do not compare, and `compare` refuses to mix them.
- `npc_cards_held` counts shelf copies, which are still held.
- The hold is a parameter a search can tune, and so is `trades.max_open_offers_per_day`.
- An NPC that receives a copy above its keep threshold in a trade puts it on the shelf.
```

`docs/adr/0012-an-npc-counters-once.md`:

```markdown
# 0012 – An NPC counters once

## Status

Accepted — 2026-10-13

## Context

[Trades with NPCs](../core-features.md#trades-with-npcs) says a counter-offer is possible on both
sides, and does not say how far a negotiation goes. An open-ended exchange needs a negotiation state
and a strategy for what each side asks next, and makes a run harder to reproduce and to tune.

## Decision

When a player's offer falls short of the acceptance rule, the NPC answers once with the same cards
and the smallest top-up that satisfies it. The counter is an open offer the player accepts or
ignores. When an NPC proposes, the player may counter once with different cards, and the NPC then
accepts or refuses without countering again.

## Consequences

- A trade is at most three messages, and no negotiation state exists beyond the open offers.
- A run stays deterministic from its seed.
- A multi-round negotiation would be a new ADR that supersedes this one.
```

In `docs/adr/0009-npc-archetypes-are-data.md`, replace the Status section body `Accepted — 2026-10-13` with:

```
Superseded by [ADR-0010](0010-npc-trade-behaviour-is-a-strategy.md) — 2026-10-13. Accepted
2026-10-13.
```

- [ ] **Step 4: Update the docs**

**`docs/simulation.md`:**

1. Replace the paragraph that ends "Trades, offline catch-up and persistence are not built, see [Specs review](specs-review.md#v1-scope-and-build-order)." with: `The archetype values in the defaults are starting guesses. Offline catch-up and persistence are not built, see [Specs review](specs-review.md#v1-scope-and-build-order).`

2. Insert before `## Run diagram`:

````markdown
## Trades

`trades.enabled` turns on trades between the player and the NPCs. It defaults to `false`, and a run
without trades is the same as before they existed: no `trade_*` column in `days.csv`, no `Trade`
block in `result.json`, and no `trade_*` metric in the summary. Trades need a crowd
(`npcs.population` above 0) and `npcs.surplus_hold_days` of at least 1. The rules are in
[Trades with NPCs](core-features.md#trades-with-npcs).

An NPC holds the copies above its keep threshold on a shelf for `npcs.surplus_hold_days` days
before selling them to the Counter, see [ADR-0011](adr/0011-npcs-hold-surplus-on-a-shelf.md). Only
the shelf can be traded. Each NPC has a wish list, and when a card on it is a duplicate of the
player's, the NPC may propose a swap. The player can also propose one to any NPC. An NPC accepts
when it receives at least the reference value it gives plus its margin, never negative. When an
offer falls short, the NPC counters once with the smallest coin top-up that fits, see
[ADR-0012](adr/0012-an-npc-counters-once.md). The coins the player adds go to the
`trade_topup` account, because NPCs hold no coins.

An archetype names a trade behaviour, see
[ADR-0010](adr/0010-npc-trade-behaviour-is-a-strategy.md):

| Behaviour | Wishes for |
|-----------|------------|
| `casual` | Any card it lacks, at random |
| `playset_hunter` | Commons and uncommons, up to a playset |
| `targeted_collector` | The cards it lacks, rarest first |
| `speculator` | The most valuable cards it lacks, and asks for the most valuable duplicates first |
| `whale` | Rares and specials, at random |

Every trade setting is a key of the configuration, so a scenario is a config file:

```yaml
npcs:
  population: 300
  surplus_hold_days: 2
  archetypes:
    speculator: {share: 10, min_boosters_per_day: 2, max_boosters_per_day: 4, keep_copies: 2,
                 behaviour: speculator, wish_size: 5, margin_pct: 25, offer_chance_pct: 1}
trades:
  enabled: true
  offer_expiry_days: 3
  max_cards_per_side: 10
  duplicate_keep: 1
  history_size: 0
  max_open_offers_per_day: 20
```

Run it with a player that trades:

```
go run ./cmd/crackapack simulate \
  --catalogue-dir ./catalogues --sets RAV,TSP --supplementary TSB \
  --prices prices.json --config trades.yaml --player trader --out ./out
```

Two policies trade, see `--player`:

- `trader` plays like the collector and also accepts NPC offers that bring a card it lacks, and
  swaps its duplicates for missing cards NPCs hold, paying a counter's top-up above the coin floor.
- `exploiter` chains trades across every NPC looking for a gain. It finds none, and a test checks
  that no trade, in any run, raises the player's coins plus the reference value of its cards.

A run with trades adds these metrics, summed over the run in the summary and per day in
`days.csv`: `trade_player_proposals`, `trade_npc_offers`, `trade_accepted`, `trade_countered`,
`trade_refused`, `trade_expired`, `trade_volume_coins` and `trade_topup_coins`. `days.csv` also
gets the `trade_topup` balance. `compare` refuses a run with trades against one without.

To see the cost of a long run, `go test ./internal/sim -run '^$' -bench BenchmarkRun -benchmem`
runs 90 days with 500 NPCs with and without trades.
````

3. Search the file for the lists of player policies (`grep -n "cash-out" docs/simulation.md README.md docs/*.md`) and add `trader` and `exploiter` wherever `collector`, `cash-out` and `idle` are listed, and a link to the Trades section for them.

**`docs/core-features.md`:** in the Trades section:
- Replace `- **Expiry** of an unanswered offer: duration to be set.` with `- **Expiry** of an unanswered offer: 3 days by default, \`trades.offer_expiry_days\`, the day it is made included.`
- Replace item 4, `4. A counter-offer is possible on both sides.` with `4. A counter-offer is possible on both sides, once: see [ADR-0012](adr/0012-an-npc-counters-once.md). An NPC gives only the surplus it holds on its shelf, see [ADR-0011](adr/0011-npcs-hold-surplus-on-a-shelf.md).`
- In "NPCs and simulation", replace the sentence `Trade proposals are not built: the simulation runs the supply half, see [NPC crowd](simulation.md#npc-crowd).` with `The simulation runs supply and trades, see [NPC crowd](simulation.md#npc-crowd) and [Trades](simulation.md#trades).`
- In "Open questions", mark `- [ ] Expiry duration of a trade offer.` as `- [x] Expiry duration of a trade offer: 3 days, see [Trades](simulation.md#trades).`

**`docs/architecture.md`:** after the `internal/sim` paragraph in "Packages in the simulation MVP", add: `The domain also holds the trades: \`Exchange\` prices and executes offers between the player and the crowd, \`Valuer\` gives the Counter and the Exchange the same reference value, and a \`TradeBehaviour\` per archetype picks wishes, the margin and proposals. \`internal/sim\` composes them behind \`trades.enabled\` and exposes them to player policies through \`Session\`.`

**`docs/todo.md`:**
- Under "Done so far" add `- [x] Trades with NPCs in the simulation, see [Trades](simulation.md#trades).`
- Tick, with `[x]`, the sub-items of "Trades with NPCs" that are built: wish lists, archetype margins (adding "recorded in ADR-0010, a strategy"), the shared reference value, NPC-proposed trades, player-proposed trades, the acceptance rule, counter-offers, atomic transfer, the ledger account, offer expiry, trade history and "which NPCs have this card", and the simulated policies with the invariant test. Leave the parent item and the sub-item "A trade that removes the last copy of a card can cost a milestone" unticked, and add to that sub-item: `Not built: it needs the milestones item first, and no hook exists for it yet.`
- In "Tuning system and metrics", tick `Metrics from ... that do not exist yet: trades proposed and accepted, and trade volume per day.` by rewriting it to say only what is still missing (`Money supply by flow once quests and milestones add flows.` is already its own bullet), and tick the trading half of `More player policies`.

**Links to ADR-0009:** run `grep -rn "0009\|npc-archetypes-are-data" .claude docs README.md AGENTS.md CHANGELOG.md`. Where a file states the current rule about archetypes (the `crackapack-conventions` skill, `docs/simulation.md`, `docs/core-features.md`), point it at ADR-0010, or at both when it describes supply. Leave the history in the CHANGELOG and in ADR-0009 itself alone.

**`CHANGELOG.md`:** under `## [Unreleased]` / `### Added`, add as the first bullet:

```markdown
- Trades with NPCs in the simulation, behind `trades.enabled` (default off, so existing runs are
  unchanged). NPCs hold their surplus on a shelf, wish for cards, propose swaps for the player's
  duplicates and counter a short offer once. Every setting is a config key, and two player
  policies trade: `trader` and `exploiter`. See [Trades](docs/simulation.md#trades),
  [ADR-0010](docs/adr/0010-npc-trade-behaviour-is-a-strategy.md),
  [ADR-0011](docs/adr/0011-npcs-hold-surplus-on-a-shelf.md) and
  [ADR-0012](docs/adr/0012-an-npc-counters-once.md).
```
and under `### Changed` add `- ADR-0009 is superseded by ADR-0010: an archetype now names a trade behaviour, and its supply parameters stay data.`

- [ ] **Step 5: Verify every claim in the docs**

Run each of these and compare with what the docs now say:
- `go run ./cmd/crackapack demo | head -20` and `go run ./cmd/crackapack demo --out /tmp/demo-out --rates 100 && head -c 600 /tmp/demo-out/rate-100/days.csv`: the `trade_*` columns exist.
- `grep -rn "trader\|exploiter" cmd/crackapack/policies.go`: both are registered.
- `go run ./cmd/crackapack simulate --help | grep -i player`: the help text of `--player` still describes the policies. If it lists them by name, add the two new ones in `cmd/crackapack/simulate.go` and its test.
- Open each link added to the docs and confirm the target heading exists (`grep -n "^## Trades" docs/simulation.md`, the ADR file names, `#trades-with-npcs` in core-features).
- Check the YAML example loads: save it to `/tmp/trades.yaml` and run `go run ./cmd/crackapack demo --help >/dev/null && go test ./internal/config -run TestLoad_trades`, then load it with a throwaway `go run` or a unit test that calls `config.Load` on the exact text. Fix the docs, not the code, if a key name is off.

- [ ] **Step 6: Full verification**

Run, and read the output of each:
- `gofmt -l .` (no output)
- `go test -race ./...`
- `golangci-lint run`
- `go mod tidy -diff`

Expected: all clean. Fix every lint finding in the new code; do not add `//nolint` without a reason in the comment.

- [ ] **Step 7: Remove the working documents and commit**

The design and the plan were working documents: their content now lives in the ADRs and docs.

```bash
git rm docs/trades-with-npcs-design.md docs/trades-with-npcs-plan.md
git add -A docs cmd demo CHANGELOG.md
git commit -m "Run the demo with trades, record the decisions and document trades" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

## Self-review notes

- **Spec coverage:** Valuer and ledger (Tasks 1, 2); Exchange, acceptance rule, one counter, atomic transfer, expiry, history, `HoldersOf`, the card cap and duplicate keep (Tasks 3, 6, 7); NPC shelf, wishes, behaviours and indexes (Tasks 4, 5); configuration and validation including the trade keys of every archetype (Task 9); metrics, outputs, `compare` and the byte-identical-when-off guarantee (Task 10); `Session` and the two policies (Tasks 11 to 13); the invariant as a property test and a whole-run test (Tasks 8, 13); the benchmark and bounded memory (Task 14); demo data, ADRs 0010 to 0012 and docs (Task 15). The milestone interaction is left out on purpose and recorded in `docs/todo.md`.
- **Spec changes made while planning**, already applied to the design: no milestone hook (dead code), `HoldersOf` takes a `CardKey`, the shelf hold must be at least 1 when trades are on, and the trade actions replace the `WorldView` additions.
- **Type consistency:** `Propose`, `Accept` and `Counter` all return `(Decision, error)`; `Trading.ProposeTrade` returns a bare `Decision` and puts a fatal error into the port. `Offer`, `OfferOrigin` and `TradeLine` are declared once (Tasks 3 and 4). `tradeCardCap`, `priceOf` and `cheapestDuplicates` are defined in Task 12 and reused in Task 13.
