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
