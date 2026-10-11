package domain

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
