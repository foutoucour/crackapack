package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdDemo_showsTradeMetrics(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer

	if err := run([]string{"demo", "--out", dir, "--rates", "100"}, &stdout, &stderr); err != nil {
		t.Fatalf("demo: %v\nstderr: %s", err, stderr.String())
	}

	csv, err := os.ReadFile(filepath.Join(dir, "rate-100", "days.csv"))
	if err != nil {
		t.Fatalf("read days.csv: %v", err)
	}
	header, _, _ := strings.Cut(string(csv), "\n")
	if !strings.Contains(header, "trade_accepted") {
		t.Errorf("days.csv header lacks the trade columns: the demo should run with trades on: %s", header)
	}
	result, err := os.ReadFile(filepath.Join(dir, "rate-100", "result.json"))
	if err != nil {
		t.Fatalf("read result.json: %v", err)
	}
	if !strings.Contains(string(result), "trade_accepted") {
		t.Error("result.json summary lacks trade_accepted")
	}
}
