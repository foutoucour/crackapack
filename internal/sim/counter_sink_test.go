package sim_test

import (
	"testing"

	"github.com/foutoucour/crackapack/internal/sim"
)

func TestRun_CounterSinkDiscardsWhatNPCsSell(t *testing.T) {
	t.Parallel()
	cfg := npcConfig(4, 20, 1)
	cfg.CounterShelfDays = 0

	result := runResult(t, cfg, sim.NewIdlePolicy())

	var sold int
	for _, d := range result.Days {
		sold += d.NPC.CardsSold
		for rarity, qty := range d.StockByRarity {
			if qty != 0 {
				t.Errorf("day %d: Counter stock of %s = %d, want 0: NPC sales vanish", d.Day, rarity, qty)
			}
		}
	}
	if sold == 0 {
		t.Fatal("NPCs sold nothing, so the test proves nothing")
	}
}

func TestRun_CounterSinkDiscardsWhatThePlayerSells(t *testing.T) {
	t.Parallel()
	cfg := smallConfig()
	cfg.InitialStock = nil
	cfg.CounterShelfDays = 0

	result := runResult(t, cfg, sim.NewCashOutPolicy())

	var sold int
	for _, d := range result.Days {
		sold += d.CardsSold
		for rarity, qty := range d.StockByRarity {
			if qty != 0 {
				t.Errorf("day %d: Counter stock of %s = %d, want 0: the player's sales vanish", d.Day, rarity, qty)
			}
		}
	}
	if sold == 0 {
		t.Fatal("the player sold nothing, so the test proves nothing")
	}
}

func TestRun_CounterShelfLifeBoundsTheStockToRecentNPCSales(t *testing.T) {
	t.Parallel()
	cfg := npcConfig(4, 20, 1)
	cfg.RunDays = 8
	cfg.CounterShelfDays = 1

	result := runResult(t, cfg, sim.NewIdlePolicy())

	var cumulative, previous int
	for _, d := range result.Days {
		var stock int
		for _, qty := range d.StockByRarity {
			stock += qty
		}
		cumulative += d.NPC.CardsSold
		if limit := d.NPC.CardsSold + previous; stock > limit {
			t.Errorf("day %d: stock = %d, want at most the %d cards sold today and yesterday", d.Day, stock, limit)
		}
		if d.Day >= 3 && stock >= cumulative {
			t.Errorf("day %d: stock = %d, want less than the %d cards sold so far: old ones vanish", d.Day, stock, cumulative)
		}
		previous = d.NPC.CardsSold
	}
	if cumulative == 0 {
		t.Fatal("NPCs sold nothing, so the test proves nothing")
	}
}
