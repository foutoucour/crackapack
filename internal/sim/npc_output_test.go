package sim_test

import (
	"encoding/csv"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/sim"
)

func resultWithNPCs(sold ...int) sim.Result {
	days := make([]sim.DayMetrics, len(sold))
	for i, s := range sold {
		days[i] = sim.DayMetrics{Day: i + 1, NPC: &sim.NPCDayMetrics{BoostersOpened: 10 * (i + 1), CardsSold: s, CardsHeld: 100 * (i + 1)}}
	}
	return sim.Result{Days: days}
}

func TestWriteCSV_appendsNPCColumnsWhenACrowdRan(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	if err := sim.WriteCSV(&buf, resultWithNPCs(7, 9)); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}

	rows, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	wantHeader := []string{"npc_boosters_opened", "npc_cards_sold", "npc_cards_held"}
	header := rows[0]
	if got := header[len(header)-3:]; !slices.Equal(got, wantHeader) {
		t.Fatalf("last header columns = %v, want %v", got, wantHeader)
	}
	wantRow := []string{"20", "9", "200"}
	last := rows[2]
	if got := last[len(last)-3:]; !slices.Equal(got, wantRow) {
		t.Errorf("day 2 NPC values = %v, want %v", got, wantRow)
	}
}

func TestCompare_rejectsAnNPCRunAgainstARunWithout(t *testing.T) {
	t.Parallel()

	without := sim.Result{Days: []sim.DayMetrics{{Day: 1}}}

	for _, pair := range [][2]sim.Result{{without, resultWithNPCs(1)}, {resultWithNPCs(1), without}} {
		if _, err := sim.Compare(pair[0], pair[1]); err == nil || !strings.Contains(err.Error(), "NPC") {
			t.Errorf("Compare error = %v, want one that mentions the NPC mismatch", err)
		}
	}
}

func TestCompare_reportsNPCColumnDeltas(t *testing.T) {
	t.Parallel()

	cmp, err := sim.Compare(resultWithNPCs(7), resultWithNPCs(12))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	got := map[string]int64{}
	for _, d := range cmp.Days[0].Deltas {
		got[d.Name] = d.Delta
	}
	if delta, ok := got["npc_cards_sold"]; !ok || delta != 5 {
		t.Errorf("npc_cards_sold delta = %d (present %t), want 5; deltas = %s", delta, ok, strconv.Quote(strings.Join(deltaNames(cmp.Days[0].Deltas), ",")))
	}
}

func deltaNames(deltas []sim.ColDelta) []string {
	names := make([]string, len(deltas))
	for i, d := range deltas {
		names[i] = d.Name
	}
	return names
}
