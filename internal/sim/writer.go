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

	// Determine set codes from the first day's metrics, sorted ascending.
	setCodes := extractSetCodes(r.Days)
	accounts := csvAccounts()
	rarities := csvRarities()

	// Build header.
	header := buildCSVHeader(setCodes, accounts, rarities)
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	for _, d := range r.Days {
		row := buildCSVRow(d, setCodes, accounts, rarities)
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

// buildCSVHeader constructs the header row.
func buildCSVHeader(setCodes []string, accounts []domain.Account, rarities []domain.Rarity) []string {
	var h []string
	h = append(h, "day")
	for _, a := range accounts {
		h = append(h, string(a))
	}
	for _, r := range rarities {
		h = append(h, "stock_"+string(r))
	}
	h = append(h, "collection_usd", "collection_coins")
	h = append(h, "boosters_opened", "cards_sold", "cards_bought", "trades_refused")
	for _, code := range setCodes {
		lc := strings.ToLower(code)
		h = append(h,
			lc+"_owned",
			lc+"_total",
			lc+"_cu_owned",
			lc+"_cu_total",
			lc+"_playsets",
		)
	}
	return h
}

// buildCSVRow constructs one data row for a DayMetrics.
func buildCSVRow(d DayMetrics, setCodes []string, accounts []domain.Account, rarities []domain.Rarity) []string {
	var row []string
	row = append(row, strconv.Itoa(d.Day))
	for _, a := range accounts {
		row = append(row, strconv.FormatInt(int64(d.MoneySupply[a]), 10))
	}
	for _, r := range rarities {
		row = append(row, strconv.Itoa(d.StockByRarity[r]))
	}
	row = append(row, strconv.FormatInt(int64(d.CollectionUSD), 10))
	row = append(row, strconv.FormatInt(int64(d.CollectionCoins), 10))
	row = append(row, strconv.Itoa(d.BoostersOpened))
	row = append(row, strconv.Itoa(d.CardsSold))
	row = append(row, strconv.Itoa(d.CardsBought))
	row = append(row, strconv.Itoa(d.TradesRefused))
	for _, code := range setCodes {
		sp := d.SetProgress[code]
		row = append(row,
			strconv.Itoa(sp.Owned),
			strconv.Itoa(sp.Total),
			strconv.Itoa(sp.CUOwned),
			strconv.Itoa(sp.CUTotal),
			strconv.Itoa(sp.Playsets),
		)
	}
	return row
}
