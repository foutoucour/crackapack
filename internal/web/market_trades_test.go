package web_test

import (
	"strings"
	"testing"
)

// section is the part of the trades page between the heading of a group and the next one.
func section(t *testing.T, body, heading, next string) string {
	t.Helper()
	start := strings.Index(body, ">"+heading+"<")
	end := strings.Index(body, ">"+next+"<")
	if start < 0 || end < start {
		t.Fatalf("the page has no %q before %q:\n%s", heading, next, body)
	}
	return body[start:end]
}

// A trade says whom it waits for: the player when an answer is open, an NPC when the answers or the
// replies are still to come.
func TestTradesPage_groupsTheTradesByWhoTheyWaitFor(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.ask(t, cheapCard)

	body := shop.get(t, "/market/trades")

	needs := section(t, body, "Needs you", "Waiting on an NPC")
	mustContain(t, "Needs you", needs, "Asking around for S-00", "1 of 2 NPCs answered", "Next answer in about 6 h", "Lapses in about 3 days", `action="/accept"`)
	waiting := section(t, body, "Waiting on an NPC", "Done")
	mustContain(t, "Waiting on an NPC", waiting, "Nothing waits on an NPC")
}

func TestTradesPage_aTradeMovesToWaitingOnceTheOpenAnswersAreTaken(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.ask(t, cheapCard)
	shop.post(t, "/decline", answerForm(shop.answerFrom(t, fastNPC)))

	body := shop.get(t, "/market/trades")

	mustContain(t, "Needs you", section(t, body, "Needs you", "Waiting on an NPC"), "Nothing waits for you")
	waiting := section(t, body, "Waiting on an NPC", "Done")
	mustContain(t, "Waiting on an NPC", waiting, "Asking around for S-00", "1 of 2 NPCs answered", "Next answer in about 6 h")
}

func TestTradesPage_aCounterWaitsOnTheNPCThatWillReply(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.ask(t, cheapCard)
	shop.clock.Advance(slowDelay)
	shop.post(t, "/decline", answerForm(shop.answerFrom(t, fastNPC)))
	counter := answerForm(shop.answerFrom(t, slowNPC))
	counter.Set("coins", "50")
	shop.post(t, "/counter-offer", counter)

	body := shop.get(t, "/market/trades")

	waiting := section(t, body, "Waiting on an NPC", "Done")
	mustContain(t, "Waiting on an NPC", waiting, "2 of 2 NPCs answered", "Karn, the Great Creator replies in about 6 h", "You countered with ◎50")
}

func TestTradesPage_aListingSaysWhenTheNextOfferComes(t *testing.T) {
	t.Parallel()
	shop := newMarketShop(t)
	shop.open(t, 1)
	shop.post(t, "/list", listForm(unpricedKey, "5"))

	body := shop.get(t, "/market/trades")

	waiting := section(t, body, "Waiting on an NPC", "Done")
	mustContain(t, "Waiting on an NPC", waiting, "Listed at ◎5", "Next offer in about 1 h", `action="/unlist"`)
}
