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
