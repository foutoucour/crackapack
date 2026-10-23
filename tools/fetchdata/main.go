// Command fetchdata downloads real simulation inputs from MTGJSON on demand: one
// catalogue file per set, and the prices of the cards of those sets, merged into a price snapshot
// that keeps the prices of the sets fetched by earlier runs.
//
// The output is what `crackapack simulate` and `crackapack sweep` read through
// --catalogue-dir and --prices.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	mtgjsonBaseURL = "https://mtgjson.com/api/v5"
	pricesFile     = "AllPricesToday.json.gz"

	// downloadTimeout covers the whole transfer; the gzipped price file is the largest at a few MB.
	downloadTimeout = 5 * time.Minute

	maxSetCodeLen = 8
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	a := app{
		client:  &http.Client{Timeout: downloadTimeout},
		baseURL: mtgjsonBaseURL,
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		pools:   borrowedPools(),
	}
	if err := a.run(ctx, os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// app holds the collaborators of one fetchdata run, so tests can point it at a local server.
type app struct {
	client  *http.Client
	baseURL string
	stdout  io.Writer
	stderr  io.Writer
	pools   []borrowedPool
}

// run parses args, downloads the requested sets and merges their prices into the price snapshot.
func (a app) run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("fetchdata", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	flags.Usage = func() {
		fmt.Fprintln(a.stderr, "Usage: go run ./tools/fetchdata --sets RAV,TSP [--catalogue-dir ./catalogues] [--prices ./prices.json]")
		flags.PrintDefaults()
	}
	sets := flags.String("sets", "", "comma-separated MTGJSON set codes to download, e.g. RAV,TSP,TSB (required)")
	catalogueDir := flags.String("catalogue-dir", "catalogues", "directory that receives one <CODE>.json per set")
	prices := flags.String("prices", "prices.json", "price snapshot file to merge the prices into")
	if err := flags.Parse(args); err != nil {
		return err
	}

	codes, err := parseSetCodes(*sets)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*catalogueDir, dirPerm); err != nil {
		return fmt.Errorf("create catalogue directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(*prices), dirPerm); err != nil {
		return fmt.Errorf("create prices directory: %w", err)
	}

	existing, err := readSnapshot(*prices)
	if err != nil {
		return err
	}

	wanted := make(map[string]struct{})
	for _, code := range withDonors(codes, a.pools) {
		path := filepath.Join(*catalogueDir, code+".json")
		if err := a.download(ctx, code+".json", path); err != nil {
			return fmt.Errorf("set %s: %w", code, err)
		}
		uuids, err := cardUUIDs(path)
		if err != nil {
			return fmt.Errorf("set %s: %w", code, err)
		}
		for _, id := range uuids {
			wanted[id] = struct{}{}
		}
		fmt.Fprintf(a.stdout, "%s: %d cards -> %s\n", code, len(uuids), path)
	}

	for _, pool := range a.pools {
		if !slices.Contains(codes, pool.host) {
			continue
		}
		count, err := writeBorrowedPool(*catalogueDir, pool)
		if err != nil {
			return err
		}
		fmt.Fprintf(a.stdout, "%s: %d cards borrowed by %s -> %s\n", pool.code, count, pool.host,
			filepath.Join(*catalogueDir, pool.code+".json"))
	}

	priced, total, err := a.downloadPrices(ctx, wanted, existing, *prices)
	if err != nil {
		return fmt.Errorf("prices: %w", err)
	}
	fmt.Fprintf(a.stdout, "prices: %d of %d cards priced, %d in the snapshot -> %s\n", priced, len(wanted), total, *prices)
	return nil
}

// parseSetCodes splits and normalises the --sets value. Codes become file names and URL paths,
// so anything but short alphanumerics is rejected.
func parseSetCodes(value string) ([]string, error) {
	if value == "" {
		return nil, errors.New("--sets is required, e.g. --sets RAV,TSP")
	}
	parts := strings.Split(value, ",")
	codes := make([]string, 0, len(parts))
	for _, part := range parts {
		code := strings.ToUpper(strings.TrimSpace(part))
		if !isSetCode(code) {
			return nil, fmt.Errorf("invalid set code %q: want 1-%d letters or digits", part, maxSetCodeLen)
		}
		codes = append(codes, code)
	}
	return codes, nil
}

func isSetCode(code string) bool {
	if code == "" || len(code) > maxSetCodeLen {
		return false
	}
	for _, r := range code {
		isLetter := r >= 'A' && r <= 'Z'
		isDigit := r >= '0' && r <= '9'
		if !isLetter && !isDigit {
			return false
		}
	}
	return true
}
