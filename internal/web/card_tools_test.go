package web_test

import (
	"strings"
	"testing"
)

// toolsOf is the part of a slot that is its tools: the name and the buttons that the card shows when
// it is opened big.
func toolsOf(t *testing.T, entry string) string {
	t.Helper()
	_, tools, found := strings.Cut(entry, `<div class="tools"`)
	if !found {
		t.Fatalf("the slot has no tools:\n%s", entry)
	}
	tools, _, _ = strings.Cut(tools, "</div>")
	return tools
}

func TestCollection_everySlotHasAButtonThatOpensItBigWithItsTools(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)

	empty := slot(t, shop.get(t, "/collection?set=S"), cheapCard)
	shop.open(t, 1)
	held := slot(t, shop.get(t, "/collection?set=S"), cheapCard)

	for what, entry := range map[string]string{"an empty slot": empty, "a held card": held} {
		mustContain(t, what, entry,
			`<button type="button" class="pick" aria-haspopup="dialog"><span class="sr-only">Open S-00</span></button>`,
			`<p class="tools-name">S-00</p>`)
		mustNotContain(t, what, entry, `tool-view`, `aria-expanded`, `class="view"`)
	}
	mustContain(t, "a held card", held, `<span class="tag count">×1</span>`)
	mustNotContain(t, "the face of a held card, which only says how many copies are held", held, `Sell</`, `Buy</`)
}

func TestCollection_theBuyAndSellButtonsAreIconsInTheToolsOfTheCard(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)

	empty := toolsOf(t, slot(t, shop.get(t, "/collection?set=S"), cheapCard))
	shop.open(t, 1)
	held := toolsOf(t, slot(t, shop.get(t, "/collection?set=S"), cheapCard))

	mustContain(t, "the tools of an empty slot", empty, `action="/buy"`, `class="tool buy"`, `<span class="tool-price">120</span>`, `<span class="sr-only">Buy S-00 for ◎120</span>`)
	mustNotContain(t, "the tools of an empty slot", empty, `action="/sell"`)
	mustContain(t, "the tools of a held card", held, `action="/sell"`, `class="tool sell"`, `<span class="tool-price">60</span>`, `name="qty" value="1"`)
	mustContain(t, "the tools of a held card, which can be bought again for a playset", held, `action="/buy"`, `class="tool buy"`, `<span class="sr-only">Buy S-00 for ◎120</span>`)
	mustNotContain(t, "the tools of a held card", held, `<details`)
}

func TestCollection_aCardTheCounterWillNotSellStillSaysWhyInItsTools(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)

	got := toolsOf(t, slot(t, shop.get(t, "/collection?set=S"), neverCard))

	mustContain(t, "the tools of a card never in stock", got, "never seen at the Counter", "◎120")
	mustNotContain(t, "the tools of a card never in stock", got, `action="/buy"`)
}

func TestCollection_sellingSeveralCopiesAsksTheQuantityInAPanelOfTheTools(t *testing.T) {
	t.Parallel()
	shop := newCounterShop(t)
	shop.open(t, 2)

	got := toolsOf(t, slot(t, shop.get(t, "/collection?set=S"), cheapCard))

	mustContain(t, "the tools of a card held twice", got,
		`<details class="tool-panel">`, `<summary class="tool sell">`, `action="/sell"`, `name="qty" value="1" min="1" max="2"`,
		`Quantity<span class="sr-only"> of S-00</span>`)
}

func TestCollection_listingACardAsksThePriceInAPanelAndAskingAroundIsOneButton(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)

	empty := toolsOf(t, slot(t, shop.get(t, "/collection?set=S"), cheapCard))
	shop.open(t, 1)
	held := toolsOf(t, slot(t, shop.get(t, "/collection?set=S"), cheapCard))

	mustContain(t, "the tools of an empty slot", empty, `action="/ask-around"`, `class="tool trade"`, `<span class="sr-only">Ask around for S-00</span>`)
	mustNotContain(t, "the tools of an empty slot", empty, `<details`, `action="/list"`)
	mustContain(t, "the tools of a held card", held,
		`<details class="tool-panel">`, `<summary class="tool trade">`, `action="/list"`, `name="price"`, `aria-describedby="price-hint"`)
}

func TestCollection_aPendingRequestStaysUnderTheCardAndTakesTheTradeButtonAway(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.post(t, "/ask-around", cardForm("S", cheapCard, "nonfoil"))

	got := slot(t, shop.get(t, "/collection?set=S"), cheapCard)

	mustContain(t, "the slot after asking", got, `class="request"`, `action="/withdraw"`)
	mustNotContain(t, "the tools after asking", toolsOf(t, got), `class="tool trade"`, `action="/withdraw"`)
}
