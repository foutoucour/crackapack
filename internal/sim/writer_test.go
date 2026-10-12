package sim_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/domain"
	"github.com/foutoucour/crackapack/internal/sim"
)

func minimalResult() sim.Result {
	return sim.Result{
		Days: []sim.DayMetrics{
			{
				Day: 1,
				MoneySupply: map[domain.Account]domain.Coins{
					domain.AccountPlayer:          300,
					domain.AccountWelcomeBonus:    -200,
					domain.AccountDailyReward:     -100,
					domain.AccountCounterSale:     0,
					domain.AccountCounterPurchase: 0,
				},
				StockByRarity: map[domain.Rarity]int{
					domain.RarityCommon:   20,
					domain.RarityUncommon: 8,
					domain.RarityRare:     2,
					domain.RaritySpecial:  0,
				},
				CollectionUSD:   5000,
				CollectionCoins: 5,
				SetProgress: map[string]sim.SetDayProgress{
					"RAV": {Owned: 3, Total: 15, CUOwned: 2, CUTotal: 14, Playsets: 0},
				},
				BoostersOpened: 2,
				CardsSold:      1,
				CardsBought:    0,
				TradesRefused:  0,
			},
		},
		Summary: sim.Summary{
			"final_balance":         300,
			"total_refused_trades":  0,
			"days_to_50pct_cu.rav":  91,
			"days_to_100pct_cu.rav": 91,
			"sub_coin_cards":        0,
			"daily_reward_commons":  5,
		},
	}
}

func TestWriteCSV_columnOrder(t *testing.T) {
	t.Parallel()
	r := minimalResult()
	var buf bytes.Buffer
	if err := sim.WriteCSV(&buf, r); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines (header + data), got %d", len(lines))
	}
	header := lines[0]
	wantPrefix := "day,player,welcome_bonus,daily_reward,counter_sale,counter_purchase,stock_common,stock_uncommon,stock_rare,stock_special,collection_usd,collection_coins,boosters_opened,cards_sold,cards_bought,trades_refused"
	if !strings.HasPrefix(header, wantPrefix) {
		t.Errorf("CSV header does not start with expected columns.\ngot:  %q\nwant prefix: %q", header, wantPrefix)
	}
}

func TestWriteJSON_deterministicByteIdentical(t *testing.T) {
	t.Parallel()
	r := minimalResult()

	var buf1, buf2 bytes.Buffer
	if err := sim.WriteJSON(&buf1, r); err != nil {
		t.Fatalf("WriteJSON 1: %v", err)
	}
	if err := sim.WriteJSON(&buf2, r); err != nil {
		t.Fatalf("WriteJSON 2: %v", err)
	}

	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Error("WriteJSON: two calls with identical input produced different output")
	}
}

func TestWriteJSON_coinRateChangesOutput(t *testing.T) {
	t.Parallel()
	r1 := minimalResult()
	r2 := minimalResult()
	r2.Days[0].CollectionCoins = 10 // simulates a different coin rate result

	var buf1, buf2 bytes.Buffer
	if err := sim.WriteJSON(&buf1, r1); err != nil {
		t.Fatalf("WriteJSON 1: %v", err)
	}
	if err := sim.WriteJSON(&buf2, r2); err != nil {
		t.Fatalf("WriteJSON 2: %v", err)
	}

	if bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Error("WriteJSON: different CollectionCoins produced identical output")
	}
}

func TestWriteSummary_sortedAndFormatted(t *testing.T) {
	t.Parallel()
	s := sim.Summary{
		"zzz_metric": 42,
		"aaa_metric": 3.5,
		"mmm_metric": 0,
	}
	var buf bytes.Buffer
	if err := sim.WriteSummary(&buf, s); err != nil {
		t.Fatalf("WriteSummary: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d:\n%s", len(lines), buf.String())
	}
	// Lines must be sorted by name.
	if !strings.HasPrefix(lines[0], "aaa_metric ") {
		t.Errorf("line 0 = %q, want aaa_metric first", lines[0])
	}
	if !strings.HasPrefix(lines[1], "mmm_metric ") {
		t.Errorf("line 1 = %q, want mmm_metric second", lines[1])
	}
	if !strings.HasPrefix(lines[2], "zzz_metric ") {
		t.Errorf("line 2 = %q, want zzz_metric last", lines[2])
	}
	// Whole-number float should format without decimal point.
	if lines[2] != "zzz_metric 42" {
		t.Errorf("zzz_metric line = %q, want %q", lines[2], "zzz_metric 42")
	}
	// Non-integer should include decimal.
	if lines[0] != "aaa_metric 3.5" {
		t.Errorf("aaa_metric line = %q, want %q", lines[0], "aaa_metric 3.5")
	}
}
