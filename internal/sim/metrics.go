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

// NPCDayMetrics is what the NPC crowd did on one day.
type NPCDayMetrics struct {
	BoostersOpened int
	// CardsSold is the number of cards the NPCs sent to the Counter as surplus during the day.
	CardsSold int
	// CardsHeld is the number of cards all NPC collections hold at the end of the day.
	CardsHeld int
}

// CounterDayMetrics is what the Counter did on its own on one day.
type CounterDayMetrics struct {
	// BoostersOpened is the boosters the Counter opened to stock itself.
	BoostersOpened int
	// CardsOpened is the cards those boosters held, all put on the Counter's shelf.
	CardsOpened int
	// FeeCoins is the coin fees the Counter paid from its treasury to open boosters.
	FeeCoins domain.Coins
	// Treasury is the coins in the Counter's treasury at the end of the day.
	Treasury domain.Coins
	// BrokeRefusals is the sales the Counter refused because its treasury could not pay.
	BrokeRefusals int
}

// BoosterFeeDayMetrics is what the player paid in coin fees to open boosters on one day.
type BoosterFeeDayMetrics struct {
	// Coins is the fees paid from the player's wallet during the day.
	Coins domain.Coins
}

// CoinTopupDayMetrics records how many energy units the player bought with coins on one day,
// and the coins paid for them.
type CoinTopupDayMetrics struct {
	// Units is the energy units bought with coins during the day.
	Units int
	// Coins is the coins paid into the coin_topup sink during the day.
	Coins domain.Coins
}

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

