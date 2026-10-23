package sim_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/foutoucour/crackapack/internal/sim"
)

// TestReadResult_roundTrip writes a result via WriteJSON and reads it back with
// ReadResult. The re-encoded bytes must be byte-identical to the original.
func TestReadResult_roundTrip(t *testing.T) {
	t.Parallel()

	cfg := smallConfig()
	e := buildEngine(t, cfg, sim.NewCollectorPolicy())
	r, err := e.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var buf bytes.Buffer
	if err := sim.WriteJSON(&buf, r); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	originalJSON := buf.Bytes()

	got, err := sim.ReadResult(bytes.NewReader(originalJSON))
	if err != nil {
		t.Fatalf("ReadResult: %v", err)
	}

	// Re-encode the round-tripped result and compare bytes.
	var buf2 bytes.Buffer
	if err := sim.WriteJSON(&buf2, got); err != nil {
		t.Fatalf("WriteJSON round-trip: %v", err)
	}

	if !bytes.Equal(originalJSON, buf2.Bytes()) {
		t.Errorf("round-trip: JSON differs\noriginal:\n%s\nround-tripped:\n%s",
			originalJSON, buf2.Bytes())
	}
}

// TestReadResult_unknownField verifies that ReadResult rejects JSON with fields
// that are not part of the Result schema.
func TestReadResult_unknownField(t *testing.T) {
	t.Parallel()
	data := []byte(`{"Days":[],"Summary":{},"UnknownField":true}`)
	_, err := sim.ReadResult(bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

// TestReadResult_malformed verifies that ReadResult returns an error for invalid JSON.
func TestReadResult_malformed(t *testing.T) {
	t.Parallel()
	_, err := sim.ReadResult(bytes.NewReader([]byte(`{invalid`)))
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

// TestCompare_sameResult_zeroDeltas verifies that Compare(r, r) produces
// all-zero Summary deltas and all-zero per-day column deltas.
func TestCompare_sameResult_zeroDeltas(t *testing.T) {
	t.Parallel()

	cfg := smallConfig()
	e := buildEngine(t, cfg, sim.NewCollectorPolicy())
	r, err := e.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	cmp, err := sim.Compare(r, r)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	for k, diff := range cmp.Summary {
		if diff.Delta != 0 {
			t.Errorf("Summary[%q].Delta = %g, want 0", k, diff.Delta)
		}
	}

	for _, day := range cmp.Days {
		for _, cd := range day.Deltas {
			if cd.Delta != 0 {
				t.Errorf("day %d col %q delta = %d, want 0", day.Day, cd.Name, cd.Delta)
			}
		}
	}
}

// TestCompare_differentCoinRate_nonZeroDeltas verifies that comparing two runs
// with different coin rates produces non-zero deltas on expected metrics.
//
// With CoinRate=100 vs CoinRate=200:
//   - final_balance: the rate scales prices → delta must be non-zero.
//   - per-day collection_coins scales with CoinRate → at least one day differs.
func TestCompare_differentCoinRate_nonZeroDeltas(t *testing.T) {
	t.Parallel()

	cfg1 := smallConfig()
	cfg1.CoinRate = 100
	e1 := buildEngine(t, cfg1, sim.NewCollectorPolicy())
	r1, err := e1.Run()
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}

	cfg2 := smallConfig()
	cfg2.CoinRate = 200
	e2 := buildEngine(t, cfg2, sim.NewCollectorPolicy())
	r2, err := e2.Run()
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}

	cmp, err := sim.Compare(r1, r2)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	// The rate scales every price, so the player's final balance differs.
	diff, ok := cmp.Summary["final_balance"]
	if !ok {
		t.Fatal("final_balance not in comparison")
	}
	if diff.Delta == 0 {
		t.Errorf("final_balance delta = 0; expected non-zero (a=%g, b=%g)", diff.A, diff.B)
	}

	// At least one per-day column delta must also be non-zero.
	anyNonZero := false
	for _, day := range cmp.Days {
		for _, cd := range day.Deltas {
			if cd.Delta != 0 {
				anyNonZero = true
				break
			}
		}
		if anyNonZero {
			break
		}
	}
	if !anyNonZero {
		t.Error("all per-day column deltas are zero; expected at least one non-zero delta")
	}
}

// TestCompare_mismatchedDayCount verifies that Compare returns an error naming
// the day counts when the results have different numbers of days.
func TestCompare_mismatchedDayCount(t *testing.T) {
	t.Parallel()

	ra := sim.Result{Days: []sim.DayMetrics{{Day: 1}}}
	rb := sim.Result{Days: []sim.DayMetrics{{Day: 1}, {Day: 2}}}

	_, err := sim.Compare(ra, rb)
	if err == nil {
		t.Fatal("expected error for mismatched day count, got nil")
	}
	if !strings.Contains(err.Error(), "day counts") {
		t.Errorf("error %q does not mention 'day counts'", err.Error())
	}
}

// TestCompare_mismatchedSetCodes verifies that Compare returns an error naming
// the set codes when the results have different set codes.
func TestCompare_mismatchedSetCodes(t *testing.T) {
	t.Parallel()

	ra := sim.Result{
		Days: []sim.DayMetrics{
			{Day: 1, SetProgress: map[string]sim.SetDayProgress{"RAV": {}}},
		},
	}
	rb := sim.Result{
		Days: []sim.DayMetrics{
			{Day: 1, SetProgress: map[string]sim.SetDayProgress{"CLS": {}}},
		},
	}

	_, err := sim.Compare(ra, rb)
	if err == nil {
		t.Fatal("expected error for mismatched set codes, got nil")
	}
	if !strings.Contains(err.Error(), "set codes") {
		t.Errorf("error %q does not mention 'set codes'", err.Error())
	}
}

// TestCompare_dayNumberMismatch verifies that Compare returns an error naming
// the day number and index when per-day Day numbers do not match.
func TestCompare_dayNumberMismatch(t *testing.T) {
	t.Parallel()

	ra := sim.Result{Days: []sim.DayMetrics{{Day: 1}}}
	rb := sim.Result{Days: []sim.DayMetrics{{Day: 2}}} // same count, different Day number

	_, err := sim.Compare(ra, rb)
	if err == nil {
		t.Fatal("expected error for day number mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "day number") {
		t.Errorf("error %q does not mention 'day number'", err.Error())
	}
}

// TestCompare_metricPresentInOneOnly verifies MetricDiff presence flags and that
// the Delta field is 0 when either side is absent.
func TestCompare_metricPresentInOneOnly(t *testing.T) {
	t.Parallel()

	ra := sim.Result{
		Days:    []sim.DayMetrics{{Day: 1}},
		Summary: sim.Summary{"only_a": 5.0, "shared": 10.0},
	}
	rb := sim.Result{
		Days:    []sim.DayMetrics{{Day: 1}},
		Summary: sim.Summary{"only_b": 7.0, "shared": 15.0},
	}

	cmp, err := sim.Compare(ra, rb)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	// only_a: present in A, absent in B, Delta must be 0.
	onlyA, ok := cmp.Summary["only_a"]
	if !ok {
		t.Fatal("only_a not in comparison summary")
	}
	if !onlyA.APresent {
		t.Error("only_a: APresent should be true")
	}
	if onlyA.BPresent {
		t.Error("only_a: BPresent should be false")
	}
	if onlyA.A != 5.0 {
		t.Errorf("only_a.A = %g, want 5", onlyA.A)
	}
	if onlyA.Delta != 0 {
		t.Errorf("only_a.Delta = %g, want 0 (absent side)", onlyA.Delta)
	}

	// only_b: absent in A, present in B, Delta must be 0.
	onlyB, ok := cmp.Summary["only_b"]
	if !ok {
		t.Fatal("only_b not in comparison summary")
	}
	if onlyB.APresent {
		t.Error("only_b: APresent should be false")
	}
	if !onlyB.BPresent {
		t.Error("only_b: BPresent should be true")
	}
	if onlyB.B != 7.0 {
		t.Errorf("only_b.B = %g, want 7", onlyB.B)
	}
	if onlyB.Delta != 0 {
		t.Errorf("only_b.Delta = %g, want 0 (absent side)", onlyB.Delta)
	}

	// shared: present in both, Delta = 15 - 10 = 5.
	shared, ok := cmp.Summary["shared"]
	if !ok {
		t.Fatal("shared not in comparison summary")
	}
	if !shared.APresent || !shared.BPresent {
		t.Errorf("shared: APresent=%v BPresent=%v, both should be true", shared.APresent, shared.BPresent)
	}
	if shared.Delta != 5.0 {
		t.Errorf("shared.Delta = %g, want 5.0", shared.Delta)
	}
}
