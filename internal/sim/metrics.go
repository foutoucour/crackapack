package sim

import (
	"fmt"
	"sort"
	"strings"

	"github.com/foutoucour/crackapack/internal/domain"
)

// SetDayProgress is the binder progress snapshot for one set on one day.
type SetDayProgress struct {
	Owned    int // distinct cards owned (any finish)
	Total    int // total non-basic-land cards in the set
	CUOwned  int // commons + uncommons owned
	CUTotal  int // commons + uncommons total
	Playsets int // cards with at least 4 copies across all finishes
}

// DayMetrics records everything observable about one simulated day.
// Day is 1-based: the first simulated day is Day 1.
type DayMetrics struct {
	Day int

	// MoneySupply holds the balance of every ledger account at end of day.
	// encoding/json sorts Account (string) keys alphabetically.
	MoneySupply map[domain.Account]domain.Coins

	// StockByRarity holds the Counter's total stock per rarity at end of day.
	// encoding/json sorts Rarity (string) keys alphabetically.
	StockByRarity map[domain.Rarity]int

	CollectionUSD   domain.USDCents
	CollectionCoins domain.Coins // 0 when sub-coin; not an error

	// SetProgress maps each openable set code to its progress snapshot.
	SetProgress map[string]SetDayProgress

	BoostersOpened int
	CardsSold      int
	CardsBought    int
	TradesRefused  int
}

// Summary is a map of named scalar metrics computed from a completed run.
// All keys are lowercase snake_case. Set-specific metrics use the pattern
// <name>.<setcode> where setcode is the catalogue Code field lowercased,
// for example "days_to_50pct_cu.rav" or "days_to_100pct_cu.cls".
//
// Days-to metrics are 1-based (matching DayMetrics.Day). A milestone not
// reached within the run is encoded as float64(RunDays+1) — a censored value
// greater than the run horizon, which no valid at_most target can satisfy.
//
// Named metrics:
//   - final_balance: player coin balance after the last day
//   - total_refused_trades: sum of TradesRefused across all days
//   - sub_coin_cards: distinct non-foil, non-basic-land cards whose Counter sell
//     value (at SellPct) is sub-coin at the configured rate
//   - daily_reward_commons: floor(DailyReward / medianCommonBuyCoins) where
//     medianCommonBuyCoins is PriceToCoins(median common non-basic price, rate, BuyPct);
//     0 when the median converts to sub-coin
//   - days_to_50pct_cu.<setcode>: first day where CUOwned/CUTotal >= 0.5
//   - days_to_100pct_cu.<setcode>: first day where CUOwned == CUTotal
type Summary map[string]float64

// Result is the complete output of one simulation run.
type Result struct {
	Days    []DayMetrics
	Summary Summary
}

// computeSummary derives the Summary from the day metrics and the run parameters.
func computeSummary(
	days []DayMetrics,
	runDays int,
	cats []domain.SetCatalogue,
	prices domain.PriceSnapshot,
	fallback domain.USDCents,
	coinRate domain.CoinRate,
	sellPct domain.Percent,
	buyPct domain.Percent,
	dailyReward domain.Coins,
) Summary {
	s := make(Summary)

	// final_balance
	if len(days) > 0 {
		last := days[len(days)-1]
		s["final_balance"] = float64(last.MoneySupply[domain.AccountPlayer])
	}

	// total_refused_trades
	var totalRefused int
	for _, d := range days {
		totalRefused += d.TradesRefused
	}
	s["total_refused_trades"] = float64(totalRefused)

	// sub_coin_cards: distinct non-foil, non-basic-land cards whose sell value is sub-coin
	s["sub_coin_cards"] = float64(countSubCoinCards(cats, prices, fallback, coinRate, sellPct))

	// daily_reward_commons
	s["daily_reward_commons"] = float64(dailyRewardCommons(cats, prices, fallback, coinRate, buyPct, dailyReward))

	// days_to_50pct_cu.<setcode> and days_to_100pct_cu.<setcode>
	censored := float64(runDays + 1)
	for _, cat := range cats {
		code := strings.ToLower(cat.Code)
		key50 := fmt.Sprintf("days_to_50pct_cu.%s", code)
		key100 := fmt.Sprintf("days_to_100pct_cu.%s", code)
		s[key50] = censored
		s[key100] = censored
		for _, d := range days {
			prog, ok := d.SetProgress[cat.Code]
			if !ok || prog.CUTotal == 0 {
				continue
			}
			if s[key50] == censored && prog.CUOwned*2 >= prog.CUTotal {
				s[key50] = float64(d.Day)
			}
			if s[key100] == censored && prog.CUOwned == prog.CUTotal {
				s[key100] = float64(d.Day)
			}
		}
	}

	return s
}

// countSubCoinCards counts distinct non-foil, non-basic-land cards across cats
// whose Counter sell value (at sellPct) converts to sub-coin.
func countSubCoinCards(
	cats []domain.SetCatalogue,
	prices domain.PriceSnapshot,
	fallback domain.USDCents,
	rate domain.CoinRate,
	sellPct domain.Percent,
) int {
	// Collect all distinct non-foil, non-basic-land CardIDs sorted for determinism.
	seen := make(map[domain.CardID]struct{})
	ids := make([]domain.CardID, 0)
	for _, cat := range cats {
		cardIDs := make([]domain.CardID, 0, len(cat.Cards))
		for id := range cat.Cards {
			cardIDs = append(cardIDs, id)
		}
		sort.Slice(cardIDs, func(i, j int) bool { return cardIDs[i] < cardIDs[j] })
		for _, id := range cardIDs {
			card := cat.Cards[id]
			if card.IsBasicLand {
				continue
			}
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}

	count := 0
	for _, id := range ids {
		price, ok := prices[id]
		if !ok {
			price = fallback
		}
		_, err := domain.PriceToCoins(price, rate, sellPct)
		if err != nil {
			// Both ErrSubCoin and a zero price (returns nil with 0) count here.
			// PriceToCoins returns ErrSubCoin when result rounds to zero on a non-zero price.
			count++
		}
	}
	return count
}

// dailyRewardCommons returns floor(dailyReward / medianCommonBuyCoins).
// Returns 0 when the median converts to sub-coin.
func dailyRewardCommons(
	cats []domain.SetCatalogue,
	prices domain.PriceSnapshot,
	fallback domain.USDCents,
	rate domain.CoinRate,
	buyPct domain.Percent,
	dailyReward domain.Coins,
) int {
	// Collect prices of all non-basic common cards across all catalogues.
	var commonPrices []domain.USDCents
	for _, cat := range cats {
		ids := make([]domain.CardID, 0, len(cat.Cards))
		for id := range cat.Cards {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			card := cat.Cards[id]
			if card.IsBasicLand || card.Rarity != domain.RarityCommon {
				continue
			}
			price, ok := prices[id]
			if !ok {
				price = fallback
			}
			commonPrices = append(commonPrices, price)
		}
	}
	if len(commonPrices) == 0 {
		return 0
	}
	median, err := domain.MedianValue(commonPrices)
	if err != nil {
		return 0
	}
	buyCoins, err := domain.PriceToCoins(median, rate, buyPct)
	if err != nil || buyCoins == 0 {
		return 0
	}
	return int(dailyReward / buyCoins)
}
