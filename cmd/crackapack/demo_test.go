package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCmdDemo_basicOutput verifies that crackapack demo with a single rate
// prints the heading and the sweep table to stdout.
func TestCmdDemo_basicOutput(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	err := run([]string{"demo", "--rates", "100"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "crackapack demo:") {
		t.Errorf("heading not found in stdout; got:\n%s", out)
	}
	if !strings.Contains(out, "100") {
		t.Errorf("rate 100 not found in sweep table; got:\n%s", out)
	}
}

// TestCmdDemo_outFlag verifies that --out writes per-rate files and sweep.json.
func TestCmdDemo_outFlag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := run([]string{"demo", "--out", dir, "--rates", "100"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "rate-100", "result.json")); err != nil {
		t.Errorf("rate-100/result.json not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "rate-100", "days.csv")); err != nil {
		t.Errorf("rate-100/days.csv not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sweep.json")); err != nil {
		t.Errorf("sweep.json not written: %v", err)
	}
}

// TestCmdDemo_ratesOverride verifies that --rates overrides the demo grid.
func TestCmdDemo_ratesOverride(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	err := run([]string{"demo", "--rates", "100,200"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "100") {
		t.Errorf("rate 100 missing from output; got:\n%s", out)
	}
	if !strings.Contains(out, "200") {
		t.Errorf("rate 200 missing from output; got:\n%s", out)
	}
}

// TestCmdDemo_invalidRates verifies that an invalid --rates value returns an error.
func TestCmdDemo_invalidRates(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	err := run([]string{"demo", "--rates", "abc"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for invalid rates, got nil")
	}
	if !strings.Contains(err.Error(), "crackapack demo:") {
		t.Errorf("error prefix missing; got: %v", err)
	}
}
