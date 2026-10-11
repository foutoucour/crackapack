package domain

import "math/rand/v2"

// Rarity is the rarity printed on a card. The V1 catalogue (2005-2007) has no mythics.
// Special is the rarity of Time Spiral's timeshifted cards.
type Rarity string

const (
	RarityCommon   Rarity = "common"
	RarityUncommon Rarity = "uncommon"
	RarityRare     Rarity = "rare"
	RaritySpecial  Rarity = "special"
)

// CardID identifies one printing of a card. It is the MTGJSON UUID.
type CardID string

// USDCents is a price in US cents. Money is never a float.
type USDCents int64

// Coins is an amount of in-game currency. Coins have no monetary value.
type Coins int64

// CoinRate is the number of coins one US dollar is worth. It is a whole number.
type CoinRate int64

// PriceSnapshot is the fixed set of prices a simulation runs on. A card absent from it has
// no price and falls back to the configured fallback price.
type PriceSnapshot map[CardID]USDCents

// Percent is a whole percentage: 100 means 100%. The Counter's rates are Percents (60 to sell,
// 120 to buy), so a price scaled by a rate stays an integer computation.
type Percent int64

// CardKey identifies one collectible: a printing in a given finish. The collection and the
// Counter's stock are both keyed by it.
type CardKey struct {
	ID     CardID
	Finish Finish
}

// Booster is the cards drawn from one pack, in reveal order with the rare last.
type Booster struct {
	SetCode string
	Cards   []CardKey
}

// BoosterGenerator builds one booster from a set catalogue. Randomness comes only from rng, so
// a booster is reproducible from the seed. Implementations must not depend on map iteration
// order: sort keys before drawing.
type BoosterGenerator interface {
	Generate(cat SetCatalogue, rng *rand.Rand) (Booster, error)
}

// PlaysetSize is the number of copies of a card that make a playset, the reference unit of
// Magic collecting.
const PlaysetSize = 4