// DayMetrics records everything observable about one simulated day.
// Day is 1-based: the first simulated day is Day 1.
type DayMetrics struct {
	Day int

	// NPC is nil when the run has no NPC crowd, which keeps the output of such runs unchanged.
	NPC *NPCDayMetrics `json:",omitempty"`

	// Counter is nil when the Counter does not open boosters, which keeps the output of such runs
	// unchanged.
	Counter *CounterDayMetrics `json:",omitempty"`

	// BoosterFee is nil when the player pays no coin fee, which keeps the output of such runs
	// unchanged.
	BoosterFee *BoosterFeeDayMetrics `json:",omitempty"`

	// CoinTopup is nil when the coin top-up feature is not enabled, which keeps the output of
	// such runs unchanged.
	CoinTopup *CoinTopupDayMetrics `json:",omitempty"`

	// Trade is nil when trades are not enabled, which keeps the output of such runs unchanged.
	Trade *TradeDayMetrics `json:",omitempty"`

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

	// TradesRefused counts individual copy-sell attempts refused by the Counter
	// (ErrSubCoin) during this day, not distinct cards. A single sub-coin card
	// that cannot be sold above a playset generates one refusal per copy per
	// session per day; at low coin rates this number can be large. Use the
	// summary metric sub_coin_cards for the distinct-card count, and
	// total_refused_trades as a volume signal for how much stuck inventory
	// the economy is accumulating.
	TradesRefused int
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
//   - total_refused_trades: sum of TradesRefused across all days; a volume signal
//     for stuck inventory (one sub-coin card above playset generates one refusal
//     per copy per session per day). Use sub_coin_cards for the distinct-card count.
//   - sub_coin_cards: distinct non-foil, non-basic-land cards across openable and
//     supplementary catalogues whose Counter sell value (at SellPct) is sub-coin
//     at the configured rate, including cards priced at zero (zero price returns
//     (0, nil) from PriceToCoins but the Counter refuses it identically to ErrSubCoin);
//     the economy is calibrated when this is zero
//   - daily_reward_commons: floor(DailyReward / medianCommonBuyCoins) where
//     medianCommonBuyCoins is PriceToCoins(median common non-basic price, rate, BuyPct)
//     over openable and supplementary catalogues; 0 when the median converts to sub-coin
//   - unpriced_cards: distinct non-basic-land cards across openable and supplementary
//     catalogues that are absent from the price snapshot and use fallback_price
//   - unsold_duplicate_copies: copies above a playset (domain.PlaysetSize) still
//     held at the end of the run; non-zero when sub-coin cards cannot be sold.
//     This is a policy-independent measure: it counts copies above domain.PlaysetSize
//     regardless of the policy's own keep count.
//   - days_to_50pct_cu.<setcode>: first 1-based day where CUOwned/CUTotal >= 0.5
//     (openable catalogues only)
//   - days_to_100pct_cu.<setcode>: first 1-based day where CUOwned == CUTotal
//     (openable catalogues only)
//   - days_to_100pct.<setcode>: first 1-based day where Owned == Total, the whole binder
//     of the set (openable catalogues only)
//   - coins_after_full_binder: player balance gained from the first day every openable set
//     is complete to the end of the run; 0 when that day is never reached
//   - max_booster_wait_minutes: longest wait, from an empty reserve and empty savings, to
//     afford one booster of any openable set; set by the energy settings, not by play
//
// A run with an NPC crowd adds three more, absent from every other run:
//   - npc_boosters_opened: boosters the NPCs opened over the run
//   - npc_cards_sold: cards the NPCs sent to the Counter over the run
//   - npc_cards_held: cards all NPC collections hold after the last day
//
// A run where the Counter opens boosters or has a treasury adds five more, absent from every
// other run:
//   - counter_boosters_opened: boosters the Counter opened over the run
//   - counter_cards_opened: cards those boosters held, all put on the Counter's shelf
//   - counter_fee_coins: coin fees the Counter paid from its treasury to open boosters
//   - counter_broke_refusals: sales refused because the treasury could not pay
//   - counter_treasury: coins in the treasury after the last day
//
// A run where the player pays a coin fee for energy adds one, absent from every other run:
//   - booster_fee_coins: coin fees the player paid to open boosters over the run
//
// A run with coin top-up enabled adds two, absent from every other run:
//   - coin_topup_units: energy units the player bought with coins over the run
//   - coin_topup_coins: coins paid into the coin_topup sink over the run
//
// A run with trades adds eight more, absent from every other run, each summed over the run:
//   - trade_player_proposals, trade_npc_offers: offers the player and the NPCs made
//   - trade_accepted, trade_countered, trade_refused, trade_expired: how offers ended
//   - trade_volume_coins: reference value of the cards moved, in coins
//   - trade_topup_coins: coins the player paid on top of cards
type Summary map[string]float64

// addCounterSummary adds the totals of the Counter's own openings to s. Every entry of days must
// carry Counter metrics.
func addCounterSummary(s Summary, days []DayMetrics) {
	var boosters, cards, refusals int
	var fees domain.Coins
	for _, d := range days {
		boosters += d.Counter.BoostersOpened
		cards += d.Counter.CardsOpened
		fees += d.Counter.FeeCoins
		refusals += d.Counter.BrokeRefusals
	}
	s["counter_boosters_opened"] = float64(boosters)
	s["counter_cards_opened"] = float64(cards)
	s["counter_fee_coins"] = float64(fees)
	s["counter_broke_refusals"] = float64(refusals)
	if len(days) > 0 {
		s["counter_treasury"] = float64(days[len(days)-1].Counter.Treasury)
	}
}

// addBoosterFeeSummary adds the total of the player's booster fees to s. Every entry of days must
// carry booster fee metrics.
func addBoosterFeeSummary(s Summary, days []DayMetrics) {
	var fees domain.Coins
	for _, d := range days {
		fees += d.BoosterFee.Coins
	}
	s["booster_fee_coins"] = float64(fees)
}

// addCoinTopupSummary adds the coin top-up totals to s.
func addCoinTopupSummary(s Summary, days []DayMetrics) {
	var units int
	var coins domain.Coins
	for _, d := range days {
		if d.CoinTopup != nil {
			units += d.CoinTopup.Units
			coins += d.CoinTopup.Coins
		}
	}
	s["coin_topup_units"] = float64(units)
	s["coin_topup_coins"] = float64(coins)
}

// addNPCSummary adds the crowd totals to s. Every entry of days must carry NPC metrics.
func addNPCSummary(s Summary, days []DayMetrics) {
	var opened, sold int
	for _, d := range days {
		opened += d.NPC.BoostersOpened
		sold += d.NPC.CardsSold
	}
	s["npc_boosters_opened"] = float64(opened)
	s["npc_cards_sold"] = float64(sold)
	if len(days) > 0 {
		s["npc_cards_held"] = float64(days[len(days)-1].NPC.CardsHeld)
	}
}

// Result is the complete output of one simulation run.
type Result struct {
	Days    []DayMetrics
	Summary Summary
}

// computeSummary derives the Summary from the day metrics, run parameters,
// and the final collection state.
//
// cats holds the openable catalogues; supplementary holds extra catalogues whose
// cards are known to the Counter but not tracked in binder progress. sub_coin_cards,
// daily_reward_commons, and unpriced_cards are computed over cats+supplementary.
// days_to_* metrics are computed over cats only.
func computeSummary(
	days []DayMetrics,
	runDays int,
	cats []domain.SetCatalogue,
	supplementary []domain.SetCatalogue,
	prices domain.PriceSnapshot,
	fallback domain.USDCents,
	coinRate domain.CoinRate,
	sellPct domain.Percent,
	buyPct domain.Percent,
	dailyReward domain.Coins,
	collection *domain.Collection,
) Summary {
	s := make(Summary)

	// Combine openable + supplementary for metrics that span all known cards.
	allCats := make([]domain.SetCatalogue, 0, len(cats)+len(supplementary))
	allCats = append(allCats, cats...)
	allCats = append(allCats, supplementary...)

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

	// sub_coin_cards: distinct non-foil, non-basic-land cards across openable and
	// supplementary catalogues whose Counter sell value is sub-coin (including zero price).
	s["sub_coin_cards"] = float64(countSubCoinCards(allCats, prices, fallback, coinRate, sellPct))

	// daily_reward_commons: median over commons of openable + supplementary.
	s["daily_reward_commons"] = float64(dailyRewardCommons(allCats, prices, fallback, coinRate, buyPct, dailyReward))

	// unpriced_cards: distinct non-basic-land cards absent from the price snapshot.
	s["unpriced_cards"] = float64(countUnpricedCards(allCats, prices))

	// unsold_duplicate_copies: copies above a playset still held at end of run.
	// This is a policy-independent measure; it always uses domain.PlaysetSize regardless
	// of the policy's own keep count.
	s["unsold_duplicate_copies"] = float64(countUnsoldDuplicates(collection))

	// days_to_50pct_cu.<setcode> and days_to_100pct_cu.<setcode> (openable catalogues only).
	censored := float64(runDays + 1)
	for _, cat := range cats {
		code := strings.ToLower(cat.Code)
		key50 := fmt.Sprintf("days_to_50pct_cu.%s", code)
		key100 := fmt.Sprintf("days_to_100pct_cu.%s", code)
		keyBinder := fmt.Sprintf("days_to_100pct.%s", code)
		s[key50] = censored
		s[key100] = censored
		s[keyBinder] = censored
		for _, d := range days {
			prog, ok := d.SetProgress[cat.Code]
			if !ok {
				continue
			}
			if s[keyBinder] == censored && prog.Total > 0 && prog.Owned == prog.Total {
				s[keyBinder] = float64(d.Day)
			}
			if prog.CUTotal == 0 {
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

	// coins_after_full_binder: coins the player gains once nothing is left to collect.
	if i, ok := fullBinderDay(days, cats); ok {
		last := days[len(days)-1]
		s["coins_after_full_binder"] = float64(last.MoneySupply[domain.AccountPlayer] - days[i].MoneySupply[domain.AccountPlayer])
	} else {
		s["coins_after_full_binder"] = 0
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
		coins, err := domain.PriceToCoins(price, rate, sellPct)
		// Count as sub-coin when PriceToCoins errors (ErrSubCoin for non-zero prices that
		// round down) OR when coins == 0 with no error (zero price returns (0, nil) but the
		// Counter's tradeCoins refuses it identically, wrapping ErrSubCoin).
		if err != nil || coins == 0 {
			count++
		}
	}
	return count
}

// countUnpricedCards counts distinct non-basic-land cards across cats whose
// CardID is absent from prices (they use fallback_price instead).
func countUnpricedCards(cats []domain.SetCatalogue, prices domain.PriceSnapshot) int {
	seen := make(map[domain.CardID]struct{})
	count := 0
	for _, cat := range cats {
		for id, card := range cat.Cards {
			if card.IsBasicLand {
				continue
			}
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			if _, ok := prices[id]; !ok {
				count++
			}
		}
	}
	return count
}

// countUnsoldDuplicates returns the total number of card copies above a playset
// (domain.PlaysetSize) still held in the collection at end of run. This is
// non-zero when sub-coin cards cannot be sold to the Counter.
func countUnsoldDuplicates(collection *domain.Collection) int {
	if collection == nil {
		return 0
	}
	surplus, err := collection.DuplicatesAbove(domain.PlaysetSize)
	if err != nil {
		return 0 // invalid PlaysetSize is a code error; fail silently in metrics
	}
	total := 0
	for _, s := range surplus {
		total += s.Quantity
	}
	return total
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

// maxBoosterWaitMinutes returns the longest time an empty player waits to afford one booster of
// any set in costs. A player who must wait days for a booster stops playing, so the sweep makes
// this a number it can check.
func maxBoosterWaitMinutes(p domain.EnergyParams, costs map[string]int) (float64, error) {
	longest := 0
	for code, cost := range costs {
		ticks, err := p.TicksToAfford(cost)
		if err != nil {
			return 0, fmt.Errorf("booster wait for %q: %w", code, err)
		}
		longest = max(longest, ticks)
	}
	return float64(longest * domain.MinutesPerTick), nil
}

// fullBinderDay returns the index into days of the first day on which every set in cats has all
// of its cards owned, and false when that never happens.
func fullBinderDay(days []DayMetrics, cats []domain.SetCatalogue) (int, bool) {
	if len(cats) == 0 {
		return 0, false
	}
	for i, d := range days {
		complete := true
		for _, cat := range cats {
			prog, ok := d.SetProgress[cat.Code]
			if !ok || prog.Total == 0 || prog.Owned != prog.Total {
				complete = false
				break
			}
		}
		if complete {
			return i, true
		}
	}
	return 0, false
}
