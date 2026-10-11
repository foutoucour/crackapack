package sim

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/foutoucour/crackapack/internal/domain"
)

// csvAccounts returns the fixed order of ledger accounts in CSV output.
// Returned as a function (not a package-level var) to avoid the global-variable lint rule.
func csvAccounts() []domain.Account {
	return []domain.Account{
		domain.AccountPlayer,
		domain.AccountWelcomeBonus,
		domain.AccountDailyReward,
		domain.AccountCounterSale,
		domain.AccountCounterPurchase,
	}
}

// csvRarities returns the fixed order of rarities in CSV output.
func csvRarities() []domain.Rarity {
	return []domain.Rarity{
		domain.RarityCommon,
		domain.RarityUncommon,
		domain.RarityRare,
		domain.RaritySpecial,
	}
}

// column is a named numeric metric for one simulated day.
// It is shared between WriteCSV and the comparison writer (WriteComparisonCSV)
// so both always emit the same column set in the same order.
type column struct {
	name  string
	value int64
}

// dayColumns returns all per-day metrics in the stable CSV column order.
// The returned slice does not include the "day" column (always first, emitted separately).
// setCodes must be sorted ascending.
//
// Accessing nil maps in DayMetrics (MoneySupply, StockByRarity, SetProgress)
// returns zero values; callers may safely pass a zero-valued DayMetrics to
// derive column names without needing real data.
func dayColumns(d DayMetrics, setCodes []string) []column {
	accounts := csvAccounts()
	rarities := csvRarities()

	cols := make([]column, 0, len(accounts)+len(rarities)+6+5*len(setCodes))

	for _, a := range accounts {
		cols = append(cols, column{name: string(a), value: int64(d.MoneySupply[a])})
	}
	for _, r := range rarities {
		cols = append(cols, column{name: "stock_" + string(r), value: int64(d.StockByRarity[r])})
	}
	cols = append(cols,
		column{name: "collection_usd", value: int64(d.CollectionUSD)},
		column{name: "collection_coins", value: int64(d.CollectionCoins)},
		column{name: "boosters_opened", value: int64(d.BoostersOpened)},
		column{name: "cards_sold", value: int64(d.CardsSold)},
		column{name: "cards_bought", value: int64(d.CardsBought)},
		column{name: "trades_refused", value: int64(d.TradesRefused)},
	)
	for _, code := range setCodes {
		lc := strings.ToLower(code)
		sp := d.SetProgress[code]
		cols = append(cols,
			column{name: lc + "_owned", value: int64(sp.Owned)},
			column{name: lc + "_total", value: int64(sp.Total)},
			column{name: lc + "_cu_owned", value: int64(sp.CUOwned)},
			column{name: lc + "_cu_total", value: int64(sp.CUTotal)},
			column{name: lc + "_playsets", value: int64(sp.Playsets)},
		)
	}
	return cols
}

// WriteJSON encodes r as JSON and writes it to w.
// encoding/json sorts string-kinded map keys (Account, Rarity, string) alphabetically,
// ensuring byte-identical output for equal inputs.
func WriteJSON(w io.Writer, r Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	return nil
}

// WriteCSV encodes r.Days as CSV with a stable column order and writes it to w.
//
// Column order:
//  1. day
//  2. money supply per account (fixed order: player, welcome_bonus, daily_reward,
//     counter_sale, counter_purchase)
//  3. counter stock per rarity (fixed order: common, uncommon, rare, special)
//  4. collection_usd, collection_coins
//  5. boosters_opened, cards_sold, cards_bought, trades_refused
//  6. per-set columns sorted by set code:
//     <code>_owned, <code>_total, <code>_cu_owned, <code>_cu_total, <code>_playsets
//
// Set codes are determined from the first DayMetrics entry if present.
func WriteCSV(w io.Writer, r Result) error {
	cw := csv.NewWriter(w)
	setCodes := extractSetCodes(r.Days)

	// Build header from stable column names via dayColumns.
	headerCols := dayColumns(DayMetrics{}, setCodes)
	header := make([]string, 0, 1+len(headerCols))
	header = append(header, "day")
	for _, c := range headerCols {
		header = append(header, c.name)
	}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	for _, d := range r.Days {
		cols := dayColumns(d, setCodes)
		row := make([]string, 0, 1+len(cols))
		row = append(row, strconv.Itoa(d.Day))
		for _, c := range cols {
			row = append(row, strconv.FormatInt(c.value, 10))
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("write csv row day %d: %w", d.Day, err)
		}
	}

	cw.Flush()
	return cw.Error()
}

// WriteSummary writes one "name value" line per metric in s, sorted by name, to w.
// Values are formatted as decimal integers when they are whole numbers, otherwise
// as floating-point with enough precision to round-trip.
func WriteSummary(w io.Writer, s Summary) error {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		v := s[k]
		var formatted string
		if v == float64(int64(v)) {
			formatted = strconv.FormatInt(int64(v), 10)
		} else {
			formatted = strconv.FormatFloat(v, 'f', -1, 64)
		}
		sb.WriteString(k)
		sb.WriteByte(' ')
		sb.WriteString(formatted)
		sb.WriteByte('\n')
	}
	_, err := io.WriteString(w, sb.String())
	if err != nil {
		return fmt.Errorf("write summary: %w", err)
	}
	return nil
}

// extractSetCodes returns a sorted slice of set codes from the first day entry.
func extractSetCodes(days []DayMetrics) []string {
	if len(days) == 0 {
		return nil
	}
	codes := make([]string, 0, len(days[0].SetProgress))
	for code := range days[0].SetProgress {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}
