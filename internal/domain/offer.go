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
