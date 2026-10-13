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
